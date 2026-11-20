// Package service installs and controls the OpenPaaS API systemd unit.
package service

import (
	"fmt"
	"os"
	"strings"

	"github.com/diezo/openpaas/internal/system"
)

const (
	Name     = "openpaas-api"
	unitPath = "/etc/systemd/system/" + Name + ".service"
)

func unit(binPath string) string {
	return fmt.Sprintf(`# Managed by OpenPaaS. Re-running `+"`op setup`"+` regenerates this file.
[Unit]
Description=OpenPaaS deploy API
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s serve
Restart=always
RestartSec=2
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=read-only
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`, binPath)
}

// Install writes the unit file, enables it on boot, and (re)starts it.
func Install() error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	if err := system.WriteFile(unitPath, unit(bin), 0o644); err != nil {
		return err
	}
	if _, err := system.Run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err := system.Run("systemctl", "enable", Name); err != nil {
		return err
	}
	_, err = system.Run("systemctl", "restart", Name)
	return err
}

// Status returns systemd's active state ("active", "failed", "inactive", ...).
func Status() string {
	out, _ := system.Run("systemctl", "is-active", Name)
	s := strings.TrimSpace(out)
	if s == "" {
		return "unknown"
	}
	return s
}
