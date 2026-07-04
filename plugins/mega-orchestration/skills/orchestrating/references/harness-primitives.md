# Harness primitives

What each orchestration concept maps to per runtime, as of 2026-07-04. Names
and availability drift with releases; when a primitive is absent or you cannot
confirm it, fall back to sequential inline work and say so. Never fabricate a
call to a primitive the runtime does not expose.

## Claude Code

- **Subagents**: the Agent tool. Multiple Agent calls in one message run in
  parallel; subagents run in the background by default and can nest to a fixed
  depth of 5. Forks (`/fork`, on by default) inherit the full conversation and
  share the parent's prompt cache, so they are cheaper than fresh subagents for
  same-context candidates. A stopped subagent auto-resumes when it receives a
  SendMessage and keeps its full history, so re-dispatch-with-recap is obsolete.
  This is the surface dispatching-parallel-agents and subagent-driven-development
  use.
- **Agent teams**: experimental and disabled by default, gated behind
  `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`. Without it no team is set up and no
  teammates spawn, so do not route to teams in a default session. Limits: one
  team per session, no nested teams, in-process teammates cannot be resumed. For
  a role that persists across many exchanges, use a resumable background subagent
  instead.
- **Workflows**: the trigger keyword is `ultracode` (also `/effort ultracode`).
  Use for large audits, migrations, and repeated many-agent jobs; use skills for
  normal process guidance. Caveat: workflow subagents always run in acceptEdits,
  so file edits are auto-approved regardless of session mode; single-writer and
  review disciplines must account for that. Caps: 16 concurrent agents, 1000
  agents per run; resume works only within the same session.
- **Effort**: `/effort` low..max session dial, a per-subagent `effort` override
  on dispatch, and `/fast` on Opus-class models. Spend high effort on
  verify/judge/decide steps, low on mechanical ones.
- **Scheduling and unattended**: cloud routines (`/schedule`: cron, HTTP
  endpoint, and GitHub triggers) run fully autonomously with no permission
  prompts, so effect-broker gating must live in the routine prompt. `/loop`
  covers in-session recurrence. Maps to mega-orchestration:autonomous-run, with
  that trust caveat.
- **Isolation**: git worktrees (megapowers:using-git-worktrees), or the Agent
  tool's `isolation: worktree` option on dispatch.

## Codex

- **Subagents**: native, parallel, and Codex-orchestrated. Roles are TOML files
  in `~/.codex/agents/` and `.codex/agents/` (built-ins: default, worker,
  explorer); `[agents] max_threads` defaults to 6. Codex runs the
  spawn/route/wait/close loop itself via spawn_agent / send_input / resume_agent
  (stable, on by default). Per-thread delegation modes (disabled,
  explicit-request-only, proactive; default explicit-request-only) since
  v0.142.0. Do not degrade to sequential dispatches here; fan out to native
  subagents.
- **Channel from another runtime**: reach Codex via `codex exec`, the Codex SDK,
  or `codex mcp-server` (the first-party MCP channel exposing the codex /
  codex-reply tools). See multi-agent-delegation channel notes.
- **Teams / workflows**: no distinct primitive this repo relies on; native
  subagent fan-out covers it.
- **Effort**: `model_reasoning_effort` is set per dispatch or per role (config
  or call), not a global constant. Spend high on verify/judge/decide steps.

## OpenCode

- **Subagents**: markdown agent files in `.opencode/agents/` (or
  `~/.config/opencode/agents/`) with per-agent model overrides, so cross-model
  delegation works natively without a bridge. Read-only built-ins explore (code
  navigation) and scout (external docs) fit research fan-out. The `task`
  permission gates which subagents an agent may spawn (patterns like
  `{"*":"deny","orchestrator-*":"allow"}`), giving single-writer enforcement at
  the harness level rather than by prompt discipline alone.
- **Skills**: a native `skill` tool, gated by `permission.skill` (allow / ask /
  deny). Discovery uses six paths (project and global); they are named in
  docs/harness-support.md and not duplicated here.
- **Effort**: no numeric dial; set a stronger or weaker per-agent model instead.

## Antigravity

- **Subagents**: the main agent spawns parallel subagents with per-subagent
  workspace isolation (it auto-creates worktrees for them and cleans up
  afterward). Maps to megapowers:dispatching-parallel-agents. Oversight is the
  Agent Manager surface.
- **Scheduling and unattended**: Scheduled Tasks run cron-style prompts
  periodically (via `/schedule`); maps to mega-orchestration:autonomous-run
  recurrence. Scheduled-task agents are pinned to Gemini 3.5 Flash.
- **Effort**: Plan vs Fast agent modes (Plan for multi-step planning, Fast for
  quick tasks). Antigravity also ships native hooks and skills, but its command
  vocabulary differs from Claude Code.
- **Models**: a multi-vendor roster (Gemini 3.5 Flash default, Gemini 3.1 Pro,
  Claude Sonnet/Opus 4.6, gpt-oss-120b) chosen per agent. Subagents inherit the
  parent's model, so scale by choosing the lead model rather than assigning
  vendors per subagent within one job.
- **Disambiguation**: command names do not port across harnesses. Antigravity
  manages subagents through the Agent Manager; Claude Code's `/agents` is
  unrelated (and no longer an interactive wizard as of v2.1.198).

## The absent-primitive rule

If the shape you routed to needs a primitive the runtime lacks (no parallel
subagents, no background tasks), degrade honestly: run the same steps
sequentially in the lead context, keep the same review and single-writer
discipline, and note the degradation in your status or journal line. The
structure is negotiable; the discipline is not.
