package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/diezo/openpaas/internal/api"
)

// newServeCmd is run by systemd (openpaas-api.service); users don't call it.
func newServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:         "serve",
		Short:       "Run the deploy API server (used by systemd)",
		Hidden:      true,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annNeedsRoot: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return api.Serve(fmt.Sprintf("127.0.0.1:%d", port))
		},
	}
	cmd.Flags().IntVar(&port, "port", api.Port, "port to listen on (localhost only)")
	return cmd
}
