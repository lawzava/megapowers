# megapowers

Skills, plugins, and hooks that give a coding agent engineering discipline:
design before code, test first, verify before claiming done, delegate to the
best model per role, run long tasks unattended. For Claude Code, Codex,
OpenCode, and Google Antigravity; install only the parts you want. What sets
it apart: every claim of effect has a published, reproducible run behind it,
including the nulls ([`evals/RESULTS.md`](./evals/RESULTS.md)).

## Quickstart (Claude Code)

```
/plugin marketplace add lawzava/megapowers
/plugin install megapowers@megapowers
/plugin install mega-orchestration@megapowers
```

What you will see change:

- At session start, the agent receives one standing rule: before acting on any
  request, check whether a skill applies, and follow it if one does.
- On a matching task, the agent announces "Using [skill] to [purpose]" and
  follows that skill's checklist. Ask for "a function with unit tests" and it
  writes the failing test first, watches it fail, then implements.
- `/plugin` lists the installed plugins and their skills.

Install, update, uninstall, verification, and per-tool details:
[`docs/setup.md`](./docs/setup.md).

## How it works

No framework, no service, no API key. The mechanism:

- A skill is a markdown file: a one-line description plus a body of
  instructions. The descriptions sit in the agent's context permanently and
  act as triggers. The body loads only when the skill is invoked.
- A session-start hook injects the check-for-a-skill rule (the
  `using-megapowers` skill), so the agent looks for a matching skill on its
  own instead of waiting for you to name one.
- Hooks are deterministic backstops for what wording alone cannot guarantee.
  A Stop hook blocks the session from quietly stopping while an autonomous run
  is mid-flight. Another blocks finishing a risky diff (auth, billing,
  concurrency) without an independent review. A PreToolUse hook denies a short
  list of catastrophic shell commands. Hooks run on Claude Code only and fail
  open by absence elsewhere.
- Everything executable is plain bash reading stdin and writing stdout. You
  can run any hook by hand from a checkout.

## What's inside

| Plugin | What it gives you |
|---|---|
| `megapowers` | The workflow core: brainstorming, planning, TDD, systematic debugging, code review, worktrees, subagent orchestration, project memory. |
| `mega-orchestration` | Multi-model orchestration: route each task to the right structure and each role to the best model, best-of-N selection, cross-model verification, council adjudication, autonomous runs with an autonomy dial, an effect broker for irreversible actions. |
| `mega-go` | Greenfield Go: an opinionated stack picker plus idiomatic Go patterns. |
| `mega-python` | Greenfield Python: stack picker plus idiomatic patterns (typing, async, errors). |
| `mega-ts` | Greenfield TypeScript: stack picker plus idiomatic patterns (types, async, errors). |
| `mega-guardrails` | Claude Code safety hooks and dev tooling: block destructive commands, format-on-save, an optional Linux statusline. |

Which do I want?

- Daily engineering workflow (brainstorm, plan, TDD, review, merge): `megapowers`
- Multi-model delegation, verification, autonomous runs: `mega-orchestration`
- Safety hooks and statusline (Claude Code only): `mega-guardrails`
- Starting a new Go / Python / TypeScript project: `mega-go` / `mega-python` / `mega-ts`

Context cost: a full six-plugin install adds roughly 2,000 words (~2,600
tokens) of always-on context, the skill descriptions plus the session-start
note. The `megapowers` bundle alone is about half that. Skill bodies load only
when invoked.

Plugins are independent; the pairing that adds the most is `megapowers` plus
`mega-orchestration`. Nine skills are also published as standalone marketplace
entries for cherry-pickers (listed in [`docs/setup.md`](./docs/setup.md)).
Install a bundle or its standalone skill, never both: a skill installed twice
registers twice.

## Evidence

The eval harness and study protocols are committed in this repo; every
published number is reproducible from them. The two results that frame what
these skills buy:

- Disciplines the harness does not already enforce move behavior completely.
  Asked to add a function and its tests, agents wrote the failing test first
  in 0/36 control runs and 36/36 runs with the test-driven-development skill's
  wording in context, identically on frontier Claude, Claude Haiku, and
  GPT-5.5. Both arms completed the task, so the discipline cost nothing.
- Pattern advice frontier models have already internalized measures at zero.
  In the code-correctness study, all 184 generated programs passed in both
  arms, skill and control. The null is published, not hidden.

Those runs measure a skill's wording in context. Whether an agent reaches for
the right skill organically after install is measured separately by the
[trigger-recall study](./evals/studies/trigger-recall/): 100% recall with zero
false fires on the tuned task set; held-out and orchestration arms are
committed and await a keyed run.

Full tables, methods, and reproduce commands: [`evals/RESULTS.md`](./evals/RESULTS.md).
Prompts, fixtures, and oracles for the TDD result:
[`evals/studies/process-behavior/`](./evals/studies/process-behavior/).

## Install on Codex and other tools

- Codex installs the plugins from a local checkout
  (`codex plugin marketplace add ./`). `mega-guardrails` is not offered there;
  its hooks are Claude Code scripts.
- Every other harness (OpenCode, Antigravity, Cursor, Copilot, ...) installs
  the skills with the open [skills CLI](https://github.com/vercel-labs/skills):
  `npx skills add lawzava/megapowers`. Skills only; hooks and delegate agents
  do not travel that channel.

Exact steps, fleet sync across machines, and updating:
[`docs/setup.md`](./docs/setup.md).

## Relationship to Superpowers

megapowers began as a restyled fork of
[Superpowers](https://github.com/obra/superpowers) (MIT, © 2025 Jesse Vincent)
and keeps its process core. It adds the orchestration layer, portability to
Codex, OpenCode, and Antigravity, and the eval harness with published effect
sizes. If you want single-agent process discipline on Claude Code only,
upstream Superpowers serves that well. A head-to-head protocol (bare harness
vs megapowers vs upstream Superpowers, organic triggering, shared oracles) is
committed at [`evals/studies/head-to-head/`](./evals/studies/head-to-head/).
It has no published numbers yet; whatever a keyed run produces will be
published, including "upstream wins".

## Scope

- This is one maintainer's setup, shared because it is useful, and maintained:
  CI gates every change (structural validation plus the eval suite), plugins
  are versioned with a [changelog](./CHANGELOG.md), and behavioral changes are
  baseline-tested before they ship ([CONTRIBUTING.md](./CONTRIBUTING.md)).
- The opinions age. Stack picks, model IDs, and delegate routes are
  time-stamped in [`docs/tool-support.md`](./docs/tool-support.md); fork and
  adapt rather than tracking blindly.
- Nothing here is a security boundary; see [SECURITY.md](./SECURITY.md).
- This is not an Anthropic product, is not officially supported, and is not a
  stable API.

## Fork and adapt

The opinions are meant to be edited. The edit points:

- Routing and models: `plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml`
  maps each role to a delegate (channel, model, invocation). Change a model,
  point a role at a different backend, or enable Antigravity here.
- Stacks: the greenfield pickers live in `plugins/mega-go`,
  `plugins/mega-python`, and `plugins/mega-ts`. Swap frameworks, database,
  auth, or payments there.
- Settings: `templates/settings.example.json` holds safe, generic defaults.
  Copy the keys you want into your own `~/.claude/settings.json`.
- Tool support: `docs/tool-support.md` records what is native to each runtime
  and what is intentionally documented-only.

## Attribution and license

megapowers is [MIT-licensed](./LICENSE). It builds on
[Superpowers](https://github.com/obra/superpowers) (MIT, © 2025 Jesse Vincent)
and other upstream work; see [`ATTRIBUTION.md`](./ATTRIBUTION.md).
