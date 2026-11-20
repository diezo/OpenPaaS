// Package docker is a thin wrapper around the docker CLI.
package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/diezo/openpaas/internal/system"
)

// Image is a parsed image reference.
type Image struct {
	Registry string // "" means Docker Hub
	Repo     string // e.g. "user/app" or "nginx"
	Tag      string // e.g. "latest"
}

// Ref is the reference to pass to docker, e.g. "ghcr.io/user/app:latest".
func (i Image) Ref() string {
	r := i.Repo + ":" + i.Tag
	if i.Registry != "" {
		r = i.Registry + "/" + r
	}
	return r
}

// IsGHCR reports whether the image lives on GitHub Container Registry.
func (i Image) IsGHCR() bool { return i.Registry == "ghcr.io" }

var (
	repoRe = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*$`)
	tagRe  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	hostRe = regexp.MustCompile(`^[a-z0-9.-]+(:[0-9]+)?$`)
)

// ParseImage accepts things like "ghcr.io/user/app:latest", "nginx",
// "user/app:1.2" or "https://ghcr.io/user/app". A missing tag means "latest".
func ParseImage(in string) (Image, error) {
	s := strings.TrimSpace(in)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return Image{}, errors.New("image is empty")
	}
	if strings.Contains(s, "@") {
		return Image{}, errors.New("digests aren't supported; use a tag like :latest")
	}

	var img Image
	// Tag: a ":" after the last "/".
	if i := strings.LastIndex(s, ":"); i > strings.LastIndex(s, "/") {
		img.Tag, s = s[i+1:], s[:i]
	}
	if img.Tag == "" {
		img.Tag = "latest"
	}
	// Registry: first component with a "." or ":" (or "localhost").
	if i := strings.Index(s, "/"); i > 0 {
		first := strings.ToLower(s[:i])
		if strings.ContainsAny(first, ".:") || first == "localhost" {
			img.Registry, s = first, s[i+1:]
		}
	}
	img.Repo = strings.ToLower(s)

	switch {
	case img.Registry != "" && !hostRe.MatchString(img.Registry):
		return Image{}, fmt.Errorf("invalid registry %q", img.Registry)
	case !repoRe.MatchString(img.Repo):
		return Image{}, fmt.Errorf("invalid image name %q", img.Repo)
	case !tagRe.MatchString(img.Tag):
		return Image{}, fmt.Errorf("invalid tag %q", img.Tag)
	}
	return img, nil
}

// Login authenticates docker against a registry (token passed on stdin).
func Login(registry, user, token string) error {
	if _, err := system.RunInput(token, "docker", "login", registry, "-u", user, "--password-stdin"); err != nil {
		return fmt.Errorf("docker login to %s failed: %w", registry, err)
	}
	return nil
}

// Pull fetches an image; failure means it doesn't exist or we lack access.
func Pull(ref string) error {
	if _, err := system.Run("docker", "pull", ref); err != nil {
		return fmt.Errorf("couldn't pull %s (does it exist, and does your token have access?): %w", ref, err)
	}
	return nil
}

// ExposedPorts returns the TCP ports the image declares with EXPOSE.
func ExposedPorts(ref string) ([]int, error) {
	out, err := system.Run("docker", "image", "inspect", "--format", "{{json .Config.ExposedPorts}}", ref)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &m); err != nil {
		return nil, nil // "null" or unexpected: treat as none declared
	}
	var ports []int
	for k := range m {
		num, proto, _ := strings.Cut(k, "/")
		if proto == "" || proto == "tcp" {
			if p, err := strconv.Atoi(num); err == nil {
				ports = append(ports, p)
			}
		}
	}
	return ports, nil
}

// Run starts a container that restarts on crash/boot, with internalPort
// published on 127.0.0.1:hostPort (reachable only through nginx).
func Run(name, ref string, hostPort, internalPort int, slug string) error {
	_, err := system.Run("docker", "run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", hostPort, internalPort),
		"--label", "openpaas.slug="+slug,
		ref)
	return err
}

// State returns the container state ("running", "exited", ...) or "" if it
// doesn't exist.
func State(name string) string {
	out, err := system.Run("docker", "inspect", "--format", "{{.State.Status}}", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Info is a container's live runtime state.
type Info struct {
	Status   string    // "running", "exited", "restarting", ... or "" if missing
	Started  time.Time // when the current run started
	Finished time.Time // when it last stopped
	ExitCode int
	Restarts int
}

// Inspect returns live state for a container (Status "" if it doesn't exist).
func Inspect(name string) Info {
	out, err := system.Run("docker", "inspect", "--format",
		`{{json .State}}|{{.RestartCount}}`, name)
	if err != nil {
		return Info{}
	}
	stateJSON, restarts, _ := strings.Cut(strings.TrimSpace(out), "|")
	var st struct {
		Status     string
		StartedAt  time.Time
		FinishedAt time.Time
		ExitCode   int
	}
	if json.Unmarshal([]byte(stateJSON), &st) != nil {
		return Info{}
	}
	n, _ := strconv.Atoi(restarts)
	return Info{Status: st.Status, Started: st.StartedAt, Finished: st.FinishedAt, ExitCode: st.ExitCode, Restarts: n}
}

// Exists reports whether a container with this name exists.
func Exists(name string) bool { return State(name) != "" }

// WaitRunning waits until the container has stayed running for a few seconds,
// catching apps that crash on start.
func WaitRunning(name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	stableSince := time.Time{}
	for time.Now().Before(deadline) {
		switch st := State(name); st {
		case "running":
			if stableSince.IsZero() {
				stableSince = time.Now()
			} else if time.Since(stableSince) >= 3*time.Second {
				return nil
			}
		case "created", "restarting":
			stableSince = time.Time{}
		default:
			return fmt.Errorf("container stopped (state: %s). Last logs:\n%s", st, Logs(name, 15))
		}
		time.Sleep(500 * time.Millisecond)
	}
	if State(name) == "running" {
		return nil
	}
	return fmt.Errorf("container didn't start within %s. Last logs:\n%s", timeout, Logs(name, 15))
}

// Logs returns the last n lines of a container's output.
func Logs(name string, n int) string {
	out, _ := system.Run("docker", "logs", "--tail", strconv.Itoa(n), name)
	return "    " + strings.ReplaceAll(strings.TrimRight(out, "\n"), "\n", "\n    ")
}

// Remove force-removes a container; a missing container isn't an error.
func Remove(name string) error {
	if !Exists(name) {
		return nil
	}
	_, err := system.Run("docker", "rm", "-f", name)
	return err
}
