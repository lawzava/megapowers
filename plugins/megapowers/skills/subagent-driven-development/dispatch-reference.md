# Dispatch Reference

How to pick a model for each role, what goes into a dispatch, and how artifacts
move between the controller and its subagents.

## Model Selection

When the dispatch surface exposes a per-worker model selector, use the least
capable model that can handle each role and specify it explicitly. Transcribing
a complete spec and single-file mechanical fixes take the cheapest tier;
multi-file integration takes a standard model; design judgment and the final
whole-branch review take the most capable. Turn count beats token price: the
cheapest models routinely take two to three times the turns on multi-step work,
so hold a mid-tier floor for reviewers and for implementers working from prose
descriptions.

Codex v2 inherits the session model and effort even with fresh context;
`fork_turns = "none"` controls transcript inheritance, not model selection. Omit
the model field on that surface. If a task requires a different Codex model or
effort, use a separate role-aware surface or bounded `codex exec` run. Use
`delegate-resolve` when the role requires another provider.

## Constructing Dispatch Prompts

A dispatch describes one task, not the session's history. A fresh subagent needs its task, the interfaces it touches, and the global constraints. Do not paste prior-task summaries.

For reviewers:

- Copy the binding requirements verbatim from the plan's Global Constraints section or the spec: exact values, exact formats, and the stated relationships between components. The reviewer's template already carries the process rules; the constraints block is for what this project's spec demands.
- Never pre-judge findings. Do not tell a reviewer to ignore, downgrade, or not flag anything; if you expect a false positive, let the reviewer raise it and adjudicate it in the loop. Likewise skip open-ended directives ("check all uses") without a concrete task-specific reason, and do not ask the reviewer to re-run tests the implementer already ran on the same code.
- A finding the plan itself mandates is the human's decision, like any plan contradiction: present the finding beside the plan text and ask which governs. Do not dismiss it because the plan mandates it, and do not dispatch a fix that contradicts the plan without asking.

For fixes:

- Dispatch fix subagents for every Specification Compliance Fail and for Critical and Important engineering findings. A failed specification verdict with only locally Minor findings still requires correction or explicit requirement-owner authorization and re-review; never record it as deferred Minor work. Record Engineering Standards Minor findings in the ledger and point the final whole-branch review at that list so it can triage what must be fixed before merge; a roll-up nobody reads is a silent discard.
- A Specification Compliance Fail requires correction or explicit approval and
  must never be treated as Minor.
- Every fix dispatch carries the implementer contract: the fixer re-runs the tests covering its change and reports the covering test files, the command run, and the output. Name the covering tests in the dispatch; a one-line fix does not need the whole suite. Dispatch the re-review only once all three pieces of evidence are present.
- If the final whole-branch review returns findings, dispatch one fix subagent with the complete findings list, not one fixer per finding. Per-finding fixers each rebuild context and re-run the suites.

## File Handoffs

Everything you paste into a dispatch, and everything a subagent prints back, stays resident in your context for the rest of the session. Hand artifacts over as files: senior-engineer register (see using-megapowers, Communication), conclusion first, self-contained. `scripts/sdd-workspace` resolves the working-tree directory all of these artifacts live in.

Each delegate has one report channel. For small reports, return the report
directly. For bulky reports, write the report file and return only status plus
its path. Do not duplicate claims and evidence across chat and file.

- **Task brief:** `scripts/task-brief PLAN_FILE N` extracts the task's full text to a file and prints the path. The brief is the single source of requirements; exact values (numbers, magic strings, signatures, test cases) appear only there. The dispatch adds where the task fits in the project, the brief path introduced as the requirements to follow verbatim, interfaces and decisions from earlier tasks the brief cannot know, your resolution of any ambiguity you noticed in it, and the report path with its contract. Never hand a subagent the whole plan file.
- **Report file:** named after the brief (task-N-brief.md pairs with
  task-N-report.md). For a bulky report, the implementer writes the full report
  there and returns only status plus the path. Fixes append to the same file.
- **Reviewer inputs:** the brief, the report, and the review package as three paths, plus the binding constraints. `scripts/review-package BASE HEAD` writes the commit list, stat summary, and full diff with context to one file and prints its path, so the reviewer reads everything in one call. The final review gets the same treatment with the branch's merge base (for example `git merge-base main HEAD`) as BASE.

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
