package services

import "testing"

func TestValidateSlug(t *testing.T) {
	for _, ok := range []string{"app", "my-app", "a1", "x"} {
		if err := ValidateSlug(ok); err != nil {
			t.Errorf("ValidateSlug(%q) = %v; want ok", ok, err)
		}
	}
	for _, bad := range []string{"", "My-App", "-app", "app-", "my_app", "a.b", "deploy", "www"} {
		if err := ValidateSlug(bad); err == nil {
			t.Errorf("ValidateSlug(%q) = nil; want error", bad)
		}
	}
}

func TestStoreRoundTripAndFreePort(t *testing.T) {
	t.Setenv("OPENPAAS_CONFIG_DIR", t.TempDir())
	s, err := Load()
	if err != nil || len(s.Services) != 0 {
		t.Fatalf("Load empty: %v %v", s, err)
	}

	busy := map[int]bool{PortMin + 1: true} // something else listens here
	portFree = func(p int) bool { return !busy[p] }

	s.Services["a"] = &Service{Slug: "a", HostPort: PortMin}
	p, err := s.FreePort()
	if err != nil || p != PortMin+2 {
		t.Fatalf("FreePort = %d, %v; want %d", p, err, PortMin+2)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, err := Load()
	if err != nil || s2.Services["a"] == nil || s2.Services["a"].HostPort != PortMin {
		t.Fatalf("round trip: %+v, %v", s2, err)
	}
}
