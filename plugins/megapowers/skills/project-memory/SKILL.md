---
name: project-memory
description: >-
  Use to remember durable, project-specific knowledge across sessions — a
  non-obvious decision and why, a constraint not visible in the code, a hard-won
  gotcha, a pointer to an external resource — in a repo-local markdown store any
  runtime can read. Triggers on "remember this for the project", "note this
  decision", "recall what we decided about ...", "why did we ...". Distinct from a
  single run's resumability (that's mega-orchestration:autonomous-run's journal).
---

# Project Memory

Cross-session memory is durable project knowledge that outlives any one run. It
lives in the repo as plain markdown, so it travels with the project and works on
every runtime — no host memory API required.

Store: `.megapowers/memory/`
- `INDEX.md` — one line per memory (title + a one-line hook); read it at the start
  of a session to know what's remembered.
- `<slug>.md` — one fact per file, with frontmatter (`name`, `title`, `hook`,
  `type`) and the fact as the body. Link related memories with `[[slug]]`.

## What earns a memory (the load-bearing discipline)

**Save** when it's durable and not otherwise recoverable:
- a non-obvious decision and its rationale ("we chose X over Y because ...");
- a project constraint not derivable from the code (an external contract, a quota,
  a "never touch prod on Fridays");
- a durable preference the user stated;
- a hard-won gotcha (a bug that cost hours and how to avoid it);
- a pointer to an external resource (a dashboard, a ticket, a doc).

**Don't save** what's already recorded elsewhere — code structure, git history,
what the README says, or anything true only for the current task. If asked to
remember something derivable, save what was *non-obvious* about it instead.

Rules: one fact per file; update the existing file rather than duplicating; delete a
memory that turns out to be wrong; convert relative dates to absolute.

## Helpers

```bash
scripts/mem-add <slug> --title T --hook H [--type decision|constraint|preference|gotcha|reference] [--update]
    # body read from stdin; writes .megapowers/memory/<slug>.md and regenerates the index.
    # refuses to overwrite an existing slug unless --update.
scripts/mem-index          # rebuild INDEX.md from the valid memory files (a file
                           # missing its closing '---' is skipped with a warning)
scripts/mem-recall <query> # print memories whose title/hook/slug match the query
```

## Recall

At the start of a session (or when a task touches remembered ground), read
`INDEX.md`, then pull a memory's full file only when its hook matches what you're
doing — `mem-recall <query>` does this executably. Treat a recalled memory as
background context that was true *when written*: if it names a file, flag, or
command, verify that still exists before acting on it.

## Scope

This is **project** memory: a plain-markdown store any runtime can read that
travels in the repo. Two native stores now overlap it on Claude Code: auto
memory (self-written under `~/.claude/projects/<project>/memory/`, machine-local
and never in the repo) and subagent memory (the `memory: project` frontmatter
field, committed to `.claude/agent-memory/<name>/`). The surviving differentiator
is exactly this store's shape: repo-committed and readable by every harness, not
Claude-only and not machine-local. Watch for double-bookkeeping: on Claude Code
all three fill in parallel, so keep one home per fact and cross-reference rather
than copy a fact into two stores that then drift.

By default it lives under `.megapowers/memory/`, which the repo git-ignores
(personal notes). To **share** memory with a team, don't just `git add -f` into an
ignored path — instead either point `MEGAPOWERS_MEMORY_DIR` at a committed location
(e.g. `docs/memory/`), or add a negation for the shared subpath to `.gitignore`
(e.g. `!.megapowers/memory/`) so it's tracked normally. One choice, made per
project.
