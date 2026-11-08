---
name: design-and-plan
description: "Load for unresolved behavior or interface contracts, consequential architecture tradeoffs, or an explicit plan or spec request. Settle shared contracts before dependent work starts. Do not load merely because implementation has several steps; a small clear task, mechanical edit, routine status update, or settled plan needs no design workflow."
---

# Design and Plan

## Understand the system

Read repository instructions, entry points, callers, conventions, and tests.
Establish blast radius and evidence for safety across wire formats, stored
data, and flags. Name the one fact that makes the change safe and unread areas
that could change the decision.

Check existing code, standard library, platform features, and dependencies
before custom design. Defer speculative needs, not requirements or acceptance criteria.

Settle factual prerequisites with inspection or probes before asking preference
or policy questions.
For a high-impact, underconstrained, or hard-to-reverse design, compare sketches
and recommend one with its tradeoff. Use
[design red flags](references/design-red-flags.md) when evaluating an actual
architecture choice, not for organizing already-defined tasks.

Mark assumptions and blockers. Model concepts obscured by repeated state branches
or synchronized booleans. Propose ADRs only for decisions that are hard to reverse,
surprising without context, and real tradeoffs.

## Specify observable behavior

State intent, scope, non-goals, and observable requirements before the plan.
Map requirements to scenarios, tasks, and an acceptance oracle, including
failures, boundaries, and unnamed inputs. Distinguish proposed, implemented, and verified behavior;
reconcile specifications with code and tests.

Use requirement IDs and deltas only when the repository uses them.
When an `openspec/` directory exists, read [OpenSpec conventions](references/openspec.md).
Otherwise keep requirements and evidence inline unless a durable artifact is
required. Do not create new directories or scaffolding. Small reversible changes with
clear scope, risks, and oracle need no durable plan.

## Produce an executable handoff

Name the outcome, acceptance criteria, and verification commands. Give each
task an ID, owned files, prerequisites, and contracts it consumes and produces.
Use [task contracts](references/task-contracts.md) when a multi-worker handoff
needs detailed acceptance fences; the inline fields suffice for a simple plan.
Keep implementation choices open unless the contract fixes a
signature. Start behavior tasks with a failing test. Slice
tasks vertically, each verifiable alone. Stage wide interface changes as
expand, migrate callers, then contract. Parallelize tasks only when
ownership is disjoint and neither result can reshape the other.

When code or tests disprove an assumption, update the affected tasks and evidence
map before continuing. Preserve acceptance criteria and existing authority.
Ask only when the discovery materially changes requested behavior or scope.

Exclude placeholders, unrelated cleanup, and unauthorized commit steps. Before
execution, check for missing criteria, inconsistent names, and unresolved
assumptions.
