---
name: subagent-driven-development
description: Use when a plan has independent tasks for subagents or requests recursive multi-writer execution. Triggers on "subagent per task", "fan out plan tasks", or "multi-writer". Use executing-plans for inline work.
license: MIT
---

# Subagent-Driven Development

Execute a written plan with fresh, narrowly briefed subagents where delegation
pays for itself. Scale review to risk: low-risk tasks use focused tests and
self-review, medium-risk work gets one independent boundary review, and
high-risk work gets review at each risky boundary plus final independent
verification. The default process executes tasks sequentially on one branch.
Recursive coordinator mode is an explicit exception for independent tasks with
disjoint ownership.

**Why subagents:** each task gets deliberately fresh context that you construct.
Some harnesses can inherit or fork parent history, so request a fresh context
explicitly for implementers and reviewers. Hand each one exactly what its task
needs, which keeps it focused and preserves your own context for coordination.

**Git authorization:** selecting SDD never grants permission to commit, push,
merge, or open a pull request. Preserve the user's and repository's existing
authorization. When commits are authorized, the ledger records their ranges.
Otherwise the ledger and working tree are the recovery mechanism. Recursive
children never perform Git index or ref operations.

**Continuous execution:** do not check in with your human partner between tasks. Stop only for a BLOCKED status you cannot resolve, ambiguity that prevents progress, or completion of all tasks. Narrate at most one short line between tool calls; the ledger and tool results carry the record.

**Repository-wide verification is the lead's, once, at the boundary.** Give each implementer the focused suite for the files it owns, never the whole-repository script. Parallel dispatch multiplies that script by the number of writers, and a whole-repository run is usually the most expensive command in the tree. Several at once can exhaust the machine and take the agents with them, which is silent: a killed subagent leaves a stale transcript and no status, so the lead learns about it by noticing a timestamp. The lead runs the repository suite after the tree is quiescent, and treats a self-reported pass as unverified either way.

## When to Use

Use this skill when a written plan exists, its tasks are mostly independent, and subagents are available. Use the ordinary sequential process when per-task commits are acceptable. Select recursive coordinator mode only when the harness supports nested subagents and every concurrent writer can receive disjoint ownership. With no plan, tightly coupled tasks, or no safe ownership split, execute manually or use megapowers:executing-plans.

For deterministic mechanical changes that share one oracle, use bulk
mechanical mode: one owner, one bounded batch, one focused verification set,
and one proportional review. Do not create an implementer and reviewer loop per
file.

## Recursive Coordinator Mode

Recursive coordinator mode is guidance for native Codex and Claude Code subagents, not an execution runtime. Select it explicitly when a plan has several independent roots and coordinators can assign exclusive paths before dispatch. The ordinary sequential process below remains the fallback.

All writers share the current checkout; recursive mode creates no worktrees. Each child receives exclusive ownership of exact files or non-overlapping directory roots. A coordinator may subdivide only the ownership it inherited. Overlapping ownership, shared interface changes, and dependencies stay sequential. If independence cannot be stated in one concise ownership sentence, keep the work under one writer.

Before any recursive dispatch, run
`scripts/ownership-preflight PLAN_FILE`. Do not dispatch if it reports missing,
ambiguous, globbed, duplicated, or parent-child-overlapping ownership among
parallel tasks. Correct the plan or execute the affected work sequentially.
The executable parser contract is
`scripts/tests/ownership-preflight.test.sh`.

The lead launches one native coordinator per independent root. A coordinator may launch native children for independent pieces of its own scope. It waits for every required child, reviews the combined diff, resolves integration issues within its ownership, runs the required verification, and returns one synthesized result to its parent. The lead coordinates only its direct children. Descendants report to the coordinator that spawned them.

In Codex, use native nested subagents with `fork_turns = "none"` for independent children. In Claude Code, use nested Agent calls; do not use agent teams because teams cannot nest. Respect the harness capacity and depth visible in the session. When capacity is unavailable, continue inline or serially.

Each child brief contains the assignment, done criteria, owned paths, relevant interfaces and constraints, required verification, whether it may subdivide, and the requirement to wait for its direct children and return one synthesized subtree result. Do not copy the parent transcript, full plan, repository tests, or descendant chatter into the brief.

Separate top-level sessions may share the checkout only when their exclusive ownership was partitioned before launch. There is no cross-session lock or automatic conflict resolution. Concurrent children do not run Git index or ref mutations. They do not commit, merge, rebase, reset, switch branches, update refs, push, or clean the checkout. Only the top-level lead performs any authorized Git action, after its direct children return and repository policy permits it.

Use native done, blocked, and needs-context results. The parent decides whether to add context, retry with a fresh child, reduce the task, continue inline, or surface the blocker. Recursive mode adds no separate recovery machinery.

## The Process

