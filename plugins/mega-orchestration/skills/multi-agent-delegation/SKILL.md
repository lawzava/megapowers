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

The current agent is the orchestrator and lead. It keeps the broad context, plans and
decomposes the work, does cheap bulk reads, and owns final integration and
commits. Narrow, specialized work is handed off to whichever model is best
suited for it. Routing is not hardcoded — it is declared in `delegates.toml`
(shipped in this skill's own directory). Read that file first; it is the source
of truth for which provider handles which role and how each provider is reached.

To resolve a role executably (not just by eye), run the helper:

```bash
scripts/delegate-resolve <role>                    # e.g. code_review, small_impl, visual
scripts/delegate-resolve verify --exclude openai   # drop a vendor (e.g. the lead's own)
scripts/delegate-resolve --preset read_only        # resolve a run preset
# Prints ROLE/PROVIDER/MODEL/CHANNEL/VENDOR/BINARY/ENABLED/FLOOR/NOTES. It walks the
# role's fallback chain, skipping any provider that is --excluded, disabled, or whose
# CLI is not installed (checked with `command -v`), so a route never resolves to a
# runtime you do not have. `--list` lists roles, `--list-presets` lists presets.
```

Exit codes are a stable contract a harness can branch on:

- **0** resolved. Act on the printed route.
- **2** usage or config error, including a malformed config: the message names the
  offending line, so a broken table is never mistaken for an unknown role.
- **3** unknown role, or *no available route* (every candidate in the chain was
  excluded, disabled, or its CLI is absent).
- **4** a single-route role whose only provider is disabled in config. Enable it or
  re-route before dispatching.

Prefer resolving through the helper so the route you act on is the route the config
declares: if a provider is disabled, excluded, absent, or a role is unrouted, you
find out before you dispatch, not after.

**Routing is relative to the lead.** The value of a delegate is that it is a
*different* model or runtime from the one orchestrating — that's what makes an
independent review independent. So read every default below as "route to that
provider *unless you are already it*." If you are already running as the routed
provider (e.g. the lead is Codex and the role routes to Codex), send the
independent pass to a different provider instead, or — when you only need
parallelism, not a second opinion — use same-model fan-out
(dispatching-parallel-agents), which is the skill for that.

For the cross-vendor roles (`verify`, `judge`, `council_member`) this is
executable, not just advisory: each carries a `[fallbacks]` chain to a second
vendor, so resolve with `--exclude <your own vendor>` (a Codex lead runs
`delegate-resolve verify --exclude openai`) and the helper walks that chain to a
different-vendor route. If no different-vendor route is available it exits 3
rather than silently handing the work back to the author's vendor.

## Role Routing

Read the `[roles]` and `[providers.*]` tables in `delegates.toml`. The defaults:

- **Plan review, code review, small well-scoped implementation -> Codex (gpt-5.5).**
  From another runtime, use `codex exec` (add `--output-schema` for a
  machine-checkable verdict), the Codex SDK (`@openai/codex-sdk`, `openai-codex`),
  or `codex mcp-server` (the first-party MCP channel). Codex is a strong fit for
  well-specified, testable,
  isolated work: a clear
  acceptance test and a bounded module. Also use it for hard self-contained
  single-file logic and for an independent adversarial pass on risky code
  (billing, auth, concurrency) — "find the bug in this diff." (If the lead is
  itself Codex, route the independent pass to a different model.)
- **Visual / UI work and browser / end-to-end testing -> Codex (native computer
  use).** A cost-adjusted call, dated in the note above `[roles]` in
  delegates.toml: Claude leads the computer-use benchmarks by a modest margin,
  but Codex runs several times cheaper in tokens, and below-bar output has the
  escalation redo path. Whoever drives, evidence discipline holds: screenshots
  land in `.megapowers/evidence/` and the lead re-reads them rather than
  trusting the text summary.
- **Independent verification of rendered UI/UX work (`visual_verify`) -> the
  browser provider**, so the verifying vendor differs from the authoring one:
  drive the UI with `playwright-cli` and reason over the screenshots with a
  vision-capable model (the lead itself when it is vision-capable, e.g. Claude;
  otherwise route the screenshot to one). This role has no fallback: on a host
  without `playwright-cli` installed, delegate-resolve reports no available
  route (exit 3) rather than fabricating one. Two passes verify the same pixels,
  the visual analog of the Codex adversarial pass on code. The vendor split is
  the point: the vision model reading the screenshots must not be from the
  vendor that authored the work, so when the lead is itself Codex, route the
  read to a non-Codex vision model. This route depends only on a standalone
  CLI, not on any one vendor's browser agent, and doubles as the redo path when
  Codex-led visual work misses the bar — and a browser-provider redo then gets
  its verification pass from Codex, keeping author and verifier distinct. See
  [browser-delegate](../../agents/browser-delegate.md).
- **Antigravity CLI** is a disabled alternative in the config. Leave it off
  unless you have verified `agy` automation, approvals, and artifact review in
  your local environment.

Keep planning, decomposition, broad multi-file context, bulk reads, and the
final write + integration with the lead.

## Presets — How a Delegate Runs

Presets describe the sandbox and integration discipline for a delegated run.
They live as a `[presets.*]` table in `delegates.toml` (resolve one with
`scripts/delegate-resolve --preset <name>`), and are referenced when you dispatch
work:

- **read_only** — sandbox read-only, approvals never. For reviews and
  verification. The delegate looks and reports; it changes nothing.
- **build** — workspace-write inside a dedicated worktree, approvals never. For
  small scoped implementation. Hand the delegate a tight spec plus the
  acceptance test. The **lead** then runs the tests and integrates — the
  delegate does not merge its own work.
- **parallel** — one worktree-isolated delegate per task, capped at 3–5 to
  avoid disk pressure. Each runs in its own worktree; collect the patches and
  integrate them serially back on the lead.

## Single-Writer Discipline

- Delegates write **only** inside worktrees, or they return patches. They never
  write to the shared tree.
- The lead owns integration and commits. Nothing lands without going through the
  lead.
- **Never trust a self-reported pass.** The lead re-runs the tests before
  believing a task is done.

## Channel and Sandbox Constraint

Prefer the native orchestration surface of the tool you are already in:
Codex subagents in Codex, Claude subagents or workflows in Claude Code, and
OpenCode subagents in OpenCode. When crossing runtimes, use the public CLI or
SDK path first (`codex exec` for Codex roles including visual/browser,
`playwright-cli` for visual verification and browser fallback, `agy` once
verified). For a persistent Codex thread from another harness, `codex mcp-server`
is the first-party MCP channel (it exposes the `codex` and `codex-reply` tools); a
hand-rolled bridge is a fallback only when it is explicitly configured, so do not
assume one exists.

## Long-Running and Cross-Org Channels

For a delegate call that runs long (minutes to hours), the sanctioned async channel
is **MCP Tasks**: the call-now/fetch-later augmentation (a durable task handle the
client drives with `tasks/get`) that moved out of the experimental 2025-11 core into
its own MCP extension. It is a release candidate dated 2026-07-28, so treat it as
finalizing rather than final, and reach for it only where a harness actually exposes
it. This repo stays **CLI-first** for portability: `codex exec`, `codex mcp-server`,
and `playwright-cli` run on any harness that has the binary installed, with no server
to stand up, which is why the routes above name CLIs rather than task servers.
**A2A**, the cross-organization agent-to-agent protocol, is a deliberate non-target:
megapowers routes work between models you run yourself, not across organizational
trust boundaries.

## How to Adjust Routing

Everything above is driven by `delegates.toml` (in this skill's directory). To re-route a role,
change its value in the `[roles]` table (for example, point `code_review` at a
different provider). To swap a model, edit the provider's `model` field. To
enable an alternative backend, flip its `enabled` flag. No code changes are
needed — the skill and the delegate agents read the config at dispatch time.
