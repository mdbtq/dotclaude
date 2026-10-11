package claude

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind is the type of a component Claude Code loads from markdown files.
type Kind string

const (
	KindAgent   Kind = "agent"
	KindSkill   Kind = "skill"
	KindCommand Kind = "command"
)

// Scope is where a piece of configuration comes from.
type Scope string

const (
	ScopeManaged Scope = "managed"
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
	ScopeLocal   Scope = "local"
	ScopePlugin  Scope = "plugin"
)

// Status says whether a component is in effect.
type Status string

const (
	StatusActive   Status = "active"
	StatusShadowed Status = "shadowed"
	StatusDisabled Status = "disabled"
)

// Source identifies the layer a component or setting was read from.
type Source struct {
	Scope Scope `json:"scope"`
	// Plugin is the plugin id (name@marketplace) for plugin components.
	Plugin string `json:"plugin,omitempty"`
}

// Component is an agent, skill or command definition.
type Component struct {
	Kind Kind `json:"kind"`
	// Name is the identifier the component is invoked by, including the
	// plugin namespace for plugin components.
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Source           Source         `json:"source"`
	Path             string         `json:"path"`
	Frontmatter      map[string]any `json:"frontmatter,omitempty"`
	FrontmatterError string         `json:"frontmatterError,omitempty"`
	// Error is set when the definition could not be read at all, e.g. a
	// skill directory that is a dangling symlink.
	Error      string `json:"error,omitempty"`
	Usage      *Usage `json:"usage,omitempty"`
	Status     Status `json:"status"`
	StatusNote string `json:"statusNote,omitempty"`
}

// scanAgents reads every agent definition under dir, recursively. Outside a
// plugin the subdirectory does not affect the name; inside a plugin it becomes
// part of the namespace (agents/review/security.md → plugin:review:security).
func scanAgents(dir string, src Source, pluginName string) []Component {
	var out []Component
	walkMarkdown(dir, func(path, rel string) {
		c := readComponent(KindAgent, path, src)
		stem := strings.TrimSuffix(filepath.Base(rel), ".md")
		if c.Name == "" {
			c.Name = stem
		}
		if pluginName != "" {
			parts := append([]string{pluginName}, splitDir(rel)...)
			c.Name = strings.Join(append(parts, c.Name), ":")
		}
		out = append(out, c)
	})
	return out
}

// scanCommands reads every command file under dir. Subdirectories map to
// colon-separated names: commands/frontend/component.md → frontend:component.
func scanCommands(dir string, src Source, pluginName string) []Component {
	var out []Component
	walkMarkdown(dir, func(path, rel string) {
		c := readComponent(KindCommand, path, src)
		parts := splitDir(rel)
		if pluginName != "" {
			parts = append([]string{pluginName}, parts...)
		}
		c.Name = strings.Join(append(parts, strings.TrimSuffix(filepath.Base(rel), ".md")), ":")
		out = append(out, c)
	})
	return out
}

// scanSkills reads <dir>/<skill>/SKILL.md for every entry in dir, following
// symlinks. A skill named in front matter keeps that name; otherwise the
// directory name is used.
func scanSkills(dir string, src Source, pluginName string) []Component {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Component
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		skillDir := filepath.Join(dir, e.Name())
		path := filepath.Join(skillDir, "SKILL.md")
		if e.Type()&fs.ModeSymlink != 0 && !isDir(skillDir) {
			target, _ := os.Readlink(skillDir)
			out = append(out, Component{
				Kind: KindSkill, Name: qualify(pluginName, e.Name()), Source: src, Path: skillDir,
				Error: "dangling symlink to " + target, Status: StatusActive,
			})
			continue
		}
		if !isDir(skillDir) || !isFile(path) {
			continue
		}
		c := readComponent(KindSkill, path, src)
		if c.Name == "" {
			c.Name = e.Name()
		}
		c.Name = qualify(pluginName, c.Name)
		out = append(out, c)
	}
	return out
}

func readComponent(kind Kind, path string, src Source) Component {
	c := Component{Kind: kind, Source: src, Path: path, Status: StatusActive}
	doc, err := ReadDocument(path)
	if err != nil {
		c.Error = err.Error()
		return c
	}
	c.Name = doc.String("name")
	c.Description = doc.String("description")
	if when := doc.String("when_to_use"); when != "" && kind != KindAgent {
		c.Description = strings.TrimSpace(c.Description + " " + when)
	}
	c.Frontmatter = doc.Frontmatter
	c.FrontmatterError = doc.FrontmatterError
	return c
}

// walkMarkdown calls fn for every .md file below dir, following a symlinked
// root but not symlinks inside it, in lexical order.
func walkMarkdown(dir string, fn func(path, rel string)) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil || !isDir(root) {
		return
	}
	var files []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != root && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	for _, path := range files {
		rel, _ := filepath.Rel(root, path)
		fn(filepath.Join(dir, rel), rel)
	}
}

func splitDir(rel string) []string {
	d := filepath.Dir(rel)
	if d == "." {
		return nil
	}
	return strings.Split(filepath.ToSlash(d), "/")
}

func qualify(pluginName, name string) string {
	if pluginName == "" {
		return name
	}
	return pluginName + ":" + name
}
