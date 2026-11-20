package cli

import (
	"testing"

	"github.com/diezo/openpaas/internal/config"
	"github.com/diezo/openpaas/internal/system"
)

func run(args ...string) error {
	root := newRoot()
	root.SetArgs(args)
	return root.Execute()
}

func TestGate(t *testing.T) {
	t.Setenv("OPENPAAS_CONFIG_DIR", t.TempDir())
	isRoot = func() bool { return true }
	defer func() { isRoot = system.IsRoot }()

	if err := run("version"); err != nil {
		t.Errorf("version should run before setup: %v", err)
	}
	if err := run("help"); err != nil {
		t.Errorf("help should run before setup: %v", err)
	}
	for _, args := range [][]string{{"status"}, {"list"}, {"create", "app"}, {"delete", "app"}} {
		if err := run(args...); err != errSilent {
			t.Errorf("%v before setup: got %v, want gate refusal", args, err)
		}
	}

	if err := config.Save(&config.Config{Domain: "example.com", SetupComplete: false}); err != nil {
		t.Fatal(err)
	}
	if err := run("status"); err != errSilent {
		t.Errorf("status with incomplete setup: got %v, want gate refusal", err)
	}
}

// setup starts the API service before marking setup complete, so the gate
// must never block `serve` on setup state.
func TestServeNotGatedOnSetup(t *testing.T) {
	t.Setenv("OPENPAAS_CONFIG_DIR", t.TempDir())
	isRoot = func() bool { return true }
	defer func() { isRoot = system.IsRoot }()
	if err := gate(newServeCmd()); err != nil {
		t.Errorf("serve before setup complete: got %v, want allowed", err)
	}
}

func TestNeedsRoot(t *testing.T) {
	t.Setenv("OPENPAAS_CONFIG_DIR", t.TempDir())
	isRoot = func() bool { return false }
	defer func() { isRoot = system.IsRoot }()
	if err := run("setup"); err != errSilent {
		t.Errorf("setup without root: got %v, want refusal", err)
	}
	if err := run("version"); err != nil {
		t.Errorf("version without root should work: %v", err)
	}
}
