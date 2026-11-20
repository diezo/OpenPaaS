package docker

import "testing"

func TestParseImage(t *testing.T) {
	ok := map[string]string{
		"nginx":                             "nginx:latest",
		"nginx:1.27":                        "nginx:1.27",
		"user/app":                          "user/app:latest",
		"ghcr.io/User/App:latest":           "ghcr.io/user/app:latest",
		"https://ghcr.io/user/app":          "ghcr.io/user/app:latest",
		"  ghcr.io/org/team/app:v1.2.3  ":   "ghcr.io/org/team/app:v1.2.3",
		"localhost:5000/app:dev":            "localhost:5000/app:dev",
		"registry.example.com:443/a/b:main": "registry.example.com:443/a/b:main",
	}
	for in, want := range ok {
		img, err := ParseImage(in)
		if err != nil || img.Ref() != want {
			t.Errorf("ParseImage(%q) = %q, %v; want %q", in, img.Ref(), err, want)
		}
	}
	if img, _ := ParseImage("ghcr.io/u/a"); !img.IsGHCR() {
		t.Error("expected ghcr.io image")
	}
	for _, bad := range []string{"", "has space/app", "app:bad tag", "app@sha256:abc", "-app"} {
		if img, err := ParseImage(bad); err == nil {
			t.Errorf("ParseImage(%q) = %q; want error", bad, img.Ref())
		}
	}
}
