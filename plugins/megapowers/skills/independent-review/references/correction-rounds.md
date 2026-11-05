# Correction Rounds

A later review round needs only the fixes made since the prior round. Sending
the whole diff again repeats the earlier review and adds minutes to each
round.

Use a fresh agent for each correction round with the original brief plus every
later directive. Keep the correction-round cap set before the first dispatch;
new agents do not reset it. A spent budget leaves unresolved findings open for
the user, never approved.

Every round returns `verdict`, `scope.creep`, `scope.shrink`, and findings with
severity (`blocker`, `major`, `nit`), file, line, claim, and evidence. Creep is
unrequested work, including drive-by refactors and new dependencies. Shrink is
stubs, TODOs, weakened/skipped/deleted tests, loosened assertions, or unconfirmed
acceptance items. The lead computes fail when any blocker, major, scope.creep or
scope.shrink exists, regardless of the stated verdict. Nits never fail.

For a native reviewer, identify the previously reviewed artifact, provide only
the fix delta plus prior finding IDs and relevant acceptance checks, and use
the harness's native read-only reviewer. A version-control diff or the
snapshot/delta commands below can capture that delta. Without Go or a shell,
use the harness's own diff and immutable artifact identities; if no reliable
delta exists, report that limitation rather than claiming re-review passed.

For an external reviewer, both commands below need Go and a shell. Without them,
send no package through that route; see [external review](external-review.md).

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
