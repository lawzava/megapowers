---
name: project-memory
description: >-
  Use to remember durable, project-specific knowledge across sessions — a
  non-obvious decision and why, a constraint not visible in the code, a hard-won
  gotcha, a pointer to an external resource — in a repo-local markdown store any
  runtime can read. Triggers on "remember this for the project", "note this
  decision", "recall what we decided about ...", "why did we ...". Distinct from a
  single run's resumability (that's mega-orchestration:autonomous-run's journal).
license: MIT
---

# Project Memory

Project memory is durable knowledge that outlives any single run. It lives in
the repo as plain markdown, so it travels with the project and any runtime can
read it.

Store layout: `.megapowers/memory/` holds `INDEX.md`, one line per memory, plus
one fact per file at `<slug>.md` with frontmatter fields `name`, `title`,
`hook`, and `type` (one of `decision`, `constraint`, `preference`, `gotcha`,
`reference`) and the fact as the body. Link related memories with `[[slug]]`.

## What earns a memory

Save what is durable and not otherwise recoverable: a non-obvious decision and
its rationale, a constraint invisible in the code, a stated user preference, a
hard-won gotcha, a pointer to an external resource. Don't save what code, git
history, or the README already records, or anything true only for the current
task; if asked to remember something derivable, save the non-obvious part
instead.

Hygiene: one fact per file; update the existing file rather than duplicate;
delete a memory that turns out to be wrong; convert relative dates to absolute.

## Helpers

```bash
scripts/mem-add <slug> --title T --hook H [--type decision|constraint|preference|gotcha|reference] [--update]
    # body read from stdin; writes .megapowers/memory/<slug>.md and regenerates
    # the index. refuses to overwrite an existing slug unless --update.
scripts/mem-index          # rebuild INDEX.md (a file missing its closing '---'
                           # is skipped with a warning)
scripts/mem-recall <query> # print memories matching the query
```

## Recall

Read `INDEX.md` at the start of a session and pull a memory's full file only
when its hook matches the work at hand; `mem-recall <query>` does this
executably. A recalled memory was true when written: if it names a file, flag,
or command, verify that still exists before acting on it.

## Scope

This is project memory, not a Claude-native store. Claude Code's auto memory
and subagent memory overlap it, but this store's differentiator is its shape:
repo-committed markdown every harness can read, not Claude-only and not
machine-local. Keep one home per fact and cross-reference rather than copy a
fact into stores that then drift.

The default location `.megapowers/memory/` is gitignored (personal notes). To
share memory with a team, point `MEGAPOWERS_MEMORY_DIR` at a committed location
(e.g. `docs/memory/`) or add a gitignore negation for the shared subpath; never
`git add -f` into an ignored path.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
