# Harness support matrix

Last reviewed: 2026-08-07.

Three harnesses are targeted: Claude Code, Codex, and OpenCode. They do not have
the same extension surface. Three facts apply across the whole matrix:

- `mega-guardrails` ships hook manifests for Claude Code and Codex. Claude gets
  the destructive guard, injection probe, and formatter; Codex's cross-harness
  dispatcher runs the destructive adapter and makes the others no-ops. OpenCode
  remains skills-only and receives no enforcement.
- Scope is deliberate. A harness earns a section by being one this repo is
  actually run under, not by being able to read a `SKILL.md`. Skills are portable
  markdown, so other harnesses may well load them; that is not the same as being
  supported here, and nothing is tested against one.
- The Gemini CLI was discontinued for consumer use in mid-2026, and Google
  Antigravity was dropped as a target in 2026-08. Visual/browser work routes
  through `playwright-cli` plus a vision-capable model (see
  `mega-orchestration`).

## Claude Code

Status: supported.

- Plugin marketplace: `.claude-plugin/marketplace.json`.
- Plugin manifests: `plugins/*/.claude-plugin/plugin.json`.
- Native components used here: `skills/`, `agents/`, `hooks/`.
- Skill standard: skills follow the Agent Skills open standard (agentskills.io),
  the same `skills/<name>/SKILL.md` layout the other harnesses read; Claude Code
  layers its own frontmatter extensions on top.
- Lightest install: a folder under `~/.claude/skills/` (or `.claude/skills/`)
  that contains a `.claude-plugin/plugin.json` loads as `<name>@skills-dir` the
  next session, with no marketplace or install step.
- Manifest validation: `claude plugin validate <path> --strict` treats warnings
  as errors, so it belongs in CI; `claude plugin eval` runs a plugin's eval
  cases (with a no-plugin baseline arm).
- Skill content lifecycle, which is a design constraint rather than a detail: an
  invoked skill's rendered body enters the conversation once and stays for the
  session, and the file is not re-read on later turns, so anything meant to hold
  for a whole task is a standing instruction, not a step. On compaction, Claude
  Code re-attaches the most recent invocation of each skill after the summary,
  keeping the first 5,000 tokens of each under a combined 25,000-token cap,
  filled newest-first: a long body is truncated and an old skill in a deep stack
  is dropped entirely. `scripts/validate.sh` budgets both.
- Frontmatter beyond the portable set. The Agent Skills spec has six fields
  (`name`, `description`, `license`, `compatibility`, `metadata`,
  `allowed-tools`), and the claude.ai upload, Skills API, and `package_skill.py`
  paths reject anything else. Claude Code additionally reads `context: fork`
  (run the skill as its own background subagent), `paths` (glob-gate automatic
  activation), `disable-model-invocation`, `disallowed-tools`, `arguments`, and
  `argument-hint`. Shipped skills stay portable; the extensions are worth
  adopting per skill, against the cost of leaving that packaging path.
- Dynamic workflows: Claude Code's built-in multi-agent workflow runner is
  separate from these skills. Use it for very large audits, migrations, and
  repeated orchestrated jobs; use these skills for normal process guidance.
  Repeatable multi-agent shapes can be saved to `.claude/workflows/` (shared
  through the repo, invoked as `/<name>`), but plugins cannot ship workflows, so
  the marketplace cannot distribute them; the templates carry examples instead.
  Trust caveat: workflow subagents always run in acceptEdits, so their file
  edits are auto-approved regardless of session mode.
- Recursive SDD: Claude Code supports nested Agent calls. Agent teams are
  outside this mode because they cannot nest. Megapowers requires disjoint path
  ownership in the shared checkout; Claude Code does not enforce that ownership.
  Megapowers runs a plan preflight before dispatch to reject overlapping
  declarations; no registry, scheduler, or worktree manager participates.

## Codex

Status: supported for skills, marketplace metadata, lifecycle hooks, and native
agent role templates.

- Repo instructions: `AGENTS.md`.
- Repo marketplace: `.agents/plugins/marketplace.json`.
- Plugin manifests: `plugins/*/.codex-plugin/plugin.json` for `megapowers`,
  `mega-go`, `mega-python`, `mega-ts`, `mega-orchestration`, and
  `mega-guardrails`, and `mega-frontend`.
- Native role templates: `mega-orchestration/assets/codex-agents/` packages
  Terra-pinned `builder` and `reviewer` profiles to copy into
  `~/.codex/agents/` or a project's `.codex/agents/`. They are optional for
  role-aware Codex surfaces; native v2 does not select them automatically.
