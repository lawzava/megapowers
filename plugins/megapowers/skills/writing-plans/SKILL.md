---
name: writing-plans
description: Use when requirements need a multi-step plan before coding. Triggers on "write a plan", "break into steps", "save the plan", or "do not implement yet". Brainstorm unclear work first.
license: MIT
---

# Writing Plans

## Overview

A plan is a handoff artifact. Write it for a skilled engineer who has zero
context for this codebase, does not know the toolset or problem domain, and
must not need to ask anyone anything: exact files to touch, complete code,
exact commands with expected output, docs worth checking. Use the
senior-engineer register (see using-megapowers, Communication): declarative,
self-contained, readable by an agent with no conversation context.
DRY. YAGNI. TDD.

**Save plans to:** `docs/megapowers/plans/YYYY-MM-DD-<feature-name>.md`.
User preferences for plan location override this default.

If the work needs an isolated worktree, megapowers:using-git-worktrees
creates it at execution time.

## Scope Check

If the spec covers multiple independent subsystems, suggest one plan per
subsystem. Each plan must produce working, testable software on its own.

## File Structure

Before defining tasks, map which files will be created or modified and what
each is responsible for; this is where decomposition gets locked in. One
clear responsibility per file, smaller focused files over sprawling ones,
files that change together live together. In existing codebases follow
established patterns; include a split only for a file you are already
modifying that has grown unwieldy.

## Task Boundaries

A task is the smallest unit that carries its own test cycle and is worth a
fresh reviewer's gate. Fold setup, configuration, scaffolding, and
documentation into the task whose deliverable needs them; split only where a
reviewer could reject one task while approving its neighbor. Every task ends
with an independently testable deliverable.

Within a task, each step states an outcome the executor can verify: the
failing test exists, its failure is confirmed for the right reason, minimal
code makes it pass, the pass is confirmed, a checkpoint marks the task
boundary. Each step covers one action; a step bundling several actions
obscures which one failed.

**Commit cadence is the executor's policy, not a plan mandate.** Never bake
an unconditional `git commit` into every task. Some workflows commit per
task (subagent-driven-development does, by design; invoking it opts into
that), others batch or leave committing to the human's direction. Write the
checkpoint so it commits when the chosen workflow commits per task, not as
an automatic side effect of finishing a step.

The plan itself follows the communication register: no dash punctuation in
the prose or template text you write (use colons or new sentences).

## Plan Document Header

Every plan starts with this header:

```markdown
# [Feature Name] Implementation Plan

> **For agentic workers:** Required sub-skill: use megapowers:subagent-driven-development (recommended) or megapowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** [One sentence describing what this builds]

**Architecture:** [2-3 sentences about approach]

**Tech Stack:** [Key technologies/libraries]

## Global Constraints

[The spec's project-wide requirements: version floors, dependency limits,
naming and copy rules, platform requirements. One line each, exact values
copied verbatim from the spec. Every task's requirements implicitly include
this section.]

---
```

The checkbox syntax is a contract: executing-plans flips these boxes as its
progress ledger.

## Task Structure

Each task (`### Task N: [Component Name]`) declares its files and
interfaces, then its steps as checkbox (`- [ ]`) items.

**Files:** exact paths, grouped as Create, Modify (with line ranges), and
Test.

**Interfaces:** what the task consumes from earlier tasks and produces for
later ones, with exact function names, parameter and return types. A task's
implementer sees only their own task; this block is how they learn the
names and types neighboring tasks use.

**Steps:** each carries its full content inline. The test step shows the
test code. Verification steps give the exact command and the expected
result, including the expected failure message on the red run. The
implementation step shows the code. The checkpoint step commits only if the
execution workflow commits per task, otherwise it defers to the executor's
or human's commit policy.

## No Placeholders

Every step must contain the actual content the engineer needs. These are
**plan failures**; do not write them:
- "TBD", "TODO", "implement later", "fill in details"
- "Add appropriate error handling" / "add validation" / "handle edge cases"
- "Write tests for the above" without the actual test code
- "Similar to Task N" (repeat the code: the engineer may read tasks out of
  order)
- Steps that describe what to do without showing how (code steps require
  code blocks)
- References to types, functions, or methods not defined in any task

## Self-Review

After writing the complete plan, check it against the spec with fresh eyes.
This is a checklist you run yourself, not a subagent dispatch:

1. **Spec coverage:** every spec requirement points to a task that
   implements it. A requirement with no task means adding the task.
2. **Placeholder scan:** search the plan for the No Placeholders patterns
   and fix them.
3. **Type consistency:** names, signatures, and types used in later tasks
   match what earlier tasks defined. A function called `clearLayers()` in
   Task 3 but `clearFullLayers()` in Task 7 is a bug.

Fix issues inline and move on; no re-review pass.

## Execution Handoff

Under an active autonomous run (a `.megapowers/run/<id>/charter.md` governs
this work; see mega-orchestration:autonomous-run, if installed) at level
`autonomous` or `on-the-loop`: do not ask. Choose subagent-driven
development when subagents are available (its per-task commits are part of
that choice; they land on the run's branch), otherwise inline execution;
journal the choice and proceed. The question below is for interactive work
and `in-the-loop` runs.

After saving the plan, offer the execution choice:

**"Plan complete and saved to `docs/megapowers/plans/<filename>.md`. Execution options:**

**1. Subagent-Driven (recommended):** when the plan's tasks are mostly independent, use fresh subagents per task with review
between tasks, via megapowers:subagent-driven-development. It commits each
task after it passes review, so choosing it opts into per-task commits.

**2. Inline Execution:** run tasks inline in this session via
megapowers:executing-plans; checkpoints at task boundaries, committed per
your own commit policy, not automatically.

**3. Autonomous Run:** for long or multi-session work, wrap execution in
mega-orchestration:autonomous-run (if installed): a frozen charter with
done-when criteria and an autonomy level, per-milestone execution without
per-task check-ins. This plan becomes the run's milestone source (see that
skill's "Where the charter comes from").

**Which approach?"** (If you don't want per-task commits, choose Inline or
say so.)

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
