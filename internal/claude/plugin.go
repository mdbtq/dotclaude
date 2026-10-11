package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Plugin is an installed or enabled plugin.
type Plugin struct {
	// ID is name@marketplace, the key used in enabledPlugins.
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Marketplace string    `json:"marketplace"`
	Version     string    `json:"version,omitempty"`
	Description string    `json:"description,omitempty"`
	Author      string    `json:"author,omitempty"`
	InstallPath string    `json:"installPath,omitempty"`
	InstallKind string    `json:"installScope,omitempty"`
	InstalledAt time.Time `json:"installedAt,omitzero"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero"`
	Installed   bool      `json:"installed"`
	Enabled     bool      `json:"enabled"`
	// EnabledBy is the settings layer that decided Enabled.
	EnabledBy Scope          `json:"enabledBy,omitempty"`
	Usage     *Usage         `json:"usage,omitempty"`
	Counts    map[string]int `json:"counts"`
	Error     string         `json:"error,omitempty"`
}

type installedPlugins struct {
	Plugins map[string][]struct {
		Scope       string    `json:"scope"`
		InstallPath string    `json:"installPath"`
		Version     string    `json:"version"`
		InstalledAt time.Time `json:"installedAt"`
		LastUpdated time.Time `json:"lastUpdated"`
		ProjectPath string    `json:"projectPath"`
	} `json:"plugins"`
}

type pluginManifest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Author      json.RawMessage `json:"author"`
	Agents      json.RawMessage `json:"agents"`
	Skills      json.RawMessage `json:"skills"`
	Commands    json.RawMessage `json:"commands"`
	Hooks       json.RawMessage `json:"hooks"`
	MCPServers  json.RawMessage `json:"mcpServers"`
}

// pluginContents is everything a plugin contributes.
type pluginContents struct {
	components []Component
	hooks      []Hook
	servers    []MCPServer
}

// loadPlugins lists every plugin that is installed or mentioned in
// enabledPlugins, and reads the contents of the installed ones. Plugins
// installed for a single project are only listed for that project.
func loadPlugins(configDir, projectDir string, enabled map[string]bool, enabledBy map[string]Scope, usage map[string]rawUsage) ([]Plugin, []pluginContents) {
	var reg installedPlugins
	if data, err := os.ReadFile(filepath.Join(configDir, "plugins", "installed_plugins.json")); err == nil {
		_ = json.Unmarshal(data, &reg)
	}

	ids := map[string]bool{}
	for id := range reg.Plugins {
		ids[id] = true
	}
	for id := range enabled {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)

	var plugins []Plugin
	var contents []pluginContents
	for _, id := range sorted {
		name, market, _ := strings.Cut(id, "@")
		p := Plugin{ID: id, Name: name, Marketplace: market, Enabled: enabled[id], EnabledBy: enabledBy[id], Counts: map[string]int{}}
		if u, ok := usage[id]; ok {
			p.Usage = u.usage()
		}
		entries := reg.Plugins[id]
		for _, e := range entries {
			if e.ProjectPath != "" && e.ProjectPath != projectDir {
				continue
			}
			p.Installed = true
			p.InstallPath, p.Version, p.InstallKind = e.InstallPath, e.Version, e.Scope
			p.InstalledAt, p.UpdatedAt = e.InstalledAt, e.LastUpdated
			break
		}
		if !p.Installed && !p.Enabled {
			continue // installed for another project only
		}
		if !p.Installed || !isDir(p.InstallPath) {
			if dir := marketplacePluginDir(configDir, market, name); dir != "" {
				p.Installed, p.InstallPath = true, dir
			}
		}
		var c pluginContents
		if p.Installed {
			c = readPlugin(&p)
		} else {
			p.Error = "enabled in settings but not installed"
		}
		plugins = append(plugins, p)
		contents = append(contents, c)
	}
	return plugins, contents
}

