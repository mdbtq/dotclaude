package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Config is a Claude Code configuration as it applies in one place: either a
// project directory, or nowhere in particular (the user-level view).
type Config struct {
	// Project is the directory the configuration is resolved for; empty for
	// the user-level view.
	Project string `json:"project,omitempty"`
	// ProjectRoot is the repository root above Project, or Project itself
	// outside a repository.
	ProjectRoot    string          `json:"projectRoot,omitempty"`
	SettingsLayers []SettingsLayer `json:"settingsLayers"`
	Settings       []SettingEntry  `json:"settings"`
	Instructions   []Instruction   `json:"instructions"`
	Components     []Component     `json:"components"`
	Plugins        []Plugin        `json:"plugins"`
	Hooks          []Hook          `json:"hooks"`
	MCPServers     []MCPServer     `json:"mcpServers"`
	Memory         []MemoryStore   `json:"memory"`
	Projects       []Project       `json:"projects"`
	Warnings       []string        `json:"warnings,omitempty"`

	inRepo bool
}

// Project is a directory Claude Code has been started in.
type Project struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	HasMemory bool   `json:"hasMemory"`
}

// Load reads the configuration. With an empty projectDir it returns the
// user-level view: everything that applies regardless of project, plus every
// memory store.
func Load(p Paths, projectDir string) (*Config, error) {
	c := &Config{}
	if projectDir != "" {
		abs, err := filepath.Abs(p.Expand(projectDir))
		if err != nil {
			return nil, err
		}
		if !isDir(abs) {
			return nil, fmt.Errorf("project directory does not exist: %s", abs)
		}
		c.Project = abs
		c.ProjectRoot, c.inRepo = repoRoot(abs)
	}

	g, err := readGlobalConfig(p.GlobalConfig)
	if err != nil {
		c.Warnings = append(c.Warnings, fmt.Sprintf("could not read %s: %v", p.GlobalConfig, err))
	}

	// The user's config dir doubles as the project .claude dir when the
	// project is the home directory; its files then count once, as user files.
	projectClaudeDirs := c.projectClaudeDirs(p)

	// Settings, lowest precedence first.
	// The settings docs list settings.local.json for projects only, but Claude
	// Code also honours ~/.claude/settings.local.json for every project (it is
	// where switching a skill off for all projects lands), so it is read as
	// a second user layer.
	layers := []SettingsLayer{
		readSettings(filepath.Join(p.ConfigDir, "settings.json"), ScopeUser),
		readSettings(filepath.Join(p.ConfigDir, "settings.local.json"), ScopeUser),
	}
	layers = append(layers, c.projectSettings(p)...)
	layers = append(layers, readSettings(filepath.Join(p.ManagedDir, "managed-settings.json"), ScopeManaged))
	for _, l := range layers {
		if l.Error != "" {
			c.Warnings = append(c.Warnings, fmt.Sprintf("%s: %s", l.Path, l.Error))
		}
	}
	merged, entries := mergeSettings(layers)
	c.Settings = entries
	for i := len(layers) - 1; i >= 0; i-- {
		c.SettingsLayers = append(c.SettingsLayers, layers[i])
	}
	if c.Project != "" && c.ProjectRoot != c.Project {
		if root := readSettings(filepath.Join(c.ProjectRoot, ".claude", "settings.json"), ScopeProject); root.Exists {
			root.Values = nil
			root.Ignored = "Claude Code reads .claude/settings.json from the directory it starts in, so this file at the repository root does not apply here"
			c.SettingsLayers = append(c.SettingsLayers, root)
		}
	}

	// Plugins.
	enabled, enabledBy := map[string]bool{}, map[string]Scope{}
	for _, l := range layers {
		obj, _ := l.Values["enabledPlugins"].(map[string]any)
		for id, v := range obj {
			on, _ := v.(bool)
			enabled[id], enabledBy[id] = on, l.Scope
		}
	}
	plugins, contents := loadPlugins(p.ConfigDir, c.Project, enabled, enabledBy, g.PluginUsage)
	c.Plugins = plugins

	// Components.
	user := Source{Scope: ScopeUser}
	c.Components = append(c.Components, scanAgents(filepath.Join(p.ConfigDir, "agents"), user, "")...)
	c.Components = append(c.Components, scanSkills(filepath.Join(p.ConfigDir, "skills"), user, "")...)
	c.Components = append(c.Components, scanCommands(filepath.Join(p.ConfigDir, "commands"), user, "")...)
	c.Components = append(c.Components, scanSkills(filepath.Join(p.ManagedDir, ".claude", "skills"), Source{Scope: ScopeManaged}, "")...)
	project := Source{Scope: ScopeProject}
	for _, dir := range projectClaudeDirs {
		c.Components = append(c.Components, scanAgents(filepath.Join(dir, "agents"), project, "")...)
		c.Components = append(c.Components, scanSkills(filepath.Join(dir, "skills"), project, "")...)
		c.Components = append(c.Components, scanCommands(filepath.Join(dir, "commands"), project, "")...)
	}
	for _, pc := range contents {
		c.Components = append(c.Components, pc.components...)
	}
	for i := range c.Components {
		comp := &c.Components[i]
		if comp.Kind == KindAgent {
			continue
		}
		if u, ok := g.SkillUsage[comp.Name]; ok {
			comp.Usage = u.usage()
		} else if j := strings.LastIndex(comp.Name, ":"); j >= 0 {
			if u, ok := g.SkillUsage[comp.Name[j+1:]]; ok && comp.Source.Scope == ScopePlugin {
				comp.Usage = u.usage()
			}
		}
	}
	resolveComponents(c.Components, stringMap(merged, "skillOverrides"))

	// Hooks: every layer's hooks run, so they are listed rather than merged.
	for i := len(layers) - 1; i >= 0; i-- {
		hooks, err := parseHooks(layers[i].Values["hooks"], Source{Scope: layers[i].Scope}, layers[i].Path)
		if err != nil {
			c.Warnings = append(c.Warnings, fmt.Sprintf("%s: %v", layers[i].Path, err))
		}
		c.Hooks = append(c.Hooks, hooks...)
	}
	for _, pc := range contents {
		c.Hooks = append(c.Hooks, pc.hooks...)
	}
	if off, _ := merged["disableAllHooks"].(bool); off {
		for i := range c.Hooks {
			c.Hooks[i].Status = StatusDisabled
		}
	}

	c.MCPServers = c.loadMCP(p, g, contents)
	c.Instructions = c.loadInstructions(p)
	c.Memory = c.loadMemory(p, g, merged)

	for path := range g.Projects {
		c.Projects = append(c.Projects, Project{
			Path: path, Exists: isDir(path),
			HasMemory: isDir(filepath.Join(p.ConfigDir, "projects", ProjectKey(path), "memory")),
		})
	}
	sort.Slice(c.Projects, func(i, j int) bool { return c.Projects[i].Path < c.Projects[j].Path })
	c.nonNil()
	return c, nil
}

