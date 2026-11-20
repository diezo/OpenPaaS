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
		// Not gated on setup: `op setup` starts this service before it marks
		// setup complete (TLS comes last), and only setup installs the unit.
		Annotations: map[string]string{annNeedsRoot: "true", annSkipSetup: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return api.Serve(fmt.Sprintf("127.0.0.1:%d", port))
		},
	}
	cmd.Flags().IntVar(&port, "port", api.Port, "port to listen on (localhost only)")
	return cmd
}
