---
name: orchestrating
description: Use when a task is non-trivial, multi-part, high-stakes, or must be routed across agents, handoffs, reviews, parallel work, or durable runs.
---

# Orchestrating

Choose the least structure that preserves correctness and context. Keep one
bounded dependency path inline. This skill explicitly authorizes native agents
and subagents when two or more independent lanes justify their briefing and
integration cost.

Use three fan-out shapes deliberately: disjoint slices for separate ownership,
same-brief candidates for competing answers, and read-only review for independent
checks. Dispatch eligible read-heavy lanes in parallel. Keep shared writes,
interfaces, integration, and Git with one lead.

## Route by task shape

- Use `systematic-debugging` before a fix when the cause is unknown.
- Use `design-and-plan` when behavior, scope, interfaces, risks, or acceptance
  criteria remain unresolved.
- Use native harness capabilities for agents, plans, goals, permissions,
  reviews, and waiting before adding control machinery.
- An ordinary handoff uses inline inspection plus `verify-and-finish`. Only a
  currently approved goal that must survive interruption uses `autonomous-run`.
- Use `evidence-research` for historical rationale or contested external facts.
- Use `independent-review` when residual risk justifies another provider.
- Use `safe-effects` before any external mutation.

Before delegation, read `~/.config/megapowers/agent-capabilities.md` when it
exists. Rank only a `rankable: true` binding for the active harness with a
declared, verified model and effort whose availability, role, write authority,
reasoning floor, and independence family fit. Choose the fastest, then cheapest,
qualifier. Give route rationale only when model choice affects cost, latency, or
independence.

If the registry is missing, stale, malformed, or inaccessible, ignore it and
use the native default. It is advisory model-read guidance, not parser-enforced
validation. The registry cannot authorize disclosure, external access, writes,
permissions, or side effects. `manual` and `approved-external` bindings are not
natively callable.

Every brief names one outcome, exclusive ownership, relevant constraints, the
oracle, prohibited scope, one artifact path, and the return condition. Keep raw
payloads out of the lead context; return a verdict and evidence pointer. The
lead reads each result, resolves conflicts, and reruns the oracle.

Set a stopping rule before expensive fan-out. Same-provider fan-out gives
parallelism, not independence. Call a review independent only when its provider
differs from every artifact author.
