# megapowers

[![CI](https://github.com/lawzava/megapowers/actions/workflows/ci.yml/badge.svg)](https://github.com/lawzava/megapowers/actions/workflows/ci.yml)
[![Latest tag](https://img.shields.io/github/v/tag/lawzava/megapowers?label=release)](https://github.com/lawzava/megapowers/tags)
[![License](https://img.shields.io/github/license/lawzava/megapowers)](./LICENSE)

megapowers is a plugin marketplace for coding agents: seven installable plugins
of skills, hooks, and delegate agents. Together they give the agent
engineering discipline: design before code, test first, verify before
claiming done, delegate to the best model per role, run long tasks
unattended. Works on Claude Code, Codex, OpenCode, and Google Antigravity;
install only the parts you want.

Every claim of effect has a committed, re-runnable protocol behind it, including
the null results (runs where a skill measurably bought nothing):
[`evals/RESULTS.md`](./evals/RESULTS.md).

## Quickstart (Claude Code)

```
/plugin marketplace add lawzava/megapowers
/plugin install megapowers@megapowers
/plugin install mega-orchestration@megapowers
```

The format is `plugin@marketplace`; the marketplace and its core plugin share
the name `megapowers`.

What you will see change, starting with your next session:

- At session start, the agent receives one standing rule: before acting on any
  request, check whether a skill applies, and follow it if one does.
- On a matching task, the agent announces "Using [skill] to [purpose]" and
  follows that skill's checklist. Ask for "a function with unit tests" and it
  writes the failing test first, watches it fail, then implements.
- `/plugin` lists the installed plugins and their skills.

Install, update, uninstall, verification, and details for each harness (the
program the agent runs in: Claude Code, Codex, an IDE agent):
[`docs/setup.md`](./docs/setup.md).

Or hand the install to the agent itself. Read
[`docs/agent-install.md`](./docs/agent-install.md) yourself first, it is short,
then paste this into any coding agent, on any harness:

> Install megapowers on this machine by fetching and following
> https://raw.githubusercontent.com/lawzava/megapowers/v0.3.0/docs/agent-install.md

The guide has the agent detect its harness, install through the right
channel, avoid registering any skill twice, verify by asking for a sentence
that exists only inside an installed skill, and report what it did.
Anything that would widen permissions or edit your settings requires your
explicit approval.

## How to use it

There is no command surface to learn. Describe the work in plain language; the
session-start rule makes the agent check for a matching skill and follow it.
Naming a skill explicitly also works ("use the brainstorming skill"), but the
trigger tuning exists so you do not have to.

The core loop, idea to merged code (all in the `megapowers` plugin):

1. "I want to add <feature>": `brainstorming` pins down intent, requirements,
   and approach before any code.
2. "Write the plan": `writing-plans` turns the agreed design into steps.
3. "Implement it": `test-driven-development` writes the failing test first,
   watches it fail, then implements.
4. "Is it done?": `verification-before-completion` runs the checks and shows
   the output; no success claim without evidence.
5. "Ship it": `requesting-code-review`, then `finishing-a-development-branch`
   (merge, PR, keep, or discard).

Each skill has its own entry conditions, so you can join the loop anywhere; a
one-line fix does not have to start at step 1.

The five most common uses:

1. Build a feature end to end: the loop above, from one sentence of intent.
2. Fix a bug: "this test is failing" triggers `systematic-debugging`, which
   finds the root cause before any fix, then hands off to
   `test-driven-development` for a regression test.
3. Get risky code independently reviewed: for auth, billing, or concurrency
   changes, `cross-model-verification` has a different vendor's model try to
   refute the work (`mega-orchestration`).
4. Run a long task unattended: `autonomous-run` keeps a charter, plan,
   journal, and machine-readable status the run can resume from across
   sessions (`mega-orchestration`).
5. Start a new project: the greenfield stack pickers give an opinionated,
   time-stamped stack for Go, Python, or TypeScript (`mega-go`,
   `mega-python`, `mega-ts`).

## How it works

No framework, no service, no API key of its own. The mechanism:

- A skill is a markdown file: a one-line description plus a body of
  instructions. The descriptions sit in the agent's context permanently and
  act as triggers. The body loads only when the skill is invoked.
- A session-start hook injects the check-for-a-skill rule (the
  `using-megapowers` skill), so the agent looks for a matching skill on its
  own instead of waiting for you to name one.
- Hooks are deterministic backstops for what wording alone cannot guarantee.
  A Stop hook interrupts a quiet stop once while an autonomous run is mid-flight.
  Another interrupts finishing a risky diff (auth, billing, concurrency) once,
  asking for an independent review. A PreToolUse hook denies a short list of
  catastrophic shell commands. These hook scripts ship for Claude Code only
  today; other harnesses have hook surfaces, but a default install wires no
  ports there (a manual Codex pilot of the destructive-command guard exists, see
  docs/setup.md), so out of the box none of these backstops apply there and the
  discipline rides on the skill wording alone.
- Everything executable is plain bash reading stdin and writing stdout. You
  can run any hook by hand from a checkout.

The multi-model features drive agent CLIs you already have installed and
authenticated; for example, the Codex CLI serves the roles (kinds of
delegated work, like code review) routed to GPT-5.6 Sol. megapowers adds no key
or service; roles whose tools you lack simply sit unused.

## Vocabulary

These docs use a few terms consistently, defined here once.

- **Harness**: the program the agent runs in (Claude Code, Codex, OpenCode,
  Antigravity). "Eval harness" is a different thing: the test runner under
  [`evals/`](./evals/README.md).
- **Marketplace**: this repo, a catalog a harness installs plugins from.
- **Channel**: the path an install travels: a harness's native plugin
  marketplace, or the skills CLI for everything else. One channel per harness
  per machine; two channels register every skill twice.
- **Plugin** (or bundle): one installable unit, a directory shipping skills
  plus, where applicable, hooks and delegate agents.
- **Standalone entry**: a marketplace entry that republishes a single skill
  out of a bundle, for cherry-pickers.
- **Skill**: one `SKILL.md` file, description plus instruction body, loaded as
  described above.
- **Hook**: a script the harness itself runs at a fixed event (session start,
  before a tool call, on stop). Deterministic: it fires whether or not the
  model remembers. Claude Code only.
- **Delegate agent**: an agent definition (a markdown file under a plugin's
  `agents/`) that hands one role (a kind of delegated work, such as code
  review or browser testing) to another model or CLI. The lead (the session
  you are talking to) dispatches it and owns integration.
- **Eval scenario**: a deterministic pass/fail check under `evals/scenarios/`,
  run in CI with no model involved.
- **Study**: a protocol under `evals/studies/` that runs real agents and
  produces the numbers in [`evals/RESULTS.md`](./evals/RESULTS.md).
- **Keyed run**: a run that needs real model credentials and API spend. CI has
  none, so it runs only the deterministic scenarios and a mock agent; "awaits
  a keyed run" marks a protocol whose numbers do not exist yet.
- **Oracle**: the script that decides pass or fail from artifacts (files, git
  state, transcripts), never from the agent's self-report.

## What's inside

Before you install, skim the trust boundaries and what the hooks can and cannot
do: [SECURITY.md](./SECURITY.md).

| Plugin | What it gives you |
|---|---|
| [`megapowers`](./plugins/megapowers/README.md) | The workflow core: brainstorming, planning, TDD, systematic debugging, code review, worktrees, subagent orchestration, project memory. |
| [`mega-orchestration`](./plugins/mega-orchestration/README.md) | Multi-model orchestration: route each task to the right structure, delegate each role to the best model, best-of-N candidate selection, cross-model verification, council adjudication, autonomous runs, and the effect broker (gate irreversible actions). |
| [`mega-go`](./plugins/mega-go/README.md) | Greenfield Go: an opinionated stack picker plus idiomatic Go patterns. |
| [`mega-python`](./plugins/mega-python/README.md) | Greenfield Python: stack picker plus idiomatic patterns (typing, async, errors). |
| [`mega-ts`](./plugins/mega-ts/README.md) | Greenfield TypeScript: stack picker plus idiomatic patterns (types, async, errors). |
| [`mega-guardrails`](./plugins/mega-guardrails/README.md) | Claude Code safety hooks and dev tooling: block destructive commands, format-on-save, an optional Linux statusline. |
| [`mega-frontend`](./plugins/mega-frontend/README.md) | Frontend design guidance: distinctive visual direction, typography, layout, and UX copy that don't read as templated defaults. |

Which do I want?

- Daily engineering workflow (brainstorm, plan, TDD, review, merge): `megapowers`
- Multi-model delegation, verification, autonomous runs: `mega-orchestration`
- Safety hooks and statusline (Claude Code only): `mega-guardrails`
- Starting a new Go / Python / TypeScript project: `mega-go` / `mega-python` / `mega-ts`
- Building or reshaping UI with a design bar: `mega-frontend`

Context cost: a full seven-plugin install adds about 1,938 words of always-on
context (the 30 skill descriptions summed with `wc -w`, 1,590 words, plus the
injected session-start note, 348 words); at ~1.3 tokens per word, about 2,520
tokens. The `megapowers` bundle alone is ~1,080 words (729 in its descriptions
plus the same note), ~1,400 tokens. Skill bodies load only when invoked.

Plugins are independent; the pairing that adds the most is `megapowers` plus
`mega-orchestration`. Nine skills are also published as standalone entries
(listed in [`docs/setup.md`](./docs/setup.md)). Install a bundle or its
standalone skill, not both: a skill installed twice registers twice.

## Evidence

The eval harness and study protocols are committed in this repo; every
published number is reproducible from them. The two results that frame what
these skills buy:

- Where the harness does not already enforce a discipline, a skill's wording
  has moved behavior completely on every model tested. Asked to add a
  function and its tests, agents
  wrote the failing test first in 0/36 control runs and 36/36 runs with the
  test-driven-development skill's wording in context, identically on frontier
  Claude (`claude-fable-5` in the published runs), Claude Haiku, and GPT-5.5.
  Both arms completed the task, so the discipline cost nothing.
- Where frontier models have already internalized the pattern, a skill
  measures at zero. In the code-correctness study, all 184 generated programs
  passed with and without the skill in context. The null is published, not
  hidden. An independent benchmark, SkillsBench (arXiv 2602.12670), reports the
  same shape: software-engineering shows the smallest skill gain of any domain
  (+4.5pp), because frontier models already cover this ground from pretraining.

Those runs measure a skill's wording placed in context. Whether an installed
agent reaches for the right skill on its own is measured separately by the
[trigger-recall study](./evals/studies/trigger-recall/): 100% recall with zero
false fires on the tuned task set; held-out and orchestration variants are
committed and await a keyed run.

Full tables, methods, and reproduce commands: [`evals/RESULTS.md`](./evals/RESULTS.md).
Prompts, fixtures, and oracles for the TDD result:
[`evals/studies/process-behavior/`](./evals/studies/process-behavior/).

## What it does not do

- Enforce anything outside Claude Code. On Codex, OpenCode, and Antigravity a
  default install blocks or gates nothing; the skills are advisory wording
  there (see the hooks note under "How it works").
- Act as a security boundary. The destructive-command tripwire stops
  accidents, not anyone trying; see [SECURITY.md](./SECURITY.md).
- Improve single-shot code correctness on current frontier models. That
  effect measured zero (the 184/184 null above); the measured wins are
  process disciplines the harness does not enforce, like test-first ordering.
- Guarantee the agent picks the right skill. Trigger recall is measured and
  tuned through the [trigger-recall study](./evals/studies/trigger-recall/),
  not guaranteed.

## Install on Codex and other harnesses

- Codex adds this repo as a remote marketplace
  (`codex plugin marketplace add lawzava/megapowers`), then
  `codex plugin add <plugin>@megapowers` per plugin. `mega-guardrails` is not
  offered there; its hook scripts are Claude Code scripts (the manual Codex
  pilot of the destructive-command guard is covered in docs/setup.md).
- Every other harness (OpenCode, Antigravity, Cursor, Copilot, ...) installs
  the skills with the open [skills CLI](https://github.com/vercel-labs/skills):
  `npx skills add lawzava/megapowers`. Skills only; hooks and delegate agents
  do not travel that channel.

Exact steps, fleet sync across machines, and updating:
[`docs/setup.md`](./docs/setup.md).

## Where it fits

Zero-infrastructure by design: markdown and bash, no runtime, service, or vendor
SDK of its own, so it is auditable in an afternoon with nothing to lock into (the
opposite pole from meta-harness runtimes that ship a heavy agent framework). The
distinctive pieces: an effect broker that gates irreversible actions behind
approval, best-of-N candidates judged blind by an executable oracle before any
model opinion, one auditable file
([`delegates.toml`](./plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml))
routing each role to a model, and executable certification of a done-claim over
the agent's self-report.

## Relationship to Superpowers

megapowers began as a restyled fork of
[Superpowers](https://github.com/obra/superpowers) (MIT, © 2025 Jesse Vincent)
and keeps its process core. Upstream has moved on: it now installs on roughly ten
harnesses (Claude Code, Codex, Cursor, OpenCode, Antigravity, and more), carries
a large following, and runs its own eval lab (Quorum) that drives real agent
CLIs. So portability and "has an eval harness" no longer separate the two. What
still does: megapowers publishes effect sizes including the nulls, each with a
committed, re-runnable protocol (the 36/36 test-first flip, the 184/184 null;
Quorum publishes no numbers), and it adds the cross-vendor orchestration and
deterministic hook backstops named above on top of the shared single-agent core.

If you want single-agent process discipline, upstream Superpowers serves that
well, across more harnesses than megapowers targets. A head-to-head protocol
(bare harness vs megapowers vs upstream Superpowers, organic triggering, shared
oracles) is committed at
[`evals/studies/head-to-head/`](./evals/studies/head-to-head/). It has no
published numbers yet; whatever a keyed run produces will be published, including
"upstream wins".

## Scope

- This is one maintainer's setup, shared and maintained:
  CI gates every change (structural validation plus the eval suite), plugins
  are versioned with a [changelog](./CHANGELOG.md), and behavioral changes are
  baseline-tested before they ship ([CONTRIBUTING.md](./CONTRIBUTING.md)).
- The opinions age. Stack picks, model IDs, and delegate routes are
  time-stamped in [`docs/harness-support.md`](./docs/harness-support.md); fork and
  adapt rather than tracking blindly.
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
- Hooks: each hook-shipping plugin's `hooks/hooks.json` (megapowers,
  mega-orchestration, mega-guardrails) maps harness events to the scripts in
  the same directory. Edit or remove entries there.
- Marketplace entries: `.claude-plugin/marketplace.json` lists every bundle
  and standalone entry. A standalone entry reuses a bundle's `source` with
  `"strict": false` plus a `skills` list naming the one skill it exposes;
  copy that pattern to publish a single skill from your fork.
  `scripts/validate.sh` checks the wiring.
- Settings: `templates/settings.example.json` holds safe, generic defaults.
  Copy the keys you want into your own `~/.claude/settings.json`.
- Tool support: `docs/harness-support.md` records what is native to each harness
  and what is intentionally documented-only.

## Attribution and license

megapowers is [MIT-licensed](./LICENSE). It builds on
[Superpowers](https://github.com/obra/superpowers) (MIT, © 2025 Jesse Vincent)
and other upstream work; see [`ATTRIBUTION.md`](./ATTRIBUTION.md).
