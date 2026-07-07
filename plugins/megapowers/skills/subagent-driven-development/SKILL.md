---
name: subagent-driven-development
description: Use when executing a written plan of independent tasks in the current session by dispatching each task to a fresh subagent, with per-task review — same session, per-task subagent + review, no per-phase human checkpoint (distinct from executing-plans, which you run inline yourself without per-task subagents). Triggers on "dispatch a subagent per task", "subagent-driven", "fan out the plan tasks".
license: MIT
---

# Subagent-Driven Development

Execute a written plan by dispatching a fresh implementer subagent per task, reviewing each task in two stages (spec compliance, then code quality) with a fresh reviewer, and running one broad whole-branch review at the end. Plan tasks execute sequentially on a single branch: one writer at a time, never parallel implementers, because concurrent writers conflict.

**Why subagents:** each gets an isolated context you construct. A subagent never inherits your session history; you hand it exactly what its task needs, which keeps it focused and preserves your own context for coordination.

**Commit cadence:** this workflow commits once per task, and that commit stream is its recovery mechanism: the ledger records commit ranges, and git history survives the compactions that erase conversation memory. Choosing this skill is how the human opts into per-task commits; it is not a hidden side effect. Commits land on the feature branch or worktree; do not start implementation on a main or master branch without explicit consent. If per-task commits do not fit, use megapowers:executing-plans instead.

**Continuous execution:** do not check in with your human partner between tasks. Stop only for a BLOCKED status you cannot resolve, ambiguity that prevents progress, or completion of all tasks. Narrate at most one short line between tool calls; the ledger and tool results carry the record.

## When to Use

Use this skill when a written plan exists, its tasks are mostly independent, subagents are available, and per-task commits are acceptable. With no plan, or tightly coupled tasks, execute manually or brainstorm first. When subagents are unavailable, or the human wants inline single-writer execution with their own commit cadence, use megapowers:executing-plans (the same criterion, stated the same way, appears in writing-plans and executing-plans).

## The Process

Setup, once: read the plan file once, create todos for every task, and check for an existing progress ledger (see Durable Progress). Then scan the plan for conflicts before dispatching Task 1: tasks that contradict each other or the Global Constraints, and anything the plan explicitly mandates that the review rubric treats as a defect. Present everything you find to your human partner as one batched question, each finding beside the plan text that mandates it, asking which governs, rather than one interrupt per discovery mid-plan. Under an active autonomous-run charter at level `autonomous` or `on-the-loop`, do not stop for the batch: resolve each conflict with the least-surprise reading and journal it with a confidence, though a conflict that makes a task's acceptance criteria contradictory is still a real blocker. A clean scan needs no comment; the review loop remains the net for conflicts that only emerge from implementation.

Per task, in order:

1. Record the BASE commit in the ledger, generate the task's brief with `scripts/task-brief PLAN_FILE N`, and dispatch an implementer using [implementer-prompt.md](implementer-prompt.md) with the brief path, report path, and scene-setting context. Answer any questions it asks before it proceeds. It implements, tests, commits, and self-reviews; self-review never replaces the task review.
2. On DONE, generate the review package with `scripts/review-package BASE HEAD` and dispatch a fresh task reviewer using [task-reviewer-prompt.md](task-reviewer-prompt.md) with the printed path. Both verdicts are required; a report missing either spec compliance or code quality is incomplete.
3. Findings go to a fresh fix subagent: the implementer side fixes, the reviewer never edits, and you never fix in your own context. Regenerate the package and dispatch a fresh re-review; repeat until the spec verdict is clean and quality is approved. "Close enough" on spec compliance is a failed review, and a task with open Critical or Important findings is not done.
4. Mark the task complete in the todo list and append the ledger line.

After all tasks: dispatch the final whole-branch review using megapowers:requesting-code-review's [code-reviewer.md](../requesting-code-review/code-reviewer.md) on the most capable available model. For a branch touching billing, auth, concurrency, or security, add an independent different-vendor pass via mega-orchestration:cross-model-verification if installed; a same-model review shares the author's blind spots. Then use megapowers:finishing-a-development-branch.

## Model Selection

Use the least capable model that can handle each role. Transcribing a complete spec and single-file mechanical fixes take the cheapest tier; multi-file integration takes a standard model; design judgment and the final whole-branch review take the most capable. Turn count beats token price: the cheapest models routinely take two to three times the turns on multi-step work, so hold a mid-tier floor for reviewers and for implementers working from prose descriptions. Specify the model explicitly in every dispatch; an omitted model inherits your session's model, usually the most expensive, which silently defeats this section.

## Handling Implementer Status

