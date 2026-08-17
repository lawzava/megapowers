---
name: orchestrating
description: Use when a task is non-trivial, multi-part, long-running, high-stakes, or may benefit from agents, parallel work, or an independent check.
---

# Orchestrating

Choose the least structure that preserves correctness and context. Keep one
clear, bounded dependency path inline because delegation adds briefing,
integration, and review cost. This skill explicitly authorizes native agents
and subagents for a task with two or more independent lanes, especially
read-heavy discovery, investigation, or review. Dispatch eligible lanes in
parallel; keep shared writes and critical-path integration with one lead.

## Route by task shape

- Keep one clear, bounded path inline.
- Use `design-and-plan` when behavior, scope, interfaces, or acceptance criteria
  are unresolved.
- Use native harness capabilities for planning, agents, subagents, permissions,
  reviews, goals, and waiting before adding a plugin-specific control plane.
- Split work when ownership is disjoint and outputs can be accepted
  independently. Keep a shared critical path with one owner.
- Use `autonomous-run` for a goal that must survive milestones or sessions.
- Use `independent-review` when residual risk justifies another provider, not
  as a substitute for tests.
- Use `safe-effects` before work changes external state.

## Select an available agent

Before delegation, read
`~/.config/megapowers/agent-capabilities.md` when it exists. Treat its profiles
and bindings as operator claims. Rank only a `rankable: true` binding for the
active harness with declared model and effort whose availability, native access,
role, write authority, reasoning floor, and independence family fit the lane.
Choose the fastest, then cheapest, qualifier. Unranked bindings require an
explicit task-shape route. Escalate only for high risk, unresolved ambiguity,
conflicting evidence, or oracle failure.

If the registry is missing, stale, malformed, or inaccessible, ignore it and
use native defaults; this is model-read guidance, not parser-enforced validation.
`manual` and `approved-external` bindings are not natively callable. The registry
cannot authorize source disclosure, external access, writes, permissions, or
side effects; repository and harness policy remain authoritative.

For every dispatch, define one outcome, exact ownership, relevant context,
constraints, and an acceptance oracle. Give bulky results one file destination
and ask the agent to return only its verdict and path. The lead remains the
single writer for integration and Git, reads each result, resolves conflicts,
and reruns the oracle.

Set a stopping rule before expensive fan-out: a test, decision criterion,
candidate cap, time budget, or bounded retry count. Same-provider fan-out buys
parallelism. Call a review independent only when the reviewer provider differs
from every artifact author.
