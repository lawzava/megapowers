---
name: design-and-plan
description: Use to specify non-trivial behavior or interfaces, resolve requirements or tradeoffs, or produce a multi-step implementation plan. Do not use for a mechanical edit, routine status update, or execution of a settled plan.
when_to_use: "Trigger phrases: write a spec, plan the implementation, define requirements, compare design tradeoffs, architecture for a new feature or interface."
metadata:
  short-description: Specification, tradeoffs, and executable plan before building
---

# Design and Plan

Keep mechanical edits, status updates, and settled plans inline unless
requirements or material tradeoffs change.

## Understand the system

Read repository instructions and relevant code. Establish entry points,
callers, blast radius, the nearest convention-setting sibling, test coverage,
and any unread area that could change the decision.

Before custom design, check existing code, the standard library, native platform
features, and installed dependencies. Simplify while preserving requirements
and conventions. Defer speculative needs while preserving requested behavior
and acceptance criteria.

Resolve factual prerequisites before preference or policy questions. Present
competing sketches only for a high-impact, underconstrained, or hard-to-reverse
design. Recommend one and state its tradeoff.

Mark assumptions and blockers. Model a domain term when repeated state branches
or synchronized booleans obscure one concept. Do not force a glossary or ADR.

## Specify observable behavior

For a non-trivial behavior change, state intent, scope, non-goals, and
implementation-independent requirements before the plan. Give each requirement
concrete scenarios and an acceptance oracle.

Map each requirement ID to scenarios, implementation tasks, and evidence,
including failure and boundary cases. Separate proposed, implemented, and
verified behavior; plans prove only planning.

Describe behavior as Added, Modified, or Removed deltas. Reconcile existing
specifications with code and tests; a stale specification is not evidence.

Use an existing specification system when the repository has one; preserve its
requirement IDs and format. When an `openspec/` directory exists, read
[OpenSpec conventions](references/openspec.md). Without a specification
system, keep proportional requirements and their evidence map inline unless
repository convention or the user requires a durable artifact. Do not create
new directories or scaffolding. Skip a durable plan for a small reversible
change with clear scope, risks, and oracle.

## Produce an executable handoff

Name the outcome, owned files, interfaces, error behavior, acceptance criteria,
dependencies, and exact verification commands. Map each criterion to an
implementation target and local or external oracle. Start behavior tasks with a
failing test. Keep tasks sequential unless ownership is disjoint and neither
result can reshape the other.

Exclude speculative options, placeholders, unrelated cleanup, and unauthorized
commit steps. Before execution, check for missing criteria, inconsistent names,
and unresolved assumptions.
