// Package nginx writes OpenPaaS-managed nginx site configs.
package nginx

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/diezo/openpaas/internal/system"
)

//go:embed templates/*.tmpl
var tmplFS embed.FS

var tmpl = template.Must(template.ParseFS(tmplFS, "templates/*.tmpl"))

const (
	sitesAvailable = "/etc/nginx/sites-available"
	sitesEnabled   = "/etc/nginx/sites-enabled"
	confD          = "/etc/nginx/conf.d"
	nginxConf      = "/etc/nginx/nginx.conf"
	WebRoot        = "/var/www/openpaas"
)

// Site names (files are fixed, so a domain change simply overwrites them).
const (
	RootSite   = "openpaas-root.conf"
	DeploySite = "openpaas-deploy.conf"
)

type data struct {
	Domain  string
	WebRoot string
	APIPort int
}

// Render returns the rendered template text (exported for tests).
func Render(name, domain string, apiPort int) (string, error) {
	var b bytes.Buffer
	err := tmpl.ExecuteTemplate(&b, name, data{Domain: domain, WebRoot: WebRoot, APIPort: apiPort})
	return b.String(), err
}

// Configure writes the root + deploy sites, validates, and reloads nginx.
func Configure(domain string, apiPort int) error {
	if err := ensureHashBucket(); err != nil {
		return err
	}
	page, err := Render("index.html.tmpl", domain, apiPort)
	if err != nil {
		return err
	}
	if err := system.WriteFile(filepath.Join(WebRoot, "index.html"), page, 0o644); err != nil {
		return err
	}
	for site, tpl := range map[string]string{RootSite: "root.conf.tmpl", DeploySite: "deploy.conf.tmpl"} {
		conf, err := Render(tpl, domain, apiPort)
		if err != nil {
			return err
		}
		if err := enable(site, conf); err != nil {
			return err
		}
	}
	return TestAndReload()
}

func enable(site, conf string) error {
	avail := filepath.Join(sitesAvailable, site)
	if err := system.WriteFile(avail, conf, 0o644); err != nil {
		return err
	}
	link := filepath.Join(sitesEnabled, site)
	if err := os.MkdirAll(sitesEnabled, 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(link); err == nil {
		os.Remove(link)
	}
	return os.Symlink(avail, link)
}

// TestAndReload runs `nginx -t` and reloads (or starts) nginx.
func TestAndReload() error {
	if _, err := system.Run("nginx", "-t"); err != nil {
		return fmt.Errorf("nginx config test failed: %w", err)
	}
	if _, err := system.Run("systemctl", "is-active", "--quiet", "nginx"); err != nil {
		_, err = system.Run("systemctl", "enable", "--now", "nginx")
		return err
	}
	_, err := system.Run("systemctl", "reload", "nginx")
	return err
}

var bucketRe = regexp.MustCompile(`(?m)^\s*server_names_hash_bucket_size`)

// ensureHashBucket raises server_names_hash_bucket_size so long domains don't
// break `nginx -t`, unless the main config already sets it.
func ensureHashBucket() error {
	main, err := os.ReadFile(nginxConf)
	if err != nil || bucketRe.Match(main) {
		return nil
	}
	return system.WriteFile(filepath.Join(confD, "openpaas.conf"),
		"# Managed by OpenPaaS\nserver_names_hash_bucket_size 128;\n", 0o644)
}
