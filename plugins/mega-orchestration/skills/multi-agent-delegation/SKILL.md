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
layer to change routing: the skill, the browser delegate, and the session-start
catalog block read the config live. What the docs claim about routing is pinned
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

Pass `--caller-model <your model id>` whenever you are not the catalog `[lead]`.
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
For plan_review, code_review, visual_verify, verify, judge, and council_member
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
than reporting a review that never happened. The Stop-hook nudge reads the same
role-scoped signal and asks for human sign-off instead of prescribing a command
that cannot succeed.

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

## Where the Rest Lives

| Reference | Carries |
| --- | --- |
| references/dispatch-contract.md | every resolver flag, the floor, route fields, presets, worktree and single-writer rules, integration ownership |
| references/receipts-and-rounds.md | the delegate-run exit map, what a receipt binds, the round ledger and its cap |
| references/providers/codex.md | Codex channels and how to word a dispatch |
| references/providers/claude.md | Claude channels and how to word a dispatch |
| references/providers/moonshot.md | Moonshot (Kimi) channels, currently declared and not reachable |
| references/providers/browser.md | the browser driver, which is a driver and not a model provider |

Read the resolved provider's file, and the driver's when the role needs one,
before dispatching. `visual_verify` is the role that needs two things at once:
an independent vision-capable model route AND the `playwright-cli` driver, whose
capture contract is [browser-delegate](../../agents/browser-delegate.md).
Resolution fails without either, so read that file from here rather than
discovering the driver half after the model half already resolved.
