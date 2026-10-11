// Package lint finds problems in a Claude Code configuration: broken
// references, drift between memory files and their index, definitions that
// shadow each other, and skills that compete for the same trigger.
package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mdbtq/dotclaude/internal/claude"
)

// Severity orders findings by how much they matter.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

// Finding is one problem.
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Path     string   `json:"path,omitempty"`
	Line     int      `json:"line,omitempty"`
	// Related is a second file involved, e.g. the other half of an overlap.
	Related string `json:"related,omitempty"`
}

// Rule describes a check, for documentation and the UI.
type Rule struct {
	ID          string   `json:"id"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
}

// Rules lists every check Run performs.
var Rules = []Rule{
	{"component/unreadable", Error, "An agent, skill or command could not be read, e.g. a skill directory that is a dangling symlink."},
	{"component/invalid-frontmatter", Info, "Front matter is not valid YAML. Claude Code reads it leniently, but other tools that parse it may not."},
	{"component/missing-name", Error, "An agent has no name in its front matter. The name is required and is the agent's identity."},
	{"component/missing-description", Warning, "An agent or skill has no description, so Claude cannot tell when to use it."},
	{"component/shadowed", Info, "A definition is hidden by another one with the same name in a higher-precedence location."},
	{"skill/trigger-overlap", Info, "Two active skills have very similar descriptions and may compete for the same requests."},
	{"skill/never-used", Info, "No use of the skill is recorded in Claude Code's usage counters."},
	{"instructions/import-missing", Warning, "An @import in an instruction file points at a file that does not exist."},
	{"instructions/too-long", Info, "An instruction file is over 200 lines; the docs advise keeping them shorter."},
	{"memory/index-dangling", Error, "MEMORY.md links to a memory file that does not exist."},
	{"memory/not-indexed", Warning, "A memory file is not listed in MEMORY.md, so it is never recalled."},
	{"memory/index-over-limit", Warning, "MEMORY.md is over 200 lines or 25KB; the rest is not loaded."},
	{"memory/missing-frontmatter", Warning, "A memory file lacks a name, description or type."},
	{"memory/unresolved-link", Info, "A [[link]] names a memory that does not exist yet."},
	{"memory/similar", Info, "Two memories have very similar descriptions and may be near-duplicates."},
	{"settings/not-read", Info, "A settings file exists but does not apply where Claude Code starts, e.g. a repository-root settings.json when starting in a subdirectory."},
	{"plugin/not-installed", Error, "A plugin is enabled in settings but is not installed, or its install path is gone."},
	{"hook/command-missing", Warning, "A hook runs an executable path that does not exist."},
}

// Options tunes the checks.
type Options struct {
	// SimilarityThreshold is the word-overlap score (0–1) above which two
	// descriptions count as similar.
	SimilarityThreshold float64
}

// DefaultOptions are the thresholds used when none are given.
var DefaultOptions = Options{SimilarityThreshold: 0.5}

// Run checks a configuration.
func Run(c *claude.Config, opt Options) []Finding {
	var out []Finding
	out = append(out, components(c, opt)...)
	out = append(out, instructions(c.Instructions)...)
	out = append(out, memory(c.Memory, opt)...)
	out = append(out, plugins(c.Plugins)...)
	for _, l := range c.SettingsLayers {
		if l.Ignored != "" {
			out = append(out, Finding{Rule: "settings/not-read", Severity: Info, Path: l.Path, Message: l.Ignored})
		}
	}
	out = append(out, hooks(c.Hooks)...)
	order := map[Severity]int{Error: 0, Warning: 1, Info: 2}
	sort.SliceStable(out, func(i, j int) bool {
		if order[out[i].Severity] != order[out[j].Severity] {
			return order[out[i].Severity] < order[out[j].Severity]
		}
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func components(c *claude.Config, opt Options) []Finding {
	var out []Finding
	var skills []claude.Component
	for _, comp := range c.Components {
		label := fmt.Sprintf("%s %q", comp.Kind, comp.Name)
		if comp.Error != "" {
			out = append(out, Finding{Rule: "component/unreadable", Severity: Error, Path: comp.Path, Message: fmt.Sprintf("%s: %s", label, comp.Error)})
			continue
		}
		if comp.FrontmatterError != "" {
			out = append(out, Finding{Rule: "component/invalid-frontmatter", Severity: Info, Path: comp.Path, Message: fmt.Sprintf("%s: %s", label, comp.FrontmatterError)})
		}
		if comp.Kind == claude.KindAgent && comp.Frontmatter["name"] == nil {
			out = append(out, Finding{Rule: "component/missing-name", Severity: Error, Path: comp.Path, Message: label + " has no name in its front matter"})
		}
		if comp.Kind != claude.KindCommand && comp.Description == "" {
			out = append(out, Finding{Rule: "component/missing-description", Severity: Warning, Path: comp.Path, Message: label + " has no description"})
		}
		if comp.Status == claude.StatusShadowed {
			out = append(out, Finding{Rule: "component/shadowed", Severity: Info, Path: comp.Path, Message: fmt.Sprintf("%s is %s", label, comp.StatusNote)})
		}
		if comp.Kind == claude.KindSkill && comp.Status == claude.StatusActive {
			skills = append(skills, comp)
			if comp.Usage == nil && comp.Source.Scope != claude.ScopePlugin {
				out = append(out, Finding{Rule: "skill/never-used", Severity: Info, Path: comp.Path, Message: label + " has no recorded use"})
			}
		}
	}

	sets := make([]wordSet, len(skills))
	for i, s := range skills {
		sets[i] = words(s.Description)
	}
	for i := range skills {
		for j := i + 1; j < len(skills); j++ {
			if score := similarity(sets[i], sets[j]); score >= opt.SimilarityThreshold {
				out = append(out, Finding{
					Rule: "skill/trigger-overlap", Severity: Info, Path: skills[i].Path, Related: skills[j].Path,
					Message: fmt.Sprintf("skills %q and %q have similar descriptions (%.0f%% overlap)", skills[i].Name, skills[j].Name, score*100),
				})
			}
		}
	}
	return out
}

func instructions(list []claude.Instruction) []Finding {
	var out []Finding
	var walk func(in claude.Instruction, parent string)
	walk = func(in claude.Instruction, parent string) {
		if in.Kind == "import" && !in.Exists {
			out = append(out, Finding{Rule: "instructions/import-missing", Severity: Warning, Path: parent, Line: in.Line, Related: in.Path, Message: fmt.Sprintf("%s does not resolve to a file", in.Import)})
			return
		}
		if in.Kind != "import" && in.Lines > 200 {
			out = append(out, Finding{Rule: "instructions/too-long", Severity: Info, Path: in.Path, Message: fmt.Sprintf("%d lines", in.Lines)})
		}
		for _, child := range in.Imports {
			walk(child, in.Path)
		}
	}
	for _, in := range list {
		walk(in, "")
	}
	return out
}

func memory(stores []claude.MemoryStore, opt Options) []Finding {
	var out []Finding
	for _, s := range stores {
		keys := map[string]bool{}
		for _, m := range s.Entries {
			keys[m.Key()] = true
			keys[strings.TrimSuffix(m.File, ".md")] = true
		}
		indexed := map[string]bool{}
		if s.Index != nil {
			for _, l := range s.Index.Links {
				indexed[filepath.Clean(l.File)] = true
				if !l.Exists {
					out = append(out, Finding{Rule: "memory/index-dangling", Severity: Error, Path: s.Index.Path, Line: l.Line, Message: fmt.Sprintf("links to %s, which does not exist", l.File)})
				}
			}
			if s.Index.Lines > 200 || s.Index.Bytes > 25*1024 {
				out = append(out, Finding{Rule: "memory/index-over-limit", Severity: Warning, Path: s.Index.Path, Message: fmt.Sprintf("%d lines, %d KB; only the first 200 lines or 25 KB load", s.Index.Lines, s.Index.Bytes/1024)})
			}
		}
		for _, m := range s.Entries {
			if s.Index != nil && !indexed[m.File] {
				out = append(out, Finding{Rule: "memory/not-indexed", Severity: Warning, Path: m.Path, Message: fmt.Sprintf("%s is not listed in MEMORY.md", m.File)})
			}
			var missing []string
			if m.Name == "" {
				missing = append(missing, "name")
			}
			if m.Description == "" {
				missing = append(missing, "description")
			}
			if m.Type == "" {
				missing = append(missing, "type")
			}
			if len(missing) > 0 {
				out = append(out, Finding{Rule: "memory/missing-frontmatter", Severity: Warning, Path: m.Path, Message: "missing " + strings.Join(missing, ", ")})
			}
			for _, l := range m.Links {
				if !keys[l] {
					out = append(out, Finding{Rule: "memory/unresolved-link", Severity: Info, Path: m.Path, Message: fmt.Sprintf("[[%s]] has no memory yet", l)})
				}
			}
		}
		sets := make([]wordSet, len(s.Entries))
		for i, m := range s.Entries {
			sets[i] = words(m.Description)
		}
		for i := range s.Entries {
			for j := i + 1; j < len(s.Entries); j++ {
				if score := similarity(sets[i], sets[j]); score >= opt.SimilarityThreshold {
					out = append(out, Finding{
						Rule: "memory/similar", Severity: Info, Path: s.Entries[i].Path, Related: s.Entries[j].Path,
						Message: fmt.Sprintf("%s and %s have similar descriptions (%.0f%% overlap)", s.Entries[i].File, s.Entries[j].File, score*100),
					})
				}
			}
		}
	}
	return out
}

func plugins(list []claude.Plugin) []Finding {
	var out []Finding
	for _, p := range list {
		if p.Enabled && p.Error != "" {
			out = append(out, Finding{Rule: "plugin/not-installed", Severity: Error, Path: p.InstallPath, Message: fmt.Sprintf("%s: %s", p.ID, p.Error)})
		}
	}
	return out
}

func hooks(list []claude.Hook) []Finding {
	var out []Finding
	for _, h := range list {
		if h.Type != "command" || h.Status != claude.StatusActive {
			continue
		}
		exe := executable(h)
		if exe == "" {
			continue
		}
		if _, err := os.Stat(exe); err != nil {
			out = append(out, Finding{Rule: "hook/command-missing", Severity: Warning, Path: h.Path, Message: fmt.Sprintf("%s hook runs %s, which does not exist", h.Event, exe)})
		}
	}
	return out
}

// executable returns the first word of a hook command when it is a path dotclaude
// can check: absolute, under the home directory, or under the plugin root.
// Commands that go through the PATH or a shell construct are skipped.
func executable(h claude.Hook) string {
	fields := strings.Fields(h.Detail)
	if len(fields) == 0 {
		return ""
	}
	exe := strings.Trim(fields[0], `"'`)
	if (exe == "bash" || exe == "sh" || exe == "zsh" || exe == "python3" || exe == "node") && len(fields) > 1 {
		exe = strings.Trim(fields[1], `"'`)
	}
	if h.PluginRoot != "" {
		exe = strings.ReplaceAll(exe, "${CLAUDE_PLUGIN_ROOT}", h.PluginRoot)
	}
	if strings.HasPrefix(exe, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			exe = filepath.Join(home, exe[2:])
		}
	}
	if !filepath.IsAbs(exe) || strings.ContainsAny(exe, "$*?") {
		return ""
	}
	return exe
}
