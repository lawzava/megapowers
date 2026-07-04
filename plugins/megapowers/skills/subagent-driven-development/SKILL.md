---
name: subagent-driven-development
description: Use when executing a written plan of independent tasks in the current session by dispatching each task to a fresh subagent, with per-task review — same session, per-task subagent + review, no per-phase human checkpoint (distinct from executing-plans, which you run inline yourself without per-task subagents). Triggers on "dispatch a subagent per task", "subagent-driven", "fan out the plan tasks".
license: MIT
---

# Subagent-Driven Development

Execute a plan by dispatching a fresh implementer subagent per task, a task review (spec compliance + code quality) after each, and a broad whole-branch review at the end.

**Why subagents:** You delegate tasks to specialized agents with isolated context. By precisely crafting their instructions and context, you keep them focused and set them up to succeed. They should never inherit your session's context or history — you construct exactly what they need. This also preserves your own context for coordination work.

**Core principle:** Fresh subagent per task + task review (spec + quality) + broad final review gives high quality with fast iteration.

**Narration:** between tool calls, narrate at most one short line — the ledger and the tool results carry the record.

**Continuous execution:** Do not pause to check in with your human partner between tasks. Execute all tasks from the plan without stopping. The only reasons to stop are: a BLOCKED status you cannot resolve, ambiguity that genuinely prevents progress, or all tasks complete. "Should I continue?" prompts and progress summaries waste their time — they asked you to execute the plan, so execute it.

**Commit cadence:** this workflow commits once per task — that per-task commit is its recovery mechanism (the ledger records the commit range; a compaction can wipe conversation memory but not git history). Choosing this skill *is* how the human opts into that cadence; it is not a hidden side effect. Those commits land on the feature branch / worktree (never main without explicit consent). If the human doesn't want per-task commits, use megapowers:executing-plans or manual execution instead.

## When to Use

Walk the decision from the top:

1. No implementation plan yet? Do manual execution, or brainstorm first.
2. Have a plan, but the tasks are tightly coupled? Do manual execution, or brainstorm first.
3. Plan with mostly independent tasks, subagents available, per-task commits acceptable? Use this skill.
4. Subagents unavailable, or the human wants inline single-writer execution with their own commit cadence? Use megapowers:executing-plans instead. (Same criterion, stated the same way, in writing-plans and executing-plans.)

**vs. Executing Plans (inline execution):**
- Fresh subagent per task (no context pollution)
- Review after each task (spec compliance + code quality), broad review at the end
- Faster iteration (no human-in-loop between tasks)

## The Process

Setup, once:

1. Read the plan file once. Note context and global constraints, then create todos for all tasks.

Then, per task, in order:

2. Run `scripts/task-brief PLAN_FILE N` and dispatch an implementer subagent using [implementer-prompt.md](implementer-prompt.md), with the brief path, report path, and scene-setting context.
3. If the implementer asks questions, answer them and provide context, then let it proceed. It implements, tests, commits, and self-reviews.
4. When the implementer reports DONE, run `scripts/review-package BASE HEAD` and dispatch a task reviewer using [task-reviewer-prompt.md](task-reviewer-prompt.md), passing the printed diff path.
5. If the reviewer's spec verdict is not clean, or code quality is not approved, dispatch a fix subagent for Critical/Important findings, then regenerate the review package and re-review. Repeat until the spec verdict is clean and quality is approved.
6. Mark the task complete in the todo list and append a line to the progress ledger.

After all tasks:

7. Dispatch the final whole-branch code reviewer using megapowers:requesting-code-review's [code-reviewer.md](../requesting-code-review/code-reviewer.md).
   For a branch touching billing, auth, concurrency, or security, add an
   independent different-vendor pass via
   mega-orchestration:cross-model-verification (if installed) — a same-model
   review shares the author's blind spots.
8. Use megapowers:finishing-a-development-branch.

## Pre-Flight Plan Review

Before dispatching Task 1, scan the plan once for conflicts:

- tasks that contradict each other or the plan's Global Constraints
- anything the plan explicitly mandates that the review rubric treats as a
  defect (a test that asserts nothing, verbatim duplication of a logic block)

