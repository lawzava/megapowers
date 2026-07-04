---
name: executing-plans
description: Use when executing a written implementation plan yourself, inline, in a dedicated execution session — running tasks top to bottom and stopping only on blockers (distinct from subagent-driven-development, which dispatches each task to a fresh subagent with per-task review).
license: MIT
---

# Executing Plans

## Overview

Load the plan, review it critically, execute all tasks, and report when complete.

**When this skill vs subagent-driven-development:** one criterion, stated the
same way in writing-plans and SDD. Prefer megapowers:subagent-driven-development
when subagents are available *and* per-task commits fit the workflow — review
quality is significantly higher with fresh per-task subagents (Claude Code,
Codex CLI/App, OpenCode, and Antigravity all support them; see the per-platform
tool refs in `../using-megapowers/references/`). Use this skill when subagents
are unavailable, or when you or your human partner want inline single-writer
execution with your own commit cadence.

## The Process

### Step 1: Load and Review Plan
1. Read the plan file.
2. Review it critically - identify any questions or concerns about the plan.
3. If you have concerns: raise them with your human partner before starting.
   Under an active autonomous-run charter at level `autonomous` or
   `on-the-loop`, resolve non-blocking concerns yourself, journal each
   resolution, and proceed; stop only for a genuine blocker.
4. If no concerns: create todos for the plan items and proceed.

### Step 2: Execute Tasks

For each task:
1. Mark it as in_progress.
2. Follow each step exactly (the plan has bite-sized steps).
3. Run verifications as specified.
4. Mark it as completed — in the todo list, **in the plan file** (`- [ ]` →
   `- [x]`), and in the ledger (see Durable Progress). Persist all three in the
   same step.

### Durable Progress

A long inline execution is as exposed to compaction and crashes as any other
run — do not rely on conversation memory or in-context todos alone.

- At start, read the plan's checkboxes and, if present,
  `cat "$(git rev-parse --show-toplevel)/.megapowers/sdd/progress.md"`. Tasks
  already checked off (or marked complete in the ledger) are done — resume at the
  first unchecked task; never re-run completed work.
- After each task's verification passes, check its box in the plan file and
  append `Task N: complete (<sha7 if you committed>)` to
  `.megapowers/sdd/progress.md`. If `.megapowers/sdd/` does not exist yet, create
  it with a `*` `.gitignore` inside so the scratch ledger is never committed (the
  same self-ignoring scratch dir `subagent-driven-development`'s `sdd-workspace`
  makes).
- After compaction, trust the plan's checkboxes, the ledger, and `git log` over
  your own recollection.

### Step 3: Complete Development

After all tasks are complete and verified:
- Get the branch reviewed via megapowers:requesting-code-review — the inline
  path earns the same review discipline as the subagent path.
- Announce: "I'm using the finishing-a-development-branch skill to complete this work."
- Use megapowers:finishing-a-development-branch.
- Follow that skill to verify tests, present options, and execute the choice.

## When to Stop and Ask for Help

Stop executing immediately when:
- You hit a blocker (missing dependency, failing test, unclear instruction).
- The plan has critical gaps that prevent starting.
- You don't understand an instruction.
- Verification fails repeatedly.

Ask for clarification rather than guessing. Under an active autonomous-run
charter at `autonomous` or `on-the-loop`, apply the runbook first: fix and
re-verify up to the attempt cap, journal the outcome, and stop only for a
blocker the cap didn't clear — then journal it as blocked rather than asking
mid-run.

## When to Revisit Earlier Steps

Return to Review (Step 1) when:
- Your partner updates the plan based on your feedback.
- The fundamental approach needs rethinking.

Don't force through blockers - stop and ask.

## Remember
- Review the plan critically first.
- Follow the plan steps exactly.
- Don't skip verifications.
- Reference skills when the plan says to.
- Stop when blocked; don't guess.
- Never start implementation on main/master branch without explicit user consent.

## Integration

Required workflow skills:
- **megapowers:using-git-worktrees** - Ensures an isolated workspace (creates one or verifies an existing one).
- **megapowers:writing-plans** - Creates the plan this skill executes.
- **megapowers:requesting-code-review** - Whole-branch review before finishing.
- **megapowers:finishing-a-development-branch** - Completes development after all tasks.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
