---
name: design-and-plan
description: Use when behavior or an interface needs a specification, requirements need tradeoffs, or approved work needs a multi-step plan.
---

# Design and Plan

Resolve repository facts before user-owned decisions.

## Understand the system

Read repository instructions and relevant code. Establish:

1. Entry points, callers, implementations, and blast radius.
2. The nearest sibling that defines naming, errors, and test conventions.
3. Existing tests and whether they catch the proposed failure.
4. The one load-bearing safety fact that could make a small-looking change
   unsafe, and a proportionate way to prove it.
5. What was not read and why it cannot affect the decision.

Separate factual prerequisites from preference, policy, and other user-owned
decisions. Ask dependency-frontier questions only after repository facts are
resolved. Present competing sketches only for a high-impact, underconstrained,
or hard-to-reverse design. Recommend one and state its tradeoff.

For multi-phase work, mark facts that later evidence may overturn and map each
blocker to the work it prevents. Model domain terms when repeated state branches,
synchronized booleans, or ambiguous names obscure one concept. Do not force a
glossary or ADR. Persistent documentation requires approval.

## Specify observable behavior

For a non-trivial change to observable behavior, write a proportional behavior
contract before the implementation plan. State intent, scope, and non-goals.
Keep requirements independent from implementation. Give each requirement
concrete scenarios and an acceptance oracle.

Describe behavior changes as Added, Modified, or Removed deltas. First reconcile
existing specifications with code and tests; a stale specification is not
evidence of current behavior. After verification, update an existing
repository-owned behavior specification. Do not create a durable artifact
without repository convention or user approval.

Use plain Markdown. Do not require a CLI, package, fixed directory, generated
command, or archive. Skip durable specification and plan artifacts for a small
reversible change with clear scope, risks, and oracle.

## Produce an executable handoff

Name the outcome, owned files, interfaces, error behavior, acceptance criteria,
dependencies, and exact verification commands. Map every criterion to its
implementation target and local or external oracle. Divide work into the
smallest independently testable tasks. Start each behavior task with a failing
test, then minimal implementation and verification. Keep tasks sequential unless
ownership is disjoint and neither result can reshape the other.

Do not add speculative options, placeholder steps, unrelated cleanup, or commit
steps without commit authority. Re-read the plan for missing criteria,
inconsistent names, and unresolved assumptions before execution.
