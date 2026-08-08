---
name: project-memory
description: 'Use to save or recall durable project knowledge across sessions: decisions, hidden constraints, gotchas, or references. Triggers on "remember this", "note this decision", or "what did we decide". Not for run progress.'
license: MIT
---

# Project Memory

Project memory records durable, non-obvious knowledge that should survive a
session. The default store is `.megapowers/memory/`: `INDEX.md` plus one
markdown file per fact. Each file has `name`, `title`, `hook`, and `type`
(`decision`, `constraint`, `preference`, `gotcha`, or `reference`) frontmatter,
then the fact as its body.

## Save

Save a decision and its rationale, a constraint not visible in code, a stated
preference, a hard-won gotcha, or a useful external reference. Don't save facts
already recorded in code, history, project documentation, or accepted design
records, and don't save temporary task progress. Update or delete an existing
memory instead of creating contradictory duplicates. Use absolute dates when a
date matters.

The included helpers read the body from standard input, create, index, and
search the default memory store. Resolve `scripts/` from this skill's installed
directory before invoking them:

```bash
scripts/mem-add <slug> --title T --hook H [--type decision|constraint|preference|gotcha|reference] [--update]
scripts/mem-index
scripts/mem-recall <query>
```

## Recall

Start with the index, then read only records whose hooks match the work. A
memory is historical evidence, not current truth. Before acting on any
referenced file, flag, command, or external detail, verify it still exists.
Surface contradictions with current sources or observed behavior for resolution;
do not silently choose between them.

Keep personal and shared memory stores distinct so each fact has one source of
truth. Set `MEGAPOWERS_MEMORY_DIR` to a committed shared location when needed;
do not force ignored personal notes into version control.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
