---
name: subagent-driven-development
description: Use when a written plan has independent tasks for per-task subagent implementation and review, including recursive coordinators with isolated branches. Triggers on "subagent per task", "fan out plan tasks", or "multi-writer execution". Use executing-plans for inline work.
license: MIT
---

# Subagent-Driven Development

Execute a written plan with fresh implementers, two-stage task review (spec
compliance, then code quality), and one broad whole-branch review at the end.

## Execution modes

**Sequential mode:** one writer at a time on one feature branch. Use it when
plan tasks overlap, dependencies form one critical path, task-local commits are
not authorized, or the harness cannot provide nested subagents.

**Recursive coordinator mode:** independent plan roots run concurrently on
owned branches and ephemeral worktrees. Each coordinator may decompose an
authorized node, review and integrate its children, and return one verified
branch to its parent. One elected run owner alone advances the feature target.

Recursive mode is explicit. It starts only when the accepted plan supplies
`Blocked by`, `Parallel safety`, `Ownership`, and `May decompose`, the human has
separately authorized task-local commits for that run, every participant uses
the same Git clone, and the selected harness supports the requested depth.
Otherwise use sequential mode without weakening review or worktree isolation.

**Why subagents:** each task gets deliberately fresh context that you construct.
Some harnesses can inherit or fork parent history, so request a fresh context
explicitly for implementers and reviewers. Hand each one exactly what its task
needs, which keeps it focused and preserves your own context for coordination.

**Commit cadence:** Sequential SDD commits once per task when the human selects
this skill, retaining its existing per-task commit opt-in semantics. Its ledger
records commit ranges. Recursive SDD requires separate, explicit, run-specific
human authorization for task-local commits, recorded by `sdd-run init
--allow-task-commits`. Selecting this skill or the recursive workflow alone is
not authorization. Recursive run refs record node branches and results. Git
history survives the compactions that erase conversation memory. Commits land
on the feature branch or assigned worktree; do not start implementation on a
main or master branch without explicit consent. In neither mode does task-local
commit authorization grant push, merge-to-main, pull request, release, or
deploy authority. If per-task commits do not fit, use
megapowers:executing-plans instead.

**Continuous execution:** do not check in with your human partner between tasks. Stop only for a BLOCKED status you cannot resolve, ambiguity that prevents progress, or completion of all tasks. Narrate at most one short line between tool calls; the ledger or run refs and tool results carry the record.

## When to Use

Use this skill when a written plan exists and subagents are available. In
sequential mode, selecting the skill opts into per-task commits. Select
sequential mode for overlapping or tightly coupled tasks. Select recursive
coordinator mode only for explicitly safe, disjoint roots under the execution
gate above, including its separate run-specific commit authorization. With no
plan, brainstorm first. When subagents are unavailable or the human wants
inline execution with their own commit cadence, use megapowers:executing-plans
(the same criterion appears in writing-plans and executing-plans).

## Workspace boundary

Both modes maintain one active writer per branch and worktree. Sequential mode
keeps the existing `sdd-workspace` and `megapowers:using-git-worktrees` flow.
Recursive mode uses `sdd-run` plus `sdd-worktree` exclusively and never layers the two workspace managers.
`sdd-worktree` resolves linked worktrees through
the same Git clone, including calls from an assigned node worktree. Never write
in the parent checkout.

Recursive coordinators call `review-package BASE HEAD OUTFILE` with an explicit
ignored path inside the assigned node worktree. They never let
`review-package` fall back to `sdd-workspace`. The coordinator owns only this
ignored handoff path, not the child's source paths or Git state. After the
reviewer and `result-put` no longer need the package, remove that exact generated
file. Unknown ignored files keep the worktree dirty and block removal.

## Sequential process

Setup, once: read the plan file once, create todos for every task, and check for an existing progress ledger (see Durable Progress). Then scan the plan for conflicts before dispatching Task 1: tasks that contradict each other or the Global Constraints, and anything the plan explicitly mandates that the review rubric treats as a defect. Present everything you find to your human partner as one batched question, each finding beside the plan text that mandates it, asking which governs, rather than one interrupt per discovery mid-plan. Under an active autonomous-run charter at level `autonomous` or `on-the-loop`, do not stop for the batch: resolve each conflict with the least-surprise reading and journal it with a confidence, though a conflict that makes a task's acceptance criteria contradictory is still a real blocker. A clean scan needs no comment; the review loop remains the net for conflicts that only emerge from implementation.

