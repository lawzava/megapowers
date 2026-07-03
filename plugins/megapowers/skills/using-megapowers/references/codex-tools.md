# Codex Tool Mapping

Codex loads `AGENTS.md` automatically and supports skills, plugins, hooks, and
explicit subagent workflows. Do not require a `multi_agent` feature flag; current
Codex subagents are a native workflow surface.

## Subagents

Use Codex subagents only when the user explicitly asks for parallel agent work
or when a task is naturally independent across files, risks, or research tracks.
Keep prompts narrow and ask for summaries, file references, and verification
evidence instead of raw transcripts.

Good fits:

- independent code review passes such as security, test gaps, and maintainability
- read-only exploration of separate modules
- isolated implementation tasks in separate worktrees

Poor fits:

- tasks with a single critical path
- broad delegation without acceptance criteria
- work that needs one writer touching the same files

## Non-interactive Runs

Use `codex exec` from other tools or automation when Codex should handle a
bounded task and return one final answer. Prefer explicit sandbox flags:

```bash
codex exec --sandbox read-only "review this diff for auth and concurrency bugs"
codex exec --sandbox workspace-write "implement the scoped change and run tests"
```

Avoid deprecated compatibility flags such as `--full-auto` in new scripts.

## Environment Detection

Skills that create worktrees or finish branches should detect their environment
with read-only git commands before proceeding:

```bash
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
BRANCH=$(git branch --show-current)
```

- `GIT_DIR != GIT_COMMON`: already in a linked worktree; skip creation
- empty `BRANCH`: detached HEAD; cannot branch, push, or PR from sandbox

See `using-git-worktrees` Step 0 and `finishing-a-development-branch` Step 1 for
how each skill uses these signals.

## Record & Replay

Record & Replay can create Codex skills from demonstrated app workflows. Treat
those generated skills like any other public skill: review the instructions,
remove local secrets and machine-specific paths, and validate before sharing.
