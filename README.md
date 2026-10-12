# dotclaude

See and check your Claude Code configuration.

`~/.claude` and every repository's `.claude` fill up fast: skills, agents,
commands, plugins, hooks, MCP servers, memories and `CLAUDE.md` files spread
over several levels. dotclaude answers three questions about them:

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

It runs locally as one binary with the web UI built in.

## Install

```sh
make build/install      # builds the UI and installs into ~/.local/bin
```

Requires Go 1.26+ and Node 24+ to build.

## Use

```sh
dotclaude serve --open          # web UI on http://localhost:7878
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

## Privacy

dotclaude only reads; it never writes to your configuration. The server binds
to 127.0.0.1 and refuses requests addressed to any other host name. The file
viewer serves only files that are part of the loaded configuration, and never
settings files. The account details in `~/.claude.json` are never decoded. MCP
arguments, environment and headers are not read at all, and settings values
under `env` or with credential-like names are redacted.

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
