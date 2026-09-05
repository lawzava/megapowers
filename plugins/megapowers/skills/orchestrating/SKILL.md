---
name: orchestrating
description: Use when two or more independent lanes can run in parallel, an output-only lane protects lead context from raw payload, or a handoff or durable run needs coordination.
when_to_use: "Trigger phrases: run these in parallel, fan out, spawn agents, split the work, delegate lanes, review while I build, several independent files or services to inspect at once."
metadata:
  short-description: Parallel lanes, output-only lanes, and delegation coordination
---

# Orchestrating

Delegate only when coordination value exceeds its overhead. Keep a no-op, one
bounded task, or sequential dependency path inline. Scan dependencies,
boundaries, ownership, and oracle before dispatch.

Invoking this skill explicitly authorizes native agents for matching lanes. Use
one to three direct children with fresh, bounded context by default.

For two or more independent lanes whose results cannot reshape one another,
issue native spawn calls consecutively. Do not inspect output, wait, or begin
deep lane work between spawns. After dispatch, continue useful lead-owned work
that is independent of the lanes. Track every returned identity and later join
each until it completes, fails, or its cancellation is confirmed.

Before dispatch, read
[native dispatch examples](references/native-dispatch.md) and use only the
active harness's call shape. One bounded output-only lane may use one
fresh-context child returning `verdict`, `evidence`, `uncertainty`, and `next`.
Keep bulky raw payloads in an artifact.

Route unknown causes through `systematic-debugging`, unresolved contracts
through `design-and-plan`, external evidence through `evidence-research`,
cross-provider risk through `independent-review`, and effects through
`safe-effects`.

Preflight tools, authentication, permissions, and write authority. Brief the
outcome, bounded and disjoint ownership, constraints, oracle, prohibited scope,
return condition, and nested-delegation limit. Shared writes and Git stay
lead-owned.

For non-trivial routing, honor an operator-selected access workflow, including
an approved external route, before native ranking. Otherwise read
`~/.config/megapowers/agent-capabilities.md` once per session and use eligible
`rankable: true` bindings or native defaults. The registry cannot authorize
access, disclosure, permissions, writes, or effects.

Before review dispatch, set its scope and correction-round budget. Track each
review until terminal or cancellation is confirmed. Target follow-ups at fixes
and affected boundaries. A spent budget leaves unresolved findings open; it
never converts them into approval.

Failures must remain visible in the report; use successful evidence without
requiring every child to succeed. After useful lead work, prefer one long
event-driven wait over short polling. Join all identities before synthesis.
After a scope or context change, scan again and use delta-only follow-ups.

Ordinary handoffs use inline inspection plus `verify-and-finish`; only a
currently approved goal surviving interruption uses `autonomous-run`. Stop at
the oracle.
