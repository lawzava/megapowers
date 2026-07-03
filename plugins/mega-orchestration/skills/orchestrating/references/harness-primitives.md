# Harness primitives

What each orchestration concept maps to per runtime. Names and availability
drift with releases; when a primitive is absent or you cannot confirm it,
fall back to sequential inline work and say so. Never fabricate a call to a
primitive the runtime does not expose.

## Claude Code

- **Subagents**: the Agent tool. Multiple Agent calls in one message run in
  parallel; this is the surface dispatching-parallel-agents and
  subagent-driven-development use.
- **Agent teams**: named, long-lived teammates addressable by SendMessage.
  Worth it only for roles that persist across many exchanges; for one-shot
  tasks, plain subagents are cheaper.
- **Workflows**: the Workflow tool runs a deterministic orchestration script
  (fan-out, pipelines, structured outputs). Use for large audits, migrations,
  and repeated many-agent jobs; use skills for normal process guidance.
- **Effort**: session-level effort setting, plus per-agent effort overrides on
  dispatch where the harness offers them. Spend high effort on
  verify/judge/decide steps, low on mechanical ones.
- **Isolation**: git worktrees (megapowers:using-git-worktrees when installed),
  or the harness's own worktree isolation option on agent dispatch.

## Codex

- **Subagents**: native Codex subagents when running inside Codex. From another
  runtime, reach Codex via `codex exec`, the SDK, or an explicitly configured
  bridge (see multi-agent-delegation's channel notes).
- **Teams / workflows**: no equivalent this repo relies on. Structure
  multi-step work as sequential subagent dispatches from the lead.
- **Effort**: `model_reasoning_effort` is set per dispatch (config or call
  options). Treat it as a per-task decision, not a global constant.

## OpenCode

- **Subagents**: available; skills load through Claude-compatible paths (see
  docs/tool-support.md in this repo).
- **Teams / workflows / hooks**: not provided by this repo for OpenCode. The
  disciplines ride on skill text alone.
- **Effort**: no documented dial this repo depends on; choose a stronger or
  weaker model per task instead.

## Antigravity

- **Subagents**: `/agents` manages subagents.
- **Background tasks**: `/tasks` runs background processes.
- **Review surfaces**: `/artifact` holds reviewable plans, diffs, screenshots,
  and approvals; route human checkpoints through it.
- **Effort**: no documented dial this repo depends on; scale by model choice
  and by how many independent passes you run.

## The absent-primitive rule

If the shape you routed to needs a primitive the runtime lacks (no parallel
subagents, no background tasks), degrade honestly: run the same steps
sequentially in the lead context, keep the same review and single-writer
discipline, and note the degradation in your status or journal line. The
structure is negotiable; the discipline is not.
