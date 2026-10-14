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

Native agents are the default for matching lanes. Use one to three direct
children with fresh, bounded context. For independent lanes whose results
cannot reshape one another, issue spawn calls consecutively without intervening
waits, continue independent lead work, and join every returned identity until
completion, failure, or confirmed cancellation.

Before dispatch, read
[native dispatch and lane lifecycle](references/native-dispatch.md) and use
only the active harness's call shape. One bounded output-only lane may use one
fresh-context child returning `verdict`, `evidence`, `uncertainty`, and `next`.
Keep bulky raw payloads in an artifact. Run verbose checks through the
[run receipt tool](../verify-and-finish/references/run-receipt.md) and report
command, exit status, result counts, and log path.

Preflight tools, authentication, permissions, and write authority. Brief the
outcome, disjoint ownership, constraints, oracle, prohibited scope, return
condition, nested-delegation limit, and an advisory time budget; keep a hard
timeout where needed. Shared guidance has one writer; shared writes and Git
stay lead-owned.

Honor an operator-selected route before native ranking. Otherwise, when
`~/.config/megapowers/agent-capabilities.md`
([template](assets/agent-capabilities.md)) exists, read it once per session for
eligible bindings; without it, use native defaults. Apart from a bound
independent reviewer, the registry cannot authorize access, disclosure,
permissions, writes, or effects.

Before review dispatch, set its scope and correction-round budget. A spent
budget leaves unresolved findings open; it never converts them into approval.
Give related failures to one lane so it sees their shared cause. Join all
identities before synthesis; a missing or silent result is a gap, never a pass.
Treat each report as a claim and check its evidence before using it: re-run
load-bearing claims yourself and sample the rest. Ordinary handoffs use inline
inspection plus `verify-and-finish`; only an approved goal surviving
interruption uses `autonomous-run`.
