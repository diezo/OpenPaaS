package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/diezo/openpaas/internal/config"
	"github.com/diezo/openpaas/internal/docker"
	"github.com/diezo/openpaas/internal/nginx"
	"github.com/diezo/openpaas/internal/prompt"
	"github.com/diezo/openpaas/internal/services"
	"github.com/diezo/openpaas/internal/system"
	"github.com/diezo/openpaas/internal/tls"
	"github.com/diezo/openpaas/internal/ui"
)

func newDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:         "delete <slug>",
		Aliases:     []string{"rm"},
		Short:       "Remove a service (container, subdomain and certificate)",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annNeedsRoot: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(strings.ToLower(strings.TrimSpace(args[0])), yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask for confirmation")
	return cmd
}

func runDelete(slug string, yes bool) error {
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
	svc, ok := store.Services[slug]
	if !ok {
		return fmt.Errorf("no service named %q (see: sudo op list)", slug)
	}
	host := slug + "." + cfg.Domain

	if !yes {
		c, err := prompt.Choice(fmt.Sprintf("Delete %s (%s)? This stops the container and removes its subdomain. [y]es/[n]o",
			ui.Bold(slug), host), []string{"y", "n"}, "n")
		if err != nil || c != "y" {
			ui.Info("Nothing deleted.")
			return nil
		}
	}

	// Each step tolerates the thing already being gone, so delete also
	// cleans up after a half-finished create.
	if err := ui.Step("Removing container "+svc.Container, func() error { return docker.Remove(svc.Container) }); err != nil {
		return err
	}
	if err := ui.Step("Removing nginx route for "+host, func() error { return nginx.RemoveService(slug) }); err != nil {
		return err
	}
	if err := ui.Step("Removing TLS certificate", func() error { return tls.Delete(host) }); err != nil {
		ui.Warn("Couldn't remove the certificate; it's harmless and will simply expire.")
	}

	delete(store.Services, slug)
	if err := store.Save(); err != nil {
		return err
	}
	ui.OK("Deleted %s", ui.Bold(slug))
	return nil
}
