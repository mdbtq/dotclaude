package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdbtq/dotclaude/internal/server"
	"github.com/mdbtq/dotclaude/internal/testfixture"
)

func get(t *testing.T, h http.Handler, host, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServer(t *testing.T) {
	f := testfixture.New(t)
	h := (&server.Server{Paths: f.Paths}).Handler()
	file := func(path string) string { return "/api/file?path=" + url.QueryEscape(path) }

	cases := []struct {
		name, host, target string
		status             int
	}{
		{"config", "localhost:7878", "/api/config", http.StatusOK},
		{"project config", "127.0.0.1:7878", "/api/config?project=" + url.QueryEscape(f.Project), http.StatusOK},
		{"missing project", "localhost", "/api/config?project=/does/not/exist", http.StatusBadRequest},
		{"foreign host", "attacker.example:7878", "/api/config", http.StatusForbidden},
		{"configuration file", "localhost", file(filepath.Join(f.Home, ".claude", "CLAUDE.md")), http.StatusOK},
		{"settings file", "localhost", file(filepath.Join(f.Home, ".claude", "settings.json")), http.StatusForbidden},
		{"global config", "localhost", file(filepath.Join(f.Home, ".claude.json")), http.StatusForbidden},
		{"traversal", "localhost", file(filepath.Join(f.Home, ".claude", "agents", "..", "..", ".claude.json")), http.StatusForbidden},
		{"no ui", "localhost", "/", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, h, tc.host, tc.target)
			if rec.Code != tc.status {
				t.Errorf("status %d, want %d: %s", rec.Code, tc.status, strings.TrimSpace(rec.Body.String()))
			}
			if strings.Contains(rec.Body.String(), testfixture.Secret) {
				t.Errorf("response contains the planted secret")
			}
		})
	}
}
