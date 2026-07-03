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
- `multi-agent-delegation`: when and how the lead (the agent session you are
  talking to, which orchestrates and owns integration) hands work to a
  delegate (a separately invoked model or CLI that returns results), plus
  `scripts/delegate-resolve` to resolve a role to its provider executably.
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
The skills and the delegate agents read `delegates.toml` (inside the
`multi-agent-delegation` skill directory) for backend and model choices.

## Role routing

| Role | Default delegate | Used for |
| --- | --- | --- |
| Plan / code review | Codex (`gpt-5.5`) | Reviewing plans and diffs, adversarial "find the bug" passes |
| Small implementation | Codex (`gpt-5.5`) | Well-specified, testable, single-file or isolated changes |
| Visual / browser | `playwright-cli` + a vision-capable model | Screenshots, visual diffs, browser-driven checks |
| Visual / browser (alt) | Antigravity CLI | Disabled by default, see note below |

The visual/browser route drives the UI with `playwright-cli` (a standalone
CLI) and reasons over the screenshots with a vision-capable model: the lead
itself when it is vision-capable (e.g. Claude), otherwise any vision-capable
model. It replaced the retired Gemini-CLI route (see
[`docs/tool-support.md`](../../docs/tool-support.md)).

Antigravity is included as a documented alternative but is disabled until you
verify a local `agy` automation path, approval behavior, and artifact
workflow. Its current CLI exposes `/agents`, `/tasks`, and `/artifact`, but
this repo does not ship an `agy` wrapper.

To adjust the routing, edit `skills/multi-agent-delegation/delegates.toml`. It
maps each role to a delegate (channel, model, and how to invoke it). Change a
model, point a role at a different backend, or enable Antigravity there. No
code changes needed.

## Prerequisites

- Codex roles (plan/code review, small impl): Codex native subagents when
  running in Codex, or the Codex CLI/SDK from another harness.
- Visual/browser role: `playwright-cli` plus a vision-capable model to read
  the screenshots. Install: `npm i -g @playwright/cli`, then
  `playwright-cli install --skills` (Microsoft's own playwright-cli skill;
  not vendored here because a shipped copy would register twice).

Roles you don't use don't need their tools installed; the routing simply won't
call them.

## Delegate agents

A delegate agent is a markdown agent definition the plugin registers with the
harness; the lead invokes it like a subagent, and it routes the work out. Two
ship here:

- `agents/codex-delegate.md`: plan/code review and small, testable
  implementation, via Codex
- `agents/browser-delegate.md`: visual and browser work, driven by
  playwright-cli

Each reads `delegates.toml` from the `multi-agent-delegation` skill to decide
which backend and model to invoke. Edit the agent files to change how the
handoff works; edit `delegates.toml` to change where work goes.

## Stop hooks (Claude Code only)

Stop hooks are a Claude Code feature. On Codex, OpenCode, and Antigravity
these hooks do not run, and the disciplines ride on the skills' instructions
instead. Treat them as backstops where they fire, not as the reason the
discipline holds. Both are fail-open (any error or uncertainty allows the
stop), self-suppressing (they honor the `stop_hook_active` flag Claude Code
sets in the hook's stdin payload on re-entry, so a blocked stop cannot loop),
and depend only on `jq`, `git`, `grep`, and `sed`.

`hooks/run-loop.sh` keeps an active autonomous run's loop turning. When the
session tries to stop while a run it touched still reads active
(`.megapowers/run/<id>/status` STATE is initialized/working), it blocks once
and points at the next unmet milestone, the journal helper, and the verify
step. A run exits the loop by journaling a blocked, paused, or final result
entry and re-deriving the status (`run-derive-status` derives done when every
milestone is done, paused from a trailing paused entry). Hand-editing STATE is
not an exit: the next derive overwrites it, and `run-verify-status` fails any
done-claim the journal does not support. Runs at `in-the-loop` are exempt;
milestone checkpoints belong to the human there. The hook ignores runs the
current session never touched (it requires the run's path in the transcript,
not a bare name match).

`hooks/delegate-nudge.sh` asks for an independent review of risky diffs. When
the session's git diff touches risky logic (auth, OAuth, JWT, billing,
payments, Stripe, webhooks, concurrency) and the transcript shows no
independent review by a delegate (neither a Codex pass via `codex exec`, the
`mcp__codex__codex` channel, or a configured bridge, nor an Antigravity pass),
it blocks the stop and asks for an independent review before finishing.

## Install

```
/plugin install mega-orchestration@megapowers
```

The `multi-agent-delegation` skill is also published as a standalone
marketplace entry. Install the bundle or the standalone skill, not both: a
skill installed twice registers twice.
