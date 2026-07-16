# Codex Tool Mapping

Codex loads `AGENTS.md` automatically and supports skills, plugins, hooks, and
explicit subagent workflows. Stable subagents require no `multi_agent` feature
flag. This repo's optional v2 baseline enables `multi_agent_v2`, removes the v1
`agents.max_threads` key, and uses v2's session-thread cap instead. Codex 0.144.4
does not hard-enforce `agents.max_depth` under v2, so the template uses
`multi_agent_mode_hint_text` for its depth-five policy.

## Subagents

Use Codex subagents only when the user explicitly asks for parallel agent work
or when a task is naturally independent across files, risks, or research tracks.
Keep prompts narrow and ask for summaries, file references, and verification
evidence instead of raw transcripts.

Treat v2 as same-model context sharding. Its spawn surface exposes a task name,
brief, and `fork_turns`, not a per-worker role, model, or effort selector. For
independent work, use `fork_turns = "none"` and put all required paths,
constraints, acceptance criteria, and expected output in the brief. Use a small
positive count only for essential recent turns, and `all` only for a genuine
same-context continuation. Named profiles do not automatically route v2 work;
use a separate role-aware surface or bounded `codex exec` run when a different
Codex model or effort is required.

The root owns spawning and final-target integration by default. Keep an ordinary
batch to six workers even though the configured ceiling allows ten, wait for
every gating worker, validate results yourself, and only then finish the task.
A completed worker is idle: measure whether idle threads still count against
the running session's cap, follow up only for the same assignment, create a
fresh worker for a new problem, and interrupt only a worker that is still
running.

Recursive coordinator mode must be selected explicitly. Every writing child
uses its assigned branch and linked worktree with a self-contained brief and
fresh `fork_turns = "none"` context. Each coordinator alone integrates and
validates its subtree, joins its descendants, and returns one final result to
its parent; the root alone integrates the final target. The depth-five policy
is model-visible rather than hard-enforced in Codex 0.144.4.

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

See the detect-before-creating rule in `using-git-worktrees` (comparing resolved git-dir and git-common-dir, with the submodule guard) and the test-verification requirement before `finishing-a-development-branch`'s option menu for how each skill uses these signals.

## Record & Replay

Record & Replay can create Codex skills from demonstrated app workflows. Treat
those generated skills like any other public skill: review the instructions,
remove local secrets and machine-specific paths, and validate before sharing.
