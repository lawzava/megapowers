# Tool Support Matrix

Last reviewed: 2026-07-02.

This repo is intentionally cross-tool, but not every runtime has the same
extension surface.

`mega-guardrails` is Claude Code only: its value is Claude Code hook scripts
(PreToolUse/PostToolUse) plus a Linux statusline, none of which run on Codex,
OpenCode, or Antigravity. It therefore ships no Codex, OpenCode, or Antigravity
manifest — installing it elsewhere would advertise protection that does not
exist. Note also that the Gemini CLI was discontinued for consumer use in
mid-2026 and is no longer a target; visual/browser work routes through
`playwright-cli` plus a vision-capable model (see `mega-orchestration`).

## Claude Code

Status: supported.

- Plugin marketplace: `.claude-plugin/marketplace.json`.
- Plugin manifests: `plugins/*/.claude-plugin/plugin.json`.
- Native components used here: `skills/`, `agents/`, `hooks/`.
- Modern workflows: Claude Code dynamic workflows are separate from these
  skills. Use them for very large audits, migrations, and repeated orchestrated
  jobs; use these skills for normal process guidance.

## Codex

Status: supported for skills and marketplace metadata.

- Repo instructions: `AGENTS.md`.
- Repo marketplace: `.agents/plugins/marketplace.json`.
- Plugin manifests: `plugins/*/.codex-plugin/plugin.json` for `megapowers`,
  `mega-go`, `mega-python`, `mega-ts`, and `mega-orchestration`.
- Native multi-agent work: prefer Codex subagents when running inside Codex.
  From other tools, use `codex exec` or an explicitly configured private bridge.
- Record & Replay is a Codex app workflow creator; it is not required for this
  repository.
- `mega-guardrails` is intentionally not listed for Codex because its existing
  hooks are Claude Code hook scripts.

## OpenCode

Status: supported through shared instructions and portable skills.

- Repo instructions: `AGENTS.md`.
- Skill format: `skills/<name>/SKILL.md` with `name` and `description`.
- Installation: `npx skills add lawzava/megapowers` (the skills CLI discovers
  this repo's skills through the marketplace manifest), or copy/symlink
  selected canonical skill directories into `.opencode/skills/`,
  `.agents/skills/`, or another OpenCode-supported skill path.
- Plugins: OpenCode plugins are JavaScript or TypeScript modules. This repo does
  not ship an OpenCode plugin module because the current shell hooks are
  Claude-specific.

## Google Antigravity

Status: documented with minimal manifests.

- CLI plugin manifests: `plugins/*/plugin.json`.
- Native skill shape: flat markdown files in `.agents/skills/` or plugin
  `skills/`.
- This repo keeps canonical skills in the open agent skill layout
  `skills/<name>/SKILL.md`. Convert or symlink only the specific skills you need
  if your Antigravity CLI does not import that nested layout directly.
- CLI plugin manifests ship for `megapowers`, `mega-go`, `mega-python`, `mega-ts`,
  and `mega-orchestration`. `mega-guardrails` is intentionally not offered here
  because its hooks are Claude Code hook scripts and would provide no active
  enforcement on Antigravity.
- Current Antigravity workflow concepts to mirror in docs: `/agents` for
  subagent management, `/tasks` for background processes, and `/artifact` for
  reviewable plans, diffs, screenshots, and approvals.

## Operating systems

Skills are plain markdown and work wherever the host tool runs. Everything
executable — hooks, skill helper scripts, the eval harness — is bash (plus
jq/git/grep), developed on Linux and exercised in CI on Linux only. macOS is
expected to work (the destructive-command guard knows macOS paths and device
names) but is not CI-covered. **Windows is untested**: the hooks and helper
scripts have not been run under Git Bash or WSL, and native Windows (no bash)
will not execute them at all. The `run-hook.cmd` polyglot wrapper finds Git
Bash on Windows and runs the SessionStart hook through it (silent no-op when
no bash exists), but the hook scripts themselves have never been verified
under Git Bash or WSL. Treat hook-based enforcement as unverified on Windows
until someone tests it; the skills themselves still work.
