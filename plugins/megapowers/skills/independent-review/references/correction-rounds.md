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

Identify the previously reviewed artifact, provide only
the fix delta plus prior finding IDs and relevant acceptance checks, and use
the selected read-only reviewer. A version-control diff can capture that
delta. Without a shell,
use the harness's own diff and immutable artifact identities; if no reliable
delta exists, report that limitation rather than claiming re-review passed.

Record the reviewed revision or preserve a copy of the reviewed artifact under
`$TMPDIR`, including relevant uncommitted or untracked content. Name prior
findings and fixes in the correction brief. Check the brief and delta for
secrets and disclosure scope before sending off-machine. If nothing changed,
skip re-review. Preserve the new artifact identity for any further round.

Run focused tests for each fix. Run the full acceptance suite once, after the
last round, unless a fix touches shared infrastructure that the focused tests
do not exercise.
