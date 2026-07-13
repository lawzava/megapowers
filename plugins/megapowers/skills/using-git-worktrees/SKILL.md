---
name: using-git-worktrees
description: Use when starting feature work or an implementation plan that should verify or establish an isolated workspace. Reuse an existing worktree when suitable.
license: MIT
---

# Using Git Worktrees

## Overview

Ensure work happens in an isolated workspace. Detect existing isolation first, prefer the harness's native worktree tools, and fall back to manual git worktrees only when no native tool exists. Don't fight the harness.

## Detect before creating

Compare the resolved paths of `git rev-parse --git-dir` and `git rev-parse --git-common-dir`. If they differ, you are in a linked worktree, unless `git rev-parse --show-superproject-working-tree` returns a path, which means you are in a submodule and should treat it as a normal repo. Never create a nested worktree. When already isolated, report the path and branch state (a detached HEAD is externally managed and needs a branch at finish time) and go straight to setup.

In a normal checkout, honor any worktree preference the user has already expressed, without asking. Otherwise a worktree is reversible and protects the current branch, so set one up by default and say so, offering to work in place instead; do not block on sign-off. Ask first only when isolation would be surprising or costly here, such as a second checkout of a very large repo. If the user has declined isolation, work in place.

## Native tools first

If the harness provides a worktree tool (a tool named like `EnterWorktree` or `WorktreeCreate`, a `/worktree` command, a `--worktree` flag), use it. It owns directory placement, branch creation, and cleanup; running `git worktree add` behind its back creates phantom state the harness cannot see or manage. Use the git fallback only when no native worktree tool exists.

## Git fallback

Directory priority: an explicit user instruction wins; next, an existing `.worktrees/` beats an existing `worktrees/`; with no other guidance, default to `.worktrees/` at the project root.

Before creating a project-local worktree, verify the directory is ignored with `git check-ignore`. If it is not ignored, add it to .gitignore, then proceed. The ignore takes effect immediately whether or not it is committed, so don't commit as a side effect of this skill — the entry rides along with your next commit under your own commit policy. This keeps worktree contents out of the repository.

Create the worktree with `git worktree add <location>/<branch> -b <branch>` and work there. If creation fails with a sandbox permission error, tell the user the sandbox blocked it and work in place instead.

## Setup and baseline

Install the project's dependencies and run its test suite so the workspace starts from a known state. If the baseline fails, report the failures and ask whether to proceed or investigate; a dirty baseline hides which failures the new work introduced. When the baseline is clean, report the worktree path, the test result, and what you are about to implement.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
