---
name: using-git-worktrees
description: Use when starting feature work that needs isolation from current workspace or before executing implementation plans - ensures an isolated workspace exists via native tools or git worktree fallback
license: MIT
---

# Using Git Worktrees

## Overview

Ensure work happens in an isolated workspace. Prefer your platform's native worktree tools. Fall back to manual git worktrees only when no native tool is available.

**Core principle:** detect existing isolation first, then use native tools, then fall back to git. Don't fight the harness.

## Step 0: Detect Existing Isolation

Before creating anything, check whether you are already in an isolated workspace.

```bash
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
BRANCH=$(git branch --show-current)
```

Submodule guard: `GIT_DIR != GIT_COMMON` is also true inside git submodules. Before concluding "already in a worktree," verify you are not in a submodule:

```bash
# If this returns a path, you're in a submodule, not a worktree — treat as normal repo
git rev-parse --show-superproject-working-tree 2>/dev/null
```

If `GIT_DIR != GIT_COMMON` (and not a submodule), you are already in a linked worktree. Skip to Step 2 (Project Setup). Don't create another worktree.

Report with branch state:
- On a branch: "Already in isolated workspace at `<path>` on branch `<name>`."
- Detached HEAD: "Already in isolated workspace at `<path>` (detached HEAD, externally managed). Branch creation needed at finish time."

If `GIT_DIR == GIT_COMMON` (or in a submodule), you are in a normal repo checkout.

Has the user already indicated a worktree preference (in your instructions, settings, or this conversation)? Honor it without asking. Otherwise, a worktree is reversible and protects the current branch, so set one up by default and say so rather than blocking on a sign-off:

> "Setting up an isolated worktree at `<path>` to protect your current branch — tell me if you'd rather work in place."

Only stop to ask first when isolation would be surprising or costly here (e.g. a very large repo where a second checkout is expensive). If the user has declined isolation, work in place and skip to Step 2.

## Step 1: Create Isolated Workspace

You have two mechanisms. Try them in this order.

### 1a. Native Worktree Tools (preferred)

You're setting up an isolated workspace (Step 0). Do you already have a way to create a worktree? It might be a tool with a name like `EnterWorktree`, `WorktreeCreate`, a `/worktree` command, or a `--worktree` flag. If you do, use it and skip to Step 2.

Native tools handle directory placement, branch creation, and cleanup automatically. Using `git worktree add` when you have a native tool creates phantom state your harness can't see or manage.

Only proceed to Step 1b if you have no native worktree tool available.

### 1b. Git Worktree Fallback

Use this only if Step 1a does not apply — you have no native worktree tool available. Create a worktree manually using git.

#### Directory Selection

Follow this priority order. Explicit user preference always beats observed filesystem state.

1. Check your instructions for a declared worktree directory preference. If the user has already specified one, use it without asking.

2. Check for an existing project-local worktree directory:
   ```bash
   ls -d .worktrees 2>/dev/null     # Preferred (hidden)
   ls -d worktrees 2>/dev/null      # Alternative
   ```
   If found, use it. If both exist, `.worktrees` wins.

3. If there is no other guidance available, default to `.worktrees/` at the project root.

#### Safety Verification (project-local directories only)

Verify the directory is ignored before creating the worktree:

```bash
git check-ignore -q .worktrees 2>/dev/null || git check-ignore -q worktrees 2>/dev/null
```

If it is not ignored, add it to .gitignore, then proceed. The ignore takes effect immediately whether or not it is committed, so don't commit as a side effect of this skill — the entry rides along with your next commit under your own commit policy. This prevents accidentally committing worktree contents to the repository.

#### Create the Worktree

```bash
# Determine path based on chosen location
path="$LOCATION/$BRANCH_NAME"

git worktree add "$path" -b "$BRANCH_NAME"
cd "$path"
```

Sandbox fallback: if `git worktree add` fails with a permission error (sandbox denial), tell the user the sandbox blocked worktree creation and you're working in the current directory instead. Then run setup and baseline tests in place.

## Step 2: Project Setup

Auto-detect and run appropriate setup:

```bash
# Node.js
if [ -f package.json ]; then npm install; fi

# Rust
if [ -f Cargo.toml ]; then cargo build; fi

# Python
if [ -f requirements.txt ]; then pip install -r requirements.txt; fi
if [ -f pyproject.toml ]; then poetry install; fi

# Go
if [ -f go.mod ]; then go mod download; fi
```

## Step 3: Verify Clean Baseline

Run tests to ensure the workspace starts clean:

```bash
# Use project-appropriate command
npm test / cargo test / pytest / go test ./...
```

If tests fail, report the failures and ask whether to proceed or investigate.

If tests pass, report ready.

### Report

```
Worktree ready at <full-path>
Tests passing (<N> tests, 0 failures)
Ready to implement <feature-name>
```

## Quick Reference

| Situation | Action |
|-----------|--------|
| Already in linked worktree | Skip creation (Step 0) |
| In a submodule | Treat as normal repo (Step 0 guard) |
| Native worktree tool available | Use it (Step 1a) |
| No native tool | Git worktree fallback (Step 1b) |
| `.worktrees/` exists | Use it (verify ignored) |
| `worktrees/` exists | Use it (verify ignored) |
| Both exist | Use `.worktrees/` |
| Neither exists | Check instruction file, then default `.worktrees/` |
| Directory not ignored | Add to .gitignore (no commit — it applies immediately) |
| Permission error on create | Sandbox fallback, work in place |
| Tests fail during baseline | Report failures + ask |
| No package.json/Cargo.toml | Skip dependency install |

## Common Mistakes

### Fighting the Harness

- Problem: using `git worktree add` when the platform already provides isolation.
- Fix: Step 0 detects existing isolation. Step 1a defers to native tools.

### Skipping Detection

- Problem: creating a nested worktree inside an existing one.
- Fix: always run Step 0 before creating anything.

### Skipping Ignore Verification

- Problem: worktree contents get tracked and pollute git status.
- Fix: always use `git check-ignore` before creating a project-local worktree.

### Assuming Directory Location

- Problem: creates inconsistency, violates project conventions.
- Fix: follow the priority — explicit instructions > existing project-local directory > default.

### Proceeding with Failing Tests

- Problem: can't distinguish new bugs from pre-existing issues.
- Fix: report failures and get explicit permission to proceed.

## Key Rules

The single most common mistake is reaching for `git worktree add` when a native worktree tool (such as `EnterWorktree`) is available — if you have the tool, use it. Beyond that:

- Run Step 0 detection first, and don't create a worktree when Step 0 already detects isolation.
- Prefer native tools over the git fallback; don't jump straight to Step 1b's git commands.
- Follow the directory priority: explicit instructions > existing project-local directory > default.
- Verify the directory is ignored before creating a project-local worktree.
- Auto-detect and run project setup.
- Verify a clean test baseline before proceeding.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
