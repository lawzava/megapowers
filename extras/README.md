# Repository-only extras

These helpers, skills, and output style do not ship with the plugin or its HTTP skill catalog.
The plugin uses manual procedures with normal agent tools and needs no Go runtime.

## Helpers

Nine Go helpers moved unchanged into `extras/scripts/<skill>/`, preserving nested paths.
Use Go 1.25 or newer from this checkout, for example:

```bash
go run ./extras/scripts/megapowers-doctor/doctor.go --root ./plugins/megapowers --harness codex
```

The helpers retain their command interfaces. Read the source before running
them. Contract tests live under `scripts/contracts/` and
`scripts/internal/maintain/`.

## Skills

`skills/megapowers-doctor/`, `skills/upgrading-megapowers/`, and
`skills/mcp-setup/` retain their original text for repository-only use. Their
old plugin-relative helper paths are historical; run helpers from `extras/scripts/`
as shown above and supply the plugin root explicitly. These skills are not
part of the supported shipped set.

## Instruction references and templates

`skills/writing-agent-instructions/` holds repository-only guidance moved out
of the plugin in 0.56.1:

- [Session mining](skills/writing-agent-instructions/references/session-mining.md)
- [Learning loop](skills/writing-agent-instructions/references/learning-loop.md)
- [Global instructions](skills/writing-agent-instructions/references/global-instructions.md)
- [Codex global template](skills/writing-agent-instructions/assets/global-AGENTS.md)
- [Claude Code global template](skills/writing-agent-instructions/assets/global-CLAUDE.md)

These references and templates do not ship with the plugin or its HTTP skill
catalog. The learning loop links to the shared retrospective and memory-hygiene
references that remain in the plugin source tree.

## Output style

The unchanged `output-styles/megapowers.md` is an optional Claude Code style.
From this checkout, install it manually:

```bash
mkdir -p ~/.claude/output-styles
cp extras/output-styles/megapowers.md ~/.claude/output-styles/megapowers.md
```

Select `Megapowers` in `/config` or run `/output-style Megapowers`, then start
a fresh session. It preserves built-in coding instructions and stays off until
selected. The old plugin-qualified value `megapowers:Megapowers` no longer
resolves from the shipped plugin; replace it with the manually installed style.

## OpenCode catalog maintenance

After changing a shipped skill or plugin version, regenerate the catalog:

```bash
go run ./scripts/cmd/maintainer opencode-index
scripts/validate.sh
```

Each entry uses the plugin version. Bump it when publishing changed skill files
so OpenCode refreshes its HTTP cache. Hidden local files and directories are
excluded from the catalog.
