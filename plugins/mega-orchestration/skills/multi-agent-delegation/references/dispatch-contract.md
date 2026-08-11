# Dispatch Contract

How a resolved route turns into a call: the resolver's flags, the fields it
prints, the presets that set sandbox and write discipline, and who owns the
result. SKILL.md carries the decision and the first call; this file carries the
mechanics behind them.

- [The config tables](#the-config-tables)
- [Resolver flags](#resolver-flags)
- [The floor](#the-floor)
- [Route fields to branch on](#route-fields-to-branch-on)
- [Identity: who wrote it, who is running](#identity-who-wrote-it-who-is-running)
- [Ambiguous model ids](#ambiguous-model-ids)
- [Role defaults](#role-defaults)
- [Presets](#presets)
- [Single-writer discipline](#single-writer-discipline)
- [Provider identity](#provider-identity)
- [Channels and portability](#channels-and-portability)

## The config tables

- `models.toml`, the model catalog: who leads (`[lead]`), the vendor-neutral
  tier scale and per-tier purposes (`[tiers]`, `[tiers.use]`), the providers
  with their tier maps, capabilities, and channel data, and the ship floor
  (`[defaults]`).
- `delegates.toml`, the routing: which provider handles which role (`[roles]`,
  `[requires]`, `[fallbacks]`), the required tier and effort (`[role_tiers]`,
  `[role_efforts]`), author-vendor independence (`[independence]`), evidence
  drivers (`[drivers]`, `[role_drivers]`), and how each run preset behaves
  (`[presets]`).

## Resolver flags

`scripts/delegate-resolve <role>` takes: `--preset <name>` for presets,
`--author-vendor <vendor>` once per artifact-author vendor for independent
roles, `--author-model <id>` to name an author by model id and let the resolver
derive the vendor, `--author-provider <name>` to name the author's own backend
when an id is not unique, `--caller-model <id>` / `--caller-provider <name>` /
`--caller-adapter <name>` to say which model backend and runtime session are
RUNNING (native dispatch only, never exclusion),
`--exclude <vendor|provider>` to drop a backend, `--exclude-lead` as a
compatibility exclusion, `--allow-context-separation` to authorize the degraded
same-vendor tier described under `INDEPENDENCE` below,
`--models <file>` to pin the catalog, `--lead` to print
the declared orchestrator, `--where` to print the active config layers,
`--check` to validate the table, `--list` and `--list-presets` to enumerate, and
`--vendors` to print reachable vendors.

Bare `--vendors` reports every installed provider, which is a weaker claim than
the role-scoped form: a vendor the role does not route to cannot serve it.
Always pass the role when the answer decides whether a review can happen.

Put provider data in a `models.toml` layer, not a `delegates.toml` one: the
always-loaded session block renders from catalog layers only.

## The floor

The floor is `[defaults] floor` in the catalog, written as tier:effort on the
`[tiers]` and `[efforts]` scales (shipped: `"strong:low"`). Nothing that ships
routes below it. A provider whose tier or declared default effort sits below the
corresponding floor is skipped at resolution; providers without an effort
setting are compared by tier only.

## Route fields to branch on

`DISPATCH` says which kind of call to make:

- `DISPATCH=native`: the route landed on your OWN provider. Run it with the
  harness's own primitive (Claude Code subagents or a saved workflow, Codex
  native subagents, whatever the runtime gives you). `CHANNEL` and `BINARY`
  describe the cross-runtime path and do not apply. Invoking your own CLI here
  would spawn a cold session, throw away the context that made delegating worth
  doing, and bill for it twice.
- `DISPATCH=cli`: the route crosses to another provider. Use `CHANNEL`/`BINARY`
  and the resolved provider's reference file.

`CALLER` says how that was decided; `CALLER_ADAPTER` names the runtime adapter.
`declared` means you named the running session. `assumed-lead` means nobody
did, so `native` rests on the catalog `[lead]` default and the resolver says so
on stderr. That distinction matters on a machine where more than one harness
leads: an undeclared non-lead session reading `native` would run its own
subagent against another vendor's model, believing it was home.

`AUTHOR_VENDOR` is repeated once per author and is authoritative. The joined
`AUTHOR_VENDORS` remains for consumers written against the older contract;
prefer the repeated form, which needs no delimiter and so cannot split a vendor
name or hide a blank identity inside a joined string.

`ALTERNATES` appears on independence roles and counts the vendors that could
still serve the role with the authors excluded. `ALTERNATES=1` means the next
outage takes independent review with it.

`INDEPENDENCE` appears on independence roles and says which separation the route
actually achieved:

- `cross-vendor`: the reviewer's vendor authored none of the artifact. The
  shipped default and the only value that satisfies the risky-logic Stop gate.
- `context-separation`: no cross-vendor route was reachable, and `--allow-context-separation`
  authorized a fresh same-vendor session instead. `ALTERNATES=0` always
  accompanies it.

The degraded tier is opt-in, never automatic, and it is the better-evidenced
half of the policy rather than a consolation prize. The controlled study behind
independent review (arXiv 2603.12123, 360 reviews over 150 injected errors)
varied context, not model: fresh-session artifact-only review scored 28.6% F1
against 24.6% for same-session self-review, and handing the reviewer the
generation transcript scored 23.8%, worse than doing nothing. Cross-vendor
review is a motivated prior about uncorrelated blind spots, not a measured
result, which is why it remains the default and remains mandatory where
correlated blind spots are the specific risk. When the other vendor is simply
unreachable, a fresh same-vendor session is the proven condition, and taking it
beats skipping review, provided nobody reports it as the cross-vendor pass.

Two limits keep that honest. A `judge` role (`all_author_vendors`) refuses the
tier outright, because blind ranking fails on self-preference and a fresh
session does not repair that. And `delegate-run` writes `independent: false`
alongside `independence: "context-separation"`, so the risky-logic gate on auth,
billing, payment, and concurrency keeps blocking: those are exactly the changes
where the vendor prior is worth paying for.

Even the best measured condition caught under 30% of injected errors. No review
is an oracle at either tier; the tests are.

## Identity: who wrote it, who is running

Identity splits along three axes. **Who wrote the artifact** (`--author-*`) drives
exclusion and receipt provenance. **Who is running** (`--caller-model` /
`--caller-provider` / `--caller-adapter`) drives native dispatch only, never exclusion, so declaring
the caller can never make a review look independent. The two coincide for a
`self` role and differ for every review of someone else's work: the author is
excluded while the route lands on the caller, which is a native dispatch. With
no caller declared the session is assumed to be the catalog `[lead]`.

`--caller-adapter` alone names only a launch surface. It cannot prove which
model provider that runtime configured, so the resolver keeps routes CLI until
`--caller-model` or `--caller-provider` supplies the model-provider identity.

Announce the running identity rather than assuming it. A harness always knows the
model id it is running, so `--caller-model claude-opus-5` works from any session
without hardcoding a vendor name per harness. This matters for BYO-model
runtimes (OpenCode, Cursor CLI, pi), which have no fixed vendor: their vendor is
whichever model is configured, so it can only be reported at runtime, never
declared in a config file.

`TIER_FALLBACK=<requested>-><resolved>` appears when a `self` route could not get
the tier its role asked for. `self` promises the caller's own backend; the tier is
an optimisation over a backend already chosen, and several catalogued providers
publish one tier only, so a BYO-model caller can be running a model whose provider
has nothing at the requested tier. Rather than strand that caller, the resolver
takes the nearest tier the provider does publish, preferring the stronger one when
both neighbours exist, and says so on this field. `TIER` always reports what the
route actually got, and the absence of `TIER_FALLBACK` is the contract for "the
role got the tier it asked for". Only `self` routes retier: a static or chained
route has other candidates to fall through to, so retiering one would hide a
misconfigured chain. The floor is unaffected either way, so a provider whose
nearest tier sits below `[defaults] floor` is still skipped rather than rescued.

The author flags answer two different questions, and only one of them wants a
single answer:

- **Who must be excluded** (every independence role). Repeating the flags is how
  you declare several artifact authors, which is what `judge` is for: each one
  contributes a vendor and all of them are excluded. Multiple identities are
  normal here and are never a conflict.
- **Who is calling** (a `self` role). There is one caller, so every flag
  occurrence is read as an assertion about it and they intersect. Several ids
  naming the same provider (a lead and its subagent on different tiers of one
  backend) answer the question; occurrences that share no provider are refused
  as contradictory, and an intersection still holding two is refused as
  ambiguous.

Authorship is always a vendor claim, whatever you name it with.
`--author-vendor` accepts a provider name for convenience and normalizes it to
that provider's vendor before excluding, so naming one backend excludes its
siblings too. A value matching no declared vendor or provider is rejected (exit
2) rather than treated as an exclusion of nobody: a placeholder author is how a
review comes back "independent" without having excluded anyone.

Declaring yourself is safe on any config that declares the policy, which is
every config that ships here. Author exclusion applies to roles carrying an
`[independence]` entry, so identifying yourself does not cost you your own
vendor on a role that never asked for a second opinion.

The exception is a legacy table: a config with no `[independence]` section that
also never uses the `self` sentinel predates per-role policy, and there author
exclusion stays unconditional, so declaring yourself does exclude your vendor
everywhere. Using `self` anywhere proves a config is not one of those.
`--exclude` stays the policy-free way to drop a backend on any config.

## Ambiguous model ids

An id is only an identity while it is unique. Open-weights models are reachable
through more than one host, so the same id can appear under several providers:

- Different vendors: `--author-model` is refused (exit 2), because excluding
  either one would leave the artifact's actual author eligible to review it.
  `--check` reports it wherever it appears in the catalog.
- One vendor, several providers: independence is unaffected, so review roles
  resolve normally. A `self` role is refused, because it promises the caller's
  own backend and the id does not say which one. `--author-provider <name>`
  names it and still contributes that provider's vendor to the exclusion set.

## Role defaults

Current assignments live in `[roles]`; the rationale and its date sit in the
comment above that table in delegates.toml. Read every default as "route to that
provider unless you are already it": the table names a provider, not a
destination, so what it means depends on who is reading it. The stable shape:

- plan_review and code_review fit a provider that handles the independent
  adversarial pass on risky code (billing, auth, concurrency). small_impl wants
  the same shape of work, well-specified and testable against a clear acceptance
  test in a bounded module, but resolves in-vendor because nothing about it
  requires a second opinion. Word the dispatch per the resolved provider's
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
  [browser-delegate](../../../agents/browser-delegate.md).

small_impl defaults to `self`: the caller's own provider, taken from
`--author-model`/`--author-vendor` or the catalog `[lead]`. It ships that way
because a scoped implementation has no independence requirement, so leaving the
vendor for it spends a third-party call and buys nothing. A role cannot be both
`self` and `[independence]`-bound; `--check` rejects that combination.

`visual` and `browser_test` are the exception in the other direction: they name
a provider for capability and cost (the computer-use bench), not for
independence, so they can leave the caller's vendor without an independence
policy behind them. Read `[roles]` rather than inferring a role's vendor from
whether it is listed under `[independence]`.

When you only need parallelism rather than a second opinion, use same-model
parallel fan-out (mega-orchestration:orchestrating). Keep planning,
decomposition, broad multi-file context, bulk reads, and the final write plus
integration with the lead.

## Presets

The `[presets.*]` tables in delegates.toml declare the sandbox and integration
discipline for a delegated run; resolve one with `scripts/delegate-resolve
--preset <name>`. read_only is for reviews and verification: the delegate looks
and reports, it changes nothing. build is for small scoped implementation in a
dedicated worktree; hand the delegate a tight spec plus the acceptance test.
parallel runs one worktree-isolated delegate per task, capped to avoid disk
pressure, with patches integrated serially on the lead. single_writer names the
write discipline below.

## Single-writer discipline

Delegates write only inside worktrees, or they return patches; they never write
to the shared tree. The lead owns integration and commits, and nothing lands
without going through the lead. Never trust a self-reported pass: the lead
re-runs the tests before believing a task is done.

## Provider identity

Provider identity means the vendor that actually runs the model, not the name of
the harness or compatibility protocol in front of it. A gateway or proxy is
acceptable only as a distinct provider entry with a truthful `vendor` key. Never
route an OpenAI model through a provider declared as Anthropic, or the reverse:
author-vendor exclusion would report a false independent pass because vendor
identity is the exclusion boundary.

When Claude is the different-vendor reviewer or judge, the launcher uses
`--bare` with an API key. For OAuth, it copies only the credential into a
disposable config home and runs from a disposable directory; this isolates user
plugins, hooks, memory, and project instructions, but enterprise-managed Claude
configuration may still apply. Both paths are one-shot and receive a
self-contained prompt.

## Channels and portability

Per-harness native primitives are mapped in
`mega-orchestration:orchestrating`'s `references/harness-primitives.md`. The
resolver decides `DISPATCH` by comparing the resolved provider against the
RUNNING session, so pass `--caller-model <your model id>` whenever you are not
the catalog lead. Reviewing another agent's work is the case that needs it:
without it the artifact's provider would be mistaken for the running one, and
your own route would be treated as foreign.

Prefer the native orchestration surface of the tool you are already in; when
crossing runtimes, use the public CLI or SDK path first. Per-provider channel
mechanics (auth and sandbox caveats, thread resume, MCP fallbacks) live in the
provider's reference file under `references/providers/`, which that provider's
`reference` key in the catalog names, so a new provider declares its own file
rather than needing one added to a list here. Consult the resolved provider's
file rather than assuming another vendor's behavior. A hand-rolled
bridge is a fallback only when explicitly configured, so do not assume one
exists.

Routes name CLIs because CLI-first is what stays portable across harnesses, but
that portability is for CROSSING runtimes. Inside your own, the harness's teams,
subagents, and workflows are the better instrument and `DISPATCH=native` says
so. Use a harness-native async channel for a long-running delegate call where
one exists. Megapowers routes work between models you run yourself, so nothing
here crosses an organizational trust boundary.
