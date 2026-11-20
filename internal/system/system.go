// Package system wraps shell commands, root checks, and network lookups.
package system

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LogDir holds OpenPaaS logs.
const LogDir = "/var/log/openpaas"

var logw io.Writer = io.Discard

// OpenLog starts logging every command run to LogDir/<name>. Returns a closer.
func OpenLog(name string) (func(), error) {
	if err := os.MkdirAll(LogDir, 0o750); err != nil {
		return func() {}, err
	}
	f, err := os.OpenFile(filepath.Join(LogDir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return func() {}, err
	}
	logw = f
	fmt.Fprintf(f, "\n===== %s =====\n", time.Now().Format(time.RFC3339))
	return func() { logw = io.Discard; f.Close() }, nil
}

// Logf writes a line to the active log.
func Logf(format string, a ...any) { fmt.Fprintf(logw, format+"\n", a...) }

// IsRoot reports whether the process runs as root.
func IsRoot() bool { return os.Geteuid() == 0 }

// Run executes a command, logging its output. On failure the error includes
// the last lines of output so users can see what went wrong.
func Run(name string, args ...string) (string, error) {
	Logf("$ %s %s", name, strings.Join(args, " "))
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.MultiWriter(&buf, logw)
	cmd.Stderr = io.MultiWriter(&buf, logw)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	err := cmd.Run()
	out := buf.String()
	if err != nil {
		return out, fmt.Errorf("%s failed: %w\n%s", name, err, tail(out, 15))
	}
	return out, nil
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return "    " + strings.Join(lines, "\n    ")
}

// Has reports whether a binary is on PATH.
func Has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// PublicIP asks a couple of public "what's my IP" services for this VM's IPv4.
func PublicIP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, u := range []string{"https://api.ipify.org", "https://ifconfig.me/ip", "https://icanhazip.com"} {
		resp, err := client.Get(u)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		ip := strings.TrimSpace(string(b))
		if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil {
			return ip, nil
		}
	}
	return "", fmt.Errorf("couldn't detect this VM's public IP")
}

// resolver queries public DNS directly so local /etc/hosts or stale caches
// don't hide a wrong record.
var resolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		d := net.Dialer{Timeout: 3 * time.Second}
		return d.DialContext(ctx, "udp", "1.1.1.1:53")
	},
}

// ResolveA returns the IPv4 addresses a name resolves to (sorted).
func ResolveA(name string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	ips, err := resolver.LookupIP(ctx, "ip4", name)
	if err != nil {
		// fall back to the system resolver (e.g. outbound UDP 53 blocked)
		ips, err = net.DefaultResolver.LookupIP(ctx, "ip4", name)
		if err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	sort.Strings(out)
	return out, nil
}

// WriteFile writes content to path, creating parent dirs.
func WriteFile(path, content string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), perm)
}