Present everything you find to your human partner as one batched question —
each finding beside the plan text that mandates it, asking which governs —
before execution begins, not one interrupt per discovery mid-plan. Under an
active autonomous-run charter at level `autonomous` or `on-the-loop`, do not
stop for the batch: resolve each conflict with the least-surprise reading,
journal every resolution with a confidence, and let the review loop catch
wrong calls; a conflict that makes a task's acceptance criteria contradictory
is still a real blocker. If the scan is clean, proceed without comment. The
review loop remains the net for conflicts that only emerge from
implementation.

## Model Selection

Use the least powerful model that can handle each role to conserve cost and increase speed.

**Mechanical implementation tasks** (isolated functions, clear specs, 1-2 files): use a fast, cheap model. Most implementation tasks are mechanical when the plan is well-specified.

**Integration and judgment tasks** (multi-file coordination, pattern matching, debugging): use a standard model.

**Architecture and design tasks**: use the most capable available model.
The final whole-branch review is one of these — dispatch it on the most
capable available model, not the session default.

**Review tasks**: choose the model with the same judgment, scaled to the
diff's size, complexity, and risk. A small mechanical diff does not need the
most capable model; a subtle concurrency change does.

**Specify the model explicitly when dispatching a subagent.** An omitted
model inherits your session's model — often the most capable and most
expensive — which silently defeats this section.

**Turn count beats token price.** Wall-clock and context cost scale with how
many turns a subagent takes, and the cheapest models routinely take 2-3× the
turns on multi-step work — costing more overall. Use a mid-tier model as the
floor for reviewers and for implementers working from prose descriptions.
When the task's plan text contains the complete code to write, the
implementation is transcription plus testing: use the cheapest tier for
that implementer. Single-file mechanical fixes also take the cheapest tier.

**Task complexity signals (implementation tasks):**
- Touches 1-2 files with a complete spec → cheap model
- Touches multiple files with integration concerns → standard model
- Requires design judgment or broad codebase understanding → most capable model

## Handling Implementer Status

Implementer subagents report one of four statuses. Handle each appropriately:

**DONE:** Generate the review package (`scripts/review-package BASE HEAD`, from this skill's directory — it prints the unique file path it wrote; BASE is the commit you recorded before dispatching the implementer, never `HEAD~1`, which silently drops all but the last commit of a multi-commit task), then dispatch the task reviewer with the printed path.

**DONE_WITH_CONCERNS:** The implementer completed the work but flagged doubts. Read the concerns before proceeding. If the concerns are about correctness or scope, address them before review. If they're observations (e.g., "this file is getting large"), note them and proceed to review.

**NEEDS_CONTEXT:** The implementer needs information that wasn't provided. Provide the missing context and re-dispatch. On a harness with resumable subagents (Claude Code's SendMessage), resume the same implementer with the missing context instead of re-dispatching a recap; it keeps its full history. A fix after review still uses a fresh subagent (never the spent implementer).

**BLOCKED:** The implementer cannot complete the task. Assess the blocker:
1. If it's a context problem, provide more context and re-dispatch with the same model
2. If the task requires more reasoning, re-dispatch with a more capable model
3. If the task is too large, break it into smaller pieces
4. If the plan itself is wrong, escalate to the human

Do not ignore an escalation, and do not force the same model to retry without changes. If the implementer said it's stuck, something needs to change.

## Handling Reviewer Cannot-Verify Items

The task reviewer may report "Cannot verify from diff" items — requirements
that live in unchanged code or span tasks. These do not block the rest of the
review, but resolve each one yourself before marking the task complete: you
hold the plan and cross-task context the reviewer lacks. If you confirm an
item is a real gap, treat it as a failed spec review — send it back to the
implementer and re-review.

## Constructing Reviewer Prompts

Per-task reviews are task-scoped gates. The broad review happens once, at the
final whole-branch review. When you fill a reviewer template:

- Do not add open-ended directives like "check all uses" or "run race tests
  if useful" without a concrete, task-specific reason
- Do not ask a reviewer to re-run tests the implementer already ran on the
  same code — the implementer's report carries the test evidence
- Do not pre-judge findings for the reviewer — never instruct a reviewer to
  ignore or not flag a specific issue. If you believe a finding would be a
  false positive, let the reviewer raise it and adjudicate it in the review
  loop. If the prompt you are writing contains "do not flag," "don't treat X
  as a defect," "at most Minor," or "the plan chose," stop: you are
  pre-judging, usually to spare yourself a review loop.
