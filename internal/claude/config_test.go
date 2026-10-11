package claude_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdbtq/dotclaude/internal/claude"
	"github.com/mdbtq/dotclaude/internal/testfixture"
)

func find(t *testing.T, c *claude.Config, kind claude.Kind, name string, scope claude.Scope) claude.Component {
	t.Helper()
	for _, comp := range c.Components {
		if comp.Kind == kind && comp.Name == name && comp.Source.Scope == scope {
			return comp
		}
	}
	t.Fatalf("no %s %q from %s", kind, name, scope)
	return claude.Component{}
}

func setting(c *claude.Config, key string) (claude.SettingEntry, bool) {
	for _, s := range c.Settings {
		if s.Key == key {
			return s, true
		}
	}
	return claude.SettingEntry{}, false
}

func TestUserLevel(t *testing.T) {
	f := testfixture.New(t)
	c, err := claude.Load(f.Paths, "")
	if err != nil {
		t.Fatal(err)
	}

	if got := find(t, c, claude.KindCommand, "deploy", claude.ScopeUser).Status; got != claude.StatusShadowed {
		t.Errorf("command deploy: status %s, want shadowed by the skill of the same name", got)
	}
	if got := find(t, c, claude.KindSkill, "deploy", claude.ScopeUser); got.Status != claude.StatusActive || got.Usage == nil || got.Usage.Count != 3 {
		t.Errorf("skill deploy: status %s usage %+v, want active with 3 uses", got.Status, got.Usage)
	}
	if got := find(t, c, claude.KindSkill, "quiet", claude.ScopeUser).Status; got != claude.StatusDisabled {
		t.Errorf("skill quiet: status %s, want disabled by skillOverrides", got)
	}
	if got := find(t, c, claude.KindSkill, "dangling", claude.ScopeUser).Error; !strings.Contains(got, "dangling symlink") {
		t.Errorf("skill dangling: error %q", got)
	}
	find(t, c, claude.KindCommand, "frontend:component", claude.ScopeUser)
	find(t, c, claude.KindAgent, "helper", claude.ScopeUser) // subdirectory does not namespace user agents
	find(t, c, claude.KindAgent, "example:review:security", claude.ScopePlugin)
	find(t, c, claude.KindSkill, "example:lint", claude.ScopePlugin)

	var ghost, example *claude.Plugin
	for i := range c.Plugins {
		switch c.Plugins[i].ID {
		case "ghost@market":
			ghost = &c.Plugins[i]
		case "example@market":
			example = &c.Plugins[i]
		}
	}
	if ghost == nil || ghost.Installed || ghost.Error == "" {
		t.Errorf("ghost plugin: %+v, want listed as enabled but not installed", ghost)
	}
	if example == nil || !example.Enabled || example.Author != "Someone" || example.Counts["hooks"] != 1 {
		t.Errorf("example plugin: %+v", example)
	}

	if env, ok := setting(c, "env.API_TOKEN"); !ok || env.Value != claude.Redacted {
		t.Errorf("env.API_TOKEN: %+v, want redacted", env)
	}
	if len(c.Memory) != 1 || len(c.Memory[0].Entries) != 2 || c.Memory[0].Entries[0].Type != "feedback" {
		t.Errorf("memory: %+v", c.Memory)
	}
	if got := len(c.Instructions); got != 2 {
		t.Errorf("instructions: %d, want user CLAUDE.md and one user rule", got)
	}
}

func TestNothingSecretLeaks(t *testing.T) {
	f := testfixture.New(t)
	for _, project := range []string{"", f.Project} {
		c, err := claude.Load(f.Paths, project)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(c)
		if strings.Contains(string(data), testfixture.Secret) {
			t.Errorf("project %q: output contains the planted secret", project)
		}
	}
}

