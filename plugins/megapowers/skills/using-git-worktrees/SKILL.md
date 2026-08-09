---
name: using-git-worktrees
description: Use when starting feature work or an implementation plan that should verify or establish an isolated workspace. Not for finishing or merging a branch.
license: MIT
---

# Using Git Worktrees

## Overview

Ensure work happens in an isolated workspace. Detect existing isolation first,
use an available workspace mechanism, and fall back to manual Git worktrees
only when necessary. Record who created a worktree before work begins.

## Detect before creating

Compare the resolved paths of `git rev-parse --git-dir` and `git rev-parse
--git-common-dir`. If they differ, you are in a linked worktree, unless `git
rev-parse --show-superproject-working-tree` returns a path, which means you are
in a submodule and should treat it as a normal repo. Never create a nested
worktree. When already isolated, report the path and branch state (a detached
HEAD is externally managed and needs a branch at finish time) and go straight
to setup.

In a normal checkout, honor any worktree preference the user has already
expressed, without asking. Otherwise a worktree is reversible and protects the
current branch, so set one up by default and say so, offering to work in place
instead; do not block on sign-off. Ask first only when isolation would be
surprising or costly here, such as a second checkout of a very large repo. If
the user has declined isolation, work in place.

## Native tools first

If the environment provides a worktree tool, use it. Preserve its creation
receipt or other explicit ownership record. Do not create a second worktree
behind that mechanism. Use the Git fallback only when no such mechanism exists.

## Git fallback

Directory priority: an explicit user instruction wins; next, an existing
`.worktrees/` beats an existing `worktrees/`; with no other guidance, default
to `.worktrees/` at the project root.

Before creating a project-local worktree, verify the directory is ignored with
`git check-ignore`. If it is not ignored, add it to .gitignore, then proceed.
The ignore takes effect immediately whether or not it is committed, so don't
commit as a side effect of this skill. The entry rides along with your next
commit under your own commit policy. This keeps worktree contents out of the
repository.

Create the worktree with `git worktree add <location>/<branch> -b <branch>` and
record provenance before working. Store it outside the worktree at
`<resolved-GIT_COMMON>/megapowers-worktree-ownership/<key>.record`, where
`<key>` is the output of `printf '%s' "$WORKTREE_PATH" | git hash-object
--stdin`. Feed the resolved worktree path with no trailing newline. This
directory is discoverable after changing directories and survives removal of
the worktree.

The record is UTF-8 key-value text with this schema:

```text
version=1
worktree_path=<resolved path>
common_git_dir=<resolved GIT_COMMON>
branch=<branch name>
creator=<creation command or environment tool>
cleanup_authority=process
```

Write only a record whose resolved path and common directory were just
observed, and set `cleanup_authority=process` only when this process is
authorized to remove it. An environment tool's receipt must be copied into the
same fields before cleanup is allowed. This record, not a pathname or branch
name, is the only cleanup authority.

If creation fails because the sandbox denies it, report the failure and request
an authorized isolated path or permission change. Do not silently continue in
the original checkout. Work in place only after the user explicitly declines
isolation.

## Setup and baseline

Install the project's dependencies and run its test suite so the workspace
starts from a known state. If the baseline fails, report the failures and ask
whether to proceed or investigate; a dirty baseline hides which failures the
new work introduced. When the baseline is clean, report the worktree path, the
test result, and what you are about to implement.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
