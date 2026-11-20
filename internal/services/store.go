// Package services stores the apps deployed with `op create`.
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/diezo/openpaas/internal/config"
)

// Service is one deployed app.
type Service struct {
	Slug         string    `json:"slug"`
	Image        string    `json:"image"`
	InternalPort int       `json:"internal_port"`
	HostPort     int       `json:"host_port"`
	Container    string    `json:"container"`
	TLS          bool      `json:"tls"`
	CreatedAt    time.Time `json:"created_at"`
}

// Store is the on-disk list of services, keyed by slug.
type Store struct {
	Services map[string]*Service `json:"services"`
}

// Host port range reserved for services.
const (
	PortMin = 10000
	PortMax = 19999
)

// Reserved slugs that can't be used for services.
var Reserved = map[string]bool{"deploy": true, "www": true, "api": true, "op": true}

func path() string { return filepath.Join(config.Dir(), "services.json") }

// ContainerName is the docker container name for a slug.
func ContainerName(slug string) string { return "op-" + slug }

// Load reads the store (empty if the file doesn't exist yet).
func Load() (*Store, error) {
	s := &Store{Services: map[string]*Service{}}
	b, err := os.ReadFile(path())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path(), err)
	}
	if s.Services == nil {
		s.Services = map[string]*Service{}
	}
	return s, nil
}

// Save writes the store atomically with 0600 permissions.
func (s *Store) Save() error {
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(config.Dir(), ".services-*.json")
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
	return os.Rename(tmp.Name(), path())
}

// Sorted returns services ordered by slug.
func (s *Store) Sorted() []*Service {
	out := make([]*Service, 0, len(s.Services))
	for _, svc := range s.Services {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidateSlug checks a slug is a usable, unreserved subdomain label.
func ValidateSlug(slug string) error {
	if !slugRe.MatchString(slug) {
		return fmt.Errorf("%q isn't a valid slug: use lowercase letters, numbers and hyphens (e.g. my-app)", slug)
	}
	if Reserved[slug] {
		return fmt.Errorf("%q is reserved by OpenPaaS; pick another slug", slug)
	}
	return nil
}

// portFree is a var so tests can stub it.
var portFree = func(p int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// FreePort returns the lowest port in range that no service has reserved and
// that nothing else is listening on.
func (s *Store) FreePort() (int, error) {
	used := map[int]bool{}
	for _, svc := range s.Services {
		used[svc.HostPort] = true
	}
	for p := PortMin; p <= PortMax; p++ {
		if !used[p] && portFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free ports left in %d-%d", PortMin, PortMax)
}