Per task, in order:

1. Record the BASE commit in the ledger, generate the task's brief with `scripts/task-brief PLAN_FILE N`, and dispatch an implementer using [implementer-prompt.md](implementer-prompt.md) with the brief path, report path, and scene-setting context. Answer any questions it asks before it proceeds. It implements, tests, commits, and self-reviews; self-review never replaces the task review.
2. On DONE, generate the review package with `scripts/review-package BASE HEAD` and dispatch a fresh task reviewer using [task-reviewer-prompt.md](task-reviewer-prompt.md) with the printed path. Both verdicts are required; a report missing either spec compliance or code quality is incomplete.
3. Findings go to a fresh fix subagent: the implementer side fixes, the reviewer never edits, and you never fix in your own context. Any Specification Compliance Fail requires correction or explicit requirement-owner authorization regardless of local finding severity, followed by re-review; it must never be rolled into the Minor backlog. Regenerate the package and dispatch a fresh re-review; repeat until the spec verdict is clean and quality is approved. "Close enough" on spec compliance is a failed review, and a task with open Critical or Important engineering findings is not done.
4. Mark the task complete in the todo list and append the ledger line.

After all tasks: dispatch the final whole-branch review using megapowers:requesting-code-review's [code-reviewer.md](../requesting-code-review/code-reviewer.md) on the most capable available model. For a branch touching billing, auth, concurrency, or security, add an independent different-vendor pass via mega-orchestration:cross-model-verification if installed; a same-model review shares the author's blind spots. Then use megapowers:finishing-a-development-branch.

## Recursive setup

1. Confirm the current target is a clean non-protected feature branch.
2. Read the accepted plan once. Reject missing execution fields or overlapping
   tasks marked safe.
3. Create or join the run through `scripts/sdd-run`. Record the run ID, session
   ID, owner object ID, target base, worktree limits, and agent budget.
4. The run owner publishes immutable briefs for currently unblocked roots.
   Other sessions join the same run and atomically claim disjoint roots. When a
   blocked root's dependencies are integrated, the owner publishes its brief
   from the current target head.
5. On first entry, every root or nested coordinator initializes its exact node
   branch at the immutable brief base with
   `sdd-worktree branch-init RUN NODE BASE`. This consumes no writer slot or worktree and
   must succeed before any child `brief-put` or dispatch and before candidate integration.
   A coordinator publishes child briefs only when its node says
   `May decompose: Yes`, remaining depth is positive, ownership is disjoint,
   and each child has an independent acceptance check.
6. Before a child worktree exists, resolve the primary checkout from the shared
   Git common directory and write the child brief JSON to a temporary known
   tool-owned ignored input under the primary checkout's
   `.worktrees/$run/inputs/` root. Use it only for `brief-put`; remove that exact
   input immediately after `brief-put` succeeds and before
   `sdd-worktree node-add`,
   then remove any empty known input directories. Never remove or leave an
   unknown ignored artifact.
7. For a writing child, publish the brief and claim the child with its unique
   session. The coordinator acquires a writer slot and creates the linked worktree
   with that same session before dispatch. For a direct current-node writer, use the
   coordinator's existing node session for both claim-bound operations. Record
   the exact slot number and object ID. After `node-add`, materialize the
   immutable brief and ref-derived inputs inside the new child worktree; do not
   copy the deleted temporary input. The dispatch includes the absolute path and
   all registry bindings.
8. The existing implement, fresh review, fix, and fresh re-review loop runs on
   the child branch. Reviewers stay read-only. A fixer gets exclusive access to
   the existing node worktree.
9. The final writer for a writing node, not its coordinator, publishes a
   `done` result after clean review, or a `blocked` result when it cannot
   proceed. If a fixer supersedes the implementer, terminal publication
   responsibility transfers to that fixer. It calls `sdd-run result-put` with
   the exact session that owns both the active node claim and writer slot, then
   returns the printed result OID and its immutable evidence paths. The
   coordinator verifies that OID against the exact node result ref before
   cleanup and verifies the stored status, branch head, result JSON, and evidence.