- The global-constraints block you hand the reviewer is its attention
  lens. Copy the binding requirements verbatim from the plan's Global
  Constraints section or the spec: exact values, exact formats, and the
  stated relationships between components ("same layout as X", "matches
  Y"). The reviewer's template already carries the process rules (YAGNI,
  test hygiene, review method) — the constraints block is for what this
  project's spec demands.
- Hand the reviewer its diff as a file: run this skill's
  `scripts/review-package BASE HEAD` and pass the reviewer the file path
  it prints (or, without bash: `git log --oneline`, `git diff --stat`,
  and `git diff -U10` for the range, redirected to one uniquely named
  file). The output never enters your own context, and the reviewer sees
  the commit list, stat summary, and full diff with context in one Read
  call. Use the BASE you recorded before dispatching the implementer.
- A dispatch prompt describes one task, not the session's history. Do not
  paste accumulated prior-task summaries ("state after Tasks 1-3") into
  later dispatches — a real session's dispatch hit 42k chars of which 99%
  was pasted history. A fresh subagent needs its task, the interfaces it
  touches, and the global constraints. Nothing else.
- Dispatch fix subagents for Critical and Important findings. Record Minor
  findings in the progress ledger as you go, and point the final
  whole-branch review at that list so it can triage which must be fixed
  before merge. A roll-up nobody reads is a silent discard.
- A finding labeled plan-mandated — or any finding that conflicts with
  what the plan's text requires — is the human's decision, like any plan
  contradiction: present the finding and the plan text, ask which governs.
  Do not dismiss the finding because the plan mandates it, and do not
  dispatch a fix that contradicts the plan without asking.
- The final whole-branch review gets a package too: run
  `scripts/review-package MERGE_BASE HEAD` (MERGE_BASE = the commit the
  branch started from, e.g. `git merge-base main HEAD`) and include the
  printed path in the final review dispatch, so the final reviewer reads
  one file instead of re-deriving the branch diff with git commands.
- Every fix dispatch carries the implementer contract: the fix subagent
  re-runs the tests covering its change and reports the results. Name the
  covering test files in the dispatch — a one-line fix does not need the
  whole suite. Before re-dispatching the reviewer, confirm the fix report
  contains the covering tests, the command run, and the output; dispatch
  the re-review once all three are present.
- If the final whole-branch review returns findings, dispatch one fix
  subagent with the complete findings list — not one fixer per finding.
  Per-finding fixers each rebuild context and re-run suites; a real
  session's final-review fix wave cost more than all its tasks combined.

## File Handoffs

Everything you paste into a dispatch prompt — and everything a subagent
prints back — stays resident in your context for the rest of the session
and is re-read on every later turn. Briefs, reports, and dispatch prompts
are handoff artifacts: senior-engineer register (see using-megapowers,
Communication), conclusion first, self-contained. Hand artifacts over as
files:

- **Task brief:** before dispatching an implementer, run this skill's
  `scripts/task-brief PLAN_FILE N` — it extracts the task's full text to a
  uniquely named file and prints the path. Compose the dispatch so the
  brief stays the single source of requirements. Your dispatch should
  contain: (1) one line on where this task fits in the project; (2) the
  brief path, introduced as "read this first — it is your requirements,
  with the exact values to use verbatim"; (3) interfaces and decisions
  from earlier tasks that the brief cannot know; (4) your resolution of
  any ambiguity you noticed in the brief; (5) the report-file path and
  report contract. Exact values (numbers, magic strings, signatures, test
  cases) appear only in the brief.
- **Report file:** name the implementer's report file after the brief
  (brief `…/task-N-brief.md` → report `…/task-N-report.md`) and put it in
  the dispatch prompt. The implementer writes the full report there and
  returns only status, commits, a one-line test summary, and concerns.
- **Reviewer inputs:** the task reviewer gets three paths — the same brief
  file, the report file, and the review package — plus the global
  constraints that bind the task.
- Fix dispatches append their fix report (with test results) to the same
  report file and return a short summary; re-reviews read the updated file.

## Durable Progress

Conversation memory does not survive compaction. In real sessions,
controllers that lost their place have re-dispatched entire completed task
sequences — the single most expensive failure observed. Track progress in
a ledger file, not only in todos.

- At skill start, check for a ledger:
  `cat "$(git rev-parse --show-toplevel)/.megapowers/sdd/progress.md"`. Tasks listed there
  as complete are done — do not re-dispatch them; resume at the first task
  not marked complete.
