---
name: multi-agent-delegation
description: >-
  Use when the best executor is a different model or runtime rather than another
  instance of the same one — route a scoped build, plan or code review,
  adversarial verification, or visual/browser task while the lead orchestrates.
  Distinct from dispatching-parallel-agents and subagent-driven-development, which
  fan work out to same-model agents. Triggers on "delegate to Codex", "delegate
  the visual/browser work", "hand this off to another model", "get an independent
  model's review".
license: MIT
---

# Multi-Agent Delegation

Unsure whether delegation is the right structure at all? Start at
mega-orchestration:orchestrating, the decision root; this skill executes the
delegation route it picks.

## The Idea

The lead keeps the broad context, plans and decomposes the work, does cheap
bulk reads, and owns final integration and commits. Narrow, specialized work
goes to whichever model is best suited for it. Routing lives in
`delegates.toml`: who leads (`[lead]`), the vendor-neutral tier scale
(`[tiers]`), the providers with their tier maps and capabilities, which
provider handles which role (`[roles]`), and how each run preset behaves. The
shipped file in this skill's directory is the default layer; a project
`.megapowers/delegates.toml` or user `~/.config/megapowers/delegates.toml`
overrides it per key, so a new model release is one line in a file that
survives plugin updates (`scripts/delegate-resolve --where` shows the active
layers). Edit an override layer to change routing; the skill and the delegate
agents read the config at dispatch time, so no code changes are needed.

Each provider's `reference` key names that provider's channel mechanics and
prompting guidance: references/providers/codex.md, references/providers/claude.md,
and references/providers/browser.md. Read the resolved provider's file before
dispatching instead of assuming any vendor's quirks from memory.

Antigravity is disabled in the config; leave it disabled until you verify its
approvals, artifact review, and artifact edit behavior on this machine.

The nine roles: plan_review, code_review, small_impl, visual, browser_test,
visual_verify, verify, judge, council_member.

The floor is `[defaults] floor`, written as tier:effort on the `[tiers]` scale
(shipped: `"strong:low"`). Nothing that ships routes below it; a provider whose
tier sits below the floor is skipped at resolution.

## Resolving a Route

`scripts/delegate-resolve <role>` resolves the config executably (`--preset
<name>` for presets, `--exclude <vendor|provider>` to drop a backend,
`--exclude-lead` to drop whatever `[lead]` declares, `--lead` to print the
declared orchestrator, `--where` to print the active config layers, `--check`
to validate the table, `--list` and `--list-presets` to enumerate). It walks
the role's fallback chain, skipping any provider that is excluded, disabled,
missing a required capability, below the floor tier, or whose CLI is not
installed, so a route never resolves to a runtime you do not have, and prints
ROLE/PROVIDER/MODEL/TIER/EFFORT/CHANNEL/ENABLED/VENDOR/BINARY/FLOOR/NOTES.

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
same-model fan-out (dispatching-parallel-agents).

For the cross-vendor roles (verify, judge, council_member) this is executable,
not advisory: each carries a `[fallbacks]` chain to a second vendor, and they
must resolve to a vendor different from the author's. Resolve with
`--exclude-lead` (the config's `[lead]` names the orchestrator, so
the exclusion follows a lead swap automatically) or an explicit `--exclude
<vendor>`; the helper walks the chain to a different-vendor route, and if none
is available it exits 3 rather than handing the work back to the author's
vendor.

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
- visual_verify routes to the browser provider: drive the UI with
  `playwright-cli` and reason over the screenshots with a vision-capable
  model, so the vendor reading the pixels differs from the vendor that
  authored the work. The role has no fallback; without `playwright-cli` the
  helper exits 3 rather than fabricating a route. It also serves as the redo
  path when the visual route's output misses the bar, and the original
  provider then verifies the redo, keeping author and verifier distinct. See
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

For a delegate call that runs long, the sanctioned async channel is MCP Tasks,
the durable call-now/fetch-later extension; it is still finalizing, so reach
for it only where a harness actually exposes it. This repo stays CLI-first for
portability, which is why the routes above name CLIs rather than task servers.
A2A, the cross-organization agent-to-agent protocol, is a deliberate
non-target: megapowers routes work between models you run yourself, not across
organizational trust boundaries.
