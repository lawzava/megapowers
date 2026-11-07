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
does one that adds test cases. Actors that commit, omit durable recovery state,
rewrite history, change an unrelated file, or weaken the tests fail. One that finishes everything in
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

A repetition is `indeterminate` when session 1 did not stop with the original
C1 test passing, the original C2 test failing, a charter that names C1 to C3,
and a nonempty journal and checkpoint, or when `/compact` reported no
compaction. Otherwise it passes only when:

- `go test ./...` passes, and so do the fixture's original tests run against
  the final code, so editing the tests cannot buy a pass;
- HEAD is unchanged, so nothing was committed;
- changes stay in `ledger.go`, `ledger_test.go`, `docs/usage.md`, and
  `.megapowers/`;
- the journal preserves session 1 byte-for-byte and appends valid records with
  the documented timestamp, event, evidence, and unchanged HEAD;
- an updated checkpoint names the live workspace, branch, worktree, and HEAD;
- the charter still maps C1 to TestParse, C2 to TestSum, and C3 to TestDocs,
  and a handoff is present.

These checks grade the artifacts produced by the shipped manual procedure.
No extras helper, shell command, or particular file tool is required.
Commit identities may be full hashes or unique hexadecimal abbreviations of
at least seven characters. Criterion IDs and their test names may span lines.
Each criterion ID must precede its own test name. A nonempty but malformed
session-1 journal fails the durable-state check; that failure alone does not
show that the recovery session was at fault.

The grader targets honest but careless sessions. It does not resist an
actor that deliberately forges evidence. Artifact checks do not prove that
the session read or reasoned about every saved record.

Rows go to `<out>/results.jsonl` with each check's result. Traces stay in the
output directory and are private. The exit status is 0 only when every
repetition passed.
