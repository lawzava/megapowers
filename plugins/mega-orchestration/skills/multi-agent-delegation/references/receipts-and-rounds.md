# Receipts and Rounds

What `scripts/delegate-run` returns, what a receipt does and does not prove, and
how many times one role may be dispatched on one branch before the decision goes
back to the human.

- [What delegate-run does](#what-delegate-run-does)
- [Exit map](#exit-map)
- [The round ledger](#the-round-ledger)
- [The round cap](#the-round-cap)
- [Output and transcripts](#output-and-transcripts)
- [Schemas and versions](#schemas-and-versions)
- [The review package budget](#the-review-package-budget)
- [What subject.id binds](#what-subjectid-binds)
- [What can fool a fingerprint](#what-can-fool-a-fingerprint)
- [Two programs, one format](#two-programs-one-format)

## What delegate-run does

For read-only independent review, prefer
`scripts/delegate-run --role ROLE --author-vendor VENDOR --artifact
worktree|FILE --claim TEXT`. It resolves and executes the safe provider adapter,
requires the verdict schema, computes the complete worktree or file identity,
and atomically writes a provenance receipt. The receipt is evidence only for
that exact subject identity; any tracked, staged, unstaged, or untracked change
invalidates it.

## Exit map

The exit map is the contract to branch on: 0 approved, 2 a usage or setup error,
3 no route resolved, 4 a single-route role whose only provider is disabled in
config, 5 a valid needs-attention verdict, 6 a provider failure, 7 invalid
provider output, 8 refused to dispatch because the review package holds no
substantive content, 9 the review package could not be captured, 10 the round
cap was reached, and 11 the provider outran the wall-clock budget. 3 and 4 are
the resolver's own codes, propagated unchanged, so an unroutable role reads the
same from either program. The launcher never exits with a raw git status: a tree
git cannot read arrives as 9, naming the path, not as 128.

Round accounting cuts across that map, and the question it answers is whether a
model was asked. The round is reserved immediately before the provider is
invoked and never earlier, so everything that fails on the way there costs
nothing: 2, 8, 9, 10, and every provider preflight failure all happen before a
round is reserved, dispatch nothing, and leave the count where it was. 5, 7, 11,
and a provider that ran and then failed all happen after the dispatch was made
and paid for, so the round counts and is closed as failed. An interrupt or
terminate (130, 143) closes a reserved round the same way, because the provider
was already running when the signal arrived.

6 is the one code on both sides of that line, so read its message rather than
the number alone. Every preflight refusal ends with "nothing was dispatched and
no round was consumed"; a provider that ran and failed prints the provider's own
stderr instead. The preflight covers a reviewer CLI missing the isolation or
schema flags, a Claude route with neither `ANTHROPIC_API_KEY` nor a credentials
file, a Codex route whose stored authentication `codex login status` cannot read,
and a resolved vendor this launcher has no adapter for. Each of those used to be
charged a round, so three setup errors exhausted a cap of three with no review
performed.

Both credential checks are local reads that ask no model, which is what lets them
sit ahead of the reservation: `login status` prints whose credential is stored and
separates "not logged in" from "present but unreadable", the sandbox case that
used to reach the provider and burn a round. A CLI too old to offer that
subcommand is gated on a readable `~/.codex/auth.json` or `OPENAI_API_KEY`
instead, because authentication that cannot be established is not authentication
that works.

10 and 11 are still the pair to branch on. An 11 you retry spends the next round
against `max_rounds` and can walk a loop into a 10, while a 10 cannot be retried
at all until an approve clears the ledger or the cap is raised.

11 is a wall clock, not a verdict: `MEGAPOWERS_DELEGATE_TIMEOUT` bounds the
provider dispatch at 540 seconds by default, chosen to sit under the 600 second
cap a foreground Bash tool call gets so the verdict comes back instead of being
backgrounded. Raise it for a genuinely slow reviewer rather than reading the
absence of a verdict as an absence of findings. The bound is not optional, which
is what lets this page state it: 0 is refused rather than read as off, and a
host with neither `timeout(1)` nor `gtimeout(1)` fails setup with 2 instead of
making an unbounded call for the caller's own foreground cap to kill, with no
verdict and a paid round.

Exit 9 means a section of the pending tree stayed unreadable even after paths
git cannot hash were excluded, so there is no package to review and no id to
bind one to. Like 8 it happens before a round is reserved and before any
provider is reached, so it consumes nothing and produces no receipt. Unlike 8 it
is not about the tree being empty: fix or remove the path the message names,
then retry. A tracked path that is a character device, fifo, or socket is
handled rather than fatal, which is the ordinary sandbox case, so a 9 usually
means an unreadable regular file.

Exit 8 is not a provider problem and not worth retrying as one: with
`--artifact worktree` it almost always means the work is already committed, so
nothing is pending to review. Treating it as a 6 sends you debugging a healthy
provider.

## The round ledger

`receipt.round` counts how many consecutive dispatches of this role on this
branch have not reached approve. It counts dispatches, not receipts, so a round
that burned reviewer time and then failed still counts, and it is what a caller
caps a review loop on. An approve resets it; a needs-attention verdict and a
failed dispatch do not. Deliberately keyed on the branch and not on the artifact
fingerprint: the author fixes something between rounds, so a fingerprint-keyed
count would report 1 forever and never reveal the loop. A detached HEAD keys on
the checked-out commit, and outside a repository the count is scoped to the
receipt's directory. `subject.id` remains the fingerprint, which is a different
job: it binds the receipt to the exact tree reviewed and any change at all
invalidates it.

## The round cap

Counting was never the missing piece. `max_rounds` under
`[rules.risky-logic-review]` in `plugins/mega-orchestration/enforcement.toml` is
the number the ledger is compared against, shipped at 3, layered like
models.toml: a project `.megapowers/enforcement.toml` or user
`~/.config/megapowers/enforcement.toml` wins per key. The 2026-08-05 audit found
a real feature branch that reached round 11 and a repository branch that reached
round 22, because nothing capped them.

COMMIT the project layer. It counts as it stands in the base commit and never as
it stands in the worktree, exactly as the Stop hook reads it: the pending tree is
what is under review, so a cap taken from there is a cap written by the change
whose reviews it governs, and an uncommitted edit raising the number buys
unlimited reviewer shopping. A pending add, edit, or delete is ignored and said
out loud on stderr, naming the value it would have set, because raising a cap is
never something that should happen quietly. The user layer stays worktree
readable: it sits outside the repository and reaches no diff.

At the cap the launcher exits 10 without dispatching, so the round is not
consumed and no provider is billed. A reviewer that has not converged in three
passes is not converging: the next pass costs another full review and returns
another needs_attention. The decision is the human's at that point, and what
they need from you is the last verdict's unresolved findings, what changed
between rounds, and a recommendation: ship it as is, drop the change, or
restructure it so the disputed part is smaller. `--transcript-dir` is what makes
"what changed between rounds" answerable after the fact, so pass it on any loop
you expect to run more than once. An approve resets the ledger, so a
restructured change starts at round 1 rather than inheriting the deadlock, and a
repository that genuinely needs a longer loop raises the cap in its own
`.megapowers/enforcement.toml` rather than dispatching past it.

## Output and transcripts

Stdout is the receipt JSON and nothing else, so it pipes straight into jq. The
`=== VERDICT ===` block goes to stderr, carrying the verdict, the round with the
branch it counts against, the receipt path, and the transcript path when one was
kept. Read that block rather than re-deriving it from the receipt.

`--transcript-dir DIR` keeps this dispatch's prompt, review package, and raw
provider output under `DIR/<role>-<subject>-round<N>`, one directory per
dispatch, including a dispatch that fails and writes no receipt. Since an
approve resets the round, that name can repeat; a repeat gets a numeric suffix
rather than overwriting the earlier transcript. Omitted, everything is
discarded, which is the default.

## Schemas and versions

The launcher validates `schemas/review-verdict-v1.json` and emits
`schemas/review-receipt-v2.json`. Its executable regression contract is
`scripts/tests/delegate-run.test.sh`.

Receipts are v2. A `megapowers.review-receipt.v1` receipt records no base, so it
cannot say which tree state its reviewer read and there is no way to recover
that after the fact; the Stop hook rejects it and says so in the block reason.
`schemas/review-receipt-v1.json` is kept only to describe receipts already on
disk.

## The review package budget

`MEGAPOWERS_REVIEW_PACKAGE_BYTES` caps the package handed to one reviewer, at
200000 bytes by default. The largest package the audit measured was 674,630
bytes across 11,183 lines in one context, and attention degrades with the token
count, so a reviewer given that much reads the beginning and approves the rest.

The cap covers every emitted byte: git status, the one-line-per-file manifest,
each included diff with its heading, the omitted-file manifest, excluded paths,
and submodule content. Over budget, the largest files are dropped from the
inline text (smallest first buys the most complete files) and named instead. A
change whose mandatory sections alone exceed the cap is refused with 2, before
any dispatch, naming the number that would fit: a cap that is silently exceeded
is not a cap, and the sections it refuses over cannot be dropped without hiding
what the reviewer was not shown.

An omitted file is served from the snapshot, never from the working tree. Its
entry carries a `snapshot:` path inside the immutable capture holding its
complete diff, and the package and the prompt both say to read it there. This is
not a convenience. The receipt binds the capture; the working tree can move
after it, and a reviewer sent to the tree could read bytes the receipt does not
name and approve them. What the reviewer is pointed at is what an approve
covers, so every path it is given is inside the capture.

The binding itself is untouched by any of this. `subject.id` and
`subject.submodules` are computed from the complete capture, so the receipt
covers the whole pending tree whether or not every file was inlined, and an
approve that skipped a named omission is an approve of code nobody read.

## What subject.id binds

`subject.id` fingerprints the pending tree so a verdict binds to the exact state
that was reviewed. It covers the pending delta: staged, unstaged, and untracked
content, plus the identity of any path git cannot hash. A delta on its own is a
shape rather than a tree, and unrelated repositories carrying the same pending
hunks fingerprint identically.

`subject.base` is the other half. It records the commit the delta was measured
from (the empty tree in a repository with no commits), and base plus delta
determines the complete tracked content. That is what makes a receipt say "this
change, on this tree" instead of "this diff, somewhere". The Stop hook checks
both, so an unrelated checkout cannot inherit a receipt by carrying the same
pending change, and neither can a different repository placed at the reviewed
path.

`subject.submodules` covers what neither of the other two can see. A gitlink
diffs as a bare `Subproject commit <sha>` line, its worktree adds at most a
`-dirty` suffix, and under the default configuration an untracked file inside a
submodule reaches the superproject diff nowhere at all. So the review package
carries a submodule section, recursively: the staged pointer move, the worktree
content via `--submodule=diff`, and the submodule's own untracked files. That
section is what `subject.submodules` fingerprints, and the Stop hook recomputes
it with `scripts/review-diff-id --submodules`. It sits beside `subject.id` and
never inside it, because a receipt is compared against the id the shipped
algorithm produces and folding submodule content in would move the id of every
repository that has one. A tree with no gitlink carries no such field, and
absent and empty mean the same thing to the hook.

## What can fool a fingerprint

Every git read on this path runs with replacement objects disabled. `git replace
X Y` makes object reads return Y where X was asked for without moving any ref,
so `git rev-parse HEAD` keeps printing X while `git diff HEAD` renders against
Y: a repository holding X plus a replacement reproduces an approved delta on a
base nobody reviewed, and `subject.base` cannot tell. Disabling replacement
fixes what is read rather than detecting afterwards that the reviewer was shown
a tree that does not exist. An ordinary repository has no `refs/replace`, so no
id moves.

A tree whose index HIDES a modification is refused rather than reviewed.
`git update-index --assume-unchanged P` stops git stat'ing P, so an edit to it
lands in no diff, no status, and no fingerprint. The launcher exits 9 naming the
path and the Stop hook blocks with a reason no receipt can clear. The
neighbouring skip-worktree bit is not refused with it, because sparse checkout
sets it on every path outside the cone; content decides, so a bit set over a
path that still matches the index costs nothing and a materialized changed one
is caught either way.

Everything the reviewer is shown is the content the id names. Untracked files
are shown as their clean-filtered blob rather than their raw worktree bytes,
because `git hash-object` runs the filter chosen by the path and the id binds
what comes out of it. Under a `text`, `eol`, or `filter.*.clean` attribute the
two differ, and showing one while binding the other means approving bytes the
receipt does not cover.

## Two programs, one format

`scripts/review-diff-id` fingerprints a live worktree, which is what the Stop
hook uses to test a receipt against the tree in front of it; its executable
regression contract is `scripts/tests/review-diff-id.test.sh`. The launcher does
not call it. It derives the id from the same immutable snapshot it builds the
review package out of, because reading the tree once for the id and again for
the package lets the two name different trees: change the tree between the reads
and restore it after, and the receipt names a tree nobody reviewed while the
hook still accepts it. The consequence is that the format is duplicated and the
two must stay byte identical, which `delegate-run.test.sh` pins by asserting the
launcher's id equals `review-diff-id`'s across the tree shapes the pair is
expected to survive.
