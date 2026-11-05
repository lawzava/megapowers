# Writer fences and lead-leak checks

Run the installed `scripts/fence-check/fence.go` from the orchestrating skill.
It works with any Git worker checkout: native isolation, `lanes.go`, or another
worktree. It does not require a lane registry or a particular temporary root.

Fences are repo-relative, case-sensitive paths using `/`:

- `src/parser.go`: exactly that file.
- `src/parser/`: every descendant, not `src/parser-other/`.
- `src/*_test.go`: `*` matches any characters, including `/` (also nested tests).

All other characters are literal. Patterns are a union; quote them to prevent
shell expansion. Empty, absolute, backslash, `.` and `..` path components are
invalid. No exclusion syntax is implied: state prohibited work separately in
the brief's out-of-scope line.

```bash
go run <skill-dir>/scripts/fence-check/fence.go check \
  --repo <lead-checkout> --base <dispatch-revision> --head <worker-checkout> \
  --fence 'src/parser/' --fence 'tests/parser*'
```

Record the immutable dispatch commit ID and pass it as `--base`. Symbolic refs
are resolved in `--repo` at check time, not frozen at dispatch or resolved against
the worker's HEAD; the object must exist in the worker repository. `--head` can
instead name a revision in `--repo`.
Use an absolute checkout path or immutable revision ID when a name is both a
directory and a revision; ambiguous names are rejected. A revision compares only its
tree to the base. A checkout includes the union of committed, staged, unstaged,
and non-ignored untracked changes against the base; both rename endpoints count.
Every violating path prints as a quoted string, so unusual names remain
unambiguous. A violation is an automatic kickback before review. Do not silently
widen the fence after seeing the result; reconcile a genuinely needed scope
change with the lead and the acceptance boundary first.

Capture the lead before dispatch, then check it before integrating worker output:

```bash
go run <skill-dir>/scripts/fence-check/fence.go snapshot \
  --repo <lead-checkout> --out "$TMPDIR/lead-before.json"
go run <skill-dir>/scripts/fence-check/fence.go leak \
  --repo <lead-checkout> --since "$TMPDIR/lead-before.json"
```

The new private baseline records HEAD, index and worktree tree identities and
the checkout's canonical path. Existing dirty and untracked work is part of the
baseline, not a worker leak. A later change to any of these states is lead
drift, including an index-only edit. This cannot attribute authorship: account
for the lead's own concurrent edits before calling drift a worker leak. Never
discard user-owned edits to clear the check. Baselines cannot be overwritten or
written inside the checkout. They reference local Git objects; retain them only
for the lane's lifetime and do not run Git garbage collection during that time.

Both checks leave the real index, branch and checkout untouched. They write
snapshot objects into local Git storage and can run configured clean filters;
use trusted repositories. Ignored untracked files, empty directories and Git
metadata are outside the check. Sparse checkouts, unmerged indexes and
submodules fail closed as unsupported; check submodule repositories separately.
These are pre-review checks, not an OS sandbox or a continuous monitor, and
cannot detect an edit restored before the check.

Exit codes: 0 pass, 3 violations/drift, 125 usage/tool error. Any nonzero result
blocks integration until reconciled. `go run` maps failures to 1 and prints
the tool's real code in its final `result:` line. Without Git, Go or a shell,
inspect changed paths and lead state manually, report that the automated check
did not run, and keep writing sequential if isolation is unavailable.
