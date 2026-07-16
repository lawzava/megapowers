# Harness support matrix

Last reviewed: 2026-07-15.

This repo is cross-harness, but not every harness has the same extension
surface. Two facts apply across the whole matrix:

- `mega-guardrails` ships hook manifests for Claude Code and Codex. Claude gets
  the destructive guard and formatter; Codex's cross-harness dispatcher runs
  the destructive adapter and makes the formatter a no-op. OpenCode and
  Antigravity remain skills-only and receive no enforcement.
- The Gemini CLI was discontinued for consumer use in mid-2026 and is no
  longer a target. Visual/browser work routes through `playwright-cli` plus a
  vision-capable model (see `mega-orchestration`).

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
- Dynamic workflows: Claude Code's built-in multi-agent workflow runner is
  separate from these skills. Use it for very large audits, migrations, and
  repeated orchestrated jobs; use these skills for normal process guidance.
  Repeatable multi-agent shapes can be saved to `.claude/workflows/` (shared
  through the repo, invoked as `/<name>`), but plugins cannot ship workflows, so
  the marketplace cannot distribute them; the templates carry examples instead.
  Trust caveat: workflow subagents always run in acceptEdits, so their file
  edits are auto-approved regardless of session mode.
- Recursive SDD: nested Agent calls support a coordinator tree to depth five;
  agent teams cannot nest and are not used for this path. See
  `megapowers:subagent-driven-development` for the shared run registry,
  coordinator ownership, and linked-worktree contract. Dynamic workflows
  remain available for their existing use cases but are not a recursive SDD
  dependency.

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
  uses fresh context for independent work, and requires gating workers to
  return before completion. Under the model-visible depth-five hint, native v2
  also supports an explicitly selected recursive coordinator tree while
  per-spawn model selection remains unavailable. See
  `megapowers:subagent-driven-development` for the shared run registry,
  coordinator ownership, and linked-worktree contract.
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

## Google Antigravity

Status: supported as a skills target; CLI plugin manifests ship.

- Terminal harness: the Antigravity CLI (`agy`) is the successor to the
  discontinued Gemini CLI. Migrators can run `agy plugin import gemini` to
  register existing Gemini CLI extensions.
- Native skill shape: the nested `skills/<name>/SKILL.md` layout is
  Antigravity's native format, so this repo's canonical skills import as-is with
  no conversion. Project skills live in `<workspace>/.agents/skills/<name>/`;
  the global path read by all Antigravity flavors (IDE, CLI, Agent Manager) is
  `~/.gemini/config/skills/<name>/`. `description` is required; `name` defaults
  to the directory name.
- CLI plugin manifests: `plugins/*/plugin.json` ship for `megapowers`,
  `mega-go`, `mega-python`, `mega-ts`, `mega-orchestration`, and
  `mega-frontend`;
  `mega-guardrails` is not offered (see the note at the top).
- Verify a manual install with the same first-task probe used elsewhere:
  install one plugin or skill, then in a fresh Antigravity session ask the agent
  to load `test-driven-development` and quote its core-principle sentence (see
  [`setup.md`](./setup.md)). A correct quote proves discovery and loading.
- Command glossary: `/agents` (Agent Manager: subagent approvals and activity),
  `/tasks` (Task Manager: shell execution logs), `/skills` (browse loaded local
  and global skills), `/hooks` (browse active hooks), and `/artifact` (review
  agent-produced files, plans, and diffs). Antigravity keeps its implementation
  plans and walkthroughs in its own artifact/scratch area rather than
  guaranteed repo-local files, so a file contract that expects repo-local files
  (for example autonomous-run journals) should not assume the harness writes
  them into the project tree.
- Disambiguation: command names do not port across harnesses. Antigravity's
  `/agents` opens the Agent Manager; Claude Code's `/agents` is unrelated.

## Operating systems

Skills are plain markdown and work wherever the host tool runs. Hooks and most
helpers are Bash with jq, git, and grep. The optional brainstorming visual
companion is a local Node server; the eval scorer is Go. CI exercises Linux.
macOS is expected to work but is not CI-covered. Windows is untested: native
Windows cannot run the shell helpers, while Git Bash and WSL have not been
verified. The `run-hook.cmd` wrapper finds Git Bash for SessionStart and no-ops
when Bash is absent. Treat hook enforcement as unverified on Windows; the
skills themselves remain portable.
