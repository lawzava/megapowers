---
name: finishing-a-development-branch
description: Use when implementation is complete and tests pass, and you need to decide how to integrate the work — merge, open a PR, keep, or discard. Triggers on "how do I ship this", "merge or PR?", "wrap up the branch", "the work is done". Code review (requesting-code-review) typically comes first.
license: MIT
---

# Finishing a Development Branch

## Overview

The work is done and needs a destination. Verify tests, detect the workspace state, present a fixed menu, execute the choice, and clean up only what this process owns.

## Before offering options

Run the project's test suite and read the output. If tests fail, report the failures and stop; no menu until they pass.

Detect the workspace state, because it decides both the menu and the cleanup. Compare `git rev-parse --git-dir` with `git rev-parse --git-common-dir`, both resolved to physical paths:

- Equal: a normal repo. Standard menu, nothing to remove afterward.
- Unequal, on a named branch: a worktree. Standard menu, provenance based cleanup.
- Unequal, detached HEAD: an externally managed workspace. Reduced menu, no cleanup.

Identify the base branch (merge-base against main or master, or ask) so the merge and PR targets are correct.

## The menu

Normal repo and named branch worktree, exactly these 4 options, no added commentary:

```
Implementation complete. What would you like to do?

1. Merge back to <base-branch> locally
2. Push and create a Pull Request
3. Keep the branch as-is (I'll handle it later)
4. Discard this work

Which option?
```

Detached HEAD, exactly these 3 (no local merge from an externally managed workspace):

```
Implementation complete. You're on a detached HEAD (externally managed workspace).

1. Push as new branch and create a Pull Request
2. Keep as-is (I'll handle it later)
3. Discard this work

Which option?
```

Options 2 and 3 preserve the worktree. Options 1 and 4 are the only ones that clean up.

## Executing the choice

**Merge locally.** Before leaving the worktree, capture its identity: `WORKTREE_PATH` from `git rev-parse --show-toplevel`, plus the resolved `GIT_DIR` and `GIT_COMMON`. Once you cd to the main root those commands report the main repo, and cleanup would find nothing to remove. From the main repo root, update the base branch, merge the feature branch, and run the tests on the merged result before removing anything. Only after the merge succeeds and tests pass: clean up the worktree (below), then delete the branch. Guard the delete on the branch not being checked out in any remaining worktree (`git worktree list --porcelain`); a branch still checked out in a live harness owned workspace cannot be deleted, so say it will resolve when that workspace exits and leave the branch in place.

**Push and create a PR.** Push the branch, then open the PR with `gh`. If `gh` is missing or the remote is not GitHub, the option becomes push only: report that the branch is pushed, point at the compare URL, and never claim a PR was created when only a push happened. Never force-push unless the user explicitly asks. Leave the worktree in place; the user needs it to iterate on PR feedback.

**Keep as-is.** Report the branch name and worktree path. Touch nothing.

**Discard.** The one destructive path, gated on explicit consent. List exactly what will be permanently deleted: the branch, its commits, and the worktree path. Ask the user to type the word `discard` and wait for that exact word before acting. On confirmation, capture the worktree identity as in the merge path, cd to the main root, clean up the worktree, then force delete the branch with the same checked-out-anywhere guard.

## Worktree cleanup

Only the merge and discard paths reach this. Use the `WORKTREE_PATH`, `GIT_DIR`, and `GIT_COMMON` captured before any cd; do not re-derive them after moving. If `GIT_DIR` equals `GIT_COMMON` there was no worktree and you are done.

Remove a worktree only if `WORKTREE_PATH` sits under `.worktrees/` or `worktrees/`; that provenance means this process created it. Run `git worktree remove` from the main repo root, never from inside the worktree being removed, then `git worktree prune` to clear stale registrations. Any other workspace belongs to the host environment: leave it in place, or use the platform's workspace exit tool if one exists.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
