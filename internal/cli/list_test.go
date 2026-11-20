package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/diezo/openpaas/internal/docker"
)

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second: "30s",
		5 * time.Minute:  "5m",
		3 * time.Hour:    "3h",
		72 * time.Hour:   "3d",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%s) = %q; want %q", d, got, want)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		info docker.Info
		want []string
	}{
		{docker.Info{}, []string{"container missing"}},
		{docker.Info{Status: "running", Started: time.Now().Add(-2 * time.Hour)}, []string{"running", "up 2h"}},
		{docker.Info{Status: "running", Started: time.Now(), Restarts: 3}, []string{"3 restarts"}},
		{docker.Info{Status: "restarting", Restarts: 7}, []string{"crash-looping (7 restarts)"}},
		{docker.Info{Status: "exited", ExitCode: 1, Finished: time.Now().Add(-5 * time.Minute)}, []string{"exited (1)", "5m ago"}},
	}
	for _, c := range cases {
		got := describe(c.info)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("describe(%+v) = %q; missing %q", c.info, got, w)
			}
		}
	}
}

func TestListAliases(t *testing.T) {
	cmd, _, err := newRoot().Find([]string{"ls"})
	if err != nil || cmd.Name() != "list" {
		t.Fatalf("`op ls` resolved to %v, %v; want list", cmd, err)
	}
}
