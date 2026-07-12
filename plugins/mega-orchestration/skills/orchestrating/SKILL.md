---
name: orchestrating
description: >-
  Use at task arrival, before non-trivial work, to decide how to structure it:
  inline, fan out subagents, delegate to another model, generate competing
  candidates, convene a council, or start an autonomous run, and how much
  compute it deserves. Triggers on "how should we approach this", "split this
  up", "what's the best way to tackle this", a multi-part or high-stakes task,
  or uncertainty about which orchestration skill applies. This is the decision
  root; the skills it routes to do the work.
license: MIT
---

# Orchestrating

One decision, made once, at task arrival: what structure does this work
deserve? Answer it deliberately instead of drifting into doing everything
inline in one context.

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
step reshapes the next, and anything small enough that coordination costs more
than the work.

## Route by task shape

| Task shape | Structure |
|---|---|
| One clear path, routine stakes | Inline. No structure. |
| 2+ independent tasks, no shared state | megapowers:dispatching-parallel-agents (if installed): one focused agent per task, dispatched together. |
| A written plan of mostly-independent tasks | megapowers:subagent-driven-development (if installed): fresh subagent per task with per-task review. |
| A subtask another model/runtime does better (review, small scoped impl, browser/visual) | mega-orchestration:multi-agent-delegation: resolve the role via `delegates.toml`. |
| Wide solution space, high stakes work product | mega-orchestration:best-of-n: N independent candidates; select by oracle when one can exist, blind judge otherwise. |
| A hard decision, no executable oracle | mega-orchestration:council-adjudication: independent answers, blind ranking, synthesize from the best. |
| A risky claim or diff to trust (billing, auth, concurrency, security) | mega-orchestration:cross-model-verification: a different-vendor model tries to refute it. |
| A long, many-step or multi-session goal with minimal supervision | mega-orchestration:autonomous-run: charter, plan, journal, autonomy dial. |
| An action that leaves the working tree (deploy, send, migrate) | mega-orchestration:effect-broker before acting. |

These compose: an autonomous run's milestones can each run through
subagent-driven-development, best-of-n can use cross-model-verification as its
blind judge, and any structure's risky output earns a verification pass. Route
the outer shape first, then the inner steps as they arrive.

## How much compute: spend by stakes times uncertainty

Anchor the spend: a multi-agent structure runs roughly 15x the token cost of a
single chat (Anthropic's multi-agent research system), so the pay-for-itself
bar is high. Size the fan-out to the question: 1 agent for a fact-find, 2 to 4
for direct comparisons, 10 plus only for wide research.

- Routine and certain: inline, verified by tests.
- Uncertain approach, moderate stakes: one independent review (the
  `code_review` or `plan_review` role), or best-of-n with N=2.
- High stakes (money, auth, data loss, public API): cross-model verification
  is mandatory; a wide solution space also earns best-of-n with N of 3 to 5.
- Long horizon: autonomous-run, with external stop budgets (time, step, or token caps) declared in the charter up front.

Every escalation needs a stopping rule before it starts: an oracle that ends
the search, a candidate cap, or a fix/re-verify attempt cap.

Two rules cut across the ladder:

- Delegate or subagent output that misses the bar: redo it on a stronger model
  or higher effort on your own authority; do not ship it, and do not park the
  task waiting for a human to approve the extra spend. Judge the output, not
  the price tag. Scoped, named defects get a bounded fix pass first;
  structural misses get the redo. One automatic stronger redo per artifact;
  beyond that, a declared cap or a human.
- Nothing that ships routes below the floor declared in delegates.toml
  (`[defaults] floor` in mega-orchestration:multi-agent-delegation).

## Harness primitives

Subagents, agent teams, background tasks, workflow engines, and effort dials
go by different names in each runtime, and not every runtime has all of them.
See [harness-primitives](references/harness-primitives.md) for what each maps
to in Claude Code, Codex, OpenCode, and Antigravity. When a primitive is
missing, fall back to sequential inline work and say so; never fabricate a
call to a primitive the runtime does not have.

## Guardrails

- Decide the structure once, out loud, before dispatching anything. One
  journal or chat line ("structure: SDD, 6 tasks, delegate review per task")
  makes the choice reviewable.
- Single-writer always: whatever the structure, one integrator owns the tree
  and the commits (see mega-orchestration:multi-agent-delegation). On Claude
  Code the harness enforces this from v2.1.198 (no agent message counts as a
  human approval or can change permissions, CLAUDE.md, or config); on Codex,
  OpenCode, and Antigravity that guarantee is skill wording only.
- Re-route when the shape changes: a task that stops decomposing cleanly drops
  back to inline; a task that grows milestones graduates to autonomous-run.
