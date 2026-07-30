---
name: multi-agent-delegation
description: >-
  Use when a scoped build, plan or code review, or visual or browser task should
  go to a different model or runtime rather than same-model subagents.
license: MIT
---

# Multi-Agent Delegation

Unsure whether delegation is the right structure at all? Start at
mega-orchestration:orchestrating, the decision root; this skill executes the
delegation route it picks.

## The Idea

The lead keeps the broad context, plans and decomposes the work, does cheap
bulk reads, and owns final integration and commits. Narrow, specialized work
goes to whichever model is best suited for it.

Routing lives in two layered files:

- `models.toml`, the model catalog: who leads (`[lead]`), the vendor-neutral
  tier scale and per-tier purposes (`[tiers]`, `[tiers.use]`), the providers
  with their tier maps, capabilities, and channel data, and the ship floor
  (`[defaults]`).
- `delegates.toml`, the routing: which provider handles which role (`[roles]`,
  `[requires]`, `[fallbacks]`), the required tier and effort (`[role_tiers]`,
  `[role_efforts]`), author-vendor independence (`[independence]`), evidence
  drivers (`[drivers]`, `[role_drivers]`), and how each run preset behaves
  (`[presets]`).

Both resolve the same way: a project `.megapowers/<file>` or user
`~/.config/megapowers/<file>` layer overrides the shipped copy per key, so a new
model release is one tier-map line in a file that survives plugin updates.
`scripts/delegate-resolve --where` shows the active layers. Put provider data in
a models.toml layer; the always-loaded session block renders from catalog layers
only. Edit an override layer to change routing: the skill, the delegate agents,
and the session-start catalog block read the config live.

Each provider's `reference` key names that provider's channel mechanics and
prompting guidance: references/providers/codex.md,
references/providers/claude.md, and references/providers/moonshot.md. Browser
automation is a driver, not a model
provider; its mechanics live in references/providers/browser.md. Read the
resolved provider and driver references before dispatching.

The nine roles: plan_review, code_review, small_impl, visual, browser_test,
visual_verify, verify, judge, council_member.

The floor is `[defaults] floor` in the catalog, written as tier:effort on the
`[tiers]` and `[efforts]` scales (shipped: `"strong:low"`). Nothing that ships
routes below it. A provider whose tier or declared default effort sits below
the corresponding floor is skipped at resolution; providers without an effort
setting are compared by tier only.

## Resolving a Route

`scripts/delegate-resolve <role>` resolves the config executably (`--preset
<name>` for presets, `--author-vendor <vendor>` once per artifact-author vendor
for independent roles, `--exclude <vendor|provider>` to drop a backend,
`--exclude-lead` as a compatibility exclusion, `--models <file>` to pin
the catalog, `--lead` to print the declared orchestrator, `--where` to print
the active config layers, `--check` to validate the table, `--list` and
`--list-presets` to enumerate, `--vendors` to print reachable vendors). It
walks the role's fallback chain, skipping any provider that is excluded, disabled,
missing a required capability, below the configured floor, or whose CLI is not
installed, so a route never resolves to a runtime you do not have, and prints
ROLE/PROVIDER/MODEL/TIER/EFFORT/CHANNEL/ENABLED/VENDOR/BINARY/FLOOR/NOTES,
plus DRIVER fields when the role requires an evidence driver.

Exit codes are a stable contract a harness can branch on: 0 resolved, act on
the printed route; 2 usage or config error, including a malformed config, with
the message naming the offending line so a broken table is never mistaken for
an unknown role; 3 unknown role or no available route; 4 a single-route role
whose only provider is disabled in config. Resolve through the helper so the
route you act on is the route the config declares; a dead route surfaces
before you dispatch, not after.

Independence needs two reachable vendors. `<role> --vendors` prints the ones
that role could actually resolve to, applying the same capability, tier,
effort, and floor filters resolution uses; when it prints fewer than two, no
`--author-vendor` choice can route away from the author and the role will exit
3. Always pass the role when the answer decides whether a review can happen.
Bare `--vendors` reports every installed provider, which is a weaker claim: a
vendor the role does not route to cannot serve it.

Fewer than two is a real limit, not a misconfiguration. Say the cross-vendor
check did not run rather than reporting a review that never happened. The
Stop-hook nudge reads the same role-scoped signal and asks for human sign-off
instead of prescribing a command that cannot succeed.

## Routing Is Relative to the Lead

