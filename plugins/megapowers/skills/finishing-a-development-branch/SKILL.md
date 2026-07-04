---
name: finishing-a-development-branch
description: Use when implementation is complete and tests pass, and you need to decide how to integrate the work — merge, open a PR, keep, or discard. Triggers on "how do I ship this", "merge or PR?", "wrap up the branch", "the work is done". Code review (requesting-code-review) typically comes first.
license: MIT
---

# Finishing a Development Branch

## Overview

Guide completion of development work by presenting clear options and handling the chosen workflow.

**Core principle:** Verify tests, detect environment, present options, execute the choice, clean up.

## The Process

### Step 1: Verify Tests

Before presenting options, verify tests pass:

```bash
# Run project's test suite
npm test / cargo test / pytest / go test ./...
```

If tests fail:

```
Tests failing (<N> failures). Fix before completing:

[Show failures]

Cannot proceed with merge/PR until tests pass.
```

Stop here; don't proceed to Step 2.

If tests pass, continue to Step 2.

### Step 2: Detect Environment

Determine workspace state before presenting options:

```bash
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
```

This determines which menu to show and how cleanup works:

| State | Menu | Cleanup |
|-------|------|---------|
| `GIT_DIR == GIT_COMMON` (normal repo) | Standard 4 options | No worktree to clean up |
| `GIT_DIR != GIT_COMMON`, named branch | Standard 4 options | Provenance-based (see Step 6) |
| `GIT_DIR != GIT_COMMON`, detached HEAD | Reduced 3 options (no merge) | No cleanup (externally managed) |

### Step 3: Determine Base Branch

```bash
# Try common base branches
git merge-base HEAD main 2>/dev/null || git merge-base HEAD master 2>/dev/null
```

Or ask: "This branch split from main - is that correct?"

### Step 4: Present Options

Normal repo and named-branch worktree — present exactly these 4 options:

```
Implementation complete. What would you like to do?

1. Merge back to <base-branch> locally
2. Push and create a Pull Request
3. Keep the branch as-is (I'll handle it later)
4. Discard this work

Which option?
```

Detached HEAD — present exactly these 3 options:

```
Implementation complete. You're on a detached HEAD (externally managed workspace).

1. Push as new branch and create a Pull Request
2. Keep as-is (I'll handle it later)
3. Discard this work

Which option?
```

Keep the options concise; don't add explanation.

### Step 5: Execute Choice

#### Option 1: Merge Locally

```bash
# Capture worktree identity BEFORE moving — once we cd to the main root, a later
# `git rev-parse --show-toplevel` reports the main root and Step 6 finds nothing
# to remove (leaving the branch checked out, so `git branch -d` then fails).
WORKTREE_PATH=$(git rev-parse --show-toplevel)
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)

# Get main repo root for CWD safety
MAIN_ROOT=$(git -C "$(git rev-parse --git-common-dir)/.." rev-parse --show-toplevel)
cd "$MAIN_ROOT"

# Merge first — verify success before removing anything
git checkout <base-branch>
git pull
git merge <feature-branch>

# Verify tests on merged result
<test command>

# Only after merge succeeds: cleanup worktree (Step 6, using the captured
# WORKTREE_PATH), then delete branch
```

Then clean up the worktree (Step 6, using the captured variables), then delete the
branch. Step 6 removes a worktree megapowers owns, but leaves a **harness-owned**
workspace in place — and a branch still checked out in a live worktree cannot be
deleted. So guard the delete on the branch no longer being checked out anywhere:

```bash
if git worktree list --porcelain | grep -q "^branch refs/heads/<feature-branch>$"; then
  echo "Branch <feature-branch> is still checked out in a workspace this harness owns; \
it will be cleaned up when that workspace exits. Leaving the branch in place."
else
  git branch -d <feature-branch>
fi
```

#### Option 2: Push and Create PR

```bash
# Push branch, then open the PR
git push -u origin <feature-branch>
gh pr create --base <base-branch> --head <feature-branch> --fill
```

