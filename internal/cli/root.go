// Package cli defines the `op` command tree.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/diezo/openpaas/internal/config"
	"github.com/diezo/openpaas/internal/system"
	"github.com/diezo/openpaas/internal/ui"
)

// Build info, injected via -ldflags at release time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Command annotations controlling the pre-run gate.
const (
	annSkipSetup = "skipSetup" // runs even before `op setup` has completed
	annNeedsRoot = "needsRoot" // must run via sudo
)

// errSilent signals an error whose message has already been printed.
var errSilent = errors.New("")

// isRoot is a var so tests can stub it.
var isRoot = system.IsRoot

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "op",
		Short:         "OpenPaaS: turn this VM into your personal PaaS",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return gate(cmd)
		},
		// Bare `op` shows help and is always allowed.
		Annotations: map[string]string{annSkipSetup: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.CompletionOptions.HiddenDefaultCmd = true
	root.AddCommand(newVersionCmd(), newSetupCmd(), newStatusCmd(), newServeCmd())
	return root
}

// gate enforces root and "setup must be complete" rules per command.
func gate(cmd *cobra.Command) error {
	// cobra's built-in help/completion commands have no annotations; allow them.
	if cmd.Name() == "help" || cmd.Name() == "completion" || (cmd.Parent() != nil && cmd.Parent().Name() == "completion") {
		return nil
	}
	if cmd.Annotations[annNeedsRoot] == "true" && !isRoot() {
		ui.Fail("This command needs root. Run: %s", ui.Bold("sudo "+cmd.CommandPath()))
		return errSilent
	}
	if cmd.Annotations[annSkipSetup] == "true" {
		return nil
	}
	cfg, err := config.Load()
	switch {
	case errors.Is(err, os.ErrPermission):
		ui.Fail("Can't read the OpenPaaS config. Run: %s", ui.Bold("sudo "+cmd.CommandPath()))
		return errSilent
	case err != nil && !errors.Is(err, config.ErrNotFound):
		return err
	case cfg == nil || !cfg.SetupComplete:
		ui.Fail("OpenPaaS isn't set up yet. Run: %s", ui.Bold("sudo op setup"))
		return errSilent
	}
	return nil
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	if err := newRoot().Execute(); err != nil {
		if err != errSilent {
			ui.Fail("%v", err)
		}
		return 1
	}
	return 0
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "version",
		Short:       "Print the OpenPaaS version",
		Annotations: map[string]string{annSkipSetup: "true"},
		Args:        cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Printf("op %s (commit %s, built %s)\n", version, commit, date)
		},
	}
}