A delegate's value is that it is a different model or runtime from the one
orchestrating; that difference is what makes an independent review
independent. Read every default as "route to that provider unless you are
already it." When you only need parallelism rather than a second opinion, use
same-model parallel fan-out (mega-orchestration:orchestrating).

For plan_review, code_review, visual_verify, verify, judge, and council_member,
this is executable, not advisory. Pass every artifact author using repeatable
`--author-vendor`; the resolver rejects a missing author declaration and walks
the fallback chain past every matching vendor. `--exclude-lead` does not prove
authorship and cannot satisfy this policy. If no independent provider is
available, resolution fails rather than handing the work back to an author's
vendor. small_impl stays single-route because it is not an independence role.

For read-only independent review, prefer
`scripts/delegate-run --role ROLE --author-vendor VENDOR --artifact
worktree|FILE --claim TEXT`. It resolves and executes the safe provider adapter,
requires the verdict schema, computes the complete worktree or file identity,
and atomically writes a provenance receipt. The receipt is evidence only for
that exact subject identity; any tracked, staged, unstaged, or untracked change
invalidates it.

The exit map is the contract to branch on: 0 approved, 2 a usage or setup error,
3 no route resolved, 5 a valid needs-attention verdict, 6 a provider failure, 7
invalid provider output, 8 refused to dispatch because the review package holds
no substantive content, and 9 the review package could not be captured. The
launcher never exits with a raw git status: a tree git cannot read arrives as 9,
naming the path, not as 128.

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

`receipt.round` counts how many consecutive dispatches of this role on this branch
have not reached approve. It counts dispatches, not receipts, so a round that
burned reviewer time and then failed still counts, and it is what a caller caps a
review loop on. An approve resets it; a needs-attention verdict and a failed
dispatch do not. Deliberately keyed on the branch and not on the artifact
fingerprint: the author fixes something between rounds, so a fingerprint-keyed
count would report 1 forever and never reveal the loop. A detached HEAD keys on the
checked-out commit, and outside a repository the count is scoped to the receipt's
directory. `subject.id` remains the fingerprint, which is a different job: it binds
the receipt to the exact tree reviewed and any change at all invalidates it.

Stdout is the receipt JSON and nothing else, so it pipes straight into jq. The
`=== VERDICT ===` block goes to stderr, carrying the verdict, the round with the
branch it counts against, the receipt path, and the transcript path when one was
kept. Read that block rather than re-deriving it from the receipt.

`--transcript-dir DIR` keeps this dispatch's prompt, review package, and raw
provider output under `DIR/<role>-<subject>-round<N>`, one directory per dispatch,
including a dispatch that fails and writes no receipt. Since an approve resets the
round, that name can repeat; a repeat gets a numeric suffix rather than overwriting
the earlier transcript. Omitted, everything is discarded, which is the default.

The launcher validates `schemas/review-verdict-v1.json` and emits
`schemas/review-receipt-v2.json`. Its executable regression contract is
`scripts/tests/delegate-run.test.sh`.

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
section is what `subject.submodules` fingerprints, and the Stop hook recomputes it
with `scripts/review-diff-id --submodules`. It sits beside `subject.id` and never
inside it, because a receipt is compared against the id the shipped algorithm
produces and folding submodule content in would move the id of every repository
that has one. A tree with no gitlink carries no such field, and absent and empty
mean the same thing to the hook.

Every git read on this path runs with replacement objects disabled. `git replace
X Y` makes object reads return Y where X was asked for without moving any ref, so
`git rev-parse HEAD` keeps printing X while `git diff HEAD` renders against Y: a
repository holding X plus a replacement reproduces an approved delta on a base
nobody reviewed, and `subject.base` cannot tell. Disabling replacement fixes what
is read rather than detecting afterwards that the reviewer was shown a tree that
does not exist. An ordinary repository has no `refs/replace`, so no id moves.

A tree whose index HIDES a modification is refused rather than reviewed.
`git update-index --assume-unchanged P` stops git stat'ing P, so an edit to it
lands in no diff, no status, and no fingerprint. The launcher exits 9 naming the
path and the Stop hook blocks with a reason no receipt can clear. The neighbouring
skip-worktree bit is not refused with it, because sparse checkout sets it on every
path outside the cone; content decides, so a bit set over a path that still matches
the index costs nothing and a materialized changed one is caught either way.

Receipts are v2. A `megapowers.review-receipt.v1` receipt records no base, so it
cannot say which tree state its reviewer read and there is no way to recover
that after the fact; the Stop hook rejects it and says so in the block reason.
`schemas/review-receipt-v1.json` is kept only to describe receipts already on
disk.

