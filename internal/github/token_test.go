package github

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHasScope(t *testing.T) {
	if !HasScope("repo, read:packages", "read:packages") {
		t.Error("expected read:packages")
	}
	if !HasScope("write:packages", "read:packages") {
		t.Error("write:packages should imply read:packages")
	}
	if HasScope("repo, gist", "read:packages") {
		t.Error("did not expect read:packages")
	}
}

func TestValidateToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "token good":
			w.Header().Set("X-OAuth-Scopes", "read:packages")
			w.Write([]byte(`{"login":"octo"}`))
		case "token noscope":
			w.Header().Set("X-OAuth-Scopes", "repo")
			w.Write([]byte(`{"login":"octo"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	apiURL = srv.URL

	if login, err := ValidateToken("good"); err != nil || login != "octo" {
		t.Errorf("good token: %q, %v", login, err)
	}
	if _, err := ValidateToken("noscope"); err == nil {
		t.Error("expected scope error")
	}
	if _, err := ValidateToken("bad"); err == nil {
		t.Error("expected auth error")
	}
	if _, err := ValidateToken("github_pat_x"); err == nil {
		t.Error("expected fine-grained error")
	}
}
