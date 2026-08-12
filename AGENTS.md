# Repository Instructions

## Scope

This repository publishes optional skills, plugins, hooks, and templates for
Claude Code, Codex, and OpenCode. Keep it public-safe:
no personal secrets, no machine-specific absolute paths, no private bridge
requirements unless clearly marked optional.

## Edit Rules

- Make the smallest change that makes the shipped artifact accurate.
- Keep skill frontmatter portable: `name` and `description` are required; add
  tool-specific metadata only when the target tool actually consumes it.
- Do not move plugin components under manifest directories. Component folders
  stay at plugin root (`skills/`, `agents/`, `hooks/`).
- Do not commit agent planning artifacts from `docs/megapowers/`, `docs/plans/`, `docs/specs/`,
  `.omc/`, `.claude-flow/`, `.swarm/`, or other generated workspaces.

## Tool Notes

- Claude Code uses `.claude-plugin/plugin.json` plus root-level `skills/`,
  `agents/`, and `hooks/`.
- Codex uses repo guidance from `AGENTS.md`, plugin manifests under
  `.codex-plugin/plugin.json`, and repo marketplaces under
  `.agents/plugins/marketplace.json`.
- OpenCode reads `AGENTS.md` and can load `skills/<name>/SKILL.md` through
  `.opencode/`, `.agents/`, or Claude-compatible paths. Its plugins are
  JavaScript or TypeScript modules; this repository ships two, ported from
  the Claude shell hooks (`plugins/megapowers/opencode/session-catalog.js`,
  `plugins/mega-guardrails/opencode/deny-destructive.js`).
- Three harnesses are targeted and no others. Skills are portable markdown, so
  another harness may load them; that is not support, and nothing here is tested
  against one. Adding a harness is a deliberate change to this list and to
  `docs/harness-support.md`, not a side effect of a file happening to parse.
- Full per-harness support details: `docs/harness-support.md` (canonical,
  freshness-checked).

## Scripting

New helpers in this repo are Go (`go run` a stdlib file, or a small module
under `scripts/`). Do not add Python, Node, or multi-line bash for glue.
Harness entrypoints stay what the harness requires: Claude and Codex hooks
are shell, OpenCode plugins are JavaScript. Existing bash stays until
migrated. Agent-authored scripts follow the same rule in any project
language: application code matches the project, glue is Go.

## Verification

Run after meaningful changes:

```bash
scripts/validate.sh
```

If you have the Claude Code CLI installed, run its native manifest validator
(unrecognized-field warnings become errors under `--strict`):

```bash
claude plugin validate --strict .claude-plugin/marketplace.json
for d in plugins/*/; do claude plugin validate --strict "$d"; done
```

If you have the Codex `plugin-creator` validator installed locally, run it
against each plugin with a `.codex-plugin/plugin.json`.
