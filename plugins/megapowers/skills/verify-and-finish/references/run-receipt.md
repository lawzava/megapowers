# Run Receipt

Use `scripts/run-receipt.go` beside this skill's `SKILL.md` to run a check
whose output is long or whose exit status matters. It replaces
`<check> | tail`, which hides the check's exit status and drops the rest of
the output.

```bash
go run <skill-dir>/scripts/run-receipt.go -- go test ./...
go run <skill-dir>/scripts/run-receipt.go --tail 80 -- make lint
go run <skill-dir>/scripts/run-receipt.go --shell -- 'npm test 2>&1 | grep -v warn'
```

The tool prints nothing while the check runs. Afterwards it prints the last
40 lines of combined output (`--tail N` changes this) and one final line:

```text
receipt: exit=1 duration=5120ms head=3f2a9c0d1b7e tree=dirty:8be1c47a90d2 log=<path> json=<path>
```

- `exit` is the check's real exit status. `127` means the command was not
  found; `125` means the tool itself failed, for example on a bad flag.
- `head` is the first 12 characters of the commit, and `tree` is `clean` or a
  digest of uncommitted changes, both taken before the check ran. Outside a
  repository both are `none`. Cite them with the result so the result can be
  matched to the code it checked.
- `log` holds the full output. `json` holds the same facts in machine-readable
  form.

`go run` reports any failure as exit status 1, so read `exit=` from the
receipt line. To get the real exit status from the process, build the tool
once with `go build -o "$TMPDIR/run-receipt" <skill-dir>/scripts/run-receipt.go`.

With `--shell`, the exit status is the shell's. A pipeline's status is its
last command's unless the script sets `set -o pipefail` and the shell supports it.

Files go to `$TMPDIR/megapowers-receipts` unless you pass `--dir`. Each file is
readable only by its owner. Logs can contain secrets the check printed, so
keep them local and do not paste or upload them whole.
