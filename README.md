# megapowers

Skills, plugins, and hooks that change how coding agents work: the agent picks
the right discipline itself (design before code, test first, verify before
claiming done), routes work to the model best suited for each role, and runs
long tasks unattended on a durable, auditable file contract. For Claude Code,
Codex, OpenCode, and Google Antigravity. À-la-carte: install only the parts
you want.

**Measured, not vibed.** With the test-driven-development skill's guidance in
context, agents wrote the failing test first in 12/12 runs versus 0/12
without it, with the identical effect on two vendors' frontier models.
Pattern advice frontier models have already internalized measures at zero,
and the catalog says so honestly. Every number is reproducible:
[`evals/RESULTS.md`](./evals/RESULTS.md).

## Quickstart (Claude Code)

```
/plugin marketplace add lawzava/megapowers
/plugin install megapowers@megapowers
/plugin install mega-orchestration@megapowers
```

Which plugin do I want?

- Daily engineering workflow (brainstorm → plan → TDD → review → merge): `megapowers`
- Multi-model delegation, verification, autonomous runs: `mega-orchestration`
- Safety hooks + statusline (Claude Code only): `mega-guardrails`
- Starting a new Go / Python / TypeScript project: `mega-go` / `mega-python` / `mega-ts`

Browse everything with `/plugin` → Discover. Install, update, uninstall, and
per-tool details: [`docs/setup.md`](./docs/setup.md).

**What it costs:** a full six-plugin install adds roughly 2,000 words
(~2,600 tokens) of always-on context — the skill descriptions plus the
megapowers session-start note. Skill bodies load only when a skill is invoked.
The `megapowers` bundle alone is about half that.

## See it work

Verbatim output, reproducible from a checkout (the hooks are plain
stdin-to-stdout scripts, so you can run them by hand).

The guardrail hook classifies instead of blanket-blocking — a catastrophic
delete is denied with a suggested alternative, while the scoped equivalent
passes untouched:

```console
$ echo '{"tool_input":{"command":"rm -rf /"}}' \
    | plugins/mega-guardrails/hooks/deny-destructive.sh
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "recursive rm of a root, home, or system directory. Delete a specific subdirectory instead (e.g. rm -rf ./dist)."
  }
}
$ echo '{"tool_input":{"command":"rm -rf ./dist"}}' \
    | plugins/mega-guardrails/hooks/deny-destructive.sh
$   # no output: allowed
```

