// Package claude reads a Claude Code configuration from disk: the user's
// config directory, the global config file, installed plugins, and the files a
// project contributes. It never writes.
package claude

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Paths locates the files dotclaude reads. Every field can be overridden, so a
// setup that keeps its files outside the defaults still resolves.
type Paths struct {
	// Home is the user's home directory; "~" in configured paths expands to it.
	Home string
	// ConfigDir is the Claude Code config directory, ~/.claude by default.
	ConfigDir string
	// GlobalConfig is the file Claude Code writes for itself, ~/.claude.json
	// by default. It holds user and local MCP servers and usage counters.
	GlobalConfig string
	// ManagedDir holds organisation-managed settings and CLAUDE.md.
	ManagedDir string
	// MemoryDirs are extra memory directories to show next to the per-project
	// auto memory, e.g. a global memory directory a hook loads.
	MemoryDirs []string
}

// DefaultPaths returns the standard locations for the given home directory
// and config dir; an empty configDir means ~/.claude. A memory directory
// directly under the config dir is included when present: it is a common
// place for memory shared across all projects.
func DefaultPaths(home, configDir string) Paths {
	if configDir == "" {
		configDir = filepath.Join(home, ".claude")
	}
	p := Paths{
		Home:         home,
		ConfigDir:    configDir,
		GlobalConfig: filepath.Join(home, ".claude.json"),
		ManagedDir:   "/Library/Application Support/ClaudeCode",
	}
	if isDir(filepath.Join(p.ConfigDir, "memory")) {
		p.MemoryDirs = []string{filepath.Join(p.ConfigDir, "memory")}
	}
	return p
}

// Expand resolves a leading "~" against Home.
func (p Paths) Expand(path string) string {
	if path == "~" {
		return p.Home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(p.Home, path[2:])
	}
	return path
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// ProjectKey is the directory name Claude Code uses under ~/.claude/projects
// for a project path: every character other than a letter or digit becomes a
// dash. The mapping is lossy, so dotclaude maps keys back through the known
// project paths rather than decoding them.
func ProjectKey(path string) string {
	return nonAlnum.ReplaceAllString(path, "-")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
