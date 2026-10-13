# Correction Rounds

A later review round needs only the fixes made since the prior round. Sending
the whole diff again repeats the earlier review and adds minutes to each
round.

Before each review, record a snapshot of the working tree. `snapshot` prints a
Git tree id that includes untracked files, and it leaves the index unchanged:

```bash
reviewed=$(go run "$review_tool" snapshot)
```

After fixing the findings, write only the changes since that snapshot to a new
`.patch` file under `$TMPDIR`. Then inspect and review that file as usual:

```bash
go run "$review_tool" delta --since "$reviewed" --out "$TMPDIR/fixes-2.patch"
```

Name the prior findings and their fixes in `--intent`, so the reviewer checks
the fixes against those findings. `delta` refuses an existing output file and
reports when nothing changed since the snapshot; in that case, skip the
re-review. Take a new snapshot before each further round.

Run focused tests for each fix. Run the full acceptance suite once, after the
last round, unless a fix touches shared infrastructure that the focused tests
do not exercise.
