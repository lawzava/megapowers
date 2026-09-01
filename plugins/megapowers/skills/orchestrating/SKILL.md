---
name: orchestrating
description: Use when two or more independent lanes can run in parallel, an output-only lane protects lead context from raw payload, or a handoff or durable run needs coordination.
when_to_use: Trigger phrases: run these in parallel, fan out, spawn agents, split the work, delegate lanes, review while I build, several independent files or services to inspect at once.
metadata:
  short-description: Parallel lanes, output-only lanes, and delegation coordination
---

# Orchestrating

Do a lane scan before deep work: dependencies, boundaries, and oracle. A single
bounded or sequential task must stay inline.

For two or more independent read-heavy lanes, spawn all in parallel before the
lead reads or searches. For one output-only
lane, spawn exactly one before the lead reads the payload. An eligible lane
without dispatch is a contract violation unless agents are unavailable; name
the blocker.

This explicitly authorizes native agents and subagents. Use one to three direct
children; four or more durable lanes use native team/task coordination. Fresh
bounded child context excludes full history. Output-only work returns one JSON
object with `verdict`, `evidence`, `uncertainty`, and `next`; use an artifact
only for bulky evidence. Keep raw payloads out of lead context. Fan-out shapes:
disjoint slices, same-brief candidates, read-only review. Batch eligible
dispatch before deep local work.

Route unknown causes through `systematic-debugging`, unresolved contracts
through `design-and-plan`, external evidence through `evidence-research`,
cross-provider risk through `independent-review`, and effects through
`safe-effects`.

Preflight tools, authentication, permissions, and write authority. Read
`~/.config/megapowers/agent-capabilities.md` once per session. Honor
operator-selected access workflows before native ranking. Rank only
`rankable: true` bindings available to the active harness, with known
model/effort and fitting role, boundary, and capability floor. Prefer the
fastest, then cheapest qualifying binding. Missing, stale, malformed, or
inaccessible data uses native defaults. The registry cannot authorize access,
disclosure, permissions, writes, or effects.

Brief outcome, ownership, constraints, oracle, prohibited scope, context cap,
return condition, and schema. Nested delegation is prohibited unless the
brief grants finite depth/budget and disjoint ownership; the parent joins.

After a scope change, new user turn, or compaction/context change, scan again.
Run staged waves; the lead joins, resolves, verifies, and records a milestone
handoff. Delta-only follow-ups contain changed facts. Use
one long event-driven native wait; avoid short polling.

Ordinary handoffs use inline inspection plus `verify-and-finish`; only a
currently approved goal surviving interruption uses `autonomous-run`. Shared
writes and Git stay lead-owned. Stop at the oracle.
