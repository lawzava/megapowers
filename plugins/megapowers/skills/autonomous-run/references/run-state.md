# Run state tool

`scripts/run-state.go` writes the deterministic parts of `.megapowers/run/<id>/`.
Judgment stays in the skill.

Run it from the worktree root or pass `--repo DIR`:

```bash
go run <skill-dir>/scripts/run-state.go init --id ID --objective TEXT
go run <skill-dir>/scripts/run-state.go checkpoint --id ID --milestone TEXT --next TEXT [--blocker TEXT ...] [--evidence TEXT ...]
go run <skill-dir>/scripts/run-state.go journal --id ID --event KIND --evidence TEXT [--decision TEXT --why TEXT]
go run <skill-dir>/scripts/run-state.go resume --id ID
```

- `init` creates the run directory and a `charter.md` skeleton with the
  objective filled in; complete the other sections by hand. It adds
  `.megapowers/` to `.git/info/exclude` only when Git does not already
  ignore it. It refuses an existing run.
- `checkpoint` records UTC time, workspace, branch, worktree, HEAD, dirty
  state, milestone, evidence, blockers, and next safe action, then rewrites
  `handoff.md`.
- `journal` appends one JSON line. Record oracle evidence, not intent. Add
  `--decision` with `--why` to record the reason behind a choice.
- `resume` compares the latest checkpoint with the live checkout and prints
  `match` or `mismatch: recorded X, live Y` per field, the journal tail, the
  next safe action, and blockers.

Exit codes: 0 success, 3 resume mismatch (stop affected work and reconcile),
125 tool or usage error. `go run` reports a non-zero exit as `exit status N`
and itself exits 1; read that line or the final `result:` line.

Text is folded to one line and capped at 2000 characters. Files are 0600.
The journal is never rewritten.

These commands replace hand-building the four state files, the exclude
entry, and the repository, branch, worktree, and HEAD part of resume
reconciliation. Each checkpoint overwrites `checkpoint.md`, so record artifact
identities, delegate ownership, and remaining effect authority with
`--evidence` or `journal`. Runtime and external state still need manual
reconciliation on resume.
