# mega-orchestration

Config-driven multi-model orchestration. Route each kind of work to the model
that handles it best, without hard-coding a provider. The routing lives in one
config file, so swapping a backend is an edit, not a rewrite.

## Skills

- `orchestrating`: the decision root. At task arrival it routes the task's
  shape to the right structure (inline, parallel subagents, delegation,
  best-of-n, council, autonomous run) with spend-by-stakes effort defaults,
  and maps subagent/team/workflow/effort primitives per harness in its
  `references/harness-primitives.md`.
- `wayfinding`: map long-horizon work whose unknown ownership, unresolved
  decisions, or unclear sequencing prevents an honest spec or plan. It keeps a
  local uncertainty map and resolves one decision at a time, without required
  tracker or commit behavior. Codex invocation is explicit-only through the
  skill's `agents/openai.yaml`; other harnesses may still invoke it implicitly.
- `multi-agent-delegation`: when and how the lead (the agent session you are
  talking to, which orchestrates and owns integration) hands work to a
  delegate (a separately invoked model or CLI that returns results), plus
  `scripts/delegate-resolve` to resolve a role and `scripts/delegate-run` to
  execute independent reviews with structured, subject-bound receipts.
- `best-of-n`: generate N independent candidates, select by an executable
  oracle first and a blind judge second. Selection, never averaging.
- `cross-model-verification`: verify risky work with a different-vendor model
  that tries to refute it, blind to the author's reasoning.
- `council-adjudication`: for a decision with no oracle. Answer independently,
  rank the answers anonymized, synthesize from the best, not a compromise.
- `autonomous-run`: run a long, largely-unattended task on a durable file
  contract: a frozen charter, a plan with acceptance criteria, a runbook, an
  append-only journal, and a machine-readable status. An autonomy dial
  (autonomous / on-the-loop / in-the-loop) gates by reversibility and blast
  radius; reversible work always proceeds. Ends in a legible,
  confidence-ranked run report.
- `effect-broker`: the portable trust layer for real-world side effects.
  Classify an action (reversible / staged / irreversible) and enforce
  simulate-then-commit with idempotency and blast-radius caps. Irreversible
  actions never auto-fire, even at the `autonomous` level. The gate is
  declaration-based (see the repository `SECURITY.md`), distinct from the
  Claude-only deny-destructive command tripwire.

The plugin also ships two delegate agents and two Stop hooks, described below.
The skills and the delegate agents read `models.toml` (catalog) and
`delegates.toml` (routing, inside the `multi-agent-delegation` skill
directory) for backend and model choices.

## Role routing

| Role | Default delegate | Used for |
| --- | --- | --- |
| Plan review | claude (frontier tier) | The planning buddy: reviewing plans and double-checking decisions |
| Code review | codex (strong tier) | Reviewing diffs, adversarial "find the bug" passes |
| Small implementation | codex (strong tier) | Well-specified, testable, single-file or isolated changes |
| Visual / browser | codex (frontier tier, native computer use) | UI work, browser-driven checks, end-to-end testing |
| Visual verification | claude (frontier tier) + `playwright-cli` driver | Independent cross-vendor judgment of captured UI/UX evidence |

Shipped defaults; current model ids live in `models.toml` tier maps, and a
project `.megapowers/models.toml` or user `~/.config/megapowers/models.toml`
layer overrides them per key. The provider and tier data now lives in
`models.toml` (plugin root, twin of the copy shipped with the megapowers core
plugin); `delegates.toml` keeps roles, fallbacks, and presets.

The visual/browser route is a cost-adjusted call, dated in the comment above
`[roles]` in `delegates.toml`; re-bench before moving it.

To adjust the routing, edit an override layer: `models.toml` for the lead, the
tier scale, and each provider's tier map and capabilities; `delegates.toml`
for which provider handles which role. Edit the shipped copies only when
changing project defaults. `scripts/delegate-resolve` resolves it executably;
`--check` validates it and CI runs that.

