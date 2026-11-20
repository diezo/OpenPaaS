package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/diezo/openpaas/internal/config"
	"github.com/diezo/openpaas/internal/docker"
	"github.com/diezo/openpaas/internal/github"
	"github.com/diezo/openpaas/internal/nginx"
	"github.com/diezo/openpaas/internal/prompt"
	"github.com/diezo/openpaas/internal/services"
	"github.com/diezo/openpaas/internal/system"
	"github.com/diezo/openpaas/internal/tls"
	"github.com/diezo/openpaas/internal/ui"
)

func newCreateCmd() *cobra.Command {
	var imageFlag string
	var portFlag int
	cmd := &cobra.Command{
		Use:   "create <slug>",
		Short: "Deploy a Docker image at <slug>.<your-domain>",
		Long: "Pulls a Docker image, runs it, and serves it at https://<slug>.<your-domain>.\n" +
			"Prompts for the image and the container's port unless --image/--port are given.",
		Example:     "  sudo op create blog\n  sudo op create blog --image ghcr.io/me/blog:latest --port 3000",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annNeedsRoot: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runCreate(strings.ToLower(strings.TrimSpace(args[0])), imageFlag, portFlag)
			if err == prompt.ErrAborted {
				fmt.Println()
				ui.Warn("Cancelled.")
				return errSilent
			}
			return err
		},
	}
	cmd.Flags().StringVar(&imageFlag, "image", "", "image to deploy, e.g. ghcr.io/me/app:latest")
	cmd.Flags().IntVar(&portFlag, "port", 0, "port the app listens on inside the container")
	return cmd
}

func runCreate(slug, imageFlag string, portFlag int) error {
	closeLog, _ := system.OpenLog("services.log")
	defer closeLog()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := services.Load()
	if err != nil {
		return err
	}

	// Slug checks.
	if err := services.ValidateSlug(slug); err != nil {
		return err
	}
	if _, exists := store.Services[slug]; exists {
		return fmt.Errorf("a service named %q already exists (see: sudo op list)", slug)
	}
	container := services.ContainerName(slug)
	if docker.Exists(container) {
		return fmt.Errorf("a container named %q already exists; remove it with: sudo docker rm -f %s", container, container)
	}
	host := slug + "." + cfg.Domain

	fmt.Println()
	ui.Box(ui.Bold("New service: "+slug), ui.Dim("Will be served at https://"+host))
	fmt.Println()

	// Image.
	var img docker.Image
	interactive := imageFlag == ""
	if interactive {
		fmt.Println(ui.Dim("  Tip: use the :latest tag so future auto-redeploys pick up new pushes."))
	}
	for {
		in := imageFlag
		if interactive {
			if in, err = prompt.Ask("  Docker image (e.g. ghcr.io/you/app:latest)", "", ""); err != nil {
				return err
			}
		}
		img, err = docker.ParseImage(in)
		if err == nil {
			break
		}
		if !interactive {
			return err
		}
		fmt.Printf("  %s\n", ui.Red(err.Error()))
	}
	if img.Tag != "latest" {
		ui.Warn("Tag is %q, not \"latest\": future auto-redeploys will keep pulling this exact tag.", img.Tag)
		if interactive {
			c, err := prompt.Choice("  Continue with this tag? [y]es/[n]o", []string{"y", "n"}, "y")
			if err != nil {
				return err
			}
			if c == "n" {
				return prompt.ErrAborted
			}
		}
	}

	// Pull (this is the real validation: exists + we have access).
	if img.IsGHCR() {
		if err := ui.Step("Signing in to ghcr.io", func() error {
			login, err := github.ValidateToken(cfg.GitHubToken)
			if err != nil {
				return fmt.Errorf("GitHub token problem: %w (update it with: sudo op setup)", err)
			}
			return docker.Login("ghcr.io", login, cfg.GitHubToken)
		}); err != nil {
			return err
		}
	}
	if err := ui.Step("Pulling "+img.Ref(), func() error { return docker.Pull(img.Ref()) }); err != nil {
		return err
	}
	exposed, _ := docker.ExposedPorts(img.Ref())
	slices.Sort(exposed)

	// Internal port (defaults to the image's EXPOSE if it declares exactly one).
	port := portFlag
	if port == 0 {
		def := ""
		if len(exposed) == 1 {
			def = strconv.Itoa(exposed[0])
		}
		for {
			in, err := prompt.Ask("  Port your app listens on inside the container", def, "")
			if err != nil {
				return err
			}
			if p, err := strconv.Atoi(in); err == nil && p >= 1 && p <= 65535 {
				port = p
				break
			}
			fmt.Printf("  %s\n", ui.Red("enter a port number between 1 and 65535"))
		}
	} else if port < 1 || port > 65535 {
		return fmt.Errorf("--port must be between 1 and 65535")
	}
	if len(exposed) > 0 && !slices.Contains(exposed, port) {
		ui.Warn("The image declares port(s) %v, not %d. Make sure your app really listens on %d.", exposed, port, port)
	}

	hostPort, err := store.FreePort()
	if err != nil {
		return err
	}

	// Deploy, undoing earlier steps if a later one fails.
	var undo []func()
	rollback := func(err error) error {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		ui.Info("Nothing was left behind. Details: %s", system.LogDir+"/services.log")
		return err
	}

	if err := ui.Step(fmt.Sprintf("Starting container (port %d → %d)", hostPort, port), func() error {
		if err := docker.Run(container, img.Ref(), hostPort, port, slug); err != nil {
			return err
		}
		undo = append(undo, func() { _ = docker.Remove(container) })
		return docker.WaitRunning(container, 20*time.Second)
	}); err != nil {
		return rollback(err)
	}
	if err := ui.Step("Routing "+host+" through nginx", func() error {
		return nginx.ConfigureService(slug, cfg.Domain, hostPort)
	}); err != nil {
		return rollback(err)
	}
	undo = append(undo, func() { _ = nginx.RemoveService(slug) })

	svc := &services.Service{
		Slug: slug, Image: img.Ref(), InternalPort: port, HostPort: hostPort,
		Container: container, CreatedAt: time.Now().UTC(),
	}
	store.Services[slug] = svc
	if err := store.Save(); err != nil {
		return rollback(fmt.Errorf("save services: %w", err))
	}

	// TLS failure isn't fatal: the service already works over http.
	tlsErr := ui.Step("Getting TLS certificate for "+host, func() error { return tls.Issue(cfg.Email, host) })
	if tlsErr == nil {
		svc.TLS = true
		_ = store.Save()
	}

	scheme := "https://"
	if !svc.TLS {
		scheme = "http://"
	}
	fmt.Println()
	ui.Box(
		ui.Green("✓ "+slug+" is live"),
		"",
		"URL     "+ui.Cyan(scheme+host),
		"Image   "+img.Ref(),
		fmt.Sprintf("Ports   127.0.0.1:%d → container:%d", hostPort, port),
		"",
		ui.Dim("See all services: sudo op list"),
	)
	if tlsErr != nil {
		fmt.Println()
		ui.Warn("HTTPS isn't set up yet, so the service is on http only:")
		fmt.Println(ui.Dim("  " + strings.ReplaceAll(tlsErr.Error(), "\n", "\n  ")))
		ui.Info("Once DNS for %s points here, run: %s", host, ui.Bold("sudo certbot --nginx -d "+host))
	}
	fmt.Println()
	return nil
}
