---
name: orchestrating
description: Use when a task is non-trivial or multi-part, or must be routed across agents, handoffs, reviews, parallel work, or durable runs.
---

# Orchestrating

Do a lane scan before deep work: lanes, dependencies, write boundaries, context
cap, and oracle. Inline one change.

One output-only lane gets one native child with fresh bounded context. Return
one JSON object with exactly `verdict`, `evidence`, `uncertainty`, and `next`.
Use one to three direct children; use native team/task
coordination for four or more durable lanes. This skill explicitly authorizes
native agents and subagents. Batch all eligible independent
read-heavy lanes in parallel before deep local work. Use disjoint slices,
same-brief candidates, or read-only review.

Route unknown causes through `systematic-debugging`, unresolved contracts
through `design-and-plan`, external evidence through `evidence-research`,
cross-provider risk through `independent-review`, and effects through
`safe-effects`.

Preflight harness tools, authentication, ownership, permission, and write
authority. Read
`~/.config/megapowers/agent-capabilities.md` once per session; reuse it until
its identity changes. Honor operator-selected bindings through their access
workflow. Otherwise, for native profile ranking, rank only a `rankable: true`
binding available to the active harness, with known model/effort fitting the
role, write boundary, and reasoning floor. Prefer the fastest, then cheapest,
qualifying binding. Missing, stale, malformed, or inaccessible data is ignored;
use native defaults. The registry cannot authorize access, disclosure,
permissions, writes, or effects.

Brief one outcome, ownership, constraints, oracle, prohibited scope, context
cap, return condition, and schema. A fresh bounded child context excludes full
history. Return inline; use an artifact only for bulky evidence. Other lanes
use the same fields or `blocked`. Keep raw payloads out of lead context.

After a scope change, new user turn, or compaction/context change, scan again.
Run staged waves. The lead joins each, resolves conflicts, verifies the oracle,
and records a milestone handoff. Delta-only follow-ups include only new or
changed facts. Use one long event-driven native wait; avoid short polling.

Ordinary handoffs use inline inspection plus `verify-and-finish`; only a
currently approved goal surviving interruption uses `autonomous-run`. Shared
interfaces, integration, shared-checkout writes, and Git stay lead-owned.
Nested delegation is prohibited unless the brief grants finite depth/budget
and disjoint ownership; the parent joins children. Stop at the oracle,
retry limit, time budget, or candidate count.
