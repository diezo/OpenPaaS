package api

import (
	"io"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	for path, want := range map[string]string{"/": "hello", "/anything": "hello", "/healthz": "ok"} {
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		body, _ := io.ReadAll(rec.Body)
		if rec.Code != 200 || string(body) != want {
			t.Errorf("GET %s = %d %q; want 200 %q", path, rec.Code, body, want)
		}
	}
}
