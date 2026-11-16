// Package testfixture builds a synthetic home directory with a Claude Code
// configuration, for tests. Nothing in it comes from a real setup.
package testfixture

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdbtq/dotclaude/internal/claude"
)

// Secret is a value planted in places dotclaude must never expose.
const Secret = "s3cret-value-that-must-not-leak"

// Fixture is a built home directory.
type Fixture struct {
	Home    string
	Paths   claude.Paths
	Repo    string // a git repository with project configuration
	Project string // a subdirectory of Repo to start in
}

// New writes the fixture into a temporary directory.
func New(t *testing.T) Fixture {
	t.Helper()
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	repo := filepath.Join(home, "code", "repo")
	project := filepath.Join(repo, "app")
	plugin := filepath.Join(cfg, "plugins", "cache", "market", "example", "1.0.0")

	files := map[string]string{
		// User level.
		".claude/settings.json": `{
			"model": "sonnet",
			"env": {"API_TOKEN": "` + Secret + `"},
			"permissions": {"allow": ["Bash(ls *)"]},
			"enabledPlugins": {"example@market": true, "ghost@market": true},
			"skillOverrides": {"quiet": "off"},
			"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "/nonexistent/guard.sh"}]}]}
		}`,
		".claude/CLAUDE.md":                      "# User\n\nSee @rules/style.md and @missing/file.md.\nNot an import: `@code/span.md`.\n\n```\n@fenced/block.md\n```\n",
		".claude/rules/style.md":                 "Use tabs.\n",
		".claude/agents/reviewer.md":             "---\nname: reviewer\ndescription: Reviews code\n---\nUser reviewer.\n",
		".claude/agents/team/helper.md":          "---\nname: helper\ndescription: Helps out\n---\n",
		".claude/agents/nameless.md":             "---\ndescription: Has no name\n---\n",
		".claude/skills/deploy/SKILL.md":         "---\nname: deploy\ndescription: Deploy the application to production servers safely\n---\n",
		".claude/skills/quiet/SKILL.md":          "---\ndescription: A skill that is switched off\n---\n",
		".claude/skills/release/SKILL.md":        "---\ndescription: Deploy the application to production servers quickly\n---\n",
		".claude/commands/deploy.md":             "---\ndescription: Old deploy command\n---\n",
		".claude/commands/frontend/component.md": "Make a component.\n",
		// skills.sh: a store, a lock file listing one of its skills, and a
		// skill developed in a repository.
		".agents/.skill-lock.json":        `{"version": 3, "skills": {"tracked": {"source": "owner/skills", "sourceType": "github"}}}`,
		".agents/skills/tracked/SKILL.md": "---\ndescription: Installed and tracked by skills.sh\n---\n",
		".agents/skills/orphan/SKILL.md":  "---\ndescription: In the store but missing from the lock\n---\n",
		".agents/skills/copied/SKILL.md":  "---\ndescription: Store original of a copy\n---\n",
		".claude/skills/copied/SKILL.md":  "---\ndescription: Copied out of the store\n---\n",
		"code/dev-skill/SKILL.md":         "---\ndescription: Work in progress\n---\n",
		".claude/memory/MEMORY.md":        "# Memory\n\n- [Alpha](alpha.md) — first\n- [Gone](gone.md) — deleted\n",
		".claude/memory/alpha.md":         "---\nname: alpha\ndescription: First memory: with a colon\nmetadata:\n  type: feedback\n---\nLinks to [[beta]] and [[not-yet]].\n",
		".claude/memory/beta.md":          "---\nname: beta\ndescription: Second memory\nmetadata:\n  type: user\n---\nBack to [[alpha]].\n",
		".claude.json": `{
			"oauthAccount": {"emailAddress": "` + Secret + `"},
			"mcpServers": {
				"shared": {"command": "npx", "args": ["--token", "` + Secret + `"], "env": {"KEY": "` + Secret + `"}},
				"remote": {"type": "http", "url": "https://mcp.example.com/v1?key=` + Secret + `", "headers": {"Authorization": "` + Secret + `"}}
			},
			"projects": {
				"` + project + `": {"mcpServers": {"shared": {"command": "local-shared"}}}
			},
			"skillUsage": {"deploy": {"usageCount": 3, "lastUsedAt": 1700000000000}}
		}`,

		// Plugin.
		".claude/plugins/installed_plugins.json":              `{"version": 2, "plugins": {"example@market": [{"scope": "user", "installPath": "` + plugin + `", "version": "1.0.0"}]}}`,
		pluginRel(home, plugin, ".claude-plugin/plugin.json"): `{"name": "example", "description": "An example plugin", "author": {"name": "Someone"}}`,
		pluginRel(home, plugin, "agents/review/security.md"):  "---\nname: security\ndescription: Security review\n---\n",
		pluginRel(home, plugin, "skills/lint/SKILL.md"):       "---\ndescription: Lint things\n---\n",
		pluginRel(home, plugin, "hooks/hooks.json"):           `{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "bash \"${CLAUDE_PLUGIN_ROOT}/hooks/stop.sh\""}]}]}}`,
		pluginRel(home, plugin, "hooks/stop.sh"):              "#!/bin/sh\n",
		pluginRel(home, plugin, ".mcp.json"):                  `{"mcpServers": {"example-docs": {"type": "http", "url": "https://docs.example.com/mcp"}}}`,

		// Marketplaces: the one the plugins come from, with credentials in its
		// URL, and one with no plugins installed from it.
		".claude/plugins/known_marketplaces.json": `{
			"market": {"source": {"source": "git", "url": "https://user:` + Secret + `@git.example.com/market.git?token=` + Secret + `"}, "installLocation": "` + filepath.Join(cfg, "plugins", "marketplaces", "market") + `", "lastUpdated": "2026-01-02T03:04:05.000Z", "autoUpdate": true},
			"spare": {"source": {"source": "github", "repo": "owner/spare"}, "installLocation": "` + filepath.Join(cfg, "plugins", "marketplaces", "spare") + `"}
		}`,
		".claude/plugins/marketplaces/market/.claude-plugin/marketplace.json": `{"name": "market", "description": "A test marketplace", "owner": {"name": "Owner"}, "plugins": [{"name": "example", "source": "./plugins/example"}, {"name": "other", "source": "./plugins/other"}]}`,
		".claude/plugins/marketplaces/spare/.claude-plugin/marketplace.json":  `{"name": "spare", "metadata": {"description": "Nothing installed from here"}, "plugins": []}`,

		// Project.
		"code/CLAUDE.md":                                     "# Code\n",
		"code/repo/.git/HEAD":                                "ref: refs/heads/main\n",
		"code/repo/CLAUDE.md":                                "# Repo\n",
		"code/repo/.mcp.json":                                `{"mcpServers": {"shared": {"command": "project-shared"}, "team": {"command": "team-server"}}}`,
		"code/repo/.claude/settings.json":                    `{"model": "opus"}`,
		"code/repo/.claude/settings.local.json":              `{"model": "haiku"}`,
		"code/repo/app/.claude/settings.json":                `{"model": "sonnet-project", "permissions": {"allow": ["Bash(make *)"]}}`,
		"code/repo/.claude/agents/reviewer.md":               "---\nname: reviewer\ndescription: Project reviewer\n---\n",
		"code/repo/.claude/skills/deploy/SKILL.md":           "---\nname: deploy\ndescription: Project deploy\n---\n",
		"code/repo/app/CLAUDE.local.md":                      "Local notes.\n",
		"code/repo/app/.claude/rules/api.md":                 "---\npaths:\n  - \"src/api/**\"\n---\nAPI rules.\n",
		"code/repo/app/.claude/settings.local.json":          `{"model": "legacy", "permissions": {"allow": ["Bash(git status)"]}}`,
		"code/repo/app/.claude/commands/local-cmd.md":        "Local command.\n",
		"code/repo/app/skills-lock.json":                     `{"version": 1, "skills": {"project-tool": {"source": "owner/project-skills"}}}`,
		"code/repo/app/.agents/skills/project-tool/SKILL.md": "---\ndescription: Installed into the project by skills.sh\n---\n",
	}
	for rel, content := range files {
		path := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(rel, ".json") {
			content = claudeFormat(t, content)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		"dangling": filepath.Join(home, "nowhere"),
		"tracked":  "../../.agents/skills/tracked",
		"orphan":   "../../.agents/skills/orphan",
		"dev":      filepath.Join(home, "code", "dev-skill"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(cfg, "skills", name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(project, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../.agents/skills/project-tool", filepath.Join(project, ".claude", "skills", "project-tool")); err != nil {
		t.Fatal(err)
	}

	p := claude.DefaultPaths(home, "")
	p.ManagedDir = filepath.Join(home, "managed")                     // never the real one
	p.SkillsLock = filepath.Join(home, ".agents", ".skill-lock.json") // whatever XDG_STATE_HOME says
	return Fixture{Home: home, Paths: p, Repo: repo, Project: project}
}

// claudeFormat lays out JSON the way Claude Code writes its files: two-space
// indentation and a trailing newline.
func claudeFormat(t *testing.T, content string) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(content), "", "  "); err != nil {
		t.Fatalf("fixture JSON: %v", err)
	}
	return out.String() + "\n"
}

func pluginRel(home, plugin, rel string) string {
	r, _ := filepath.Rel(home, filepath.Join(plugin, rel))
	return r
}
