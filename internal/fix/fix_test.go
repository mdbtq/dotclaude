package fix_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdbtq/dotclaude/internal/claude"
	"github.com/mdbtq/dotclaude/internal/fix"
	"github.com/mdbtq/dotclaude/internal/testfixture"
)

func apply(t *testing.T, f testfixture.Fixture, r fix.Request) (*fix.Plan, fix.Result) {
	t.Helper()
	c, err := claude.Load(f.Paths, "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := fix.Prepare(c, f.Paths, r)
	if err != nil {
		t.Fatalf("prepare %+v: %v", r, err)
	}
	res, err := fix.Apply(plan, plan.Hashes(), filepath.Join(f.Home, "backups"))
	if err != nil {
		t.Fatalf("apply %+v: %v", r, err)
	}
	return plan, res
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func reload(t *testing.T, f testfixture.Fixture) *claude.Config {
	t.Helper()
	c, err := claude.Load(f.Paths, "")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func status(c *claude.Config, kind claude.Kind, name string) claude.Status {
	for _, comp := range c.Components {
		if comp.Kind == kind && comp.Name == name && comp.Source.Scope == claude.ScopeUser {
			return comp.Status
		}
	}
	return ""
}

func TestSkillOverride(t *testing.T) {
	f := testfixture.New(t)
	local := filepath.Join(f.Home, ".claude", "settings.local.json")

	_, res := apply(t, f, fix.Request{Kind: fix.SkillOverride, Target: "release", Value: "off"})
	if len(res.Backups) != 0 {
		t.Errorf("a new file needs no backup, got %v", res.Backups)
	}
	if got := read(t, local); got != "{\n  \"skillOverrides\": {\n    \"release\": \"off\"\n  }\n}\n" {
		t.Errorf("settings.local.json:\n%s", got)
	}
	if got := status(reload(t, f), claude.KindSkill, "release"); got != claude.StatusDisabled {
		t.Errorf("release after switching off: %s", got)
	}

	// Switching quiet on removes its entry from settings.json; the object
	// becomes empty and goes too, the rest of the file stays in order.
	settings := filepath.Join(f.Home, ".claude", "settings.json")
	before := read(t, settings)
	_, res = apply(t, f, fix.Request{Kind: fix.SkillOverride, Target: "quiet", Value: "on"})
	after := read(t, settings)
	if strings.Contains(after, "skillOverrides") || !strings.Contains(after, `"enabledPlugins"`) {
		t.Errorf("settings.json after switching quiet on:\n%s", after)
	}
	if len(res.Backups) != 1 || read(t, res.Backups[0]) != before {
		t.Errorf("backup %v does not hold the original", res.Backups)
	}
	if got := status(reload(t, f), claude.KindSkill, "quiet"); got != claude.StatusActive {
		t.Errorf("quiet after switching on: %s", got)
	}
}

func TestPluginEnabled(t *testing.T) {
	f := testfixture.New(t)
	apply(t, f, fix.Request{Kind: fix.PluginEnabled, Target: "ghost@market", Value: "false"})
	for _, p := range reload(t, f).Plugins {
		if p.ID == "ghost@market" && p.Enabled {
			t.Error("ghost is still enabled")
		}
	}
	c := reload(t, f)
	if _, err := fix.Prepare(c, f.Paths, fix.Request{Kind: fix.PluginEnabled, Target: "ghost@market", Value: "true"}); !errors.Is(err, fix.ErrNotApplicable) {
		t.Errorf("enabling an uninstalled plugin: %v, want not applicable", err)
	}
}

func TestMemoryIndex(t *testing.T) {
	f := testfixture.New(t)
	mem := filepath.Join(f.Home, ".claude", "memory")
	index := filepath.Join(mem, "MEMORY.md")

	apply(t, f, fix.Request{Kind: fix.MemoryIndexAdd, Target: filepath.Join(mem, "beta.md")})
	if got := read(t, index); !strings.HasSuffix(got, "- [Beta](beta.md) — Second memory\n") {
		t.Errorf("MEMORY.md after adding beta:\n%s", got)
	}
	apply(t, f, fix.Request{Kind: fix.MemoryIndexRemove, Target: index, Line: 4})
	if got := read(t, index); strings.Contains(got, "gone.md") || !strings.Contains(got, "alpha.md") {
		t.Errorf("MEMORY.md after removing the dangling line:\n%s", got)
	}

	// Done twice, it no longer applies.
	c := reload(t, f)
	if _, err := fix.Prepare(c, f.Paths, fix.Request{Kind: fix.MemoryIndexAdd, Target: filepath.Join(mem, "beta.md")}); !errors.Is(err, fix.ErrNotApplicable) {
		t.Errorf("indexing beta again: %v", err)
	}
}

func TestRemoveSymlink(t *testing.T) {
	f := testfixture.New(t)
	link := filepath.Join(f.Home, ".claude", "skills", "dangling")
	_, res := apply(t, f, fix.Request{Kind: fix.RemoveSymlink, Target: link})
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("symlink still there: %v", err)
	}
	if len(res.Backups) != 1 || !strings.Contains(read(t, res.Backups[0]), "ln -s") {
		t.Errorf("restore script missing: %v", res.Backups)
	}

	// A path that is not a dangling skill is refused.
	c := reload(t, f)
	settings := filepath.Join(f.Home, ".claude", "settings.json")
	if _, err := fix.Prepare(c, f.Paths, fix.Request{Kind: fix.RemoveSymlink, Target: settings}); !errors.Is(err, fix.ErrNotApplicable) {
		t.Errorf("removing settings.json: %v, want not applicable", err)
	}
}

func TestApplyRefusesChangedFile(t *testing.T) {
	f := testfixture.New(t)
	c := reload(t, f)
	plan, err := fix.Prepare(c, f.Paths, fix.Request{Kind: fix.PluginEnabled, Target: "ghost@market", Value: "false"})
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(f.Home, ".claude", "settings.json")
	if err := os.WriteFile(settings, []byte(read(t, settings)+" "), 0o644); err != nil {
		t.Fatal(err)
	}
	c = reload(t, f)
	fresh, err := fix.Prepare(c, f.Paths, plan.Request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fix.Apply(fresh, plan.Hashes(), t.TempDir()); !errors.Is(err, fix.ErrChanged) {
		t.Errorf("apply with stale hashes: %v, want ErrChanged", err)
	}
}

func TestDiffReplacement(t *testing.T) {
	f := testfixture.New(t)
	plan, err := fix.Prepare(reload(t, f), f.Paths, fix.Request{Kind: fix.PluginEnabled, Target: "ghost@market", Value: "false"})
	if err != nil {
		t.Fatal(err)
	}
	diff := plan.Changes[0].Diff
	if !strings.Contains(diff, "-    \"ghost@market\": true\n+    \"ghost@market\": false\n") {
		t.Errorf("a replaced line should read as removal then addition:\n%s", diff)
	}
}

func TestDiff(t *testing.T) {
	f := testfixture.New(t)
	plan, err := fix.Prepare(reload(t, f), f.Paths, fix.Request{Kind: fix.MemoryIndexRemove, Target: filepath.Join(f.Home, ".claude", "memory", "MEMORY.md"), Line: 4})
	if err != nil {
		t.Fatal(err)
	}
	want := "@@ -1,4 +1,3 @@\n # Memory\n \n - [Alpha](alpha.md) — first\n-- [Gone](gone.md) — deleted\n"
	if got := plan.Changes[0].Diff; got != want {
		t.Errorf("diff:\n%s\nwant:\n%s", got, want)
	}
}

func TestRefusesRiskyEdits(t *testing.T) {
	f := testfixture.New(t)
	mem := filepath.Join(f.Home, ".claude", "memory")
	index := filepath.Join(mem, "MEMORY.md")
	if err := os.WriteFile(index, []byte("# Memory\n\n- [Alpha](alpha.md), see also [Gone](gone.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fix.Prepare(reload(t, f), f.Paths, fix.Request{Kind: fix.MemoryIndexRemove, Target: index, Line: 3}); !errors.Is(err, fix.ErrNotApplicable) {
		t.Errorf("removing a line that also holds a valid link: %v, want not applicable", err)
	}

	// A settings.local.json that is a symlink to a missing file stays a link.
	local := filepath.Join(f.Home, ".claude", "settings.local.json")
	if err := os.Symlink(filepath.Join(f.Home, "dotfiles", "settings.local.json"), local); err != nil {
		t.Fatal(err)
	}
	if _, err := fix.Prepare(reload(t, f), f.Paths, fix.Request{Kind: fix.SkillOverride, Target: "release", Value: "off"}); !errors.Is(err, fix.ErrNotApplicable) {
		t.Errorf("writing through a dangling symlink: %v, want not applicable", err)
	}
}