Setup once: read the plan and existing progress ledger. Use those as the one
progress surface rather than duplicating every task into a second todo list.
Then scan the plan for conflicts before dispatching Task 1: tasks that
contradict each other or the Global Constraints, and anything the plan
explicitly mandates that the review rubric treats as a defect. Present
everything you find as one batched question. Under an autonomous or on-the-loop
charter, resolve non-blocking conflicts with the least-surprise reading and
journal them.

Per task, in order:

1. Record the BASE commit in the ledger, generate the task's brief with
   `scripts/task-brief PLAN_FILE N`, and dispatch an implementer using
   [implementer-prompt.md](implementer-prompt.md). The brief explicitly states
   whether pre-existing commit authorization exists. The implementer tests and
   self-reviews, and commits only when that authorization is present.
2. Apply the declared risk tier. Low-risk tasks proceed on focused evidence and
   self-review. Medium-risk tasks get one independent review at the most useful
   boundary. High-risk tasks get a fresh task reviewer for each risky boundary.
   A requested review reports specification compliance and engineering quality
   as separate verdicts.
3. Send all actionable findings for an artifact in one fix wave, then re-review.
   Cap fix and re-review at three cycles per artifact. After the third unresolved
   verdict, mark the task blocked and surface the remaining findings. Never loop
   until clean without a stopping rule.
4. Mark the task complete in the plan or ledger, whichever is the declared
   progress surface.

After all tasks, run the branch-boundary verification. Medium and high-risk
branches get one whole-branch review. High-risk billing, auth, concurrency,
security, schema, data, or external-side-effect changes also get an
author-vendor-excluded pass through the structured delegate launcher. Do not
repeat a review already performed on the identical complete diff.

## Dispatch Reference

[dispatch-reference.md](dispatch-reference.md) carries the per-role model
selection rules, what a dispatch prompt contains for reviewers and for fixers,
the file handoff contract (`scripts/task-brief`, `scripts/review-package`,
`scripts/sdd-workspace`, one report channel per delegate), and a compressed
example of one task's full loop. Read it before the first dispatch.

## Handling Implementer Status

- **DONE:** proceed to review, using the BASE you recorded before dispatch, never `HEAD~1`, which silently drops all but the last commit of a multi-commit task.
- **DONE_WITH_CONCERNS:** read the concerns first. Correctness or scope concerns get addressed before review; observations get noted and carried forward.
- **NEEDS_CONTEXT:** on a harness with resumable subagents (Claude Code's SendMessage), resume the same implementer with the missing context; it keeps its full history. A fix after review still gets a fresh subagent, never the spent implementer.
- **BLOCKED:** something must change before retry: more context, a more capable model, a smaller task, or escalation to the human if the plan itself is wrong. Never ignore the escalation or force the same model to retry unchanged.

## Handling Reviewer Cannot-Verify Items

The reviewer may report items it cannot verify from the diff, requirements that live in unchanged code or span tasks. These do not block the rest of the review, but resolve each one yourself before marking the task complete; you hold the plan and cross-task context the reviewer lacks. A confirmed gap is a failed spec review: back to a fix subagent, then re-review.

## Durable Progress

Conversation memory does not survive compaction, and a controller that loses its place re-dispatches completed tasks. The ledger at `.megapowers/sdd/progress.md` under the repo root is the recovery map.

Roll the controller into a fresh context after 8 to 10 completed tasks, or
earlier when another task would cross 80 percent of the context or cache
budget. Persist the ledger first. Reserve the final 20 percent for integration,
review, verification, and synthesis.

- At skill start, read the ledger. Tasks marked complete there are done; never re-dispatch them. Resume at the first task not marked complete.
- Before each dispatch, append `Task N: base <sha7> (in progress)` with the current short HEAD. The review step needs this exact BASE, and it otherwise lives only in volatile conversation memory.
- On a clean review, append `Task N: complete (commits <base7>..<head7>, review clean)`, superseding the in-progress line.
- On resume, an in-progress line with no matching complete line marks the task to re-check against `git log`. After compaction, trust the ledger and git history over your own recollection. `git clean -fdx` destroys the ledger (it is git-ignored scratch); if that happens, recover from `git log`.

## Prompt Templates

- [implementer-prompt.md](implementer-prompt.md) for the implementer subagent
- [task-reviewer-prompt.md](task-reviewer-prompt.md) for the task reviewer (spec compliance + code quality)
- Final whole-branch review: megapowers:requesting-code-review's [code-reviewer.md](../requesting-code-review/code-reviewer.md)

## Integration

**Required workflow skills:** for the ordinary sequential process, megapowers:using-git-worktrees ensures an isolated workspace. Recursive coordinator mode is the shared-checkout exception and creates no worktrees. megapowers:writing-plans creates the plan this skill executes; megapowers:requesting-code-review supplies the final whole-branch review template; megapowers:finishing-a-development-branch completes the branch after all tasks.

**Subagents should use** megapowers:test-driven-development for each task.

**Alternative workflow:** megapowers:executing-plans for inline single-writer execution when subagents are unavailable or per-task commits do not fit.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
