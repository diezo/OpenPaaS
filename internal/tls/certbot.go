// Package tls obtains Let's Encrypt certificates via certbot.
package tls

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/diezo/openpaas/internal/system"
)

const liveDir = "/etc/letsencrypt/live"

// Issue gets (or reuses / expands) a cert for the given names and lets the
// certbot nginx plugin install it and add an HTTP→HTTPS redirect.
// The cert is named after the first name, so re-runs stay idempotent.
func Issue(email string, names ...string) error {
	if len(names) == 0 {
		return errors.New("no domains given")
	}
	args := []string{
		"--nginx", "--non-interactive", "--agree-tos",
		"--keep-until-expiring", "--expand", "--redirect",
		"--cert-name", names[0],
	}
	if email != "" {
		args = append(args, "-m", email)
	} else {
		args = append(args, "--register-unsafely-without-email")
	}
	for _, n := range names {
		args = append(args, "-d", n)
	}
	_, err := system.Run("certbot", args...)
	return err
}

// Delete removes a certificate; a missing cert isn't an error.
func Delete(certName string) error {
	if _, err := os.Stat(filepath.Join(liveDir, certName)); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	_, err := system.Run("certbot", "delete", "--non-interactive", "--cert-name", certName)
	return err
}

// Expiry returns when the cert named certName expires.
func Expiry(certName string) (time.Time, error) {
	b, err := os.ReadFile(filepath.Join(liveDir, certName, "cert.pem"))
	if err != nil {
		return time.Time{}, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return time.Time{}, errors.New("invalid certificate file")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return c.NotAfter, nil
}
