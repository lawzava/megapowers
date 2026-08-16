---
name: design-and-plan
description: Use when a feature or behavior is unclear, requirements need tradeoffs, or approved work needs a multi-step implementation plan.
---

# Design and Plan

Resolve binding decisions before implementation. Skip a durable plan for a
small, reversible change whose scope, interface, risks, and oracle are already
clear.

## Understand the system

Read repository instructions and the relevant code. Establish:

1. Entry points and callers that reach the behavior.
2. Blast radius found by searching every caller and implementation.
3. The nearest sibling that defines naming, errors, and test conventions.
4. Existing tests and whether they would catch the proposed failure.
5. What was not read and why it cannot affect the change.

Ask only for decisions the repository cannot answer. State the goal,
constraints, success criteria, risks, and unresolved assumptions. When several
approaches remain viable, present the smallest useful set, recommend one, and
make the tradeoff explicit. Get explicit approval before hard-to-reverse or
high-stakes changes to data, security, billing, concurrency, or public
contracts.

## Produce an executable handoff

A durable plan names the outcome, owned files, interfaces, error behavior,
acceptance criteria, dependencies, and exact verification commands. Map every
criterion to its implementation target and local or external oracle. Divide
work into the smallest independently testable tasks; each behavior task starts
with a failing test, then minimal implementation and verification. Mark tasks
sequential unless ownership is disjoint and neither result can reshape the
other.

Do not add speculative options, placeholder steps, unrelated cleanup, or commit
steps without existing commit authority. Re-read the finished plan for missing
criteria, inconsistent names, and unresolved assumptions before execution.
