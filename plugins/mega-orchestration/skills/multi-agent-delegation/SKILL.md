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

Routing lives in two layered files: `models.toml`, the model catalog (who
leads, the tier scale and its purposes, the providers with their tier maps and
channel data, the ship floor), and `delegates.toml`, the routing (which provider
handles which role, at which tier and effort, under which independence, evidence
driver, and run preset). Every table name is in
references/dispatch-contract.md.

Both resolve the same way: a project `.megapowers/<file>` or user
`~/.config/megapowers/<file>` layer overrides the shipped copy per key, so a new
model release is one tier-map line in a file that survives plugin updates.
`scripts/delegate-resolve --where` shows the active layers. Edit an override
layer to change routing: every resolver and dispatch path reads the config live.
What the docs claim about routing is pinned
by `scripts/tests/delegation-docs.test.sh`.

The nine roles: plan_review, code_review, small_impl, visual, browser_test,
visual_verify, verify, judge, council_member.

## Resolving a Route

`scripts/delegate-resolve <role>` resolves the config executably. It walks the
role's fallback chain, skipping any provider that is excluded, disabled, missing
a required capability, below the configured floor, or whose CLI is not
installed, so a route never resolves to a runtime you do not have, and prints
ROLE/PROVIDER/MODEL/TIER/EFFORT/CHANNEL/ENABLED/VENDOR/BINARY/FLOOR/NOTES, plus
DRIVER fields when the role requires an evidence driver. Every flag is in
references/dispatch-contract.md.

Exit codes are a stable contract a harness can branch on: 0 resolved, act on
the printed route; 2 usage or config error, including a malformed config, with
the message naming the offending line so a broken table is never mistaken for
an unknown role; 3 unknown role or no available route; 4 a single-route role
whose only provider is disabled in config. Resolve through the helper so the
route you act on is the route the config declares; a dead route surfaces
before you dispatch, not after.

## Declare Who Is Running

Pass `--caller-model <your model id>` and `--caller-adapter <runtime>` whenever
you resolve a route. The caller identifies the running runtime; it is not the
artifact author and never enters the independence exclusion set.
It sets `DISPATCH` and nothing else. `DISPATCH=native` means the route landed on
your OWN provider: run it with the harness's own primitive, a subagent or a
saved workflow, because invoking your own CLI spawns a cold session, throws away
the context that made delegating worth doing, and bills for it twice.
`DISPATCH=cli` means the route crosses to another provider: use `CHANNEL` and
`BINARY` with that provider's reference file. Undeclared, the resolver assumes
the catalog lead and says so, which is how an undeclared non-lead session ends
up running its own subagent against another vendor's model believing it was
home.

## Independence

A delegate's value is that it is a different model or runtime from the one
orchestrating; that difference is what makes an independent review independent.
For plan_review, code_review, visual_verify, verify, and judge
it is executable, not advisory: pass every artifact author with repeatable
`--author-vendor <vendor>`, or `--author-model <id>` to let the resolver derive
the vendor, and the chain walks past every matching vendor. A missing author
declaration is rejected. `--exclude-lead` does not prove authorship and cannot
satisfy this policy. If no independent provider is available, resolution fails
rather than handing the work back to an author's vendor.

Independence needs two reachable vendors. `<role> --vendors` prints the ones
that role could actually resolve to; fewer than two means no `--author-vendor`
choice can route away from the author and the role exits 3. That is a real
limit, not a misconfiguration: say the cross-vendor check did not run rather
than reporting a review that never happened.

Which leaves a choice, and the evidence decides it. Context separation is the
proven half: a fresh session reading only the artifact measured 28.6% F1 against
24.6% for same-session self-review, while handing the reviewer the generation
transcript measured 23.8%, worse than doing nothing (arXiv 2603.12123). That
study never varied the model, so the vendor swap is a motivated prior about
uncorrelated blind spots rather than a result. `--allow-context-separation`
therefore lets an unreachable cross-vendor route fall back to a fresh same-vendor
session, labeled `INDEPENDENCE=context-separation` on the route and in the
receipt, with `independent: false`. Reach for it when the alternative is no
review; do not reach for it on auth, billing, or concurrency, where the prior is
what you are paying for and the Stop gate will keep blocking anyway. `judge`
refuses the tier: a fresh session does not remove self-preference from a blind
ranking. Neither tier is an oracle, since the best measured condition still
caught under 30% of injected errors. The tests are.

## One Dispatch

A Claude-authored change going out for independent code review, in one call:

```bash
scripts/delegate-run --role code_review --author-vendor anthropic \
  --artifact worktree --claim "caps the review loop at max_rounds"
```

It requires the verdict schema, computes the worktree identity, and atomically
writes a provenance receipt bound to that identity. Stdout is the receipt JSON
and nothing else; the `=== VERDICT ===` block on stderr carries the verdict, the
round, and the receipt path.

`delegate-run` dispatches one reviewer and writes one receipt. Council panels,
generation identifiers, and member scope are lead-managed orchestration state,
not launcher protocol. Do not claim those fields are receipt-backed.

`council_member` generates an answer before an artifact exists and therefore
does not require an author declaration. `judge` ranks those generated artifacts
and requires every author vendor, preserving the blind-ranking boundary.

## Where the Rest Lives

| Reference | Carries |
| --- | --- |
| references/dispatch-contract.md | every resolver flag, the floor, route fields, presets, worktree and single-writer rules, integration ownership |
| references/receipts-and-rounds.md | the delegate-run exit map, what a receipt binds, the round ledger and its cap |
| references/providers/codex.md | Codex channels and how to word a dispatch |
| references/providers/claude.md | Claude channels and how to word a dispatch |
| references/providers/opencode.md | OpenCode runtime identity and portable-skill compatibility boundary |
| references/providers/browser.md | the browser driver, which is a driver and not a model provider |

Read the resolved provider's file, and the driver's when the role needs one,
before dispatching. `visual_verify` is the role that needs two things at once:
an independent vision-capable model route and the `playwright-cli` driver. Read
the [browser driver](references/providers/browser.md) for its capture and
evidence contract. Resolution fails without either.