10. For a verified `done` child, the coordinator acquires the single integration
   slot, merges the child into a disposable candidate, runs integrated
   verification, and uses compare-and-swap to advance its branch. A failed
   candidate never advances the branch. After candidate removal it releases
   the integration slot.
11. After verified child integration, or immediately after verifying a
   `blocked` writer result, remove known handoffs, remove the clean node
   worktree, release the exact writer slot, then release the exact node claim.
   Use the result-owning session and compare against the recorded slot and claim
   object IDs. Retain the branch and terminal result. Unknown ignored artifacts
   or a dirty worktree block teardown and therefore block closure. After every
   required child is integrated, the coordinator records its own terminal
   result, releases its exact node claim, and returns only that result to its
   parent.
12. For each completed root, the run owner acquires the `@target` integration
    slot, creates and verifies one target candidate, promotes it, removes the
    candidate, and releases the target slot. Only then does it heartbeat the
    owner ref and start the next root. After all roots, it performs final
    review and serial full validation, then closes the run. Before close it
    confirms no writer or integration slots and no run worktrees remain.
    Publish and cleanup remain separate human decisions.

If this coordinator executes its current node through a writer, it acquires the
slot for `[NODE_PATH]` with its own coordinator session and creates that node's
worktree. The writer publishes the node's only terminal result with that same
registry session. After result verification, the coordinator follows the exact
handoff, worktree, slot, and claim teardown above, performs no child candidate
integration, and returns the verified result OID to its parent.

The coordinator dispatch is [coordinator-prompt.md](coordinator-prompt.md).
Coordinators are read-only over child-owned source paths and Git state. They
integrate child commits only through their own disposable candidate. Their sole
child-worktree write is the explicit ignored handoff artifact described above.

## Resource limits

For a coordinator with descendant budget `B`, every child allocation costs one
agent for the child plus the descendant budget passed to that child:
`sum(1 + child_budget) <= B`. Keep one agent unallocated while a write awaits
review or repair. Do not reallocate a completed Codex worker's slot until the
runtime's idle-thread accounting has been measured in the current version.

Agent capacity and worktree capacity are separate. The run defaults to three writer worktrees and one integration worktree.
Read-only coordinators and
reviewers consume no worktree. When a slot is unavailable, wait or execute the
node sequentially after a slot is released. Never write in the parent checkout.

Dispatch an authorized child coordinator with
[coordinator-prompt.md](coordinator-prompt.md) and no writer slot or worktree.
It owns integration for that child node and returns one final result. A leaf or
a coordinator that cannot safely decompose executes through one writer slot and
assigned worktree.

Run focused tests concurrently only when their caches are concurrency-safe.
Run repository-wide validation serially with one bounded shared cache or one
bounded per-run cache.

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

- Dispatch fix subagents for every Specification Compliance Fail and for Critical and Important engineering findings. A failed specification verdict with only locally Minor findings still requires correction or explicit requirement-owner authorization and re-review; never record it as deferred Minor work. Record Engineering Standards Minor findings in the ledger and point the final whole-branch review at that list so it can triage what must be fixed before merge; a roll-up nobody reads is a silent discard.
- Every fix dispatch carries the implementer contract: the fixer re-runs the tests covering its change and reports the covering test files, the command run, and the output. Name the covering tests in the dispatch; a one-line fix does not need the whole suite. Dispatch the re-review only once all three pieces of evidence are present.
- If the final whole-branch review returns findings, dispatch one fix subagent with the complete findings list, not one fixer per finding; per-finding fixers each rebuild context and re-run suites, and a real session's fix wave run that way cost more than all its tasks combined.

## File Handoffs

Everything you paste into a dispatch, and everything a subagent prints back,
stays resident in your context for the rest of the session. Hand artifacts over
as files: senior-engineer register (see using-megapowers, Communication),
conclusion first, self-contained. Sequential mode resolves these paths through
`scripts/sdd-workspace`. Recursive mode writes each handoff to an explicit ignored path in the assigned child node worktree once that worktree exists and never calls `sdd-workspace`.
The sole pre-worktree exception is the temporary child brief input under the
primary checkout's `.worktrees/$run/inputs/` root described above. Remove it
immediately after immutable publication. Derive the later worktree copy from
the immutable brief ref, not from that deleted input. To make clean worktree
removal possible, remove only that generated handoff after its reader and `result-put` no longer need it; preserve every unknown file.