- **DONE:** proceed to review, using the BASE you recorded before dispatch, never `HEAD~1`, which silently drops all but the last commit of a multi-commit task.
- **DONE_WITH_CONCERNS:** read the concerns first. Correctness or scope concerns get addressed before review; observations get noted and carried forward.
- **NEEDS_CONTEXT:** on a harness with resumable subagents (Claude Code's SendMessage), resume the same implementer with the missing context; it keeps its full history. A fix after review still gets a fresh subagent, never the spent implementer.
- **BLOCKED:** something must change before retry: more context, a more capable model, a smaller task, or escalation to the human if the plan itself is wrong. Never ignore the escalation or force the same model to retry unchanged.

## Handling Reviewer Cannot-Verify Items

The reviewer may report items it cannot verify from the diff, requirements that live in unchanged code or span tasks. These do not block the rest of the review, but resolve each one yourself before marking the task complete; you hold the plan and cross-task context the reviewer lacks. A confirmed gap is a failed spec review: back to a fix subagent, then re-review.

## Constructing Dispatch Prompts

A dispatch describes one task, not the session's history. A fresh subagent needs its task, the interfaces it touches, and the global constraints; pasted prior-task summaries have bloated real dispatches to tens of thousands of characters of dead weight.

For reviewers:

- Copy the binding requirements verbatim from the plan's Global Constraints section or the spec: exact values, exact formats, and the stated relationships between components. The reviewer's template already carries the process rules; the constraints block is for what this project's spec demands.
- Never pre-judge findings. Do not tell a reviewer to ignore, downgrade, or not flag anything; if you expect a false positive, let the reviewer raise it and adjudicate it in the loop. Likewise skip open-ended directives ("check all uses") without a concrete task-specific reason, and do not ask the reviewer to re-run tests the implementer already ran on the same code.
- A finding the plan itself mandates is the human's decision, like any plan contradiction: present the finding beside the plan text and ask which governs. Do not dismiss it because the plan mandates it, and do not dispatch a fix that contradicts the plan without asking.

For fixes:

- Dispatch fix subagents for Critical and Important findings. Record Minor findings in the ledger and point the final whole-branch review at that list so it can triage what must be fixed before merge; a roll-up nobody reads is a silent discard.
- Every fix dispatch carries the implementer contract: the fixer re-runs the tests covering its change and reports the covering test files, the command run, and the output. Name the covering tests in the dispatch; a one-line fix does not need the whole suite. Dispatch the re-review only once all three pieces of evidence are present.
- If the final whole-branch review returns findings, dispatch one fix subagent with the complete findings list, not one fixer per finding; per-finding fixers each rebuild context and re-run suites, and a real session's fix wave run that way cost more than all its tasks combined.

## File Handoffs

Everything you paste into a dispatch, and everything a subagent prints back, stays resident in your context for the rest of the session. Hand artifacts over as files: senior-engineer register (see using-megapowers, Communication), conclusion first, self-contained. `scripts/sdd-workspace` resolves the working-tree directory all of these artifacts live in.

- **Task brief:** `scripts/task-brief PLAN_FILE N` extracts the task's full text to a file and prints the path. The brief is the single source of requirements; exact values (numbers, magic strings, signatures, test cases) appear only there. The dispatch adds where the task fits in the project, the brief path introduced as the requirements to follow verbatim, interfaces and decisions from earlier tasks the brief cannot know, your resolution of any ambiguity you noticed in it, and the report path with its contract. Never hand a subagent the whole plan file.
- **Report file:** named after the brief (task-N-brief.md pairs with task-N-report.md). The implementer writes the full report there and returns only status, commits, a one-line test summary, and concerns. Fix dispatches append their fix report to the same file; re-reviews read the updated file.
- **Reviewer inputs:** the brief, the report, and the review package as three paths, plus the binding constraints. `scripts/review-package BASE HEAD` writes the commit list, stat summary, and full diff with context to one file and prints its path, so the reviewer reads everything in one call. The final review gets the same treatment with the branch's merge base (for example `git merge-base main HEAD`) as BASE.

## Durable Progress

Conversation memory does not survive compaction; controllers that lost their place have re-dispatched entire completed task sequences, the single most expensive failure observed. The ledger at `.megapowers/sdd/progress.md` under the repo root is the recovery map.

- At skill start, read the ledger. Tasks marked complete there are done; never re-dispatch them. Resume at the first task not marked complete.
- Before each dispatch, append `Task N: base <sha7> (in progress)` with the current short HEAD. The review step needs this exact BASE, and it otherwise lives only in volatile conversation memory.
- On a clean review, append `Task N: complete (commits <base7>..<head7>, review clean)`, superseding the in-progress line.
- On resume, an in-progress line with no matching complete line marks the task to re-check against `git log`. After compaction, trust the ledger and git history over your own recollection. `git clean -fdx` destroys the ledger (it is git-ignored scratch); if that happens, recover from `git log`.

## Prompt Templates

- [implementer-prompt.md](implementer-prompt.md) for the implementer subagent
- [task-reviewer-prompt.md](task-reviewer-prompt.md) for the task reviewer (spec compliance + code quality)
- Final whole-branch review: megapowers:requesting-code-review's [code-reviewer.md](../requesting-code-review/code-reviewer.md)

## Example Workflow

One task's full loop, compressed:

```
[task-brief for Task 2; dispatch implementer with brief + report paths + context]
Implementer: Added verify/repair modes, 8/8 tests passing, committed.
[review-package BASE HEAD; dispatch task reviewer with the printed path]
Reviewer: Missing progress reporting (spec: "report every 100 items");
  unrequested JSON output flag; Important: magic number.
[Dispatch fix subagent with all findings]
Fixer: Removed the flag, added progress reporting, extracted constant.
[Regenerate package; re-review]
Reviewer: Spec compliant. Quality: Approved. Mark Task 2 complete, ledger line.
```

## Integration

**Required workflow skills:** megapowers:using-git-worktrees ensures an isolated workspace; megapowers:writing-plans creates the plan this skill executes; megapowers:requesting-code-review supplies the final whole-branch review template; megapowers:finishing-a-development-branch completes the branch after all tasks.

**Subagents should use** megapowers:test-driven-development for each task.

**Alternative workflow:** megapowers:executing-plans for inline single-writer execution when subagents are unavailable or per-task commits do not fit.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
