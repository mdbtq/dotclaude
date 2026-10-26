// Package fix turns a requested change to a Claude Code configuration into a
// previewable plan, and applies that plan only when the files it touches have
// not changed since the preview. Every file is backed up before it is changed.
package fix

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mdbtq/dotclaude/internal/claude"
)

// Kinds of change.
const (
	// SkillOverride switches a skill off (Value "off") or back on (Value "on")
	// through skillOverrides. Target is the skill name.
	SkillOverride = "skill-override"
	// PluginEnabled sets enabledPlugins for a plugin (Value "true" or
	// "false"). Target is the plugin id.
	PluginEnabled = "plugin-enabled"
	// MemoryIndexAdd lists a memory file in its store's MEMORY.md. Target is
	// the memory file.
	MemoryIndexAdd = "memory-index-add"
	// MemoryIndexRemove drops the MEMORY.md line at Line that links to a file
	// that does not exist. Target is the MEMORY.md path.
	MemoryIndexRemove = "memory-index-remove"
	// RemoveSymlink deletes a dangling skill symlink. Target is the link.
	RemoveSymlink = "remove-symlink"
)

// Request asks for one change.
type Request struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Value  string `json:"value,omitempty"`
	Line   int    `json:"line,omitempty"`
	// Label is a short description of the change for buttons and menus.
	Label string `json:"label,omitempty"`
}

// ErrNotApplicable means the request no longer matches the configuration,
// e.g. the memory was indexed in the meantime.
var ErrNotApplicable = errors.New("not applicable")

// ErrChanged means a file changed between preview and apply.
var ErrChanged = errors.New("changed since the preview")

// Change is the effect on one file.
type Change struct {
	Path string `json:"path"`
	// Op is "write" or "remove".
	Op   string `json:"op"`
	Diff string `json:"diff"`
	// Hash identifies the file's current state, to detect changes between
	// preview and apply.
	Hash   string `json:"hash"`
	before string
	after  string
	exists bool
	// linkTarget is set for symlink removals.
	linkTarget string
}

// Plan is a previewable set of changes, for one request or several.
type Plan struct {
	Requests []Request `json:"requests"`
	Title    string    `json:"title"`
	Changes  []Change  `json:"changes"`
	// Skipped lists requests that no longer apply, with the reason. The rest
	// of the batch goes ahead without them.
	Skipped []Skip `json:"skipped,omitempty"`
}

// Skip is a request left out of a plan.
type Skip struct {
	Request Request `json:"request"`
	Reason  string  `json:"reason"`
}

// Result reports what Apply did.
type Result struct {
	Changed []string `json:"changed"`
	// Backups are the copies taken before changing each file.
	Backups []string `json:"backups"`
}

// Prepare checks requests against the configuration and works out the
// changes they need, as one plan: requests that touch the same file build on
// each other and end up as one change to it, with one diff and one backup.
// A request that no longer applies is skipped unless it is the only one.
// Prepare touches nothing.
func Prepare(c *claude.Config, p claude.Paths, requests []Request) (*Plan, error) {
	requests = order(dedupe(requests))
	plan := &Plan{Requests: requests}
	ws := workspace{}
	var touched []string
	removed := map[string]bool{}
	for _, r := range requests {
		var (
			part *Plan
			err  error
		)
		switch r.Kind {
		case SkillOverride:
			part, err = skillOverride(ws, c, p, r)
		case PluginEnabled:
			part, err = pluginEnabled(ws, c, p, r)
		case MemoryIndexAdd:
			part, err = memoryIndexAdd(ws, c, r)
		case MemoryIndexRemove:
			part, err = memoryIndexRemove(ws, c, r)
		case RemoveSymlink:
			part, err = removeSymlink(c, r)
		default:
			err = fmt.Errorf("unknown kind %q", r.Kind)
		}
		if err != nil {
			if len(requests) > 1 && errors.Is(err, ErrNotApplicable) {
				plan.Skipped = append(plan.Skipped, Skip{Request: r, Reason: err.Error()})
				continue
			}
			return nil, err
		}
		plan.Title = part.Title
		for _, ch := range part.Changes {
			if ch.Op == "remove" {
				if !removed[ch.Path] {
					removed[ch.Path] = true
					plan.Changes = append(plan.Changes, ch)
				}
				continue
			}
			if ws[ch.Path] == nil {
				touched = append(touched, ch.Path)
			}
			ws.record(ch)
		}
	}
	for _, path := range touched {
		ch := *ws[path]
		ch.Diff = unifiedDiff(ch.before, ch.after, 3)
		plan.Changes = append(plan.Changes, ch)
	}
	if len(plan.Changes) == 0 {
		return nil, fmt.Errorf("none of the %d changes applies any more: %w", len(requests), ErrNotApplicable)
	}
	if applied := len(requests) - len(plan.Skipped); applied > 1 {
		plan.Title = fmt.Sprintf("%d changes in %d %s", applied, len(plan.Changes), plural(len(plan.Changes), "file", "files"))
	}
	return plan, nil
}