// nonNil replaces nil lists with empty ones, so the JSON always has arrays.
func (c *Config) nonNil() {
	if c.SettingsLayers == nil {
		c.SettingsLayers = []SettingsLayer{}
	}
	if c.Instructions == nil {
		c.Instructions = []Instruction{}
	}
	if c.Components == nil {
		c.Components = []Component{}
	}
	if c.Plugins == nil {
		c.Plugins = []Plugin{}
	}
	if c.Hooks == nil {
		c.Hooks = []Hook{}
	}
	if c.MCPServers == nil {
		c.MCPServers = []MCPServer{}
	}
	if c.Memory == nil {
		c.Memory = []MemoryStore{}
	}
	if c.Projects == nil {
		c.Projects = []Project{}
	}
}

// projectSettings returns the project and local settings layers, lowest
// precedence first, following the settings docs: the shared settings.json
// comes from the start directory; settings.local.json lives at the repository
// root, unless there is no repository or its root is the home directory. A
// settings.local.json that older versions left in the start directory is
// still read, below the root one.
func (c *Config) projectSettings(p Paths) []SettingsLayer {
	if c.Project == "" {
		return nil
	}
	start := filepath.Join(c.Project, ".claude")
	var out []SettingsLayer
	if start != p.ConfigDir {
		out = append(out, readSettings(filepath.Join(start, "settings.json"), ScopeProject))
	}
	localDir := start
	if c.inRepo && c.ProjectRoot != p.Home {
		localDir = filepath.Join(c.ProjectRoot, ".claude")
		if localDir != start {
			if legacy := readSettings(filepath.Join(start, "settings.local.json"), ScopeLocal); legacy.Exists {
				out = append(out, legacy)
			}
		}
	}
	if localDir != p.ConfigDir {
		out = append(out, readSettings(filepath.Join(localDir, "settings.local.json"), ScopeLocal))
	}
	return out
}