func readPlugin(p *Plugin) pluginContents {
	root := p.InstallPath
	if !isDir(root) {
		p.Error = "install path does not exist: " + root
		return pluginContents{}
	}
	var m pluginManifest
	if data, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json")); err == nil {
		if err := json.Unmarshal(data, &m); err != nil {
			p.Error = "invalid plugin.json: " + err.Error()
		}
	}
	if m.Name != "" {
		p.Name = m.Name
	}
	p.Description = m.Description
	if p.Version == "" {
		p.Version = m.Version
	}
	p.Author = authorName(m.Author)

	src := Source{Scope: ScopePlugin, Plugin: p.ID}
	status := StatusActive
	if !p.Enabled {
		status = StatusDisabled
	}
	var c pluginContents
	for _, dir := range componentDirs(root, "agents", m.Agents) {
		c.components = append(c.components, scanAgents(dir, src, p.Name)...)
	}
	for _, dir := range componentDirs(root, "skills", m.Skills) {
		c.components = append(c.components, scanSkills(dir, src, p.Name)...)
	}
	for _, dir := range componentDirs(root, "commands", m.Commands) {
		c.components = append(c.components, scanCommands(dir, src, p.Name)...)
	}
	if hooks, err := readHooksFile(filepath.Join(root, "hooks", "hooks.json"), src); err == nil {
		c.hooks = hooks
	}
	c.servers = readMCPFile(filepath.Join(root, ".mcp.json"), src)
	if len(m.MCPServers) > 0 && m.MCPServers[0] == '{' {
		var inline map[string]json.RawMessage
		if json.Unmarshal(m.MCPServers, &inline) == nil {
			c.servers = append(c.servers, parseServers(inline, src, filepath.Join(root, ".claude-plugin", "plugin.json"))...)
		}
	}

	for i := range c.components {
		c.components[i].Status = status
		p.Counts[string(c.components[i].Kind)+"s"]++
	}
	for i := range c.hooks {
		c.hooks[i].Status = status
		c.hooks[i].PluginRoot = root
	}
	for i := range c.servers {
		c.servers[i].Status = status
	}
	p.Counts["hooks"] = len(c.hooks)
	p.Counts["mcpServers"] = len(c.servers)
	if !p.Enabled {
		note := "plugin " + p.ID + " is not enabled"
		for i := range c.components {
			c.components[i].StatusNote = note
		}
		for i := range c.servers {
			c.servers[i].StatusNote = note
		}
	}
	return c
}

// marketplacePluginDir finds a plugin inside a directory-based marketplace,
// where Claude Code loads it from the marketplace checkout instead of the
// plugin cache. It returns "" unless the marketplace lists the plugin with a
// relative source directory that exists.
func marketplacePluginDir(configDir, market, name string) string {
	var known map[string]struct {
		InstallLocation string `json:"installLocation"`
	}
	data, err := os.ReadFile(filepath.Join(configDir, "plugins", "known_marketplaces.json"))
	if err != nil || json.Unmarshal(data, &known) != nil {
		return ""
	}
	loc := known[market].InstallLocation
	if loc == "" {
		return ""
	}
	var manifest struct {
		Plugins []struct {
			Name   string          `json:"name"`
			Source json.RawMessage `json:"source"`
		} `json:"plugins"`
	}
	data, err = os.ReadFile(filepath.Join(loc, ".claude-plugin", "marketplace.json"))
	if err != nil || json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	for _, pl := range manifest.Plugins {
		var source string
		if pl.Name != name || json.Unmarshal(pl.Source, &source) != nil || !strings.HasPrefix(source, "./") {
			continue
		}
		if dir := filepath.Join(loc, source); isDir(dir) {
			return dir
		}
	}
	return ""
}

// componentDirs returns the default component directory plus any custom
// paths the manifest declares, which supplement the default.
func componentDirs(root, name string, custom json.RawMessage) []string {
	dirs := []string{filepath.Join(root, name)}
	var paths []string
	var one string
	if json.Unmarshal(custom, &one) == nil && one != "" {
		paths = []string{one}
	} else {
		_ = json.Unmarshal(custom, &paths)
	}
	for _, p := range paths {
		dir := filepath.Clean(filepath.Join(root, p))
		if dir != dirs[0] && isDir(dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func authorName(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &obj)
	return obj.Name
}
