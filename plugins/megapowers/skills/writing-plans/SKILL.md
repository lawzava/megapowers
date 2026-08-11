---
name: writing-plans
description: Use when requirements need a multi-step plan before coding. Triggers on "write a plan", "break into steps", "save the plan", or "do not implement yet". Brainstorm unclear work first.
license: MIT
---

# Writing Plans

## Overview

A plan is a handoff artifact. Write it for a skilled engineer who has zero
context for this codebase and must not guess binding decisions. Give the
outcome, owned files, dependencies, interfaces, acceptance oracle, verification
commands, and relevant docs. Use the senior-engineer register (see
`megapowers:using-megapowers`, Communication): declarative, self-contained,
readable by an agent with no conversation context. DRY. YAGNI. TDD.

**Save plans to:** `docs/megapowers/plans/YYYY-MM-DD-<feature-name>.md`. User
preferences for plan location override this default.

If the work needs an isolated worktree, megapowers:using-git-worktrees creates
it at execution time.

## Input and Source Pass

Before decomposing work, read the repository instructions. If present, read
canonical `CONTEXT.md` (or the repository-named equivalent). Read relevant
accepted ADRs when present, and matching project memories when present.
Repository instructions govern process. `CONTEXT.md` supplies current domain
vocabulary; accepted ADRs govern narrower design intent. Treat project memories
as hidden historical hints and reverify them against current sources. Surface
conflicts for resolution; never silently choose a source.

## Comprehension Pass

Documents describe intent; the code is what runs. Opening three files that
matched a grep and deciding that is enough is how a change lands that works on
its own and breaks the module around it.

Before the first task is written, answer all five from files you actually
opened, in the plan itself:

1. **Entry points.** What reaches this code, and what calls that.
2. **Blast radius.** Every caller, implementation, and subclass of what you are
   changing, found by search. State the search you ran, so the next reader can
   tell coverage from luck.
3. **Convention.** The nearest sibling that already does something similar, and
   the naming, error handling, and test layout this change has to match.
4. **Existing coverage.** The tests that touch this behavior now, and whether
   they would fail if the change were wrong.
5. **Boundary.** What you deliberately did not read, and why it cannot matter
   here.

The fifth is what makes the other four honest. A comprehension pass with no
stated boundary is an implicit claim to have read everything, which is never
true, and it hides exactly the gap that later turns into a surprise.

An unanswerable item means the plan is not ready. The remedy is more reading,
never a caveat in the plan saying the area is unclear.

## Scope Check

If the spec covers multiple independent subsystems, suggest one plan per
subsystem. Each plan must produce working, testable software on its own.

## File Structure

Before defining tasks, map which files will be created or modified and what
each is responsible for; this is where decomposition gets locked in. One clear
responsibility per file, smaller focused files over sprawling ones, files that
change together live together. In existing codebases follow established
patterns; include a split only for a file you are already modifying that has
grown unwieldy.

## Task Boundaries

A task is the smallest unit that carries its own test cycle and is worth a
fresh reviewer's gate. Fold setup, configuration, scaffolding, and
documentation into the task whose deliverable needs them; split only where a
reviewer could reject one task while approving its neighbor. Every task ends
with an independently testable deliverable.

Within a task, each step states an outcome the executor can verify: the failing
test exists, its failure is confirmed for the right reason, minimal code makes
it pass, the pass is confirmed, a checkpoint marks the task boundary. Each step
covers one action; a step bundling several actions obscures which one failed.

Before decomposing tasks, create an acceptance evidence map. Copy each
criterion verbatim and assign its implementation target, local oracle, required
external, UX, or database oracle, and evidence owner. Do not replace an exact
emulator, normal-user, published-release, or target-environment witness with a
neighboring unit test.

**Commit cadence is the executor's policy, not a plan mandate.** Selecting a
workflow never grants permission to commit.
`megapowers:subagent-driven-development` uses per-task commits only when the
user and repository already authorize them. Otherwise checkpoints persist
through the ledger and working tree.

## Parallel Safety and Ownership

**Parallel safety:** Write `Sequential`, `Parallel with Task N`, or `Parallel
after Task N`, followed by one sentence explaining the dependency boundary.

**Ownership:** List exact files or non-overlapping directory roots. Parallel
tasks must not own the same path or a parent and child path. Plans intended for
recursive coordinator mode must pass `megapowers:subagent-driven-development`'s
`scripts/ownership-preflight PLAN_FILE` before dispatch.

**May decompose:** Write `Yes` only when a coordinator can split this task into
independently testable children with disjoint ownership. Otherwise write `No`.

Shared interface changes, overlapping paths, and producer to consumer
dependencies stay sequential. A child coordinator inherits its parent's
ownership and cannot broaden it.

## Plan Format

Read [plan-format.md](plan-format.md) before writing the plan document. It
carries the required document header and Global Constraints block, the task
block fields (`Blocked by`, `Blocker`, Files, Interfaces, Steps), the expand,
migrate, and contract ordering for compatibility-sensitive replacements, and
the placeholder patterns that fail a plan.

## Self-Review

After writing the complete plan, check it against the spec with fresh eyes.
This is a checklist you run yourself, not a subagent dispatch:

1. **Spec coverage:** every spec requirement points to a task that implements
   it. A requirement with no task means adding the task.
2. **Placeholder scan:** search the plan for the No Placeholders patterns in
   [plan-format.md](plan-format.md) and fix them.
3. **Type consistency:** names, signatures, and types used in later tasks match
   what earlier tasks defined. A function called `clearLayers()` in Task 3 but
   `clearFullLayers()` in Task 7 is a bug.

Fix issues inline and move on; no re-review pass.

## Execution Handoff

Under an active autonomous run (a `.megapowers/run/<id>/charter.md` governs
this work; see mega-orchestration:autonomous-run, if installed) at level
`autonomous` or `on-the-loop`: do not ask. Choose subagent-driven development
when subagents are available, otherwise inline execution. Commit only if the
charter, user, and repository already authorize it; otherwise use the ledger
and working tree as checkpoints. Journal the choice and proceed. The question
below is for interactive work and `in-the-loop` runs.

After saving the plan, offer this execution choice verbatim:

```
Plan complete and saved to `docs/megapowers/plans/<filename>.md`. Execution options:

1. Subagent-Driven (recommended): when the plan's tasks are mostly independent,
use fresh subagents per task with review between tasks, via
megapowers:subagent-driven-development. Checkpoints use authorized commits when
available; otherwise they remain in the ledger and working tree.

2. Inline Execution: run tasks inline in this session via
megapowers:executing-plans; checkpoints stay at task boundaries and follow your
commit policy.

3. Autonomous Run: for long or multi-session work, use
mega-orchestration:autonomous-run when installed. It uses a frozen charter,
done-when criteria, and per-milestone execution without per-task check-ins.

Which approach?
```

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