// dedupe drops repeated requests, which would otherwise apply twice.
func dedupe(requests []Request) []Request {
	seen := map[Request]bool{}
	var out []Request
	for _, r := range requests {
		key := r
		key.Label = ""
		if !seen[key] {
			seen[key] = true
			out = append(out, r)
		}
	}
	return out
}

// order puts index-line removals last and bottom-up, so removing one line
// never shifts the line number of another.
func order(requests []Request) []Request {
	sort.SliceStable(requests, func(i, j int) bool {
		a, b := requests[i], requests[j]
		if (a.Kind == MemoryIndexRemove) != (b.Kind == MemoryIndexRemove) {
			return b.Kind == MemoryIndexRemove
		}
		return a.Kind == MemoryIndexRemove && a.Line > b.Line
	})
	return requests
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// workspace holds the files a batch has changed so far, so each request
// builds on the ones before it.
type workspace map[string]*Change

// read returns a file to change: its content so far in this batch, or from
// disk. Symlinks are resolved, so the write lands in the file the link points
// to and the link survives.
func (w workspace) read(path string) (Change, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return Change{}, fmt.Errorf("%s is a symlink to a missing file; dotclaude will not replace the link: %w", path, ErrNotApplicable)
		}
		path = real
	}
	if ch, ok := w[path]; ok {
		next := *ch
		next.before = ch.after
		return next, nil
	}
	return readFile(path)
}

// record keeps a change, preserving the original content and hash of a file
// an earlier request in the batch already changed.
func (w workspace) record(ch Change) {
	if prev, ok := w[ch.Path]; ok {
		prev.after = ch.after
		return
	}
	w[ch.Path] = &ch
}

// Apply carries out a plan. hashes maps each path to the hash the preview
// showed; when a file no longer matches, nothing is changed. Each file is
// copied into backupDir first.
func Apply(plan *Plan, hashes map[string]string, backupDir string) (Result, error) {
	var res Result
	for _, ch := range plan.Changes {
		if hashes[ch.Path] != ch.Hash {
			return res, fmt.Errorf("%s: %w", ch.Path, ErrChanged)
		}
	}
	stamp := time.Now().Format("20060102-150405.000000")
	for _, ch := range plan.Changes {
		// Check again right before writing: the preview may be minutes old.
		if current, err := currentHash(ch); err != nil || current != ch.Hash {
			return res, fmt.Errorf("%s: %w", ch.Path, ErrChanged)
		}
		backup, err := backupFile(ch, backupDir, stamp)
		if err != nil {
			return res, fmt.Errorf("backing up %s: %w", ch.Path, err)
		}
		if backup != "" {
			res.Backups = append(res.Backups, backup)
		}
		switch ch.Op {
		case "remove":
			err = os.Remove(ch.Path)
		default:
			err = writeFile(ch.Path, ch.after)
		}
		if err != nil {
			return res, fmt.Errorf("changing %s: %w", ch.Path, err)
		}
		res.Changed = append(res.Changed, ch.Path)
	}
	return res, nil
}

// readFile reads a file from disk as the starting point of a change.
func readFile(path string) (Change, error) {
	ch := Change{Path: path, Op: "write"}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		ch.before, ch.exists = string(data), true
	case errors.Is(err, fs.ErrNotExist):
	default:
		return ch, err
	}
	ch.Hash = hash(ch.before, ch.exists)
	return ch, nil
}

// currentHash hashes what is on disk now, the same way the plan did.
func currentHash(ch Change) (string, error) {
	if ch.Op == "remove" {
		target, err := os.Readlink(ch.Path)
		if err != nil {
			return "", err
		}
		return hash(target, true), nil
	}
	data, err := os.ReadFile(ch.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return hash("", false), nil
	}
	if err != nil {
		return "", err
	}
	return hash(string(data), true), nil
}

func hash(content string, exists bool) string {
	if !exists {
		return "absent"
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:8])
}

// writeFile replaces path atomically, keeping its permissions.
func writeFile(path, content string) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".dotclaude-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// backupFile keeps the current state of a file before it changes: a copy of
// the file, or for a symlink the command that recreates it. A file that does
// not exist yet needs no backup.
func backupFile(ch Change, dir, stamp string) (string, error) {
	if ch.Op == "write" && !ch.exists {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := stamp + "-" + strings.Trim(strings.ReplaceAll(ch.Path, string(filepath.Separator), "_"), "_")
	if ch.Op == "remove" {
		path := filepath.Join(dir, name+".restore.sh")
		script := fmt.Sprintf("#!/bin/sh\nln -s %s %s\n", shellQuote(ch.linkTarget), shellQuote(ch.Path))
		return path, os.WriteFile(path, []byte(script), 0o600)
	}
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, []byte(ch.before), 0o600)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
