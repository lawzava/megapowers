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
scripts/delegate-resolve <role>     # e.g. code_review, small_impl, visual, browser_test
# prints ROLE/PROVIDER/MODEL/CHANNEL/ENABLED/NOTES; exit 3 = unknown role,
# exit 4 = the routed provider is disabled in the config. `--list` lists roles.
```

Prefer resolving through the helper so the route you act on is the route the config
declares — if a provider is disabled or a role is unrouted, you find out before you
dispatch, not after.

**Routing is relative to the lead.** The value of a delegate is that it is a
*different* model or runtime from the one orchestrating — that's what makes an
independent review independent. So read every default below as "route to that
provider *unless you are already it*." If you are already running as the routed
provider (e.g. the lead is Codex and the role routes to Codex), send the
independent pass to a different provider instead, or — when you only need
parallelism, not a second opinion — use same-model fan-out
(dispatching-parallel-agents), which is the skill for that.

## Role Routing

Read the `[roles]` and `[providers.*]` tables in `delegates.toml`. The defaults:

- **Plan review, code review, small well-scoped implementation -> Codex (gpt-5.5).**
  From another runtime, use `codex exec`, the Codex SDK, or an explicitly
  configured private bridge. Codex is a strong fit for well-specified, testable,
  isolated work: a clear
  acceptance test and a bounded module. Also use it for hard self-contained
  single-file logic and for an independent adversarial pass on risky code
  (billing, auth, concurrency) — "find the bug in this diff." (If the lead is
  itself Codex, route the independent pass to a different model.)
- **Visual / UI work and browser / end-to-end testing -> the browser provider.**
  Drive the UI with `playwright-cli` and reason over the screenshots with a
  vision-capable model (the lead itself when it is vision-capable, e.g. Claude;
  otherwise route the screenshot to one). Screenshots land in
  `.megapowers/evidence/`; the lead re-reads them rather than trusting the text
  summary, so two passes verify the same pixels — the visual analog of the Codex
  adversarial pass on code. This route depends only on a standalone CLI, not on
  any one vendor's browser agent. See [browser-delegate](../../agents/browser-delegate.md).
- **Antigravity CLI** is a disabled alternative in the config. Leave it off
  unless you have verified `agy` automation, approvals, and artifact review in
  your local environment.

Keep planning, decomposition, broad multi-file context, bulk reads, and the
final write + integration with the lead.

## Presets — How a Delegate Runs

Presets describe the sandbox and integration discipline for a delegated run.
They are named in the config and referenced when you dispatch work:

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
SDK path first (`codex exec`, `playwright-cli` for browser/visual work, `agy`
once verified). Private MCP or bridge tools are acceptable only when they are
explicitly configured in the local environment; do not assume they exist.

## How to Adjust Routing

Everything above is driven by `delegates.toml` (in this skill's directory). To re-route a role,
change its value in the `[roles]` table (for example, point `code_review` at a
different provider). To swap a model, edit the provider's `model` field. To
enable an alternative backend, flip its `enabled` flag. No code changes are
needed — the skill and the delegate agents read the config at dispatch time.
