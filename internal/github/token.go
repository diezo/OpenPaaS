// Package github validates GitHub personal access tokens.
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RequiredScope is the classic-PAT scope OpenPaaS needs.
const RequiredScope = "read:packages"

// NewTokenURL pre-fills the token form with the right scope.
const NewTokenURL = "https://github.com/settings/tokens/new?scopes=read:packages&description=OpenPaaS"

var apiURL = "https://api.github.com/user"

// ValidateToken checks that the token works and has read:packages.
// It returns the GitHub username on success.
func ValidateToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("token is empty")
	}
	if strings.HasPrefix(token, "github_pat_") {
		return "", errors.New("this is a fine-grained token; OpenPaaS needs a classic token (starts with ghp_)")
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "openpaas-cli")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return "", errors.New("GitHub rejected this token (invalid or expired)")
	default:
		return "", fmt.Errorf("unexpected response from GitHub: %s", resp.Status)
	}

	if !HasScope(resp.Header.Get("X-OAuth-Scopes"), RequiredScope) {
		return "", fmt.Errorf("token is missing the %q scope", RequiredScope)
	}
	var u struct {
		Login string `json:"login"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&u)
	return u.Login, nil
}

// HasScope reports whether a comma-separated X-OAuth-Scopes header grants want.
// write:packages implies read:packages.
func HasScope(header, want string) bool {
	for _, s := range strings.Split(header, ",") {
		s = strings.TrimSpace(s)
		if s == want || (want == "read:packages" && s == "write:packages") {
			return true
		}
	}
	return false
}
