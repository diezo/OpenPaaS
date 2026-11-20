package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/diezo/openpaas/internal/config"
	"github.com/diezo/openpaas/internal/docker"
	"github.com/diezo/openpaas/internal/services"
	"github.com/diezo/openpaas/internal/ui"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Aliases:     []string{"ls"},
		Short:       "List services and whether they're running (alias: ls)",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annNeedsRoot: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			store, err := services.Load()
			if err != nil {
				return err
			}
			if len(store.Services) == 0 {
				fmt.Printf("No services yet. Create one with: %s\n", ui.Bold("sudo op create <slug>"))
				return nil
			}

			// Colored columns go last: ANSI codes would throw off tabwriter's widths.
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "SLUG\tURL\tIMAGE\tPORTS\tSTATUS")
			running, down := 0, 0
			for _, s := range store.Sorted() {
				scheme := "https://"
				if !s.TLS {
					scheme = "http://"
				}
				info := docker.Inspect(s.Container)
				if info.Status == "running" {
					running++
				} else {
					down++
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d→%d\t%s\n",
					s.Slug, scheme+s.Slug+"."+cfg.Domain, s.Image, s.HostPort, s.InternalPort, describe(info))
			}
			if err := w.Flush(); err != nil {
				return err
			}

			summary := fmt.Sprintf("%d service%s · %s", len(store.Services), plural(len(store.Services)),
				ui.Green(fmt.Sprintf("%d running", running)))
			if down > 0 {
				summary += " · " + ui.Red(fmt.Sprintf("%d down", down))
			}
			fmt.Printf("\n%s\n", summary)
			if down > 0 {
				fmt.Println(ui.Dim("Check a stopped service's logs with: sudo docker logs op-<slug>"))
			}
			return nil
		},
	}
}

// describe renders a status like "● running · up 3h" or "● exited (1) · 5m ago".
func describe(i docker.Info) string {
	switch i.Status {
	case "":
		return ui.Red("● container missing")
	case "running":
		s := ui.Green("● running") + ui.Dim(" · up "+humanDuration(time.Since(i.Started)))
		if i.Restarts > 0 {
			s += ui.Yellow(fmt.Sprintf(" · %d restart%s", i.Restarts, plural(i.Restarts)))
		}
		return s
	case "restarting":
		return ui.Yellow(fmt.Sprintf("● crash-looping (%d restarts)", i.Restarts))
	case "exited", "dead":
		s := ui.Red(fmt.Sprintf("● %s (%d)", i.Status, i.ExitCode))
		if !i.Finished.IsZero() {
			s += ui.Dim(" · " + humanDuration(time.Since(i.Finished)) + " ago")
		}
		return s
	default: // created, paused, removing
		return ui.Yellow("● " + i.Status)
	}
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