Everything the reviewer is shown is the content the id names. Untracked files are
shown as their clean-filtered blob rather than their raw worktree bytes, because
`git hash-object` runs the filter chosen by the path and the id binds what comes
out of it. Under a `text`, `eol`, or `filter.*.clean` attribute the two differ,
and showing one while binding the other means approving bytes the receipt does
not cover.

Two programs compute it in the same format. `scripts/review-diff-id` fingerprints
a live worktree, which is what the Stop hook uses to test a receipt against the
tree in front of it; its executable regression contract is
`scripts/tests/review-diff-id.test.sh`. The launcher does not call it. It derives
the id from the same immutable snapshot it builds the review package out of,
because reading the tree once for the id and again for the package lets the two
name different trees: change the tree between the reads and restore it after, and
the receipt names a tree nobody reviewed while the hook still accepts it. The
consequence is that the format is duplicated and the two must stay byte
identical, which `delegate-run.test.sh` pins by asserting the launcher's id
equals `review-diff-id`'s across the tree shapes the pair is expected to survive.

## Role Defaults

Current assignments live in `[roles]`; the rationale and its date sit in the
comment above that table in delegates.toml. The stable shape:

- plan_review, code_review, and small_impl fit a provider that handles
  well-specified, testable, isolated work with a clear acceptance test and a
  bounded module, plus the independent adversarial pass on risky code
  (billing, auth, concurrency). Word the dispatch per the resolved provider's
  reference file (`references/providers/`): a contract-shaped prompt with an
  output schema beats added reasoning.
- visual and browser_test route to a computer-use capable provider (the
  `[requires]` table enforces the capability). Whoever drives, evidence
  discipline holds: screenshots land in `.megapowers/evidence/` and the lead
  re-reads them rather than trusting the text summary.
- visual_verify resolves a real vision-capable model provider and separately
  requires the `playwright-cli` driver. The driver captures pixels; it cannot
  satisfy vendor independence, tier, effort, or a verdict. `delegate-run`
  requires screenshot paths and binds their hashes into the receipt. Without
  either the independent model route or the driver, resolution fails. See
  [browser-delegate](../../agents/browser-delegate.md).

Keep planning, decomposition, broad multi-file context, bulk reads, and the
final write plus integration with the lead.

## Presets

The `[presets.*]` tables in delegates.toml declare the sandbox and integration
discipline for a delegated run; resolve one with `scripts/delegate-resolve
--preset <name>`. read_only is for reviews and verification: the delegate
looks and reports, it changes nothing. build is for small scoped
implementation in a dedicated worktree; hand the delegate a tight spec plus
the acceptance test. parallel runs one worktree-isolated delegate per task,
capped to avoid disk pressure, with patches integrated serially on the lead.
single_writer names the write discipline below.

## Single-Writer Discipline

Delegates write only inside worktrees, or they return patches; they never
write to the shared tree. The lead owns integration and commits, and nothing
lands without going through the lead. Never trust a self-reported pass: the
lead re-runs the tests before believing a task is done.

## Channels

Prefer the native orchestration surface of the tool you are already in; when
crossing runtimes, use the public CLI or SDK path first. Per-provider channel
mechanics (auth and sandbox caveats, thread resume, MCP fallbacks) live in the
provider's reference file under `references/providers/`; consult the resolved
provider's file rather than assuming another vendor's behavior. A hand-rolled
bridge is a fallback only when explicitly configured, so do not assume one
exists.

Provider identity means the vendor that actually runs the model, not the name
of the harness or compatibility protocol in front of it. A gateway or proxy is
acceptable only as a distinct provider entry with a truthful `vendor` key.
Never route an OpenAI model through a provider declared as Anthropic, or the
reverse: author-vendor exclusion would report a false independent pass because
vendor identity is the exclusion boundary.

When Claude is the different-vendor reviewer or judge, the launcher uses
`--bare` with an API key. For OAuth, it copies only the credential into a
disposable config home and runs from a disposable directory; this isolates
user plugins, hooks, memory, and project instructions, but enterprise-managed
Claude configuration may still apply. Both paths are one-shot and receive a
self-contained prompt.

Routes name CLIs because CLI-first is what stays portable across harnesses. Use
a harness-native async channel for a long-running delegate call where one
exists. Megapowers routes work between models you run yourself, so nothing here
crosses an organizational trust boundary.