If `gh` is not installed or the remote is not GitHub, the option is push-only:
tell the user the branch is pushed and open the PR from the compare URL git prints
after the push (or the host's web UI). Don't claim a PR was created when it wasn't.

Leave the worktree in place — the user needs it alive to iterate on PR feedback.

#### Option 3: Keep As-Is

Report: "Keeping branch <name>. Worktree preserved at <path>."

Leave the worktree in place.

#### Option 4: Discard

Confirm first:

```
This will permanently delete:
- Branch <name>
- All commits: <commit-list>
- Worktree at <path>

Type 'discard' to confirm.
```

Wait for the exact confirmation.

If confirmed:

```bash
# Capture worktree identity BEFORE moving (same reason as Option 1).
WORKTREE_PATH=$(git rev-parse --show-toplevel)
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)

MAIN_ROOT=$(git -C "$(git rev-parse --git-common-dir)/.." rev-parse --show-toplevel)
cd "$MAIN_ROOT"
```

Then clean up the worktree (Step 6, using the captured variables), then force-delete
the branch — guarded the same way, since a harness-owned worktree is left in place
and its checked-out branch cannot be deleted:

```bash
if git worktree list --porcelain | grep -q "^branch refs/heads/<feature-branch>$"; then
  echo "Branch <feature-branch> is still checked out in a workspace this harness owns; \
it will be discarded when that workspace exits."
else
  git branch -D <feature-branch>
fi
```

### Step 6: Cleanup Workspace

Only runs for Options 1 and 4. Options 2 and 3 always preserve the worktree.

Use the `GIT_DIR`, `GIT_COMMON`, and `WORKTREE_PATH` you captured at the top of the
option **before** changing directory — do not re-derive them here, because you have
already `cd`'d to the main root and `git rev-parse --show-toplevel` would now report
the main root instead of the feature worktree.

If `GIT_DIR == GIT_COMMON`: the captured location was a normal repo (not a worktree), so there is nothing to remove. Done.

If `WORKTREE_PATH` is under `.worktrees/` or `worktrees/`: megapowers created this worktree, so we own cleanup. (You are already in `MAIN_ROOT` from the option's merge/discard step.)

```bash
git worktree remove "$WORKTREE_PATH"
git worktree prune  # Self-healing: clean up any stale registrations
```

Otherwise: the host environment (harness) owns this workspace. Don't remove it. If your platform provides a workspace-exit tool, use it. Otherwise, leave the workspace in place.

## Quick Reference

| Option | Merge | Push | Keep Worktree | Cleanup Branch |
|--------|-------|------|---------------|----------------|
| 1. Merge locally | yes | - | - | yes |
| 2. Create PR | - | yes | yes | - |
| 3. Keep as-is | - | - | yes | - |
| 4. Discard | - | - | - | yes (force) |

## Common Mistakes

**Skipping test verification**
- Problem: Merge broken code, create a failing PR
- Fix: Always verify tests before offering options

**Open-ended questions**
- Problem: "What should I do next?" is ambiguous
- Fix: Present exactly 4 structured options (or 3 for detached HEAD)

**Cleaning up worktree for Option 2**
- Problem: Remove a worktree the user needs for PR iteration
- Fix: Only clean up for Options 1 and 4

**Deleting branch before removing worktree**
- Problem: `git branch -d` fails because the worktree still references the branch
- Fix: Merge first, remove the worktree, then delete the branch

**Running git worktree remove from inside the worktree**
- Problem: Command fails silently when CWD is inside the worktree being removed
- Fix: Always `cd` to the main repo root before `git worktree remove`

**Cleaning up harness-owned worktrees**
- Problem: Removing a worktree the harness created causes phantom state
- Fix: Only clean up worktrees under `.worktrees/` or `worktrees/`

**No confirmation for discard**
- Problem: Accidentally delete work
- Fix: Require typed "discard" confirmation

## Guardrails

Avoid these:
- Proceeding with failing tests
- Merging without verifying tests on the result
- Deleting work without confirmation
- Force-pushing without an explicit request
- Removing a worktree before confirming merge success
- Cleaning up worktrees you didn't create (run the provenance check)
- Running `git worktree remove` from inside the worktree

Always do these:
- Verify tests before offering options
- Detect the environment before presenting the menu
- Present exactly 4 options (or 3 for detached HEAD)
- Get typed confirmation for Option 4
- Clean up the worktree for Options 1 and 4 only
- `cd` to the main repo root before worktree removal
- Run `git worktree prune` after removal

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