// projectClaudeDirs returns the .claude directories from Project up to the
// repository root, closest first, leaving out the user's config dir.
func (c *Config) projectClaudeDirs(p Paths) []string {
	if c.Project == "" {
		return nil
	}
	var dirs []string
	for d := c.Project; ; d = filepath.Dir(d) {
		if dir := filepath.Join(d, ".claude"); dir != p.ConfigDir && isDir(dir) {
			dirs = append(dirs, dir)
		}
		if d == c.ProjectRoot || filepath.Dir(d) == d {
			break
		}
	}
	return dirs
}

// resolveComponents marks components that another definition shadows, and
// skills that settings switch off. Precedence follows the Claude Code docs:
//
//   - agents: project (closest directory first) > user > plugin
//   - skills and commands share one namespace: managed > user > project, and
//     a skill wins over a command with the same name
//
// Plugin components are namespaced, so they rarely collide with anything.
func resolveComponents(comps []Component, overrides map[string]string) {
	rank := func(c Component) int {
		order := map[Scope]int{ScopeManaged: 0, ScopeUser: 1, ScopeProject: 2, ScopePlugin: 3}
		if c.Kind == KindAgent {
			order = map[Scope]int{ScopeManaged: 0, ScopeProject: 1, ScopeUser: 2, ScopePlugin: 3}
		}
		r := order[c.Source.Scope] * 2
		if c.Kind == KindCommand {
			r++
		}
		return r
	}
	type key struct {
		agent bool
		name  string
	}
	groups := map[key][]int{}
	for i, c := range comps {
		if c.Error != "" {
			continue
		}
		k := key{c.Kind == KindAgent, c.Name}
		groups[k] = append(groups[k], i)
	}
	for _, idx := range groups {
		// Stable: within one scope, project dirs were scanned closest first.
		sort.SliceStable(idx, func(a, b int) bool { return rank(comps[idx[a]]) < rank(comps[idx[b]]) })
		winner := comps[idx[0]]
		for _, i := range idx[1:] {
			if comps[i].Status == StatusActive {
				comps[i].Status = StatusShadowed
				comps[i].StatusNote = fmt.Sprintf("shadowed by the %s %s at %s", winner.Source.Scope, winner.Kind, winner.Path)
			}
		}
	}
	for i := range comps {
		c := &comps[i]
		if c.Kind != KindAgent && c.Status == StatusActive && overrides[c.Name] == "off" {
			c.Status = StatusDisabled
			c.StatusNote = "switched off in skillOverrides"
		}
	}
}

