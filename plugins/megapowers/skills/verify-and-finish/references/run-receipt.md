# Run Receipt

Use `scripts/run-receipt.go` beside this skill's `SKILL.md` to run a check
whose output is long or whose exit status matters. It replaces
`<check> | tail`, which hides the check's exit status and drops the rest of
the output.

```bash
go run <skill-dir>/scripts/run-receipt.go -- go test ./...
go run <skill-dir>/scripts/run-receipt.go --tail 80 -- npm test
```

Run the check itself, never a pipe around it. To filter output, search the
saved log afterwards.

The tool prints nothing while the check runs. Afterwards it prints the last
40 lines of combined output (`--tail N` changes this) and one final line:

```text
receipt: exit=1 duration=5120ms head=3f2a9c0d1b7e tree=dirty:8be1c47a90d2 log=<path> json=<path>
```

- `exit` is the check's real exit status. `127` means the command was not
  found, `126` that it could not run, and `125` that the tool itself failed.
- `head` is the first 12 characters of the commit. `tree` is `clean` or a
  digest of uncommitted changes, including untracked file contents. Both are
  taken before the check runs and are `none` outside a repository. Cite them
  with the result.
- `log` holds the full output. `json` holds the same facts in machine-readable
  form.

Through `go run`, the process exit is 1 for any failure. The receipt line's
`exit=` field carries the real code. If a caller needs the real process exit,
build once with `go build -o "$TMPDIR/run-receipt" <skill-dir>/scripts/run-receipt.go`.

`--shell` runs one quoted script through `sh -c` and reports the shell's exit
status. It is refused on Windows; pass the command as arguments instead.

Files go to `$TMPDIR/megapowers-receipts` unless you pass `--dir`. The tool
sets that directory to owner-only and refuses a symlink. Logs can contain
secrets the check printed, so keep them local and do not paste or upload them
whole.