func TestProject(t *testing.T) {
	f := testfixture.New(t)
	c, err := claude.Load(f.Paths, f.Project)
	if err != nil {
		t.Fatal(err)
	}
	if c.ProjectRoot != f.Repo {
		t.Errorf("project root %s, want %s", c.ProjectRoot, f.Repo)
	}

	// Agents: project beats user. Skills: user beats project.
	if got := find(t, c, claude.KindAgent, "reviewer", claude.ScopeProject).Status; got != claude.StatusActive {
		t.Errorf("project agent: %s", got)
	}
	if got := find(t, c, claude.KindAgent, "reviewer", claude.ScopeUser).Status; got != claude.StatusShadowed {
		t.Errorf("user agent: %s, want shadowed", got)
	}
	if got := find(t, c, claude.KindSkill, "deploy", claude.ScopeProject).Status; got != claude.StatusShadowed {
		t.Errorf("project skill: %s, want shadowed by the personal skill", got)
	}
	find(t, c, claude.KindCommand, "local-cmd", claude.ScopeProject) // from the start directory's .claude

	// Settings: shared settings from the start directory, local settings
	// from the repository root above a legacy one in the start directory;
	// scalars from the highest layer, lists merged.
	if model, _ := setting(c, "model"); model.Value != "haiku" || model.Scope != claude.ScopeLocal {
		t.Errorf("model: %+v, want haiku from the root settings.local.json", model)
	}
	allow, _ := setting(c, "permissions.allow")
	if list, _ := allow.Value.([]any); len(list) != 3 || !allow.Merged {
		t.Errorf("permissions.allow: %+v, want user, project and legacy local entries merged", allow)
	}
	var ignored []string
	for _, l := range c.SettingsLayers {
		if l.Ignored != "" {
			ignored = append(ignored, l.Path)
		}
	}
	if len(ignored) != 1 || ignored[0] != filepath.Join(f.Repo, ".claude", "settings.json") {
		t.Errorf("ignored settings layers: %v, want the repository root settings.json", ignored)
	}

	// Instructions: user first, then root-down, local last at its level, then rules.
	var order []string
	for _, in := range c.Instructions {
		rel, _ := filepath.Rel(f.Home, in.Path)
		order = append(order, rel)
	}
	want := []string{".claude/CLAUDE.md", ".claude/rules/style.md", "code/repo/CLAUDE.md", "code/repo/app/CLAUDE.local.md", "code/repo/app/.claude/rules/api.md"}
	if strings.Join(order, " ") != strings.Join(want, " ") {
		t.Errorf("instruction order:\n got %v\nwant %v", order, want)
	}
	if rule := c.Instructions[4]; !rule.Conditional {
		t.Errorf("api rule should be path-scoped")
	}
	imports := c.Instructions[0].Imports
	if len(imports) != 2 || !imports[0].Exists || imports[1].Exists {
		t.Errorf("imports of user CLAUDE.md: %+v, want one found and one missing", imports)
	}

	// MCP: local beats project beats user.
	status := map[string]claude.Status{}
	for _, s := range c.MCPServers {
		status[string(s.Source.Scope)+"/"+s.Name] = s.Status
	}
	for key, want := range map[string]claude.Status{
		"local/shared": claude.StatusActive, "project/shared": claude.StatusShadowed,
		"user/shared": claude.StatusShadowed, "project/team": claude.StatusActive, "plugin/example-docs": claude.StatusActive,
	} {
		if status[key] != want {
			t.Errorf("mcp %s: %s, want %s", key, status[key], want)
		}
	}
}

func TestFindImports(t *testing.T) {
	text := "See @docs/a.md, and @~/notes.md.\nmail me@example.com, ping @someone\n`@skip/me.md`\n```\n@skip/fenced.md\n```\n@path\\ with\\ space.md\n"
	var got []string
	for _, ref := range claude.FindImports(text) {
		got = append(got, ref.Path)
	}
	want := []string{"docs/a.md", "~/notes.md", "path with space.md"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLenientFrontmatter(t *testing.T) {
	doc := claude.ParseDocument([]byte("---\nname: x\ndescription: Use when: things happen\nmetadata:\n  type: feedback\n---\nbody\n"))
	if doc.FrontmatterError == "" {
		t.Fatal("expected a YAML error for the unquoted colon")
	}
	meta, _ := doc.Frontmatter["metadata"].(map[string]any)
	if doc.String("description") != "Use when: things happen" || meta["type"] != "feedback" || doc.Body != "body\n" {
		t.Errorf("lenient parse: %+v body %q", doc.Frontmatter, doc.Body)
	}
}
