# Run state tool

`scripts/run-state.go` writes the deterministic parts of `.megapowers/run/<id>/`.
Judgment stays in the skill.

Run it from the worktree root or pass `--repo DIR`:

```bash
go run <skill-dir>/scripts/run-state.go init --id ID --objective TEXT
go run <skill-dir>/scripts/run-state.go checkpoint --id ID --milestone TEXT --next TEXT [--blocker TEXT ...] [--evidence TEXT ...]
go run <skill-dir>/scripts/run-state.go journal --id ID --event KIND --evidence TEXT [--decision TEXT --why TEXT [--risk low|medium|high --if-wrong TEXT]]
go run <skill-dir>/scripts/run-state.go decisions --id ID
go run <skill-dir>/scripts/run-state.go resume --id ID
go run <skill-dir>/scripts/run-state.go continue --id ID --item TEXT [--max N]
go run <skill-dir>/scripts/run-state.go coverage --id ID --brief FILE
```

- `init` creates the run directory and a `charter.md` skeleton with the
  objective filled in; complete the other sections by hand. It adds
  `.megapowers/` to `.git/info/exclude` only when Git does not already
  ignore it. It refuses an existing run.
- `checkpoint` records UTC time, workspace, branch, worktree, HEAD, dirty
  state, milestone, evidence, blockers, and next safe action, then rewrites
  `handoff.md`.
- `journal` appends one JSON line. Record oracle evidence, not intent. Add
  `--decision` with `--why` to record the reason behind a choice. For a
  choice made without the user, add `--risk low|medium|high` with
  `--if-wrong` naming what it costs to reverse.
- `decisions` lists every journaled decision, `high` risk first, then
  `medium`, `low`, and unrated, in journal order within a level. It prints
  `decisions: N` first.
- `resume` compares the latest checkpoint with the live checkout and prints
  `match` or `mismatch: recorded X, live Y` per field, the journal tail, the
  next safe action, and blockers.
- `continue` journals one automatic continuation on an item and prints
  `continuation N of M`. Once the item has used its cap (default 3), it
  journals one `blocked` record and exits 4 on every later call. Pass the
  same `--item` text each time; the count comes from the journal, so it
  survives compaction.
- `coverage` reads the `C1`, `C2`, ... IDs under `## Done criteria` in the
  charter and lists any the saved brief or handoff does not name. It refuses
  a charter whose criteria still say `(fill in`. A named ID does not show
  that the brief kept the criterion's meaning; it catches a criterion that
  dropped out entirely.

Exit codes: 0 success, 3 resume mismatch or coverage gap (stop affected work
and reconcile), 4 continuation cap reached (stop that item and report it as
blocked), 125 tool or usage error, including a symlinked `info/exclude`. Through
`go run`, any failure exits 1; the final `result:` line carries the real code,
for example `result: mismatch (exit 3)`.

Text is folded to one line and capped at 2000 characters. Files are 0600.
The journal is never rewritten.

These commands replace hand-building the four state files, the exclude
entry, and the repository, branch, worktree, and HEAD part of resume
reconciliation. Each checkpoint overwrites `checkpoint.md`, so record artifact
identities, delegate ownership, and remaining effect authority with
`--evidence` or `journal`. Runtime and external state still need manual
reconciliation on resume.
