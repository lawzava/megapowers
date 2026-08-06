---
name: orchestrating
description: >-
  Use before a non-trivial, multi-part, or high-stakes task to decide how to
  approach, split, delegate, compare candidates, or run autonomously. This is
  the routing skill.
license: MIT
---

# Orchestrating

One decision, made once, at task arrival: what structure does this work
deserve? Answer it deliberately instead of drifting into one inline context.

## First decision: structure at all?

Inline, solo work is the default. Structure costs briefing, integration, and
review, so it must pay for itself. Split or delegate only when at least one of
these holds:

- Independent subtasks exist that do not reshape each other.
- Bulk reads or long execution would drown the context that has to decide
  later. Protect the orchestrator's context; spend subagent context.
- A different model or runtime is demonstrably better at a subtask.
- Stakes times uncertainty justify multiple attempts or independent checks.

Keep inline anything you cannot yet decompose, any critical path where each
step reshapes the next, and anything small enough that coordination costs more.

## Route by task shape

| Task shape | Structure |
|---|---|
| One clear path, routine stakes | Inline. No structure. |
| Long-horizon work with unknown ownership, unresolved decisions, or unclear sequencing that prevents an honest spec or plan | mega-orchestration:wayfinding: map uncertainty and resolve the next decision before design or planning. |
| 2+ independent tasks, no shared state | Parallel fan-out (below): one focused agent per task, dispatched together. |
| Deterministic mechanical changes sharing one oracle | Bulk mechanical mode: one owner, one bounded batch, one focused verification set. |
| A written plan of mostly-independent tasks | megapowers:subagent-driven-development (if installed): fresh subagent per task with per-task review. |
| A subtask another model/runtime does better (review, small scoped impl, browser/visual) | Delegate through the two scripts below; delegate-run launches the review roles, the rest dispatch on the route delegate-resolve prints. |
| Wide solution space, high stakes work product | mega-orchestration:best-of-n: N independent candidates; select by oracle when one can exist, blind judge otherwise. |
| A hard decision, no executable oracle | mega-orchestration:council-adjudication: independent answers, blind ranking, synthesize from the best. |
| A risky claim or diff to trust (billing, auth, concurrency, security) | mega-orchestration:cross-model-verification: `delegate-run --role verify` has a different-vendor model try to refute it. |
| A long, many-step or multi-session goal with minimal supervision | mega-orchestration:autonomous-run: charter, plan, journal, autonomy dial. |
| An action that leaves the working tree (deploy, send, migrate) | mega-orchestration:effect-broker before acting. |

These compose: an autonomous run's milestones can each run through
subagent-driven-development, best-of-n can use cross-model-verification as its
blind judge. Route the outer shape first, then the inner steps as they arrive.

## Delegated work: one path

Work that leaves your context for another model or runtime goes through
mega-orchestration:multi-agent-delegation's two scripts, called from Bash. They
are the whole interface: reading the routing table is a script call, not a
delegate subagent to dispatch.

- `scripts/delegate-resolve <role>` prints the route the config picks and exits
  nonzero when there is none, so a dead route surfaces before dispatch. Name
  every artifact author with `--author-vendor` on an independence role.
- `scripts/delegate-run --role ROLE --author-vendor VENDOR --artifact worktree
  --claim TEXT` resolves and runs a read-only review in one call, demands the
  verdict schema, and writes a receipt bound to the tree it read. Branch on its
  exit code; a review that produced no receipt did not happen.

Escape hatch: a route delegate-run cannot launch still resolves. small_impl is
`self`: run it as an ordinary subagent of your own harness. visual and
browser_test are `cli` on a computer-use provider: dispatch the printed channel
from Bash. Brief the spec and the acceptance test, then re-run the tests
yourself. Load multi-agent-delegation for the provider and driver references
those dispatches need, or when an exit code or a route needs explaining.

## Parallel fan-out

For 2+ independent, parallel-safe tasks: one agent per task, all dispatch
calls in one response so they run concurrently. Every brief is focused (one
scoped task, constraints on what not to touch), self-contained (paths,
identifiers, error messages; pin fresh context explicitly where the harness
supports it), and explicit about what to return. You own integration: review
each summary, check for conflicts, and run the full verification suite
yourself. If one task's outcome could reshape another, handle them in one
agent; a written plan of tasks belongs to megapowers:subagent-driven-development.

For bulk mechanical mode, partition only when the oracle or ownership genuinely
differs. One-file-per-agent fan-out multiplies briefing and review cost.

## How much compute: spend by stakes times uncertainty

Anchor the spend: a multi-agent structure runs roughly 15x the token cost of a
single chat (Anthropic's multi-agent research system), so the bar is high. Size
the fan-out to the question: 1 agent for a fact-find, 2 to 4 for a comparison,
10 plus only for wide research.

- Routine and certain: inline, verified by tests.
- Uncertain approach, moderate stakes: one independent review
  (`delegate-run --role code_review`, or `plan_review`), or best-of-n with N=2.
- High stakes (money, auth, data loss, public API): cross-model verification
  is mandatory; a wide solution space also earns best-of-n with N of 3 to 5.
- Long horizon: autonomous-run, with external stop budgets (time, step, or token caps) declared in the charter up front.

Every escalation needs a stopping rule before it starts: an oracle that ends
the search, a candidate cap, or a fix/re-verify attempt cap. Two more rules cut
across the ladder:

- Delegate or subagent output that misses the bar: redo it on a stronger model
  or higher effort on your own authority, without parking the task for a human
  to approve the spend. Judge the output, not the price tag. Named, scoped
  defects earn a bounded fix pass first; structural misses earn the redo. One
  automatic redo per artifact, then a declared cap or a human.
- Nothing that ships routes below the floor declared in models.toml
  (`[defaults] floor` in mega-orchestration:multi-agent-delegation).

After dispatch, wait for completion, blocked, needs-context, failure, or user
interruption. Do not poll unchanged state on a timer: inspect on a reported
transition or a bounded stuck-worker timeout, and prefer a native watcher.

## Harness primitives

Subagents, agent teams, background tasks, workflow engines, and effort dials go
by different names in each runtime, and not every runtime has all of them. See
[harness-primitives](references/harness-primitives.md) for what each maps to in
Claude Code, Codex, OpenCode, and Antigravity. When one is missing, fall back to
sequential inline work and say so; never fabricate a call it does not expose.

## Guardrails

- Decide the structure once, out loud, before dispatching anything. One journal
  or chat line ("structure: SDD, 6 tasks, delegate review per task") makes the
  choice reviewable.
- Use `fork_turns = "none"` by default. A positive count is exceptional, limited
  to the indispensable recent continuation, at most three turns; `all` is only
  for an explicit same-context resumption.
- Give each delegate one report channel. Small results return directly; bulky
  results go to a file and return only status plus the path.
- Single-writer always: whatever the structure, one integrator owns the tree
  and the commits (see mega-orchestration:multi-agent-delegation). Claude Code
  stopping teammate messages from counting as approval does not make ordinary
  subagents or workflow agents single-writer. Enforce it with sandbox, tool,
  and worktree controls where available, plus explicit skill wording.
- Re-route when the shape changes: a task that stops decomposing cleanly drops
  back to inline; a task that grows milestones graduates to autonomous-run.
