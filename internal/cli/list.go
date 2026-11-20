package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

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
		Short:       "List deployed services",
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
			// Status is colored after padding (ANSI codes would break tabwriter widths).
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "SLUG\tURL\tIMAGE\tPORTS\tSTATUS")
			for _, s := range store.Sorted() {
				scheme := "https://"
				if !s.TLS {
					scheme = "http://"
				}
				st := docker.State(s.Container)
				if st == "" {
					st = "missing"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d→%d\t%s\n",
					s.Slug, scheme+s.Slug+"."+cfg.Domain, s.Image, s.HostPort, s.InternalPort, colorState(st))
			}
			return w.Flush()
		},
	}
}

func colorState(st string) string {
	switch st {
	case "running":
		return ui.Green("● " + st)
	case "restarting", "created", "paused":
		return ui.Yellow("● " + st)
	default:
		return ui.Red("● " + st)
	}
}
