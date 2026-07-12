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
| Plan / code review | codex (frontier tier) | Reviewing plans and diffs, adversarial "find the bug" passes |
| Small implementation | codex (frontier tier) | Well-specified, testable, single-file or isolated changes |
| Visual / browser | codex (frontier tier, native computer use) | UI work, browser-driven checks, end-to-end testing |
| Visual verification | `playwright-cli` + a vision-capable model | Independent cross-vendor pass on rendered UI/UX work |
| Visual / browser (alt) | Antigravity CLI | Disabled by default, see note below |

Shipped defaults; current model ids live in `delegates.toml` tier maps, and a
project `.megapowers/delegates.toml` or user `~/.config/megapowers/delegates.toml`
layer overrides them per key.

The visual/browser route is a cost-adjusted call, dated in the comment above
`[roles]` in `delegates.toml`; re-bench before moving it.

Antigravity is included as a documented alternative but is disabled until you
verify a local `agy` automation path, approval behavior, and artifact
workflow. Its current CLI exposes `/agents`, `/tasks`, and `/artifact`, but
this repo does not ship an `agy` wrapper.

To adjust the routing, edit an override layer of `delegates.toml` (or the
shipped file when changing project defaults). It declares the lead, the tier
scale, each provider's tier map and capabilities, and maps each role to a
provider. `scripts/delegate-resolve` resolves it executably; `--check`
validates it and CI runs that.

## Prerequisites

- Roles route per `[roles]`: native subagents when the lead already is that
  provider, otherwise the provider's CLI, SDK, or MCP channel (see its
  `references/providers/` file).
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

Watch it work from a checkout, no session required (the hook is a plain
stdin-to-stdout script). Fake an active run and a Stop event; the output is
one JSON line, wrapped here for readability:

```console
$ mkdir -p .megapowers/run/site-migration
$ printf 'STATE=working\nCURSOR=M3: cut DNS over to the new host\nLEVEL=on-the-loop\n' \
    > .megapowers/run/site-migration/status
$ echo 'read .megapowers/run/site-migration/plan.md' > transcript.jsonl
$ printf '{"transcript_path":"transcript.jsonl","stop_hook_active":false}' \
    | plugins/mega-orchestration/hooks/run-loop.sh
{"decision":"block","reason":"Autonomous run site-migration is active
(STATE=working, CURSOR=M3: cut DNS over to the new host) and its done-when
criteria are not recorded as met. Continue the loop per its runbook: do the
next unmet milestone, run its declared acceptance check, journal the result
(scripts/run-journal), and re-derive status (scripts/run-derive-status — it
derives done when every milestone is done). If you are pausing deliberately,
or you are blocked, or a charter cap is reached, journal a paused or blocked
entry and re-derive status so STATE says so — then stopping is correct.
Verify a finished run with scripts/run-verify-status."}
```

`hooks/delegate-nudge.sh` asks for an independent review of risky diffs. When
a Stop fires with auth, billing, or concurrency changes pending and the
transcript shows no real delegate invocation (matched against the `detect`
markers each provider declares in `delegates.toml`), it blocks once per diff
state with a nudge to run an independent pass.

## Install

```
/plugin install mega-orchestration@megapowers
```

The `multi-agent-delegation` skill is also published as a standalone
marketplace entry. Install the bundle or the standalone skill, not both: a
skill installed twice registers twice.
