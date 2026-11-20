package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/openpaas/openpaas/internal/api"
	"github.com/openpaas/openpaas/internal/config"
	"github.com/openpaas/openpaas/internal/github"
	"github.com/openpaas/openpaas/internal/nginx"
	"github.com/openpaas/openpaas/internal/prompt"
	"github.com/openpaas/openpaas/internal/service"
	"github.com/openpaas/openpaas/internal/system"
	"github.com/openpaas/openpaas/internal/tls"
	"github.com/openpaas/openpaas/internal/ui"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Set up (or reconfigure) OpenPaaS on this VM",
		Long: "Interactive setup: GitHub token, domain, DNS check, nginx, the deploy API and TLS.\n" +
			"Safe to re-run at any time. Press Enter to keep a current value.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annSkipSetup: "true", annNeedsRoot: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := runSetup()
			if errors.Is(err, prompt.ErrAborted) {
				fmt.Println()
				ui.Warn("Setup cancelled. Run %s to start again.", ui.Bold("sudo op setup"))
				return errSilent
			}
			return err
		},
	}
}

func runSetup() error {
	closeLog, _ := system.OpenLog("setup.log")
	defer closeLog()

	fmt.Println()
	ui.Box(ui.Bold("OpenPaaS setup"), ui.Dim("Press Enter to keep the value shown in [brackets]."))

	// 1. Preflight: dependencies the installer should have put in place.
	var missing []string
	for _, bin := range []string{"nginx", "certbot", "docker", "systemctl"} {
		if !system.Has(bin) {
			missing = append(missing, bin)
		}
	}
	if len(missing) > 0 {
		ui.Fail("Missing dependencies: %s", strings.Join(missing, ", "))
		ui.Info("Re-run the OpenPaaS installer to install them, then run %s again.", ui.Bold("sudo op setup"))
		return errSilent
	}

	prev, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}
	if prev == nil {
		prev = &config.Config{}
	}
	cfg := *prev

	// 2. GitHub token.
	ui.Header("1/4  GitHub token")
	fmt.Printf("  OpenPaaS needs a %s personal access token with the %s scope.\n",
		ui.Bold("classic"), ui.Bold(github.RequiredScope))
	fmt.Printf("  Create one at: %s\n\n", ui.Cyan(github.NewTokenURL))
	for {
		tok, err := prompt.AskSecret("  GitHub token", prev.GitHubToken, config.MaskToken(prev.GitHubToken))
		if err != nil {
			return err
		}
		var login string
		verr := ui.Step("Checking token with GitHub", func() error {
			var e error
			login, e = github.ValidateToken(tok)
			return e
		})
		if verr == nil {
			cfg.GitHubToken = tok
			if login != "" {
				ui.Info("Signed in as %s", ui.Bold(login))
			}
			break
		}
		fmt.Printf("  %s\n", ui.Red(verr.Error()))
	}

	// 3. Domain.
	ui.Header("2/4  Domain")
	fmt.Println("  Every app gets a subdomain of this domain, e.g. myapp.example.com.")
	fmt.Println()
	for {
		in, err := prompt.Ask("  Domain", prev.Domain, "")
		if err != nil {
			return err
		}
		d, err := config.NormalizeDomain(in)
		if err == nil {
			cfg.Domain = d
			break
		}
		fmt.Printf("  %s\n", ui.Red(err.Error()))
	}

	for {
		e, err := prompt.Ask("  Email for Let's Encrypt expiry notices (optional)", prev.Email, "")
		if err != nil {
			return err
		}
		if e == "" || (strings.Contains(e, "@") && !strings.ContainsAny(e, " ")) {
			cfg.Email = e
			break
		}
		fmt.Printf("  %s\n", ui.Red("that doesn't look like an email address"))
	}

	// Save what we have so a re-run offers these as defaults. Setup stays
	// "complete" only if nothing infrastructure-related changed.
	cfg.SetupComplete = prev.SetupComplete && cfg.Domain == prev.Domain
	if err := config.Save(&cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// 4. DNS.
	ui.Header("3/4  DNS")
	var ip string
	if err := ui.Step("Detecting this VM's public IP", func() error {
		var e error
		ip, e = system.PublicIP()
		return e
	}); err != nil {
		for {
			v, err := prompt.Ask("  Enter this VM's public IPv4", prev.PublicIP, "")
			if err != nil {
				return err
			}
			if p := net.ParseIP(v); p != nil && p.To4() != nil {
				ip = v
				break
			}
			fmt.Printf("  %s\n", ui.Red("not a valid IPv4 address"))
		}
	}
	cfg.PublicIP = ip

	fmt.Println()
	fmt.Println("  Add these DNS records at your domain registrar / DNS provider:")
	fmt.Println()
	fmt.Printf("    %-6s %-28s %s\n", ui.Dim("TYPE"), ui.Dim("NAME"), ui.Dim("VALUE"))
	fmt.Printf("    %-6s %-28s %s\n", "A", cfg.Domain, ip)
	fmt.Printf("    %-6s %-28s %s\n", "A", "*."+cfg.Domain, ip)
	fmt.Println()
	fmt.Println(ui.Dim("  If you use Cloudflare, set both records to \"DNS only\" (grey cloud)."))
	fmt.Println()
	if err := prompt.PressEnter("  Press Enter once the records are added... "); err != nil {
		return err
	}
	if err := verifyDNS(cfg.Domain, ip); err != nil {
		return err
	}

	// 5. Server configuration.
	ui.Header("4/4  Configuring this VM")
	if err := openFirewall(); err != nil {
		return err
	}
	if err := ui.Step("Configuring nginx", func() error { return nginx.Configure(cfg.Domain, api.Port) }); err != nil {
		return markIncomplete(&cfg, err)
	}
	if err := ui.Step("Starting the deploy API (auto-starts on boot)", func() error {
		if err := service.Install(); err != nil {
			return err
		}
		return waitHealthy(fmt.Sprintf("http://127.0.0.1:%d/healthz", api.Port), 10*time.Second)
	}); err != nil {
		return markIncomplete(&cfg, err)
	}
	deploy := "deploy." + cfg.Domain
	if err := ui.Step(fmt.Sprintf("Getting TLS certificate for %s and %s", cfg.Domain, deploy), func() error {
		return tls.Issue(cfg.Email, cfg.Domain, deploy)
	}); err != nil {
		ui.Info("Common causes: DNS not propagated yet, or ports 80/443 blocked by your cloud provider's firewall.")
		return markIncomplete(&cfg, err)
	}

	cfg.SetupComplete = true
	cfg.SetupAt = time.Now().UTC()
	cfg.Version = version
	if err := config.Save(&cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	ui.Box(
		ui.Green("✓ OpenPaaS is ready"),
		"",
		"Domain      "+ui.Bold(cfg.Domain),
		"Deploy API  "+ui.Cyan("https://"+deploy),
		"",
		ui.Dim("Check anytime with: sudo op status"),
	)
	fmt.Println()
	return nil
}

func markIncomplete(cfg *config.Config, err error) error {
	cfg.SetupComplete = false
	_ = config.Save(cfg)
	ui.Info("Details: %s", system.LogDir+"/setup.log")
	ui.Info("Fix the issue above, then run %s again.", ui.Bold("sudo op setup"))
	return err
}

// verifyDNS checks the apex, deploy and a random subdomain (to prove the
// wildcard record) all point at ip, letting the user retry while DNS propagates.
func verifyDNS(domain, ip string) error {
	names := []string{domain, "deploy." + domain, randomLabel() + "." + domain}
	labels := []string{domain, "deploy." + domain, "*." + domain + " (wildcard)"}
	for {
		allOK := true
		for i, n := range names {
			got, err := system.ResolveA(n)
			switch {
			case err != nil:
				allOK = false
				ui.Fail("%-34s not found yet", labels[i])
			case !slices.Contains(got, ip):
				allOK = false
				ui.Fail("%-34s points to %s (expected %s)", labels[i], strings.Join(got, ", "), ip)
			default:
				ui.OK("%-34s → %s", labels[i], ip)
			}
		}
		if allOK {
			return nil
		}
		fmt.Println(ui.Dim("  DNS changes can take a few minutes to propagate."))
		c, err := prompt.Choice("  [r]etry, [c]ontinue anyway, or [q]uit?", []string{"r", "c", "q"}, "r")
		if err != nil {
			return err
		}
		switch c {
		case "c":
			ui.Warn("Continuing without verified DNS: the TLS step will likely fail.")
			return nil
		case "q":
			return prompt.ErrAborted
		}
		fmt.Println()
	}
}

func randomLabel() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "op-check-" + hex.EncodeToString(b)
}

// openFirewall opens 80/443 if ufw is installed and active.
func openFirewall() error {
	if !system.Has("ufw") {
		return nil
	}
	out, _ := system.Run("ufw", "status")
	if !strings.Contains(out, "Status: active") {
		return nil
	}
	return ui.Step("Allowing HTTP/HTTPS through ufw", func() error {
		for _, p := range []string{"80/tcp", "443/tcp"} {
			if _, err := system.Run("ufw", "allow", p); err != nil {
				return err
			}
		}
		return nil
	})
}

func waitHealthy(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	out, _ := system.Run("journalctl", "-u", service.Name, "-n", "20", "--no-pager")
	fmt.Fprintln(os.Stderr, out)
	return fmt.Errorf("deploy API didn't become healthy within %s", timeout)
}