func (c *Config) loadMCP(p Paths, g globalConfig, contents []pluginContents) []MCPServer {
	var out []MCPServer
	out = append(out, readMCPFile(filepath.Join(p.ManagedDir, "managed-mcp.json"), Source{Scope: ScopeManaged})...)
	if c.Project != "" {
		state, ok := g.Projects[c.Project]
		if !ok {
			state = g.Projects[c.ProjectRoot]
		}
		out = append(out, parseServers(state.MCPServers, Source{Scope: ScopeLocal}, p.GlobalConfig)...)
		shared := readMCPFile(filepath.Join(c.ProjectRoot, ".mcp.json"), Source{Scope: ScopeProject})
		for i := range shared {
			s := &shared[i]
			switch {
			case slices.Contains(state.DisabledMcpjsonServers, s.Name):
				s.Status, s.StatusNote = StatusDisabled, "rejected for this project"
			case !slices.Contains(state.EnabledMcpjsonServers, s.Name):
				s.StatusNote = "not yet approved for this project, unless all project servers are"
			}
		}
		out = append(out, shared...)
	}
	out = append(out, parseServers(g.MCPServers, Source{Scope: ScopeUser}, p.GlobalConfig)...)
	// Same name: managed > local > project > user, the order appended above.
	seen := map[string]MCPServer{}
	for i := range out {
		if w, ok := seen[out[i].Name]; ok && out[i].Status == StatusActive {
			out[i].Status = StatusShadowed
			out[i].StatusNote = fmt.Sprintf("shadowed by the %s server of the same name", w.Source.Scope)
		} else if !ok {
			seen[out[i].Name] = out[i]
		}
	}
	for _, pc := range contents {
		out = append(out, pc.servers...)
	}
	return out
}

func (c *Config) loadInstructions(p Paths) []Instruction {
	var out []Instruction
	skip := map[string]bool{}
	add := func(path string, scope Scope, kind string) {
		skip[path] = true
		if isFile(path) {
			out = append(out, readInstruction(path, scope, kind, "", 0, map[string]bool{}))
		}
	}
	add(filepath.Join(p.ManagedDir, "CLAUDE.md"), ScopeManaged, "claude-md")
	add(filepath.Join(p.ConfigDir, "CLAUDE.md"), ScopeUser, "claude-md")
	walkMarkdown(filepath.Join(p.ConfigDir, "rules"), func(path, _ string) {
		add(path, ScopeUser, "rule")
	})
	if c.Project != "" {
		out = append(out, projectInstructions(c.Project, c.ProjectRoot, skip)...)
	}
	return out
}

func (c *Config) loadMemory(p Paths, g globalConfig, merged map[string]any) []MemoryStore {
	var out []MemoryStore
	add := func(s *MemoryStore) {
		if s != nil {
			out = append(out, *s)
		}
	}
	for _, dir := range p.MemoryDirs {
		dir = p.Expand(dir)
		add(readMemoryStore(dir, tildePath(p, dir), "extra"))
	}
	if dir, _ := merged["autoMemoryDirectory"].(string); dir != "" {
		dir = p.Expand(dir)
		add(readMemoryStore(dir, tildePath(p, dir), "configured"))
		return out
	}

	labels := map[string]string{}
	for path := range g.Projects {
		labels[ProjectKey(path)] = path
	}
	if c.Project != "" {
		key := ProjectKey(c.ProjectRoot)
		if s := readMemoryStore(filepath.Join(p.ConfigDir, "projects", key, "memory"), c.ProjectRoot, "auto"); s != nil {
			s.Project = c.ProjectRoot
			add(s)
		}
		return out
	}
	dirs, _ := filepath.Glob(filepath.Join(p.ConfigDir, "projects", "*", "memory"))
	sort.Strings(dirs)
	for _, dir := range dirs {
		key := filepath.Base(filepath.Dir(dir))
		label, project := key, labels[key]
		if project != "" {
			label = project
		}
		s := readMemoryStore(dir, label, "auto")
		if s != nil {
			s.Project = project
		}
		add(s)
	}
	return out
}

// repoRoot walks up from dir to the nearest directory holding .git. Outside
// a repository it returns dir itself and false.
func repoRoot(dir string) (string, bool) {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d, true
		}
		if filepath.Dir(d) == d {
			return dir, false
		}
	}
}

func tildePath(p Paths, path string) string {
	if rel, err := filepath.Rel(p.Home, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.Join("~", rel)
	}
	return path
}
