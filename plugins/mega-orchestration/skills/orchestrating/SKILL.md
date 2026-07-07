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

One decision, made once, at the start: what structure does this task deserve?
Answer it deliberately here instead of drifting into the default of doing
everything inline in one context.

## First decision: structure at all?

Inline, solo work is the default. Structure has a cost (briefing, integration,
review), so it must pay for itself. Split or delegate only when at least one of
these holds:

- Independent subtasks exist that do not reshape each other.
- Bulk reads or long execution would drown the context that has to make
  decisions later. Protect the orchestrator's context; spend subagent context.
  Context is a finite resource with diminishing returns; what the deciding
  context needs is the smallest set of high-signal tokens that still decides
  well.
- A different model or runtime is demonstrably better at a subtask.
- Stakes times uncertainty justify multiple attempts or independent checks.

Do not split exploratory work you cannot yet decompose, a single critical path
where each step reshapes the next, or work so small that coordination costs
more than doing it.

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

These compose. An autonomous run's milestones can each run through
subagent-driven-development; best-of-n uses cross-model-verification as its
blind judge; any structure's risky output earns a verification pass. Route the
outer shape first, then the inner steps as they arrive.

## How much compute: spend by stakes times uncertainty

Anchor the spend: a multi-agent structure runs roughly 15x the token cost of a
single chat (Anthropic's multi-agent research system), so the pay-for-itself bar
is high. Size the fan-out to the question: 1 agent for a simple fact-find, 2-4
for direct comparisons, 10+ only for wide research.

- Routine and certain: inline, verified by tests. Nothing more.
- Uncertain approach, moderate stakes: one independent review (the `code_review`
  or `plan_review` role), or best-of-n with N=2.
- High stakes (money, auth, data loss, public API): cross-model verification is
  mandatory; wide solution space also earns best-of-n with N=3-5.
- Long horizon: autonomous-run, with budget and turn caps declared in the
  charter up front.

Every escalation needs a stopping rule before it starts: an oracle that ends
the search, a candidate cap, or a fix/re-verify attempt cap. Unbounded
structure is how compute disappears without a decision getting better.

Two rules cut across the ladder:

- Output back from a delegate or subagent that misses the bar: redo it on a
  stronger model or higher effort on your own authority; do not ship it, and
  do not park the task waiting for a human to approve the extra spend. Judge
  the output, not the price tag. Scoped, named defects earn a bounded fix pass
  first; structural misses earn the redo. One automatic stronger redo per
  artifact; going past it needs a declared cap or a human.
- Nothing that ships routes below the floor declared in delegates.toml
  (`[defaults] floor` in mega-orchestration:multi-agent-delegation).

## Harness primitives

Subagents, agent teams, background tasks, workflow engines, and effort dials go
by different names in each runtime, and not every runtime has all of them. See
[harness-primitives](references/harness-primitives.md) for what each maps to in
Claude Code, Codex, OpenCode, and Antigravity, and for the rule when a
primitive is missing: fall back to sequential inline work and say so; never
fabricate a call to a primitive the runtime does not have.

## Guardrails

- Decide the structure once, out loud, before dispatching anything. A one-line
  journal or chat note ("structure: SDD, 6 tasks, Codex review per task") makes
  the choice reviewable.
- Single-writer always: whatever the structure, one integrator owns the tree
  and the commits (see mega-orchestration:multi-agent-delegation). On Claude
  Code the harness enforces this from v2.1.198 (no agent message counts as a
  human approval or can change permissions, CLAUDE.md, or config); on Codex,
  OpenCode, and Antigravity that guarantee is skill wording only.
- Re-route when the shape changes. A task that stops decomposing cleanly drops
  back to inline; a task that grows milestones graduates to autonomous-run.
