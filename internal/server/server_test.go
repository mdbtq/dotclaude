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

func post(t *testing.T, h http.Handler, target, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Host = "localhost:7878"
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFixEndpoints(t *testing.T) {
	f := testfixture.New(t)
	h := (&server.Server{Paths: f.Paths}).Handler()
	body := `{"request": {"kind": "plugin-enabled", "target": "ghost@market", "value": "false"}}`
	ui := map[string]string{"Content-Type": "application/json", "X-Dotclaude": "1", "Origin": "http://localhost:7878"}

	cases := []struct {
		name   string
		header map[string]string
		status int
	}{
		{"from the UI", ui, http.StatusOK},
		{"plain form post", map[string]string{"Content-Type": "text/plain"}, http.StatusForbidden},
		{"no custom header", map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		{"foreign origin", map[string]string{"Content-Type": "application/json", "X-Dotclaude": "1", "Origin": "https://attacker.example"}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := post(t, h, "/api/fix/preview", body, tc.header); rec.Code != tc.status {
				t.Errorf("status %d, want %d: %s", rec.Code, tc.status, strings.TrimSpace(rec.Body.String()))
			}
		})
	}

	t.Run("not applicable", func(t *testing.T) {
		rec := post(t, h, "/api/fix/preview", `{"request": {"kind": "remove-symlink", "target": "/etc/hosts"}}`, ui)
		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, want 409", rec.Code)
		}
	})
	t.Run("stale preview", func(t *testing.T) {
		stale := `{"request": {"kind": "plugin-enabled", "target": "ghost@market", "value": "false"}, "hashes": {}}`
		if rec := post(t, h, "/api/fix/apply", stale, ui); rec.Code != http.StatusConflict {
			t.Errorf("status %d, want 409", rec.Code)
		}
	})
	t.Run("unknown project", func(t *testing.T) {
		other := `{"project": "` + f.Repo + `", "request": {"kind": "plugin-enabled", "target": "ghost@market", "value": "false"}}`
		if rec := post(t, h, "/api/fix/preview", other, ui); rec.Code != http.StatusForbidden {
			t.Errorf("status %d, want 403 for a directory Claude Code has not been started in", rec.Code)
		}
	})
	t.Run("read-only", func(t *testing.T) {
		ro := (&server.Server{Paths: f.Paths, ReadOnly: true}).Handler()
		if rec := post(t, ro, "/api/fix/preview", body, ui); rec.Code != http.StatusForbidden {
			t.Errorf("status %d, want 403", rec.Code)
		}
	})
}
