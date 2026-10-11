package lint_test

import (
	"path/filepath"
	"testing"

	"github.com/mdbtq/dotclaude/internal/claude"
	"github.com/mdbtq/dotclaude/internal/lint"
	"github.com/mdbtq/dotclaude/internal/testfixture"
)

func TestRulesOnFixture(t *testing.T) {
	f := testfixture.New(t)
	c, err := claude.Load(f.Paths, f.Project)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]lint.Finding{}
	for _, finding := range lint.Run(c, lint.DefaultOptions) {
		got[finding.Rule] = append(got[finding.Rule], finding)
	}

	mem := filepath.Join(f.Home, ".claude", "memory")
	want := map[string]string{
		"component/unreadable":        filepath.Join(f.Home, ".claude", "skills", "dangling"),
		"component/missing-name":      filepath.Join(f.Home, ".claude", "agents", "nameless.md"),
		"component/shadowed":          filepath.Join(f.Home, ".claude", "agents", "reviewer.md"),
		"skill/trigger-overlap":       filepath.Join(f.Home, ".claude", "skills", "deploy", "SKILL.md"),
		"instructions/import-missing": filepath.Join(f.Home, ".claude", "CLAUDE.md"),
		"memory/index-dangling":       filepath.Join(mem, "MEMORY.md"),
		"memory/not-indexed":          filepath.Join(mem, "beta.md"),
		"memory/unresolved-link":      filepath.Join(mem, "alpha.md"),
		"plugin/not-installed":        "",
		"hook/command-missing":        filepath.Join(f.Home, ".claude", "settings.json"),
		"settings/not-read":           filepath.Join(f.Repo, ".claude", "settings.json"),
	}
	for rule, path := range want {
		findings := got[rule]
		if len(findings) == 0 {
			t.Errorf("%s: no finding", rule)
			continue
		}
		found := false
		for _, finding := range findings {
			found = found || finding.Path == path
		}
		if !found {
			t.Errorf("%s: no finding for %s, got %+v", rule, path, findings)
		}
	}

	// The plugin hook script exists, so only the user hook is reported.
	if n := len(got["hook/command-missing"]); n != 1 {
		t.Errorf("hook/command-missing: %d findings, want 1", n)
	}
	for rule := range got {
		known := false
		for _, r := range lint.Rules {
			known = known || r.ID == rule
		}
		if !known {
			t.Errorf("finding for undocumented rule %s", rule)
		}
	}
}
