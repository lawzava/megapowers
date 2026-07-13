---
name: executing-plans
description: Use to execute a written implementation plan inline with one writer, especially coupled tasks. Use subagent-driven-development for independent tasks with per-task review.
license: MIT
---

# Executing Plans

## Overview

Load the plan, review it critically, execute the tasks in order, and hand the
finished branch off for review. You are the single writer on an isolated
branch or worktree; never start implementation on a main or master branch
without explicit user consent.

**When this skill vs subagent-driven-development:** one criterion, stated the
same way in writing-plans and SDD. Prefer megapowers:subagent-driven-development
when subagents are available, the plan's tasks are mostly independent, and
per-task commits are acceptable; review quality is significantly higher with
fresh per-task subagents. Use this skill when subagents are unavailable, when
tasks are tightly coupled, or when you or your human partner want inline
single-writer execution with your own commit cadence.

## The Process

Read the plan and review it critically before executing anything. Raise
concerns with your human partner first. Under an active autonomous-run
charter at level `autonomous` or `on-the-loop`, resolve non-blocking concerns
yourself, journal each resolution, and proceed; stop only for a real blocker.
With no open concerns, create todos for the plan's tasks and start.

Execute tasks top to bottom. The plan is the specification: follow its steps,
run its verifications as written, and invoke any skills it names. A task is
complete only after its verification passes, and its completion is persisted
in the same step: the todo list, the plan file's checkbox, and the ledger
(see Durable Progress).

## Durable Progress

A long inline execution is as exposed to compaction and crashes as any other
run. Conversation memory and in-context todos are not the record; the plan's
checkboxes and the ledger are.

- At start, read the plan's checkboxes and, if present, the ledger at
  `.megapowers/sdd/progress.md` under the repo root (the same self-ignoring
  scratch directory subagent-driven-development uses; if `.megapowers/sdd/`
  does not exist, create it with a `*` `.gitignore` inside so the ledger is
  never committed). Anything checked off or marked complete is done. Resume
  at the first unchecked task; never re-run completed work.
- As each task's verification passes, flip its checkbox in the plan file
  (`- [ ]` to `- [x]`) and append `Task N: complete (<sha7 if you committed>)`
  to the ledger.
- After compaction, trust the plan's checkboxes, the ledger, and `git log`
  over your own recollection.

## Blockers

Stop and ask rather than guess when something prevents correct progress: a
missing dependency, a verification that keeps failing, or a plan gap or
instruction you cannot interpret. If your partner revises the plan or the
approach itself proves wrong, return to critical review before continuing.
Under an active autonomous-run charter at `autonomous` or `on-the-loop`,
apply the runbook first: fix and re-verify up to the attempt cap, journal the
outcome, and stop only for a blocker the cap did not clear, journaling it as
blocked rather than asking mid-run.

## Completion

After all tasks are complete and verified, get the whole branch reviewed via
megapowers:requesting-code-review; the inline path gets the same review
discipline as the subagent path. Then use
megapowers:finishing-a-development-branch to verify tests, present the
integration options, and execute the choice.

## Integration

Required workflow skills:
- **megapowers:using-git-worktrees** ensures an isolated workspace (creates one or verifies an existing one).
- **megapowers:writing-plans** creates the plan this skill executes.
- **megapowers:requesting-code-review** reviews the whole branch before finishing.
- **megapowers:finishing-a-development-branch** completes development after all tasks.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
