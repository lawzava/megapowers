# megapowers

[![CI](https://github.com/lawzava/megapowers/actions/workflows/ci.yml/badge.svg)](https://github.com/lawzava/megapowers/actions/workflows/ci.yml)
[![Latest tag](https://img.shields.io/github/v/tag/lawzava/megapowers?label=release)](https://github.com/lawzava/megapowers/tags)
[![License](https://img.shields.io/github/license/lawzava/megapowers)](./LICENSE)

megapowers is a marketplace of seven optional plugins for coding agents. It
adds process skills, deterministic hook backstops, model routing, language
guidance, and frontend design guidance to Claude Code, Codex, OpenCode, and
Google Antigravity.

The repository publishes measured results, including null results, with the
protocols needed to reproduce them: [`evals/RESULTS.md`](./evals/RESULTS.md).

## Quickstart (Claude Code)

```text
/plugin marketplace add lawzava/megapowers
/plugin install megapowers@megapowers
/plugin install mega-orchestration@megapowers
```

Start a new session after installation. The agent will check for relevant
skills before acting, announce the skill it uses, and follow its checklist.
For example, a request for a function with unit tests triggers a failing test
before implementation.

Full install, update, verification, and uninstall instructions are in
[`docs/setup.md`](./docs/setup.md). To delegate installation to an agent, read
[`docs/agent-install.md`](./docs/agent-install.md), then give it this prompt:

> Install megapowers on this machine by fetching and following
> https://raw.githubusercontent.com/lawzava/megapowers/v0.3.5/docs/agent-install.md

The guide asks before changing permissions or settings.

## How it works

- Skills are `SKILL.md` files. Their descriptions are always visible to the
  agent; full bodies load only when selected.
- A SessionStart hook reminds the agent to check for a skill and injects the
  current model catalog.
- Stop and PreToolUse hooks backstop autonomous runs, independent review, and
  a small set of catastrophic shell commands. Claude Code receives the full
  hook set. Codex receives the compatible session catalog, review nudge, and
  destructive-command guard. OpenCode and Antigravity are skills-only.
- `models.toml` and `delegates.toml` route review, verification, browser work,
  and other roles to installed agent CLIs. megapowers adds no API key or hosted
  service.

Hooks and most helper scripts are Bash. The optional brainstorming visual
companion is a local Node server, and the eval scorer is Go. Each can be read
and run from a checkout.

## Plugins

| Plugin | What it provides |
|---|---|
| [`megapowers`](./plugins/megapowers/README.md) | Brainstorming, planning, TDD, debugging, review, worktrees, subagent development, verification, and project memory. |
| [`mega-orchestration`](./plugins/mega-orchestration/README.md) | Model routing, delegation, best-of-N, cross-model verification, councils, autonomous runs, and effect approval. |
| [`mega-go`](./plugins/mega-go/README.md) | Greenfield Go stack selection and idiomatic Go patterns. |
| [`mega-python`](./plugins/mega-python/README.md) | Greenfield Python stack selection and Python patterns. |
| [`mega-ts`](./plugins/mega-ts/README.md) | Greenfield TypeScript stack selection and TypeScript patterns. |
| [`mega-frontend`](./plugins/mega-frontend/README.md) | Frontend visual direction, typography, layout, and UX copy. |
| [`mega-guardrails`](./plugins/mega-guardrails/README.md) | Destructive-command protection for Codex and Claude Code, plus Claude Code formatting and an optional Linux statusline. |

Install only what you use. `megapowers` plus `mega-orchestration` is the main
workflow pairing. Add a language plugin for a new project, `mega-frontend` for
UI work, or `mega-guardrails` for hook backstops.

On Codex, installing all seven plugins exceeds the initial skills-list budget,
although each plugin fits by itself. Some skills then disappear from the
initial list until explicitly invoked. `scripts/validate.sh` reports the
current aggregate, so prefer an à-la-carte install.

## Other harnesses

Codex uses its native marketplace:

```text
codex plugin marketplace add lawzava/megapowers
codex plugin add megapowers@megapowers
```

Harnesses without a supported native marketplace use the open skills CLI:

```text
npx skills add lawzava/megapowers
```

The skills CLI installs skills only. Hooks and delegate agents do not travel
through that channel. Use one installation channel per harness to avoid
registering the same skill twice. See [`docs/setup.md`](./docs/setup.md) for
exact paths and mixed-harness caveats.

## Evidence

The committed studies currently show two useful boundaries:

- Test-first ordering changed from 0/36 control runs to 36/36 runs with the
  TDD skill in context across the tested Claude and GPT models. Both groups
  completed the implementation task.
- Single-shot code correctness did not improve. All 184 generated programs
  passed with and without the skill. That null result is published alongside
  the positive process result.

Trigger selection is measured separately. The tuned trigger set reached 100%
recall with no false fires; held-out and orchestration variants are committed
and await a keyed run.

Full methods, tables, prompts, and reproduction commands:
[`evals/RESULTS.md`](./evals/RESULTS.md).

## Limits and security

- This is not a security boundary. Hooks stop common accidents, not a hostile
  agent or malicious dependency. Read [`SECURITY.md`](./SECURITY.md) before
  installation.
- OpenCode and Antigravity receive no hook enforcement.
- Current frontier models already handle much single-shot coding correctly.
  The measured gains here concern process ordering and verification.
- Model IDs, stack picks, and harness capabilities age. Their review dates and
  support matrix live in [`docs/harness-support.md`](./docs/harness-support.md).

## Fork and adapt

The main edit points are:

- Model and role routing: `plugins/mega-orchestration/models.toml` and
  `plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml`.
- Stack choices: `plugins/mega-go`, `plugins/mega-python`, and
  `plugins/mega-ts`.
- Hook events: each hook-shipping plugin's `hooks/hooks.json`.
- Bundle publication: `.claude-plugin/marketplace.json`,
  `.agents/plugins/marketplace.json`, and each plugin manifest.
- Harness support: `docs/harness-support.md`.

`scripts/validate.sh` checks manifests, portable skill frontmatter, hook
wiring, shell scripts, security markers, documentation consistency, and the
deterministic eval scenarios.

## Relationship to Superpowers

megapowers began as a restyled fork of
[Superpowers](https://github.com/obra/superpowers). Superpowers supports more
harnesses and is a good choice for single-agent process discipline. megapowers
adds published effect sizes, cross-vendor orchestration, and deterministic hook
backstops. A shared-oracle head-to-head protocol is committed under
[`evals/studies/head-to-head/`](./evals/studies/head-to-head/) and has no
published run yet.

## Scope

This is one maintainer's opinionated setup, not an Anthropic or OpenAI product
and not a stable API. CI gates structural validation and deterministic evals;
behavioral changes should be measured before release. See
[`CONTRIBUTING.md`](./CONTRIBUTING.md) and [`CHANGELOG.md`](./CHANGELOG.md).

## Attribution and license

megapowers is [MIT-licensed](./LICENSE). It builds on
[Superpowers](https://github.com/obra/superpowers) (MIT, © 2025 Jesse Vincent)
and other upstream work listed in [`ATTRIBUTION.md`](./ATTRIBUTION.md).
