# Coordinator Subagent Prompt Template

Use this template only for recursive SDD nodes.

## Required inputs

- Run ID: `[RUN_ID]`
- Node: `[NODE_PATH]`
- Parent: `[PARENT_NODE]`
- Brief ref: `[BRIEF_REF]`
- Claim object: `[CLAIM_OID]`
- Session ID: `[SESSION_ID]`
- Harness: `[codex|claude]`
- Remaining depth: `[N]`
- Descendant agent budget: `[N]`
- Writer limit: `[N]`
- Integration limit: `[N]`

Read the immutable brief from the ref before acting. The brief is the complete
task requirement. Do not broaden it from conversation history.

On first entry, initialize your exact node branch at the immutable brief base
with `sdd-worktree branch-init RUN NODE BASE`. This consumes no writer slot or worktree and
must succeed before any child `brief-put` or dispatch and before candidate integration.

## Ownership

You own integration only for `[NODE_PATH]`. Children own their branches and
assigned worktrees. You are read-only over child-owned source paths and Git state.
Your only child-worktree write is an explicit ignored handoff path for the
brief, report, review package, or bounded evidence. You may not advance a child branch, your parent branch, or the run target.
Return one final result to the parent. Do not relay leaf-agent messages,
partial patches, or review chatter.

## Decomposition gate

Decompose only when the brief authorizes it, remaining depth is positive,
ownership is disjoint, dependencies are complete, and each child has an
independent executable acceptance check. Otherwise execute this node through
one writer worktree. For child budgets enforce
`sum(1 + child_budget) <= descendant_budget` and reserve one unallocated agent
for review or repair.

When a child brief authorizes decomposition and receives positive remaining
depth and descendant budget, claim it under its unique child session, then
dispatch it with this coordinator prompt and no writer slot or worktree. Pass
the reduced depth and budget plus the global limits. Join only its final result.
Any child not selected as a nested coordinator follows the writing lifecycle.

## Recovery

After interruption, inspect `sdd-run status RUN_ID`, verify any in-progress
branch, and resume from the first blocked or missing result. A blocked result is replaceable
through `result-put --expected`; never treat it as done or infer completion from
branch existence. Do not auto-release a stale claim.

## Child lifecycle

For every writing child: publish its immutable brief, atomically claim it, acquire a
writer slot, create its branch and linked worktree, then dispatch the writer
with the absolute path. Run the existing fresh review and bounded fix loop.
Integrate only a clean child through a disposable candidate and the integration
slot. Run the declared combined verification before compare-and-swap promotion.
Remove the integration candidate and release the integration slot after every
attempt. After verified integration, remove the child worktree, release the writer slot,
and release the child's exact node claim. Retain its branch and
result.

Write every handoff to its declared ignored path. After the reader and
`result-put` no longer need a generated handoff, remove only that known file so
the worktree can become clean. Preserve unknown ignored files and block instead
of removing them.

## Failure rules

Do not integrate a blocked, timed out, unreviewed, or unverified child. Do not fall back to the parent checkout.
Do not steal stale claims. Do not use reset,
force removal, force push, or cleanup. Record the blocked result and return it
when recovery cannot proceed safely.

## Result

Write the terminal result through `sdd-run result-put`. A done result names the
full base and head commits, owned branch, exact verification commands, zero exit
codes, sanitized evidence paths, and an empty unresolved list. Both statuses include the full base and head commits, owned branch, and verification array.
A blocked result contains concrete unresolved items and records the commands
that did run. After `result-put`, release this coordinator's exact node claim.
Return only: status, run ID, node, branch, head, verification summary,
unresolved items, and the result ref.

## Harness dispatch

Codex: use native subagents with `fork_turns = "none"` and self-contained
briefs. Do not assume a per-spawn model or effort selector. Stop before the fifth task-name component beneath `/root`.

Claude Code: use fresh Agent calls for independent children and resume only the
same assignment. Do not use agent teams because teams do not nest. Use Git
commands and the shipped scripts instead of writing `.git` paths directly.
