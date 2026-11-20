package cli

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/openpaas/openpaas/internal/config"
	"github.com/openpaas/openpaas/internal/service"
	"github.com/openpaas/openpaas/internal/tls"
	"github.com/openpaas/openpaas/internal/ui"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show OpenPaaS status for this VM",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annNeedsRoot: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			row := func(k, v string) { fmt.Printf("  %-14s %s\n", ui.Dim(k), v) }

			ui.Header("OpenPaaS " + version)
			row("Domain", cfg.Domain)
			row("Public IP", cfg.PublicIP)
			row("GitHub token", config.MaskToken(cfg.GitHubToken))

			st := service.Status()
			if st == "active" {
				st = ui.Green("● active")
			} else {
				st = ui.Red("● " + st)
			}
			row("API service", st)

			url := "https://deploy." + cfg.Domain
			if reachable(url + "/healthz") {
				row("API URL", ui.Green(url))
			} else {
				row("API URL", ui.Yellow(url+" (not reachable)"))
			}

			if exp, err := tls.Expiry(cfg.Domain); err == nil {
				days := int(time.Until(exp).Hours() / 24)
				s := fmt.Sprintf("valid until %s (%d days)", exp.Format("2006-01-02"), days)
				if days < 14 {
					s = ui.Yellow(s)
				}
				row("TLS cert", s)
			} else {
				row("TLS cert", ui.Yellow("not found"))
			}
			fmt.Println()
			return nil
		},
	}
}

func reachable(url string) bool {
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 16))
	return resp.StatusCode == 200 && strings.TrimSpace(string(b)) == "ok"
}
