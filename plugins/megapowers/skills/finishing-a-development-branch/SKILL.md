---
name: finishing-a-development-branch
description: Use when verified work needs branch integration or cleanup. Triggers on "work is done", "ship this", "merge or PR", or "wrap up the branch". Code review usually comes first.
license: MIT
---

# Finishing a Development Branch

## Overview

The work is done and needs a destination. Verify tests, detect the workspace
state, present a fixed menu, execute the choice, and clean up only a worktree
with recorded process ownership.

## Before offering options

Run the project's test suite and read the output. If tests fail, report the
failures and stop; no menu until they pass.

Detect the workspace state, because it decides both the menu and the cleanup.
Compare `git rev-parse --git-dir` with `git rev-parse --git-common-dir`, both
resolved to physical paths:

- Equal: a normal repo. Standard menu, nothing to remove afterward.
- Unequal, on a named branch: a worktree. Standard menu, provenance based
  cleanup.
- Unequal, detached HEAD: an externally managed workspace. Reduced menu, no
  cleanup.

Identify the base branch (merge-base against main or master, or ask) so the
merge and PR targets are correct.

## The menu

If the user already stated the destination, validate the relevant gates and
execute that choice directly. Do not show the menu. Explicit destructive
confirmation still applies to discard. Show the menu only when the destination
is unclear.

Normal repo and named branch worktree, exactly these 4 options, no added
commentary:

```
Implementation complete. What would you like to do?

1. Merge back to <base-branch> locally
2. Push and create a Pull Request
3. Keep the branch as-is (I'll handle it later)
4. Discard this work

Which option?
```

Detached HEAD, exactly these 3 (no local merge from an externally managed
workspace):

```
Implementation complete. You're on a detached HEAD (externally managed workspace).

1. Push as new branch and create a Pull Request
2. Keep as-is (I'll handle it later)
3. Discard this work

Which option?
```

Cleanup follows the option's meaning, not its number: merge and discard clean
up a worktree only when its recorded provenance says this process owns it;
push/PR and keep always leave it in place. Detached HEAD workspaces are
externally managed and are never cleaned up, whichever option is chosen.

## Executing the choice

**Merge locally.** Before leaving the worktree, capture its identity:
`WORKTREE_PATH` from `git rev-parse --show-toplevel`, plus the resolved
`GIT_DIR` and `GIT_COMMON`. Once you cd to the main root those commands report
the main repo, and cleanup would find nothing to remove. From the main repo
root, update the base branch, merge the feature branch, and run the tests on
the merged result before removing anything. Only after the merge succeeds and
tests pass: clean up the worktree (below), then delete the branch. Guard the
delete on the branch not being checked out in any remaining worktree (`git
worktree list --porcelain`); a branch still checked out in a live workspace
cannot be deleted, so say it will resolve when that workspace exits and leave
the branch in place.

**Push and create a PR.** On a detached HEAD, first create a named branch at
the current commit; use a name the user or approved plan supplied, otherwise
ask. Push the branch, then open the PR with `gh`. If `gh` is missing or the
remote is not GitHub, the option becomes push only: report that the branch is
pushed, point at the compare URL, and never claim a PR was created when only a
push happened. Never force-push unless the user explicitly asks. Leave the
worktree in place; the user needs it to iterate on PR feedback.

**Keep as-is.** Report the branch name and worktree path. On a detached HEAD,
report the commit SHA and worktree path instead. Touch nothing.

**Discard.** The one destructive path, gated on explicit consent. In a normal
repo or named-branch worktree, list exactly what will be permanently deleted:
the branch, its commits, and the worktree path. Ask the user to type the word
`discard` and wait for that exact word before acting. On confirmation, capture
the worktree identity as in the merge path, cd to the main root, clean up the
worktree, then force delete the branch with the same checked-out-anywhere
guard. On a detached HEAD, do not delete or modify the externally managed
workspace. Explain that discard leaves its unreferenced commits and files for
the workspace owner to remove, then report the commit SHA and path.

## Worktree cleanup

Only the merge and discard paths reach this. Use the `WORKTREE_PATH`,
`GIT_DIR`, and `GIT_COMMON` captured before any cd; do not re-derive them after
moving. If `GIT_DIR` equals `GIT_COMMON` there was no worktree and you are
done.

Before removal, compute the record key exactly as
`megapowers:using-git-worktrees` specifies, using the resolved `WORKTREE_PATH`
with no trailing newline, then read
`<resolved-GIT_COMMON>/megapowers-worktree-ownership/<key>.record`. It must use
the schema in `megapowers:using-git-worktrees` and name the same resolved
worktree path, common Git directory, and branch; it must identify a creator and
set `cleanup_authority=process`. A path under any particular directory is not
evidence of ownership. If the record is missing, ambiguous, or belongs to
another tool or process, leave the worktree in place and report why cleanup was
skipped.

With a matching record, run `git worktree remove` from the main repo root,
never from inside the worktree being removed, then `git worktree prune` to
clear stale registrations. Remove the provenance record only after both
commands succeed. Any other workspace belongs to its recorded owner: leave it
in place or use that owner's cleanup mechanism.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