The autonomous-run loop driver refuses to let a session quietly stop while a
run it touched is still active — the only sanctioned exits are honest journal
states. Fake an active run and a Stop event, and watch it block (output is
one line; abridged at `[...]`):

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
(scripts/run-journal), and re-derive status [...] If you are pausing
deliberately, or you are blocked, or a charter cap is reached, journal a
paused or blocked entry and re-derive status so STATE says so — then stopping
is correct. Verify a finished run with scripts/run-verify-status."}
```

## What the skills change

From the committed study protocol (task: "add `word_count(text)` to
`textkit.py` and add unit tests; make sure they pass"):

- **Without the skill's guidance:** 0 of 12 agents wrote a test before the
  implementation. Tests written after the fact pass immediately and prove
  nothing.
- **With it:** 12 of 12 wrote the failing test, watched it fail, then
  implemented — on frontier Claude, on Haiku, and on GPT-5.5 alike.

Prompts, fixtures, and the oracle are committed under
[`evals/studies/process-behavior/`](./evals/studies/process-behavior/); rerun
them yourself. Those runs measure the skill's wording in context; whether an
agent reaches for the right skill *organically* after install is measured
separately by the [trigger-recall study](./evals/studies/trigger-recall/)
(100% recall, zero false fires on the tuned task set; held-out and
orchestration arms are committed and await a keyed run).

## Install: Codex

Codex installs from a local checkout:

```
git clone https://github.com/lawzava/megapowers && cd megapowers
codex plugin marketplace add ./
codex
/plugins
```

Install `megapowers`, `mega-go`, `mega-python`, `mega-ts`, or
`mega-orchestration` from the repo marketplace. `mega-guardrails` remains
Claude-specific until its hooks are ported.

Every other harness (OpenCode, Antigravity, Cursor, Copilot, ...) installs
skills through the open [skills CLI](https://github.com/vercel-labs/skills):

```
npx skills add lawzava/megapowers
```

Skills only — hooks and delegate agents ship via the native marketplaces
above. Details, fleet sync across many devices, and updating:
[`docs/setup.md`](./docs/setup.md).

## What's inside

| Plugin | What it gives you |
|---|---|
| `megapowers` | Workflow methodology: brainstorming, planning, TDD, systematic debugging, code review, worktrees, subagent orchestration, project memory. |
| `mega-orchestration` | The orchestration layer: a decision-root skill that routes each task to the right structure, config-driven delegation to the best model per role, best-of-N selection, cross-model verification, council adjudication, autonomous runs with an autonomy dial, and an effect broker for irreversible actions. |
| `mega-go` | Greenfield Go: an opinionated stack picker + idiomatic Go patterns. |
| `mega-python` | Greenfield Python: stack picker + idiomatic patterns (typing, async, errors). |
| `mega-ts` | Greenfield TypeScript: stack picker + idiomatic patterns (types, async, errors). |
| `mega-guardrails` | Claude Code safety hooks + dev tooling: block destructive commands, format-on-save, a rich statusline (Linux, opt-in). |

Default to the bundles. Nine skills also exist as standalone marketplace
entries for cherry-pickers (listed in [`docs/setup.md`](./docs/setup.md));
install a bundle **or** its standalone skill, never both — a skill installed
twice registers twice.

## Relationship to Superpowers

megapowers began as a restyled fork of
[Superpowers](https://github.com/obra/superpowers) (MIT, © 2025 Jesse Vincent)
and keeps its process core. What it adds: the orchestration layer (delegation
routing, best-of-N, cross-model verification, autonomous runs), portability to
Codex/OpenCode/Antigravity, and an eval harness with published effect sizes —
including the honest nulls. If you want single-agent process discipline on
Claude Code only, upstream Superpowers serves that well. A committed
head-to-head protocol (bare harness vs megapowers vs upstream, organic
triggering, shared oracles) lives at
[`evals/studies/head-to-head/`](./evals/studies/head-to-head/); it has no
published numbers yet, and whatever a keyed run produces will be published,
including "upstream wins".

## What this is

- An opinionated setup, shared because it is useful — and maintained: every
  change is gated by CI (structural validation plus the eval suite), plugins
  are versioned with a [changelog](./CHANGELOG.md), and behavioral changes are
  baseline-tested before they ship (see [CONTRIBUTING.md](./CONTRIBUTING.md)).
- À-la-carte and fork-friendly: take one skill, one plugin, or none; every
  opinion has a named edit point (below).
- Honest about staleness: the stack picks, model IDs, and delegate routes are
  time-stamped in [`docs/tool-support.md`](./docs/tool-support.md) and will
  age. Fork and adapt rather than tracking blindly.
- This is not an Anthropic product, is not officially supported, and is not a
  stable API.

## Fork & adapt

The opinions are meant to be edited. The concrete edit points:

- **Routing and models** — `plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml`
  maps each role to a delegate (channel, model, invocation). Change a model,
  point a role at a different backend, or enable Antigravity here.
- **The stacks** — `plugins/mega-go`, `plugins/mega-python`, and
  `plugins/mega-ts` hold the greenfield stack pickers. Swap frameworks,
  database, auth, or payments there.
- **Settings** — `templates/settings.example.json` holds safe, generic defaults.
  Copy the keys you want into your own `~/.claude/settings.json`.
- **Tool support** — `docs/tool-support.md` records what is native to each
  runtime and what is intentionally documented-only.

## Attribution & license

megapowers is [MIT-licensed](./LICENSE). It builds on [Superpowers](https://github.com/obra/superpowers)
(MIT, © 2025 Jesse Vincent) and other upstream work — see [`ATTRIBUTION.md`](./ATTRIBUTION.md).