## Prerequisites

- Roles route per `[roles]` through a channel that can honor the resolved
  provider, model, and effort.
  Native v2 can honor only the current session model and effort, so use it for
  same-session Codex fan-out; use a role-aware
  or non-interactive Codex channel when those fields differ. Other providers
  use their first-party plugin, CLI, SDK, or MCP channel (see
  `references/providers/`).
- Visual verification role: `playwright-cli` plus a vision-capable model to read
  the screenshots. Install: `npm i -g @playwright/cli`, then
  `playwright-cli install --skills` (Microsoft's own playwright-cli skill;
  not vendored here because a shipped copy would register twice).

Roles you don't use don't need their tools installed; the routing simply won't
call them.

## Delegate agents

A delegate agent is a markdown agent definition the plugin registers with the
harness; the lead invokes it like a subagent, and it routes the work out. Two
ship here:

- `agents/model-delegate.md`: resolves a role via delegate-resolve, reads the
  provider's reference file, dispatches, and returns summary plus diff
- `agents/browser-delegate.md`: independent verification of rendered UI/UX
  work (and visual/browser fallback), driven by playwright-cli

Each reads `models.toml` (catalog) and `delegates.toml` (routing) from the
`multi-agent-delegation` skill to decide which backend and model to invoke.
Edit the agent files to change how the handoff works; edit `delegates.toml`
to change where work goes.

## Stop hooks

`run-loop` is Claude Code-only and becomes a no-op on Codex.
`delegate-nudge` runs on Claude Code and Codex and understands both transcript
formats. Neither runs on OpenCode or Antigravity. Treat them as backstops where
they fire, not as the reason the discipline holds. Both are fail-open; under
Claude Code they honor `stop_hook_active` so a blocked stop cannot loop. They
depend only on `jq`, `git`, `grep`, and `sed`.

`hooks/run-loop.sh` keeps an active autonomous run's loop turning. A controller
claims a run explicitly with `scripts/run-claim`; merely reading run files
does not claim ownership. When that session tries to stop while its run remains
active
(`.megapowers/run/<id>/status` STATE is initialized/working), it blocks once
and points at the next unmet milestone, the journal helper, and the verify
step. A run exits the loop by journaling a blocked, paused, or final result
entry and re-deriving the status (`run-derive-status` derives done when every
milestone is done, paused from a trailing paused entry). Hand-editing STATE is
not an exit: the next derive overwrites it, and `run-verify-status` fails any
done-claim the journal does not support. Runs at `in-the-loop` are exempt;
milestone checkpoints belong to the human there. The hook ignores runs the
current session never claimed.

Watch it work from a checkout, no session required (the hook is a plain
stdin-to-stdout script). Fake an active run and a Stop event; the output is
one JSON line, wrapped here for readability:

```console
$ mkdir -p .megapowers/run/site-migration
$ printf 'STATE=working\nCURSOR=M3: cut DNS over to the new host\nLEVEL=on-the-loop\n' \
    > .megapowers/run/site-migration/status
$ printf '{"type":"tool_use","name":"Bash","input":{"command":"scripts/run-claim site-migration"}}\n' > transcript.jsonl
$ printf '{"session_id":"demo-session","transcript_path":"transcript.jsonl","stop_hook_active":false}' \
    | plugins/mega-orchestration/hooks/run-loop.sh
{"decision":"block","reason":"Autonomous run site-migration is owned by this
session and active ... Continue the next unmet milestone ..."}
```

`hooks/delegate-nudge.sh` asks for an independent review of risky diffs. When
a Stop fires with auth, billing, or concurrency changes pending, it permits
completion only when `delegate-run` produced an approving receipt for the
current complete tree identity. Any tracked, staged, unstaged, or untracked
change makes the receipt stale. This raises the bar for accidental suppression;
the local receipt remains self-attested and is not a tamper-proof security
boundary.

## Install

```
/plugin install mega-orchestration@megapowers
```