- Optional per-skill metadata: Codex reads `agents/openai.yaml` beside a
  skill's `SKILL.md` for interface and policy fields. Setting
  `policy.allow_implicit_invocation: false` prevents implicit activation while
  explicit `$skill-name` invocation still works. This repo pilots that policy
  only on `wayfinding`; other harnesses may ignore the sidecar and discover the
  portable skill normally. The repository validator excludes explicit-only
  skills from Codex's implicit initial-list budget and keeps them in the
  cross-harness upper bound. See OpenAI's
  [Build skills](https://learn.chatgpt.com/docs/build-skills.md) documentation.
- Native multi-agent work: prefer Codex native subagents when running inside
  Codex. The shipped baseline deliberately opts into the under-development v2
  collaboration surface. V2 is same-model context sharding and exposes
  `fork_turns`, but no per-spawn role, model, or effort selector. Its session
  ceiling is ten subagents; the shipped policy keeps ordinary batches to six,
  uses fresh context for independent work, leaves ordinary fan-out spawning and
  integration with the root, and requires gating workers to return before
  completion. Codex
  0.144.4 does not hard-enforce `agents.max_depth` under v2, so the template
  supplies a model-visible policy that stops nesting at depth five instead.
- Recursive SDD: Codex supports native nested subagents. Megapowers requires
  disjoint path ownership in the shared checkout; Codex does not enforce that
  ownership or the Git restrictions. Megapowers runs a plan preflight before
  dispatch; no registry, scheduler, or worktree manager participates.
- From Claude Code, prefer OpenAI's first-party
  [`codex-plugin-cc`](https://github.com/openai/codex-plugin-cc) for Codex
  review, adversarial review, rescue, transfer, and background job management.
  It uses the local Codex CLI, app server, authentication, and configuration.
- Other harnesses can reach Codex through `codex exec`, the Codex SDK, or
  `codex mcp-server`. Full channel and sandbox-auth mechanics live in
  `mega-orchestration`'s `references/providers/codex.md`; a starter MCP
  registration ships as `templates/codex-mcp-settings.json`.
- `mega-guardrails` supplies the Codex destructive-command hook. Its formatter
  and statusline remain Claude Code-only.

## OpenCode

OpenCode support is portable-skill compatibility. This repository ships no
OpenCode plugin, launcher, credentials bridge, or review-receipt adapter; use
OpenCode's own agent and permission configuration for those runtime features.

Status: supported through shared instructions and portable skills.

- Repo instructions: `AGENTS.md`.
- Skill format: `skills/<name>/SKILL.md`. `name` must match the directory name
  (regex `^[a-z0-9]+(-[a-z0-9]+)*$`), and `description` is capped at 1024
  characters; every skill here validates.
- Installation: `npx skills add lawzava/megapowers` (the skills CLI discovers
  this repo's skills through the marketplace manifest), or copy/symlink
  selected canonical skill directories into any discovery path below.
- Discovery paths (project paths walk up to the git root):

  | Scope   | Paths                                                                   |
  |---------|-------------------------------------------------------------------------|
  | Project | `.opencode/skills/`, `.claude/skills/`, `.agents/skills/`               |
  | Global  | `~/.config/opencode/skills/`, `~/.claude/skills/`, `~/.agents/skills/`  |

  OpenCode invokes skills through a native `skill` tool, gated by a
  `permission.skill` config (allow / ask / deny patterns, per agent). The
  `~/.claude/skills/` and `~/.agents/skills/` fallbacks can be turned off with
  `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS` when you want OpenCode to read only its
  own paths.
- Plugins: OpenCode plugins are JavaScript or TypeScript modules with
  `tool.execute.before/after` hooks, so a guardrail port is feasible here. This
  repo does not ship one yet; the current shell hooks are Claude Code scripts
  and have not been ported.

## Operating systems

Skills are plain markdown and work wherever the host tool runs. Hooks and most
helpers are Bash with jq, git, and grep; the eval scorer is Go. CI exercises
Linux.
macOS is expected to work but is not CI-covered. Windows is untested: native
Windows cannot run the shell helpers, while Git Bash and WSL have not been
verified. The `run-hook.cmd` wrapper finds Git Bash for SessionStart and no-ops
when Bash is absent. Treat hook enforcement as unverified on Windows; the
skills themselves remain portable.
