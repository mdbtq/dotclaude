# dotclaude

A local, read-only dashboard for your Claude Code configuration.

Status: early design. Nothing is built yet.

## Why

`~/.claude` and each repository's `.claude` fill up quickly. You end up with many
skills, agents, plugins, memories and `CLAUDE.md` files spread across several
levels, and no one place answers these questions:

- **What is actually in effect in this repository, and where does each piece
  come from?** That means the merged instructions, settings, agents, skills, hooks
  and MCP servers, each with its source file, plus what is shadowed or
  overridden.
- **What is broken or redundant?** Examples: `[[links]]` in memory that point
  nowhere, a memory index that has drifted from its files, near-duplicate
  memories, skills whose descriptions compete for the same trigger, and
  references to files that no longer exist.
- **How does memory hang together?** A graph built from the links between
  memories.

## Principles

- **Read-only.** It shows what is on disk and never writes to it.
- **Local.** No account, no telemetry, nothing leaves the machine.
- **Configurable paths.** Not every setup keeps its files in the default
  locations.

## Prior art

[Agents Studio](https://github.com/davidrodriguezpozo/agents-ui) comes closest,
especially its *Library* and *Plugins* sections. It has grown into a platform
for running and landing agent work. dotclaude deliberately stays a viewer and a
linter.
