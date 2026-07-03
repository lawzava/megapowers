# Repository Instructions

## Scope

This repository publishes optional skills, plugins, hooks, and templates for
Claude Code, Codex, OpenCode, and Google Antigravity. Keep it public-safe:
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
  `.opencode/`, `.agents/`, or Claude-compatible paths; its plugins are
  JavaScript or TypeScript modules, not these Claude shell hooks.
- Antigravity CLI plugins use a root `plugin.json`; its native skills are flat
  markdown files under `.agents/skills/` or a plugin `skills/` directory.
- Full per-harness support details: `docs/tool-support.md` (canonical,
  freshness-checked).

## Verification

Run after meaningful changes:

```bash
scripts/validate.sh
```

If you have the Codex `plugin-creator` validator installed locally, run it
against each plugin with a `.codex-plugin/plugin.json`.
