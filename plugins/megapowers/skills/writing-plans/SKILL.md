---
name: writing-plans
description: Use when you have a spec or clear requirements for a multi-step task and need to turn them into a step-by-step implementation plan before coding. Triggers on "write a plan", "write an implementation plan", "break this into steps", "plan before coding", "save the plan as a file", "don't implement yet". Comes after brainstorming (once intent and approach are clear) and before test-driven-development (which implements each step).
license: MIT
---

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

Assume they are a skilled developer, but know almost nothing about our toolset or problem domain. Assume they don't know good test design very well.

Plans are handoff artifacts: write them in the senior-engineer register (see
using-megapowers, Communication) — declarative, self-contained, readable by an
agent with zero conversation context.

**Context:** If working in an isolated worktree, it should have been created via the `megapowers:using-git-worktrees` skill at execution time.

**Save plans to:** `docs/megapowers/plans/YYYY-MM-DD-<feature-name>.md`
- (User preferences for plan location override this default)

## Scope Check

If the spec covers multiple independent subsystems, it should have been broken into sub-project specs during brainstorming. If it wasn't, suggest breaking this into separate plans — one per subsystem. Each plan should produce working, testable software on its own.

## File Structure

Before defining tasks, map out which files will be created or modified and what each one is responsible for. This is where decomposition decisions get locked in.

- Design units with clear boundaries and well-defined interfaces. Each file should have one clear responsibility.
- You reason best about code you can hold in context at once, and your edits are more reliable when files are focused. Prefer smaller, focused files over large ones that do too much.
- Files that change together should live together. Split by responsibility, not by technical layer.
- In existing codebases, follow established patterns. If the codebase uses large files, don't unilaterally restructure - but if a file you're modifying has grown unwieldy, including a split in the plan is reasonable.

This structure informs the task decomposition. Each task should produce self-contained changes that make sense independently.

## Task Right-Sizing

A task is the smallest unit that carries its own test cycle and is worth a
fresh reviewer's gate. When drawing task boundaries: fold setup,
configuration, scaffolding, and documentation steps into the task whose
deliverable needs them; split only where a reviewer could meaningfully
reject one task while approving its neighbor. Each task ends with an
independently testable deliverable.

## Bite-Sized Task Granularity

**Each step is one action (2-5 minutes):**
- "Write the failing test" - step
- "Run it to make sure it fails" - step
- "Implement the minimal code to make the test pass" - step
- "Run the tests and make sure they pass" - step
- "Checkpoint (commit if the executor commits per task)" - step

The plan itself follows the communication register: no dash punctuation in the
prose or template text you write (use colons or new sentences).

**Commit cadence is the executor's policy, not a plan mandate.** Mark the natural
task boundary, but don't bake an unconditional `git commit` into every task: some
workflows commit per task (subagent-driven-development does, by design — that's
what invoking it opts into), others batch or leave committing to the human's
direction. Write the checkpoint so it commits *when the chosen workflow commits*,
not as an automatic side effect of finishing a step.

## Plan Document Header

**Every plan starts with this header:**

```markdown
# [Feature Name] Implementation Plan

> **For agentic workers:** Required sub-skill: use megapowers:subagent-driven-development (recommended) or megapowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** [One sentence describing what this builds]

**Architecture:** [2-3 sentences about approach]

**Tech Stack:** [Key technologies/libraries]

## Global Constraints

[The spec's project-wide requirements — version floors, dependency limits,
naming and copy rules, platform requirements — one line each, with exact
values copied verbatim from the spec. Every task's requirements implicitly
include this section.]

---
```

## Task Structure

````markdown
### Task N: [Component Name]

**Files:**
- Create: `exact/path/to/file.py`
- Modify: `exact/path/to/existing.py:123-145`
- Test: `tests/exact/path/to/test.py`

**Interfaces:**
- Consumes: [what this task uses from earlier tasks — exact signatures]
- Produces: [what later tasks rely on — exact function names, parameter
  and return types. A task's implementer sees only their own task; this
  block is how they learn the names and types neighboring tasks use.]

- [ ] **Step 1: Write the failing test**

```python
def test_specific_behavior():
    result = function(input)
    assert result == expected
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest tests/path/test.py::test_name -v`
Expected: FAIL with "function not defined"

- [ ] **Step 3: Write minimal implementation**

```python
def function(input):
    return expected
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pytest tests/path/test.py::test_name -v`
Expected: PASS

- [ ] **Step 5: Checkpoint**: task boundary; commit here if the execution workflow commits per task (SDD does), otherwise leave it for the executor's/human's commit policy

```bash
# when committing per task:
git add tests/path/test.py src/path/file.py
git commit -m "feat: add specific feature"
```
````

## No Placeholders

Every step must contain the actual content an engineer needs. These are **plan failures** — don't write them:
- "TBD", "TODO", "implement later", "fill in details"
- "Add appropriate error handling" / "add validation" / "handle edge cases"
- "Write tests for the above" (without actual test code)
- "Similar to Task N" (repeat the code — the engineer may be reading tasks out of order)
- Steps that describe what to do without showing how (code blocks required for code steps)
- References to types, functions, or methods not defined in any task

## Remember
- Exact file paths always
- Complete code in every step — if a step changes code, show the code
- Exact commands with expected output
- DRY, YAGNI, TDD, frequent commits

## Self-Review

After writing the complete plan, look at the spec with fresh eyes and check the plan against it. This is a checklist you run yourself — not a subagent dispatch.

**1. Spec coverage:** Skim each section/requirement in the spec. Can you point to a task that implements it? List any gaps.

**2. Placeholder scan:** Search your plan for the patterns from the "No Placeholders" section above. Fix them.

**3. Type consistency:** Do the types, method signatures, and property names you used in later tasks match what you defined in earlier tasks? A function called `clearLayers()` in Task 3 but `clearFullLayers()` in Task 7 is a bug.

If you find issues, fix them inline. No need to re-review — just fix and move on. If you find a spec requirement with no task, add the task.

## Execution Handoff

**Under an active autonomous run** (a `.megapowers/run/<id>/charter.md` governs
this work — see mega-orchestration:autonomous-run, if installed — at level
`autonomous` or `on-the-loop`): do not ask. Choose subagent-driven development
when subagents are available (its per-task commits are part of that choice;
they land on the run's branch), otherwise inline execution; journal the choice
and proceed. The question below is for interactive work and `in-the-loop` runs.

After saving the plan, offer execution choice:

**"Plan complete and saved to `docs/megapowers/plans/<filename>.md`. Execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration. This commits each task after it passes review, so choosing it opts into per-task commits.

**2. Inline Execution** - Execute tasks inline in this session using executing-plans; checkpoints at task boundaries, committed per your own commit policy (not automatically).

**3. Autonomous Run** - For long or multi-session work: wrap execution in mega-orchestration:autonomous-run (if installed) — a frozen charter with done-when criteria and an autonomy level, then per-milestone execution without per-task check-ins.

**Which approach?"** (If you don't want per-task commits, choose Inline or say so.)

**If Subagent-Driven chosen:**
- **Required sub-skill:** use megapowers:subagent-driven-development
- Fresh subagent per task + two-stage review

**If Inline Execution chosen:**
- **Required sub-skill:** use megapowers:executing-plans
- Batch execution with checkpoints for review

**If Autonomous Run chosen:**
- **Required sub-skill:** mega-orchestration:autonomous-run — this plan becomes
  the run's milestone source (see that skill's "Where the charter comes from")

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