- **Record the BASE before dispatching each implementer.** Append
  `Task N: base <sha7> (in progress)` to the ledger *before* you dispatch, where
  `<sha7>` is the current `git rev-parse --short HEAD`. The review step's
  `scripts/review-package BASE HEAD` needs this exact BASE, and it otherwise
  lives only in volatile conversation memory — a compaction between dispatch and
  review silently loses it, and `HEAD~1` is wrong for a multi-commit task. On
  resume, an `in progress` line with no matching `complete` line is the task to
  re-check: compare its BASE against `git log` to see what, if anything, landed.
- When a task's review comes back clean, append one line to the ledger in
  the same message as your other bookkeeping (it supersedes the `in progress`
  line for that task):
  `Task N: complete (commits <base7>..<head7>, review clean)`.
- The ledger is your recovery map: the commits it names exist in git even
  when your context no longer remembers creating them. After compaction,
  trust the ledger and `git log` over your own recollection.
- `git clean -fdx` will destroy the ledger (it's git-ignored scratch); if
  that happens, recover from `git log`.

## Prompt Templates

- [implementer-prompt.md](implementer-prompt.md) - Dispatch implementer subagent
- [task-reviewer-prompt.md](task-reviewer-prompt.md) - Dispatch task reviewer subagent (spec compliance + code quality)
- Final whole-branch review: use megapowers:requesting-code-review's [code-reviewer.md](../requesting-code-review/code-reviewer.md)

## Example Workflow

One task's full loop, compressed:

```
[task-brief for Task 2; dispatch implementer with brief + report paths + context]
Implementer: Added verify/repair modes, 8/8 tests passing, committed.
[review-package BASE..HEAD; dispatch task reviewer with the printed path]
Reviewer: Missing progress reporting (spec: "report every 100 items");
  extra --json flag (not requested); Important: magic number.
[Dispatch fix subagent with all findings]
Fixer: Removed --json, added progress reporting, extracted constant.
[Regenerate package; re-review]
Reviewer: Spec compliant. Quality: Approved.  → Mark Task 2 complete, ledger line.
```

Implementer questions get answered before it proceeds; after all tasks, the
final whole-branch reviewer runs, then finishing-a-development-branch.

## Practices to Hold To

Keep these invariants throughout execution:

- Don't start implementation on a main/master branch without explicit user consent.
- Don't skip task review, and don't accept a report missing either verdict — spec compliance and task quality are both required.
- Don't proceed with unfixed issues.
- Don't dispatch multiple implementation subagents in parallel (they conflict).
- Don't make a subagent read the whole plan file — hand it its task brief (`scripts/task-brief`) instead.
- Don't skip scene-setting context; the subagent needs to understand where the task fits.
- Don't ignore subagent questions; answer them before letting the subagent proceed.
- Don't accept "close enough" on spec compliance — spec issues from the reviewer mean the task is not done.
- Don't skip review loops: reviewer found issues → implementer fixes → review again.
- Don't let implementer self-review replace actual review; both are needed.
- Don't pre-judge findings for the reviewer (see Constructing Reviewer Prompts).
- Don't dispatch a task reviewer without a diff file — generate it first (`scripts/review-package BASE HEAD`) and name the printed path in the prompt.
- Don't move to the next task while the review has open Critical/Important issues.
- Don't re-dispatch a task the progress ledger already marks complete — check the ledger (and `git log`) after any compaction or resume.

**When the reviewer finds issues or a task fails:** dispatch a fresh fix
subagent carrying the implementer contract — never the spent original
implementer, and never a manual fix in your own context (context pollution).
Regenerate the review package and re-review; repeat until approved.

## Integration

**Required workflow skills:**
- **megapowers:using-git-worktrees** - Ensures an isolated workspace (creates one or verifies an existing one)
- **megapowers:writing-plans** - Creates the plan this skill executes
- **megapowers:requesting-code-review** - Code review template for the final whole-branch review
- **megapowers:finishing-a-development-branch** - Complete development after all tasks

**Subagents should use:**
- **megapowers:test-driven-development** - Subagents follow TDD for each task

**Alternative workflow:**
- **megapowers:executing-plans** - Inline single-writer execution when subagents are unavailable or per-task commits don't fit

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
