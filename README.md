# dotclaude

See, check and tidy your Claude Code configuration.

`~/.claude` and every repository's `.claude` fill up fast: skills, agents,
commands, plugins, hooks, MCP servers, memories and `CLAUDE.md` files spread
over several levels. dotclaude answers three questions about them, and fixes
what it can:

- **What is in effect here, and where does each piece come from?** Pick a
  project and see the instruction files in load order, the merged settings
  with the file each value comes from, and which agents, skills and MCP
  servers are active, shadowed by another definition, or switched off.
- **What is broken or redundant?** Dangling symlinks, imports that resolve to
  nothing, memories missing from `MEMORY.md`, skills competing for the same
  trigger, plugins that are enabled but not installed, hooks that run a
  script that is gone.
- **How does memory hang together?** A graph of the `[[links]]` between
  memories, including the ones that point at a memory not written yet.
- **Can it be fixed from here?** Switch a skill off or back on, enable or
  disable a plugin, add a memory to `MEMORY.md` or drop a line that links
  nowhere, remove a dangling skill symlink. Each change is shown as a diff
  first and backed up before it is applied.

It runs locally as one binary with the web UI built in.

## Install

```sh
make build/install      # builds the UI and installs into ~/.local/bin
```

Requires Go 1.26+ and Node 24+ to build.

## Use

```sh
dotclaude serve --open          # web UI on http://localhost:7878
dotclaude serve --read-only     # the same, without the option to change anything
dotclaude lint                  # errors and warnings; exits 1 on errors
dotclaude lint --min info       # everything, including hints
dotclaude lint -p ~/code/app    # as it applies in one project
dotclaude show -p ~/code/app    # the resolved configuration as JSON
```

Paths are configurable for setups that keep files elsewhere:

| Flag | Default |
| --- | --- |
| `--config-dir` | `$CLAUDE_CONFIG_DIR`, else `~/.claude` |
| `--global-config` | `~/.claude.json` |
| `--managed-dir` | `/Library/Application Support/ClaudeCode` |
| `--memory-dir` (repeatable) | `<config-dir>/memory`, when it exists |

Auto memory under `<config-dir>/projects/*/memory` is always read, unless
`autoMemoryDirectory` is set in settings, in which case that directory is used.

## How it resolves

The rules follow the Claude Code documentation at
[code.claude.com/docs](https://code.claude.com/docs/en/settings). They live in
`internal/claude`; each rule's doc comment says what it implements.

- **Settings**: managed > local > project > user. Lists merge, other values
  come from the highest file. `.claude/settings.json` is read from the start
  directory; `settings.local.json` from the repository root.
  `~/.claude/settings.local.json` is read as a second user file.
- **Instructions**: managed, user, user rules, then `CLAUDE.md`,
  `.claude/CLAUDE.md` and `CLAUDE.local.md` for each directory from the
  filesystem root down to the start directory, then `.claude/rules`.
  `@imports` are followed four hops deep. `AGENTS.md` is read only when no
  `CLAUDE.md` exists in that chain.
- **Agents**: project (closest directory first) > user > plugin.
- **Skills and commands** share one namespace: managed > user > project, and a
  skill beats a command with the same name. `skillOverrides: "off"` switches
  one off. Plugin components are namespaced as `plugin:name`.
- **MCP servers**: managed > local > project > user.

## Changes

The UI offers a change where a lint finding has an obvious fix, and on skills
and plugins. Nothing is written until you apply the previewed diff.

| Change | File it edits |
| --- | --- |
| Switch a skill off | `skillOverrides` in `~/.claude/settings.local.json`, so it is off in every project |
| Switch a skill back on | every non-managed settings file whose `skillOverrides` switches it off |
| Enable or disable a plugin | `enabledPlugins` in the settings file that currently decides it, else `~/.claude/settings.json` |
| List a memory in the index | appends a line to that store's `MEMORY.md` |
| Drop an index line | removes the line from `MEMORY.md` when its link points to a missing file |
| Remove a dangling skill symlink | deletes the link |

Settings files are edited in place: key order and values stay as they are,
and the file is written with two-space indentation, as Claude Code writes it.
Symlinked files are written through, so the link survives.

Before a file changes, a copy goes to `<config-dir>/backups/dotclaude/`; for a
removed symlink, that directory gets a script that recreates it. Restore by
copying the file back or running the script. A change is refused when the file
has changed since its preview, when a settings file is a symlink to a missing
file, and when an index line also links to memories that exist.

## Privacy and safety

The server binds to 127.0.0.1 and refuses requests addressed to any other host
name. Changes are accepted only from the UI: they need a JSON body, a custom
header and a loopback origin, which a page on another site cannot send without
a CORS preflight that the server never answers. The client names a change, and
the server works out the file and the edit from the configuration on disk, so
no request can choose what file to write. Changes are made only for the user
level or a project listed in `~/.claude.json`. `--read-only` turns changes off.

The file viewer serves only files that are part of the loaded configuration,
and never settings files. The account details in `~/.claude.json` are never
decoded. MCP arguments, environment and headers are not read at all, and
settings values under `env` or with credential-like names are redacted.

## Develop

```sh
make dev/serve    # API on :7878
make dev/web      # UI with hot reload on :5173, proxying /api
make qa/check     # formatting, vet, type-check, tests
make help         # every target
```

Tests run against a synthetic configuration built by `internal/testfixture`,
never against a real one.

## Prior art

[Agents Studio](https://github.com/davidrodriguezpozo/agents-ui) runs and lands
agent work and has a library of agents, skills and plugins. dotclaude focuses
on what is in effect where and on what is wrong with it.

## Licence

MIT.
