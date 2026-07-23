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
goes to whichever model is best suited for it. Routing lives in two layered
files. `models.toml` is the model catalog: who leads (`[lead]`), the
vendor-neutral tier scale and per-tier purposes (`[tiers]`, `[tiers.use]`),
the providers with their tier maps, capabilities, and channel data, and the
ship floor (`[defaults]`). `delegates.toml` is the routing: which provider
handles which role (`[roles]`, `[requires]`, `[fallbacks]`), the required tier
and effort (`[role_tiers]`, `[role_efforts]`), author-vendor independence
(`[independence]`), evidence drivers (`[drivers]`, `[role_drivers]`), and how
each run preset behaves (`[presets]`). Both resolve the same way: a project
`.megapowers/<file>` or user `~/.config/megapowers/<file>` layer overrides the
shipped copy per key, so a new model release is one tier-map line in a file
that survives plugin updates (`scripts/delegate-resolve --where` shows the
active layers of both). Provider sections written in delegates.toml layers
(pre-0.3 style) still parse and win over the catalog, so old override files
keep working. Prefer migrating provider data to a models.toml layer: the
always-loaded session block renders from catalog layers only, so a legacy
delegates-layer override resolves correctly but is not reflected in that
block, and partial overrides split across both stacks resolve per key, which
can surprise. Edit an override layer to change routing; the skill, the
delegate agents, and the session-start catalog block read the config live, so
no code changes are needed.

Each provider's `reference` key names that provider's channel mechanics and
prompting guidance: references/providers/codex.md and
references/providers/claude.md. Browser automation is a driver, not a model
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
`--list-presets` to enumerate). It walks
the role's fallback chain, skipping any provider that is excluded, disabled,
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
invalidates it. Exit 0 means approved, 5 means a valid needs-attention verdict,
6 a provider failure, and 7 invalid provider output.

The launcher validates `schemas/review-verdict-v1.json` and emits
`schemas/review-receipt-v1.json`. Its executable regression contract is
`scripts/tests/delegate-run.test.sh`.

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

For a delegate call that runs long, the sanctioned async channel is MCP Tasks,
the durable call-now/fetch-later extension; it is still finalizing, so reach
for it only where a harness actually exposes it. This repo stays CLI-first for
portability, which is why the routes above name CLIs rather than task servers.
A2A, the cross-organization agent-to-agent protocol, is a deliberate
non-target: megapowers routes work between models you run yourself, not across
organizational trust boundaries.
