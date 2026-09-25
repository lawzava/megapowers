# megapowers

[![CI](https://github.com/lawzava/megapowers/actions/workflows/ci.yml/badge.svg)](https://github.com/lawzava/megapowers/actions/workflows/ci.yml)
[![Latest tag](https://img.shields.io/github/v/tag/lawzava/megapowers?label=release)](https://github.com/lawzava/megapowers/tags)
[![License](https://img.shields.io/github/license/lawzava/megapowers)](./LICENSE)

megapowers is exactly one plugin for Claude Code and Codex: 16 task skills,
one concise output style, a destructive-command tripwire, and a doctor that
tells you why something did not fire.

It is native-first. It uses each harness's own agents, goals, permissions,
worktrees, and memory. It ships no scheduler, model router, daemon, formatter,
or status line. The always-loaded cost is about 1.5k tokens per session on
each harness; skill bodies load only when selected.

## Install in 30 seconds

Claude Code:

```bash
claude plugin marketplace add lawzava/megapowers@release
claude plugin install megapowers@megapowers
```

Codex:

```bash
codex plugin marketplace add lawzava/megapowers --ref release
codex plugin add megapowers@megapowers
```

Both registrations track the `release` branch, which only fast-forwards to
signed release tags, so marketplace refreshes never install unreleased `main`.
Start a fresh session after installing. Hooks need Go 1.25 or newer on `PATH`;
they compile once into a local cache. Codex asks you to review and trust the
plugin hooks before it runs them, and asks again when a release changes them,
because trust is recorded against the hook's hash. Pinning, scopes, and
uninstall are in [docs/install.md](./docs/install.md).

## What you get

Every skill has a short trigger description that stays visible; the full text
loads only when the skill is selected. Repository instructions and project
tools stay authoritative; skills fill gaps.

| Skill | Fires when | What it does |
|---|---|---|
| `orchestrating` | Two or more independent lanes, an output-only lane, or a handoff needs coordination | Routes work to native agents with disjoint ownership and a bounded return contract |
| `design-and-plan` | Non-trivial behavior or an interface needs a spec, tradeoffs, or a multi-step plan | Writes requirements, resolves tradeoffs, and produces an executable plan |
| `grill-me` | You ask to be grilled or want a plan or idea stress-tested | Runs a round-based interview before any action |
| `test-first-implementation` | Adding or changing behavior, fixing a confirmed bug, refactoring | Red, green, refactor with verified failing and passing runs |
| `systematic-debugging` | A bug, flaky test, incident, or regression has an unknown cause | Finds the root cause before editing |
| `verify-and-finish` | Before claiming done, committing, merging, publishing, deploying, or handing off | Re-runs the real oracle and reports honest status |
| `safe-effects` | Before a deploy, message, charge, migration, destructive query, DNS change, or external write | Confirms authority for the exact target and reads the result back |
| `autonomous-run` | An approved goal must continue unattended across steps, resets, or sessions | Keeps state honest across long or interrupted work; experimental |
| `independent-review` | Security, auth, billing, concurrency, or data-integrity work needs a second opinion | Sends one explicit artifact to a different provider after a disclosure step |
| `evidence-research` | A decision needs evidence beyond the repository | Frames the question and classifies sources before concluding |
| `humanizing-prose` | The task is to draft, rewrite, or edit human-facing prose | Removes machine-prose markers without dropping or inventing facts |
| `writing-agent-instructions` | Creating or revising a skill, `AGENTS.md`, or `CLAUDE.md` | Trigger design, progressive disclosure, and behavioral validation |
| `mcp-setup` | Installing, configuring, or repairing an MCP server, or its tools are missing | Diagnoses the configured server and discovery path |
| `memory-hygiene` | You ask to audit, prune, or fix harness memory (explicit invocation only) | Validates provenance and dates with a Go tool, then applies one approved cleanup |
| `upgrading-megapowers` | You ask to update, reinstall, or repair the plugin | Inventories the install and asks once for the exact writes |
| `megapowers-doctor` | Something did not fire, the style looks off, or a hook errored | Runs a deterministic Go check and explains the result |

Skill maturity is recorded in
[`plugins/megapowers/skills/catalog.json`](./plugins/megapowers/skills/catalog.json).
Task-shape routing is in [docs/orchestration.md](./docs/orchestration.md).

## Turn on the style

The style keeps the built-in coding instructions. It asks for the answer
first, 100 prose words by default and 250 unless you ask for depth, one short
line of intent before long work, brief progress lines during long tool chains,
and no em dashes.

Claude Code: the style is optional and is off until you select it. Choose the
plugin-qualified value `megapowers:Megapowers` in `/config` under Output style,
or run `/output-style megapowers:Megapowers`. The resulting setting is
`"outputStyle": "megapowers:Megapowers"`. The bare value `Megapowers` does not
resolve to the plugin style; transcripts of sessions configured that way show
no style attached. `megapowers-doctor` reports the value in effect.

Codex: the trusted Codex startup hook (`SessionStart`) adds the same text as
developer context on startup, resume, clear, and compaction. The Codex hooks reference
(retrieved 2026-09-25) states that `SessionStart` hooks matching
`source: "compact"` run before the next model request; the plugin's hook has no
source matcher, so it matches every source. Set `MEGAPOWERS_OUTPUT_STYLE=off`
before launching Codex to omit the style while keeping the guard.

On both harnesses the style applies to the main conversation. Subagents get the
shorter report contract described below.

## What installs

- 16 skills under `skills/`, listed in `skills/catalog.json`.
- One output style, `output-styles/megapowers.md`, shared by both harnesses.
- Go hooks on three events, compiled once into a local cache:
  - `SessionStart`: a reminder to load a matching skill before acting, plus the
    style on Codex.
  - `SubagentStart`: the same reminder and a compact report contract for
    subagents: lead with the result, cite `file:line` or command evidence, no
    padding, no em dashes.
  - `PreToolUse` on Bash and PowerShell: denies a narrow set of catastrophic
    commands, and adds a non-blocking reminder to load `verify-and-finish`
    before `git commit`, `git push`, and `gh pr create`, or `safe-effects`
    before publish and deploy commands. The reminder never blocks.
- Two Go standard-library tools that load only with their skill: the
  memory-audit validator for `memory-hygiene` and the review packager for
  `independent-review`.
- A `doctor` command behind `megapowers-doctor`.

Neither adapter edits global user configuration. The plugin supplies no
model, provider, or agent configuration. For non-trivial delegation,
`orchestrating` can read an optional personal registry at
`~/.config/megapowers/agent-capabilities.md`; it is advisory and grants no
access.

## Footprint

Per session, outside this repository, the style, startup hook text, and skill
catalog cost about 1,470 tokens on Claude Code and about 1,510 on Codex
(word-count estimate from installed session text, 2026-09-25). Each skill body
is 274 to 468 words and loads only when triggered. The style shortens replies
(in one operator's September 2026 Codex sessions, styled final messages had a
median of 70 words and em dashes in 0.6 percent, against 36 percent without
the style), but it is not a compression feature and makes no cost claim.

## Evidence

Numbers below are from [evals/RESULTS.md](./evals/RESULTS.md), an
installed-plugin A/B study completed 2026-09-05: 1,080 valid trials over 27
cases, ten balanced control/treatment pairs per case per harness. Codex ran
`gpt-6-astra` on CLI `0.153.3`; Claude ran `claude-fable-5-1` on Claude Code
`2.1.258`; both at high effort. The control had no plugin.

| Development checks (18 cases) | Codex control | Codex + megapowers | Claude control | Claude + megapowers |
|---|---:|---:|---:|---:|
| Task outcome | 85/180 | 106/180 | 89/180 | 101/180 |
| Workflow | 163/180 | 170/180 | 164/180 | 169/180 |
| Required activation profile | n/a | 77/150 | n/a | 83/150 |

| Held-out checks (6 cases, frozen before the run) | Codex control | Codex + megapowers | Claude control | Claude + megapowers |
|---|---:|---:|---:|---:|
| Task outcome | 50/60 | 60/60 | 52/60 | 53/60 |

The instruction-authoring case moved from 0/10 to 10/10 on Codex and 0/10 to
9/10 on Claude. Codex's pending-review case moved from 0/10 to 10/10. The
separate autonomy/continuity diagnostics did not pass (Codex 0/30 to 0/30,
Claude 1/30 to 0/30). Activation is scored apart from outcome: all ten Claude
pending-review replies passed task checks but omitted `verify-and-finish`.

Caveats, stated in the same file: the study precedes the `0.29.0` version
stamp, neither harness passes the full benchmark, and the results do not
establish general model superiority. The 2026-09-02 Codex trigger-recall run
reported 0/117 implicit recall despite a rendered catalog
([trigger-recall README](./evals/studies/trigger-recall/README.md)); Codex
trigger gates stay report-only. `scripts/validate.sh` and `evals/run-all.sh`
prove repository mechanics, not agent quality. Protocols are in
[docs/advanced/evals.md](./docs/advanced/evals.md).

## How it compares

Facts below were checked on 2026-09-25. Install counts on skills.sh are
self-reported by that site.

| | megapowers | [obra/superpowers](https://github.com/obra/superpowers) | [mattpocock/skills](https://github.com/mattpocock/skills) |
|---|---|---|---|
| Harnesses | Claude Code and Codex, one plugin, shared hook code | 16 listed, including Claude Code, Codex, Cursor, Gemini CLI, and Copilot CLI | Any skills-compatible client through `npx skills add` |
| Distribution | This repository's marketplace only | Listed in the official Claude Code marketplace and, per its README, the official Codex marketplace | skills.sh, 4.2M installs across the collection |
| Shape | 16 skills, one style, three hooks, a doctor | 14 skills including brainstorming, writing-plans, executing-plans, subagent-driven-development, TDD, and `diagnosing-superpowers` | Single-purpose skills: `code-review`, `tdd`, `triage`, `diagnosing-bugs`, `handoff`, `writing-for-agents`, `grill-me` (1.2M installs) |
| Published measurements | The A/B table above, with SHA-pinned artifacts | Per-change probe counts in release notes (for example, TDD control 8/10 against treatment 5/10 when a section was cut) and a quorum eval lab | None found in the repository on 2026-09-25 |

What superpowers does better: official marketplace listings on both sides,
a much longer harness list, release notes that quote probe counts per change,
a diagnostics skill since v6.4.1 (2026-09-19), and commercial support. What
mattpocock/skills does better: frictionless distribution through skills.sh,
and small single-purpose skills that are easy to adopt one at a time;
`grill-me` there is a user-invoked pointer skill, which matches the Claude
Code docs' advice for skills that over-trigger.

What megapowers does differently: both harnesses from one plugin with the
same Go hook code; calm trigger descriptions instead of emphatic "you must"
language, which Anthropic's current prompting guidance and OpenAI's
2026-09-11 skills post both warn causes over-triggering; non-blocking
reminders that load the verification and side-effect skills at the moment a
commit, push, or deploy runs; a cross-provider `independent-review` path with
a disclosure step; `safe-effects` for outward effects; a memory validator;
and a doctor.

## Troubleshooting

Ask for `megapowers-doctor` first: `/megapowers:megapowers-doctor` on Claude
Code, or the skill name after `$` as it appears in the Codex skills list. It
runs `run-hook.cmd doctor` and reports the plugin version and root, the
harness, the Go toolchain version against the cached runner, the Claude
`outputStyle` value in effect, whether the hooks are registered, and how to
see whether a skill fired in your transcripts. Then:

- Style not applied on Claude Code: the setting must read
  `megapowers:Megapowers`, not `Megapowers`.
- Hooks silent on Codex: trust the plugin hooks when prompted; a release that
  changed them prompts again.
- Hook error mentioning Go: install Go 1.25 or newer, or clear the cached
  runner the doctor names.
- Wrong plugin version after an upgrade: use `upgrading-megapowers`, which
  compares the marketplace head with the approved release tag before writing.

## Limits

- Skills are instructions, not enforcement. Model and harness behavior change.
- The style cannot suppress tool-result rendering. Codex skips the hooks until
  you trust them.
- The destructive guard matches a narrow set of command strings, including
  compound commands and absolute-path wrappers. OS sandboxing and least
  privilege remain the real boundary. See [SECURITY.md](./SECURITY.md).
- Independent review discloses approved source content to the selected
  provider. It rejects common secret patterns, not every possible secret.
- Credentialed studies need spend and a reviewed broker. Their selftests do
  not substitute for a run.
- Only current Claude Code and Codex are supported. Exact boundaries are in
  [docs/harness-support.md](./docs/harness-support.md).

## Documentation

- [Install, pin, update, uninstall](./docs/install.md)
- [Harness support and freshness](./docs/harness-support.md)
- [Orchestration and task shapes](./docs/orchestration.md)
- [Independent review workflow](./docs/advanced/independent-review.md)
- [Evaluation and release evidence](./docs/advanced/evals.md)
- [Verification maps](./docs/advanced/verification-maps.md)
- [Security policy](./SECURITY.md), [Contributing](./CONTRIBUTING.md),
  [Changelog](./CHANGELOG.md)

## Develop

```bash
scripts/validate.sh
bash evals/run-all.sh --json results.jsonl
```

The deterministic suite fails on malformed, incomplete, indeterminate,
timed-out, or harness-error results. Changes to behavioral guidance need
deterministic regression coverage. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License and origin

megapowers is MIT-licensed. It began from Superpowers by Jesse Vincent and
retains other source credits in [ATTRIBUTION.md](./ATTRIBUTION.md). It is not an
Anthropic or OpenAI product.
