# mega-orchestration

Config-driven multi-agent delegation. Route each kind of work to the model that
handles it best, without hard-coding a provider. The routing lives in a single
config file, so swapping a backend is an edit, not a rewrite.

## Contents

- The `orchestrating` skill — the decision root. At task arrival it routes the
  task's shape to the right structure (inline, parallel subagents, delegation,
  best-of-n, council, autonomous run) with spend-by-stakes effort defaults, and
  maps subagent/team/workflow/effort primitives per harness in its
  `references/harness-primitives.md`.
- The `multi-agent-delegation` skill, which teaches the lead when and how to
  hand work to a delegate, plus `scripts/delegate-resolve` to resolve a role to
  its provider executably.
- Three swarm-primitive skills built on that routing:
  - `best-of-n` — generate N independent candidates, select by an executable
    oracle first and a blind judge second (selection, never averaging).
  - `cross-model-verification` — verify risky work with a different-vendor model
    that tries to refute it, blind to the author's reasoning.
  - `council-adjudication` — for a decision with no oracle: answer independently,
    rank the answers anonymized, synthesize from the best (not a compromise).
- `autonomous-run` — run a long, largely-unattended task on a durable file
  contract (frozen charter / plan with acceptance criteria / runbook / append-only
  journal / machine-readable status) with an autonomy dial (autonomous /
  on-the-loop / in-the-loop) that gates by reversibility × blast radius — never
  gating reversible work — plus a legible, confidence-ranked run report.
- `effect-broker` — the portable trust layer for real-world side effects: classify
  an action (reversible / staged / irreversible) and enforce simulate-then-commit
  with idempotency and blast-radius caps. Irreversible actions never auto-fire, even
  at the `autonomous` level. This is the honest irreversibility gate (declaration-
  based), distinct from the Claude-only deny-destructive command tripwire.
- Two delegate agents (Codex and a vendor-neutral browser delegate) that wrap
  the routing.
- A `delegate-nudge` Stop hook that asks for an independent review of risky
  diffs, and a `run-loop` Stop hook that keeps an active autonomous run's loop
  turning instead of letting the session stop mid-run. **Claude Code only** —
  see the note below.

The skill and the delegate agents read `delegates.toml` (shipped inside the
`multi-agent-delegation` skill directory) for backend and model choices.

## Role routing

| Role | Default delegate | Used for |
| --- | --- | --- |
| Plan / code review | Codex (`gpt-5.5`) | Reviewing plans and diffs, adversarial "find the bug" passes |
| Small implementation | Codex (`gpt-5.5`) | Well-specified, testable, single-file or isolated changes |
| Visual / browser | `playwright-cli` + a vision-capable model | Screenshots, visual diffs, browser-driven checks |
| Visual / browser (alt) | Antigravity CLI | Disabled by default, see note below |

The visual/browser route drives the UI with `playwright-cli` (a standalone CLI)
and reasons over the screenshots with a vision-capable model — the lead itself
when it is vision-capable (e.g. Claude), otherwise any vision-capable model. It
replaces the retired Gemini-CLI route (the Gemini CLI was discontinued for
consumer use in mid-2026).

Antigravity is included as a documented alternative but is **disabled** until
you verify a local `agy` automation path, approval behavior, and artifact
workflow. Its current CLI exposes `/agents`, `/tasks`, and `/artifact`, but this
repo does not ship an `agy` wrapper.

## Adjusting the routing

Edit `skills/multi-agent-delegation/delegates.toml`. It maps each role to a
delegate (channel, model, and how to invoke it). Change a model, point a role at
a different backend, or enable Antigravity there — no code changes needed.

## Prerequisites

- **Codex roles** (plan/code review, small impl): Codex native subagents when
  running in Codex, or the Codex CLI/SDK from another runtime.
- **Visual/browser role**: `playwright-cli` plus a vision-capable model to read
  the screenshots.

Roles you don't use don't need their tools installed; the routing simply won't
call them.

## Delegate agents

Two agents wrap the routing so the lead can hand off cleanly:

- a **Codex delegate** for plan/code review and small, testable implementation, and
- a **browser delegate** for visual and browser work (playwright-cli driven).

Each reads `delegates.toml` from the `multi-agent-delegation` skill to decide
which backend and model to invoke.

## run-loop Stop hook (Claude Code only)

`hooks/run-loop.sh` runs on Stop. When the session tries to stop while an
autonomous run it touched still reads active (`.megapowers/run/<id>/status`
STATE is initialized/working), it blocks once and points at the next unmet
milestone, the journal helper, and the verify step. A run exits the loop
honestly: journal a blocked, paused, or final result entry and re-derive the
status (`run-derive-status` derives done when every milestone is done, paused
from a trailing paused entry). Hand-editing STATE is not an exit: the next
derive overwrites it, and `run-verify-status` fails any done-claim the journal
does not support. Runs at `in-the-loop` are exempt — milestone checkpoints
belong to the human there, and the hook never blocks them.

Same portability caveat as delegate-nudge: Claude Code only, fails open by
absence elsewhere; on other harnesses the loop rides on the autonomous-run
runbook discipline. It is self-suppressing (`stop_hook_active`), ignores runs
the current session never touched (it requires the run's path in the
transcript, not a bare name match), and depends only on `jq`, `grep`, and `sed`.

## delegate-nudge Stop hook (Claude Code only)

`hooks/delegate-nudge.sh` runs on Stop **in Claude Code**. When the session's
git diff touches risky logic (auth, OAuth, JWT, billing, payments, Stripe,
webhooks, concurrency) and the transcript shows **no** independent review by a
delegate — neither a Codex pass (`codex exec`, the `mcp__codex__codex` channel,
or a configured bridge) nor an Antigravity pass — it blocks the stop and asks
for an independent review before finishing.

**This hook is a Claude-Code-only accelerator, not a portable guarantee.** Stop
hooks are a Claude Code feature; on Codex, OpenCode, and Antigravity the hook
simply does not run (it fails open by absence), so the "did a delegate review
this?" discipline there rides on the `multi-agent-delegation` skill's
instructions, not on enforcement. Treat the nudge as a backstop where it fires,
not as the reason the discipline holds.

It is deliberately conservative: fail-open (any error or uncertainty allows the
stop), self-suppressing (`stop_hook_active` and the delegate-usage check prevent
loops), and dependent only on `jq`, `git`, and `grep`.

## Install

```
/plugin install mega-orchestration@megapowers
```

### Standalone skills

The `multi-agent-delegation` skill is also published as a standalone marketplace
entry. Install the bundle **or** the standalone skill, not both — a skill
installed twice registers twice.
