---
name: design-and-plan
description: Use when behavior or an interface needs a specification, requirements need tradeoffs, or approved work needs a multi-step plan.
when_to_use: Trigger phrases: write a spec, plan the implementation, how should we build this, break it into steps, requirements, tradeoffs, architecture for a new feature or interface.
metadata:
  short-description: Specification, tradeoffs, and executable plan before building
---

# Design and Plan

Resolve repository facts before user-owned decisions.

## Understand the system

Read repository instructions and relevant code. Establish entry points,
callers, blast radius, the nearest convention-setting sibling, existing test
coverage, the one safety fact that could make a small change unsafe, and what
was not read and why it cannot affect the decision.

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
