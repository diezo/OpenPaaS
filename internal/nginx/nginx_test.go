package nginx

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	out, err := Render("deploy.conf.tmpl", "example.com", 7777)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"server_name deploy.example.com;", "proxy_pass http://127.0.0.1:7777;"} {
		if !strings.Contains(out, want) {
			t.Errorf("deploy conf missing %q", want)
		}
	}
	out, err = Render("root.conf.tmpl", "example.com", 7777)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "server_name example.com;") {
		t.Error("root conf missing server_name")
	}
}
