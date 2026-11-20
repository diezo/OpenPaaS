package config

import (
	"os"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	ok := map[string]string{
		"example.com":                "example.com",
		"  Example.COM  ":            "example.com",
		"https://example.com/path?x": "example.com",
		"http://sub.example.co.uk/":  "sub.example.co.uk",
		"example.com.":               "example.com",
		"*.example.com":              "example.com",
		"example.com:8080":           "example.com",
	}
	for in, want := range ok {
		got, err := NormalizeDomain(in)
		if err != nil || got != want {
			t.Errorf("NormalizeDomain(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{"", "localhost", "1.2.3.4", "exa mple.com", "-bad.com", "bad-.com", "a..com"}
	for _, in := range bad {
		if got, err := NormalizeDomain(in); err == nil {
			t.Errorf("NormalizeDomain(%q) = %q; want error", in, got)
		}
	}
}

func TestMaskToken(t *testing.T) {
	cases := map[string]string{
		"":                       "",
		"short":                  "****",
		"ghp_abcdefghijklmn1234": "ghp_****1234",
		"abcdefghijkl":           "****ijkl",
	}
	for in, want := range cases {
		if got := MaskToken(in); got != want {
			t.Errorf("MaskToken(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENPAAS_CONFIG_DIR", dir)

	if _, err := Load(); err != ErrNotFound {
		t.Fatalf("Load on empty dir: got %v, want ErrNotFound", err)
	}
	in := &Config{Domain: "example.com", GitHubToken: "ghp_x", SetupComplete: true}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if out.Domain != in.Domain || out.GitHubToken != in.GitHubToken || !out.SetupComplete {
		t.Fatalf("round trip mismatch: %+v", out)
	}
	fi, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 && os.PathSeparator == '/' {
		t.Errorf("config perm = %o; want 600", perm)
	}
}
