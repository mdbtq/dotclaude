// Package server serves the dotclaude web UI and its JSON API.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdbtq/dotclaude/internal/claude"
	"github.com/mdbtq/dotclaude/internal/fix"
	"github.com/mdbtq/dotclaude/internal/lint"
)

// maxFileSize caps what /api/file returns; instruction and skill files are
// far smaller, so anything larger is not what the UI asked for.
const maxFileSize = 2 << 20

// Server answers API requests by reading the configuration from disk on
// every request, so the UI always shows the current state.
type Server struct {
	Paths   claude.Paths
	Version string
	// UI holds the built web app; nil serves a notice instead.
	UI fs.FS
	// ReadOnly refuses every change.
	ReadOnly bool
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/meta", s.meta)
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("GET /api/lint", s.lint)
	mux.HandleFunc("GET /api/file", s.file)
	mux.Handle("POST /api/fix/preview", s.writeGuard(http.HandlerFunc(s.fixPreview)))
	mux.Handle("POST /api/fix/apply", s.writeGuard(http.HandlerFunc(s.fixApply)))
	mux.Handle("/", s.ui())
	return localOnly(mux)
}

func (s *Server) meta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"version":      s.Version,
		"configDir":    s.Paths.ConfigDir,
		"globalConfig": s.Paths.GlobalConfig,
		"managedDir":   s.Paths.ManagedDir,
		"memoryDirs":   s.Paths.MemoryDirs,
		"home":         s.Paths.Home,
		"readOnly":     s.ReadOnly,
		"rules":        lint.Rules,
	})
}

func (s *Server) load(w http.ResponseWriter, r *http.Request) (*claude.Config, bool) {
	c, err := claude.Load(s.Paths, r.URL.Query().Get("project"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, false
	}
	return c, true
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	if c, ok := s.load(w, r); ok {
		writeJSON(w, c)
	}
}

func (s *Server) lint(w http.ResponseWriter, r *http.Request) {
	if c, ok := s.load(w, r); ok {
		findings := lint.Run(c, lint.DefaultOptions)
		if findings == nil {
			findings = []lint.Finding{}
		}
		writeJSON(w, findings)
	}
}

// file returns the contents of a file that is part of the loaded
// configuration. Any other path is refused, so the endpoint cannot be used to
// read arbitrary files, and settings files are never served raw because they
// can hold secrets.
func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	c, ok := s.load(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if !allowedFiles(c)[path] {
		http.Error(w, "not a file of this configuration", http.StatusForbidden)
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if info.Size() > maxFileSize {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

func allowedFiles(c *claude.Config) map[string]bool {
	allowed := map[string]bool{}
	for _, comp := range c.Components {
		allowed[comp.Path] = true
	}
	var walk func(in claude.Instruction)
	walk = func(in claude.Instruction) {
		allowed[in.Path] = true
		for _, child := range in.Imports {
			walk(child)
		}
	}
	for _, in := range c.Instructions {
		walk(in)
	}
	for _, store := range c.Memory {
		if store.Index != nil {
			allowed[store.Index.Path] = true
		}
		for _, m := range store.Entries {
			allowed[m.Path] = true
		}
	}
	return allowed
}

type fixBody struct {
	Project  string            `json:"project"`
	Requests []fix.Request     `json:"requests"`
	Hashes   map[string]string `json:"hashes"`
}

// maxBatch caps how many changes one request may ask for.
const maxBatch = 500

// prepareFix re-derives the plan from the configuration as it is now, so the
// client only names a change and never supplies a path to write to.
func (s *Server) prepareFix(w http.ResponseWriter, r *http.Request) (*fix.Plan, fixBody, bool) {
	var body fixBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return nil, body, false
	}
	c, err := claude.Load(s.Paths, body.Project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, body, false
	}
	if !s.knownProject(c) {
		http.Error(w, "changes are only made for the user level or a project Claude Code has been started in", http.StatusForbidden)
		return nil, body, false
	}
	if len(body.Requests) == 0 || len(body.Requests) > maxBatch {
		http.Error(w, fmt.Sprintf("send between 1 and %d requests", maxBatch), http.StatusBadRequest)
		return nil, body, false
	}
	plan, err := fix.Prepare(c, s.Paths, body.Requests)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, fix.ErrNotApplicable) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return nil, body, false
	}
	return plan, body, true
}

// knownProject limits changes to the user level and to projects listed in
// the global config, so a request cannot point the editor at an arbitrary
// directory's .claude.
func (s *Server) knownProject(c *claude.Config) bool {
	if c.Project == "" {
		return true
	}
	for _, p := range c.Projects {
		if p.Path == c.Project {
			return true
		}
	}
	return false
}

func (s *Server) fixPreview(w http.ResponseWriter, r *http.Request) {
	if plan, _, ok := s.prepareFix(w, r); ok {
		writeJSON(w, plan)
	}
}

func (s *Server) fixApply(w http.ResponseWriter, r *http.Request) {
	plan, body, ok := s.prepareFix(w, r)
	if !ok {
		return
	}
	res, err := fix.Apply(plan, body.Hashes, filepath.Join(s.Paths.ConfigDir, "backups", "dotclaude"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, fix.ErrChanged) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, res)
}

// writeGuard admits a change only from the UI itself. A web page on another
// site can make the browser send a plain form POST to localhost, but it
// cannot add a custom header or a JSON content type without a CORS preflight,
// which this server never answers; its Origin header would give it away too.
func (s *Server) writeGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.ReadOnly {
			http.Error(w, "dotclaude runs read-only", http.StatusForbidden)
			return
		}
		if r.Header.Get("X-Dotclaude") != "1" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "changes are only accepted from the dotclaude UI", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !loopback(u.Hostname()) {
				http.Error(w, "changes are only accepted from the dotclaude UI", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func loopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (s *Server) ui() http.Handler {
	if s.UI == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "The web UI is not part of this build. Run `make build/binary`, or use the API under /api.", http.StatusNotFound)
		})
	}
	files := http.FileServerFS(s.UI)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Client-side routes fall back to the app shell.
		if _, err := fs.Stat(s.UI, strings.TrimPrefix(r.URL.Path, "/")); err != nil || r.URL.Path == "/" {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}

// localOnly rejects requests addressed to any host name other than the
// loopback names. Binding to 127.0.0.1 keeps other machines out; checking the
// Host header also keeps out web pages that rebind their own domain to
// 127.0.0.1 to read the API from the browser.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !loopback(host) {
			http.Error(w, "dotclaude only answers on localhost", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
