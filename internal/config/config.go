// Package config loads and saves OpenPaaS's on-disk configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultDir = "/etc/openpaas"
	fileName   = "config.json"
)

// Config is persisted at /etc/openpaas/config.json (root-only, 0600).
type Config struct {
	Domain        string    `json:"domain"`
	GitHubToken   string    `json:"github_token"`
	Email         string    `json:"email,omitempty"`
	PublicIP      string    `json:"public_ip,omitempty"`
	SetupComplete bool      `json:"setup_complete"`
	SetupAt       time.Time `json:"setup_at,omitempty"`
	Version       string    `json:"version,omitempty"`
}

// ErrNotFound means no config file exists yet.
var ErrNotFound = errors.New("config not found")

// Dir returns the config directory. OPENPAAS_CONFIG_DIR overrides it (used in tests).
func Dir() string {
	if d := os.Getenv("OPENPAAS_CONFIG_DIR"); d != "" {
		return d
	}
	return defaultDir
}

// Path returns the full path to the config file.
func Path() string { return filepath.Join(Dir(), fileName) }

// Load reads the config. It returns ErrNotFound when there is none, and
// os.ErrPermission (wrapped) when the caller can't read it.
func Load() (*Config, error) {
	b, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Path(), err)
	}
	return &c, nil
}

// Save writes the config atomically with 0600 permissions.
func Save(c *Config) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(Dir(), ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), Path())
}

var labelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// NormalizeDomain turns input like "https://Example.com/path" into "example.com"
// and validates it as a hostname with at least two labels.
func NormalizeDomain(in string) (string, error) {
	d := strings.TrimSpace(strings.ToLower(in))
	if i := strings.Index(d, "://"); i >= 0 {
		d = d[i+3:]
	}
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	if h, _, err := net.SplitHostPort(d); err == nil {
		d = h
	}
	d = strings.TrimSuffix(d, ".")
	d = strings.TrimPrefix(d, "*.")
	if d == "" {
		return "", errors.New("domain is empty")
	}
	if net.ParseIP(d) != nil {
		return "", errors.New("enter a domain name, not an IP address")
	}
	if len(d) > 253 {
		return "", errors.New("domain is too long")
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%q is not a full domain (e.g. example.com)", d)
	}
	for _, l := range labels {
		if !labelRe.MatchString(l) {
			return "", fmt.Errorf("%q is not a valid domain", d)
		}
	}
	return d, nil
}

// MaskToken hides all but the prefix and last 4 chars of a token.
func MaskToken(t string) string {
	if t == "" {
		return ""
	}
	if len(t) <= 8 {
		return "****"
	}
	prefix := ""
	if i := strings.Index(t, "_"); i > 0 && i < 6 {
		prefix = t[:i+1]
	}
	return prefix + "****" + t[len(t)-4:]
}
