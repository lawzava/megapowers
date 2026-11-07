---
name: orchestrating
description: "Use when two or more independent lanes can run in parallel, an output-only lane protects lead context from raw payload, or a handoff or durable run needs coordination. Covers fan out, spawn agents, split the work, delegate lanes, review while building, and independent files or services to inspect at once."
metadata:
  short-description: Parallel lanes, output-only lanes, and delegation coordination
---

# Orchestrating

Honor explicit delegation requests. Otherwise, delegate only when coordination
value exceeds its overhead. Keep a no-op, one bounded task, or sequential
dependency path inline. Scan dependencies, boundaries, ownership, and oracle
before dispatch.

Native agents are the default for matching lanes. Use one to three direct
children with fresh, bounded context. Dispatch independent lanes before waiting,
continue independent lead work, and join every identity through completion,
failure, or confirmed cancellation. Start a planned task once its prerequisites
have passed their oracle and ownership is disjoint from running lanes; do not
wait for a whole wave.

Before dispatch, read
[native dispatch and lane lifecycle](references/native-dispatch.md) and use
the harness's native subagent tool and isolation, with sequential fallback.
One bounded output-only lane may use one
fresh-context child returning `verdict`, `evidence`, `uncertainty`, and `next`.
Keep bulky payloads in artifacts. For verbose checks use
[run receipts](../verify-and-finish/references/run-receipt.md).

Preflight tools, authentication, permissions, and write authority. Brief the
outcome, disjoint ownership, constraints, oracle, prohibited scope, return
condition, nested-delegation limit, and an advisory time budget; keep a hard
timeout where needed. Shared guidance has one writer; shared writes and Git
stay lead-owned.

Give each writer a repo-relative fence and an out-of-scope line. Run the
[fence and lead-leak checks](references/fences.md) on the actual worker checkout
before integration. A fence violation is an automatic kickback before review.
Reconcile lead drift against the baseline without overwriting existing work.

Honor an operator-selected route before native ranking. Otherwise, when
`~/.config/megapowers/agent-capabilities.md`
([template](assets/agent-capabilities.md)) exists, read it once per session for
eligible bindings; without it, use native defaults. Apart from a bound
independent reviewer, the registry cannot authorize access, disclosure,
permissions, writes, or effects.

Choose an available reviewer under the user's independence and reviewer
instructions; see [independent review](../independent-review/SKILL.md).
Before review dispatch, set its scope and correction-round budget. A spent
budget leaves unresolved findings open; it never converts them into approval.
Use the [reviewer contract](references/review-contract.md). Start a fresh agent
per correction round with the original brief and every later directive; see
[correction-round guidance](../independent-review/references/correction-rounds.md).
Group related failures in one lane. Join all
identities before synthesis; a missing or silent result is a gap, never a pass.
Treat each report as a claim and check its evidence before using it: re-run
load-bearing claims yourself and sample the rest. Ordinary handoffs use inline
inspection plus `verify-and-finish`; only an approved goal surviving
interruption uses `autonomous-run`.
