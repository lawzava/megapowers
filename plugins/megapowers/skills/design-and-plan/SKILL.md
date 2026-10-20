---
name: design-and-plan
description: Use to specify non-trivial behavior or interfaces, resolve requirements or tradeoffs, or produce a multi-step implementation plan. Do not use for a mechanical edit, routine status update, or execution of a settled plan.
when_to_use: "Trigger phrases: write a spec, plan the implementation, define requirements, compare design tradeoffs, architecture for a new feature or interface."
metadata:
  short-description: Specification, tradeoffs, and executable plan before building
---

# Design and Plan

## Understand the system

Read repository instructions and relevant code. Establish entry points,
callers, blast radius, the nearest convention-setting sibling, test coverage,
and any unread area that could change the decision. Name the one fact that
makes the change safe, and check where search stops: wire formats, stored data,
and flags.

Before custom design, check existing code, the standard library, native platform
features, and installed dependencies. Simplify and defer speculative needs
while preserving requirements, conventions, and acceptance criteria.

Resolve factual prerequisites before preference or policy questions. Present
competing sketches only for a high-impact, underconstrained, or hard-to-reverse
design, and screen them against [design red flags](references/design-red-flags.md).
Recommend one and state its tradeoff.

Mark assumptions and blockers. Model a domain term when repeated state branches
or synchronized booleans obscure one concept. Propose an ADR only for a decision
that is hard to reverse, surprising without context, and a real tradeoff; do
not force a glossary.

## Specify observable behavior

For a non-trivial behavior change, state intent, scope, non-goals, and
implementation-independent requirements before the plan. Give each requirement
concrete scenarios and an acceptance oracle.

Use requirement IDs and Added, Modified, or Removed deltas only when the
repository uses them.
Map requirements to scenarios, tasks, and evidence, including failures,
boundaries, and unnamed inputs. Separate proposed, implemented, and verified
behavior. Reconcile specifications with code and tests; a stale specification
is not evidence.

Use the repository's existing specification system and preserve its
requirement IDs and format. When an `openspec/` directory exists, read
[OpenSpec conventions](references/openspec.md). Without a specification
system, keep proportional requirements and their evidence map inline unless
repository convention or the user requires a durable artifact. Do not create
new directories or scaffolding. Skip a durable plan for a small reversible
change with clear scope, risks, and oracle.

## Produce an executable handoff

Name the outcome, acceptance criteria, and verification commands. Give each
task an ID, owned files, prerequisites, and the contracts it consumes and
produces. Set exact signatures only when the contract requires them. Keep
implementation choices open. Start behavior tasks with a failing test. Slice
tasks vertically, each verifiable alone. Stage wide interface changes as
expand, migrate callers, then contract. Parallelize tasks only when
ownership is disjoint and neither result can reshape the other.

When code or tests disprove an assumption, update the affected tasks and evidence
map before continuing. Preserve acceptance criteria and existing authority.
Ask only when the discovery materially changes requested behavior or scope.

Exclude placeholders, unrelated cleanup, and unauthorized commit steps. Before
execution, check for missing criteria, inconsistent names, and unresolved
assumptions.