- **Task brief:** `scripts/task-brief PLAN_FILE N` extracts the task's full text to a file and prints the path. The brief is the single source of requirements; exact values (numbers, magic strings, signatures, test cases) appear only there. The dispatch adds where the task fits in the project, the brief path introduced as the requirements to follow verbatim, interfaces and decisions from earlier tasks the brief cannot know, your resolution of any ambiguity you noticed in it, and the report path with its contract. Never hand a subagent the whole plan file.
- **Report file:** named after the brief (task-N-brief.md pairs with task-N-report.md). The implementer writes the full report there and returns only status, commits, a one-line test summary, and concerns. Fix dispatches append their fix report to the same file; re-reviews read the updated file.
- **Reviewer inputs:** the brief, the report, and the review package as three paths, plus the binding constraints. `scripts/review-package BASE HEAD` writes the commit list, stat summary, and full diff with context to one file and prints its path, so the reviewer reads everything in one call. The final review gets the same treatment with the branch's merge base (for example `git merge-base main HEAD`) as BASE.

## Durable Progress

Conversation memory does not survive compaction; controllers that lost their
place have re-dispatched entire completed task sequences, the single most
expensive failure observed. Use the mode-specific recovery map below.

Sequential mode keeps `.megapowers/sdd/progress.md`:

- At skill start, read the ledger. Tasks marked complete there are done; never re-dispatch them. Resume at the first task not marked complete.
- Before each dispatch, append `Task N: base <sha7> (in progress)` with the current short HEAD. The review step needs this exact BASE, and it otherwise lives only in volatile conversation memory.
- On a clean review, append `Task N: complete (commits <base7>..<head7>, review clean)`, superseding the in-progress line.
- On resume, an in-progress line with no matching complete line marks the task to re-check against `git log`. After compaction, trust the ledger and git history over your own recollection. `git clean -fdx` destroys the ledger (it is git-ignored scratch); if that happens, recover from `git log`.

Recursive mode treats its private run refs under `refs/megapowers/runs/`, node
branches, commits, immutable briefs, and result trees as authoritative. The
generation ref is the snapshot boundary for registry mutations. After
compaction or `git clean`, run `sdd-run status RUN_ID`, verify any in-progress
branch, and resume from the first blocked or missing result. A blocked result is replaceable
with `result-put --expected`; it is not completion, and the run cannot close until every root result is `done`.
Never infer `done` from branch existence and never auto-release a stale claim.

## Prompt Templates

- [implementer-prompt.md](implementer-prompt.md) for the implementer subagent
- [task-reviewer-prompt.md](task-reviewer-prompt.md) for the task reviewer (spec compliance + code quality)
- [coordinator-prompt.md](coordinator-prompt.md) for a recursive coordinator
- Final whole-branch review: megapowers:requesting-code-review's [code-reviewer.md](../requesting-code-review/code-reviewer.md)

## Example Workflow

### Sequential example

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

### Recursive coordinator example

Root coordinators A and B atomically claim disjoint roots. A publishes a child
brief for nested coordinator A1. A1 joins its writing descendants, uses the
single integration slot to verify and advance only A1's branch, records one
terminal result, and returns it to A. A then joins A1's result with its other
children through the same single integration slot and returns one terminal
result to the run owner. B does the same for its own root without touching A's
branch or worktrees. The run owner receives one final result from each root coordinator,
serially promotes A and B through the `@target` integration slot,
and sends the final joined output to the lead for whole-branch review and
validation.

## Integration

**Required workflow skills:** megapowers:writing-plans creates the plan this
skill executes; megapowers:requesting-code-review supplies the final
whole-branch review template; megapowers:finishing-a-development-branch
completes the branch after all tasks. Sequential mode also uses
megapowers:using-git-worktrees. Recursive mode uses only its same-clone
`sdd-run` and `sdd-worktree` lifecycle.

**Subagents should use** megapowers:test-driven-development for each task.

**Alternative workflow:** megapowers:executing-plans for inline single-writer execution when subagents are unavailable or per-task commits do not fit.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
