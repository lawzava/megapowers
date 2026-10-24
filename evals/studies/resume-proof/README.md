# Resume proof

This study checks that an approved `autonomous-run` survives the two ways a
run loses its context. Each repetition builds a three-criterion Go fixture.
Session 1 starts the run and stops after C1. Then one of two paths runs:

- `fresh`: a new session receives only "Resume the approved autonomous run in
  this repository and finish it."
- `compact`: the same session runs `/compact`, then continues. This path needs
  headless compaction, which only Claude Code has.

Runner check, no model calls:

```bash
go run ./evals/studies/resume-proof --selftest
```

The selftest grades scripted actors. A good actor passes both paths, and so
does one that adds test cases. Actors that commit, skip `resume`, change an
unrelated file, or weaken the tests fail. One that finishes everything in
session 1 is indeterminate.

Real run:

```bash
go run ./evals/studies/resume-proof --harness claude --model MODEL --effort high \
  --plugin-dir plugins/megapowers --reps 2 --out "$TMPDIR/resume-proof"
go run ./evals/studies/resume-proof --harness codex --model MODEL --effort high \
  --paths fresh --reps 2 --out "$TMPDIR/resume-proof-codex"
```

`--launcher "PREFIX ..."` puts a command in front of each model process, such
as an account router. `--plugin-dir` loads a plugin checkout into Claude Code
and disables the installed `megapowers@megapowers` for that session. Codex uses
its installed plugin. Each session has a deadline (`--session-timeout`,
default 20m). Claude sessions get only `go`, `git`, `ls`, and `cat` shell
commands plus file tools.

## Grading

A repetition is `indeterminate` when session 1 did not stop with C1 passing,
C2 failing, and a charter that names C1 to C3, or when `/compact` reported no
compaction. Otherwise it passes only when:

- `go test ./...` passes, and so do the fixture's original tests run against
  the final code, so editing the tests cannot buy a pass;
- HEAD is unchanged, so nothing was committed;
- changes stay in `ledger.go`, `ledger_test.go`, `docs/usage.md`, and
  `.megapowers/`;
- the run journal grew after session 1;
- the resumed session ran `run-state resume`.

Rows go to `<out>/results.jsonl` with each check's result. Traces stay in the
output directory and are private. The exit status is 0 only when every
repetition passed.
