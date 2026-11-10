<p align="center">
  <img src="plugins/megapowers/.claude-plugin/icon.svg" width="96" height="96" alt="megapowers logo">
</p>

<h1 align="center">megapowers</h1>

<p align="center">
  Plan, test, debug, and ship with more care in Claude Code and Codex.<br>
  One plugin with task skills. Optional helpers and style stay in repository-only extras.
</p>

<p align="center">
  <a href="https://github.com/lawzava/megapowers/actions/workflows/ci.yml"><img src="https://github.com/lawzava/megapowers/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/lawzava/megapowers/tags"><img src="https://img.shields.io/github/v/tag/lawzava/megapowers?label=release" alt="Latest tag"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/lawzava/megapowers" alt="License"></a>
</p>

<p align="center">
  <a href="#install">Install</a> ·
  <a href="#what-changes-when-it-is-installed">What changes</a> ·
  <a href="#why-megapowers">Why megapowers</a> ·
  <a href="#skills">Skills</a> ·
  <a href="#documentation">Docs</a>
</p>

Coding agents are fast, but they can skip steps: edit before they know the
cause, say "done" without re-running the tests, or push and deploy without a
second look. megapowers gives Claude Code and Codex a skill (a short
playbook) for each of those moments. When your request matches one, the agent
can load it and follow it.

It is exactly one plugin, with tested installs for Claude Code and Codex.
The harness-neutral skills and their supporting files may also be copied into
other Agent Skills harnesses. Local agent bindings and hooks stay in the user's
setup; copying skills does not install a plugin or the repository-only output style.
It uses each tool's own
agents, permissions, worktrees, and memory, and ships no daemon, scheduler,
model router, formatter, or status line.

## What changes when it is installed

| You ask | What the agent does |
|---|---|
| "Why is this test flaky?" | `systematic-debugging`: finds the root cause before editing anything |
| "Add rate limiting to the API" | `test-first-implementation`: writes a failing test, sees it fail, then makes it pass |
| "Commit this and open a PR" | `verify-and-finish`: re-runs your checks and reports what actually passed |
| "Deploy to production" | `safe-effects`: confirms the exact target with you, then reads the result back |
| "Get a second opinion on this auth change" | `independent-review`: guides review by another available model, following your reviewer instructions |

Skills guide the agent; they do not force it ([how often they load](#results)).
Your repository's own instructions and tools always come first.

## Install

**Claude Code**

```bash
claude plugin marketplace add lawzava/megapowers@release
claude plugin install megapowers@megapowers
```

**Codex**

```bash
codex plugin marketplace add lawzava/megapowers --ref release
codex plugin add megapowers@megapowers
```

Then start a fresh session. Loading the shipped skills needs no Go runtime.

Both plugin installs track the `release` branch, which only moves to signed
release tags. Pinning, install scopes, and uninstall are in
[docs/install.md](./docs/install.md).

### OpenCode

OpenCode uses a copy/path install or HTTP skill catalog, not a plugin install.
The primary route is the release catalog. Add it to the `skills` array in
`opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "skills": [
    "https://raw.githubusercontent.com/lawzava/megapowers/release/plugins/megapowers/skills/"
  ]
}
```

Updates arrive when the `release` branch moves. Each catalog entry uses the
plugin version so changed releases refresh OpenCode's cached skill files.

Alternatives:

- Point the same array at a checkout's `plugins/megapowers/skills` directory,
  for example `"/path/to/megapowers/plugins/megapowers/skills"`.
  Absolute paths are used as written; relative paths resolve from the active
  OpenCode working directory, not the config file.
- Copy skills with `npx skills add lawzava/megapowers -a opencode`.

Use one route to avoid duplicate skill IDs. These routes load skill text and
supporting files, not the repository-only Claude Code output style. The
[OpenCode skills documentation](https://opencode.ai/docs/skills/) defines the
`skills` array and HTTP catalog format.

## Why megapowers

- **Shared skills on Claude Code and Codex.** One plugin supplies the same
  task guidance on both.
- **Checks before you ship.** Verification and side-effect skills guide the
  agent to check evidence and authority before acting.
- **Tested with and without the plugin.** Installed-plugin A/B studies
  compare the same tasks with and without megapowers. Historical totals
  need revalidation after a grader correction.
  [Results below](#results).
- **Independent model review.** `independent-review` guides the agent to choose
  another available model, following your instructions. Cross-vendor review is
  preferred but optional. The skill supplies no provider launcher.
- **Loads on demand.** The catalog supplies trigger descriptions. A skill's
  full text loads only when you need it.

### How it compares

Facts checked on 2026-09-25. Install counts on skills.sh are self-reported by
that site.

| | megapowers | [obra/superpowers](https://github.com/obra/superpowers) | [mattpocock/skills](https://github.com/mattpocock/skills) |
|---|---|---|---|
| Works with | Claude Code and Codex, one plugin, shared skills | 16 agent tools, including Claude Code, Codex, Cursor, Gemini CLI, and Copilot CLI | Any skills-compatible client through `npx skills add` |
| Get it from | This repository's marketplace | The official Claude Code marketplace and, per its README, the official Codex marketplace | skills.sh, 4.2M installs across the collection |
| What you get | Task skills; optional helpers and Claude Code style in repository-only extras | 14 skills, including brainstorming, writing-plans, executing-plans, subagent-driven-development, TDD, and `diagnosing-superpowers` | Single-purpose skills: `code-review`, `tdd`, `triage`, `diagnosing-bugs`, `handoff`, `writing-for-agents`, `grill-me` (1.2M installs) |
| Published measurements | [Pack comparison](#skill-packs-compared-in-opencode-2026-11-08) and A/B studies, with SHA-pinned packs | Per-change probe counts in release notes (for example, TDD control 8/10 against treatment 5/10 when a section was cut) and a quorum eval lab | None found in the repository on 2026-09-25 |

Which one fits:

- **superpowers** if you use other agent tools such as Cursor, Gemini CLI, or
  Copilot CLI, or want commercial support. It has official marketplace
  listings on both sides, release notes that quote probe counts per change,
  and a diagnostics skill since v6.4.1 (2026-09-19).
- **mattpocock/skills** if you want to adopt small single-purpose skills one
  at a time through skills.sh. Its `grill-me` is a user-invoked pointer skill,
  which matches the Claude Code docs' advice for skills that over-trigger.
- **megapowers** if you use Claude Code, Codex, or both, and want one plugin
  with shared task guidance and published with-and-without measurements.

## Skills

Each skill has a short description the agent always sees. The full text loads
only when your request matches. The *experimental* labels come from
[`skills/catalog.json`](./plugins/megapowers/skills/catalog.json).

**Plan and build**

| Skill | Use it when |
|---|---|
| `design-and-plan` | A feature or interface needs a spec, a tradeoff decision, or a step-by-step plan |
| `close-off` *(experimental)* | You invoke it by name at the end of a session: it checks the task is complete, records learnings in the project's own surface, cleans up what the session started, and says it is ready for archiving |
| `grill-me` *(experimental)* | You invoke it by name to stress-test a plan or idea through a structured interview before any action |
| `test-first-implementation` | Adding or changing behavior, fixing a confirmed bug, or refactoring: red, green, refactor, with each run observed |
| `systematic-debugging` | A bug, flaky test, incident, or regression has an unknown cause |
| `orchestrating` | Independent tasks can run in parallel on the tool's own subagents, each owning different files |

**Ship safely**

| Skill | Use it when |
|---|---|
| `verify-and-finish` | Before saying done, committing, merging, publishing, deploying, or handing off: re-runs the real check |
| `safe-effects` | Before a deploy, message, charge, migration, destructive query, DNS change, or any write that leaves your machine |
| `independent-review` | Security, auth, billing, concurrency, or data-integrity work needs an independent opinion from another model |

**Research, long runs, and writing**

| Skill | Use it when |
|---|---|
| `evidence-research` *(experimental)* | A decision needs evidence from outside the repository |
| `autonomous-run` *(experimental)* | An approved goal must keep going unattended across context resets or sessions |
| `humanizing-prose` | You want human-facing text drafted or edited without filler, and without dropping or inventing facts |
| `writing-agent-instructions` *(experimental)* | You are writing or revising a skill, `AGENTS.md`, or `CLAUDE.md`, running a retrospective from the current conversation, or auditing saved memory |

Repository-only setup and upkeep skills are in [extras/](./extras/README.md);
they are not installed with the plugin.

[docs/orchestration.md](./docs/orchestration.md) explains which kinds of task
go to which skill.

## Output style

An optional repository-only style makes replies shorter and more direct: the answer
first, about 100 words by default and up to 250 unless you ask for depth, and
no em dashes. It keeps the built-in coding instructions.

- **Claude Code:** install it manually from
  [extras/output-styles/](./extras/README.md#output-style) into
  `~/.claude/output-styles/`, then run `/output-style Megapowers` or select
  `Megapowers` in `/config`. The plugin no longer ships it.
- **Codex:** the plugin does not apply a session-wide style. Invoke
  `humanizing-prose` for writing tasks that need its editing guidance.

In one operator's September 2026 Codex sessions, styled final messages had a
median of 70 words, and 0.6 percent contained an em dash, against 36 percent
without the style. These numbers are not a token-saving or cost claim.

## Results

### Skill packs compared in OpenCode (2026-11-08)

Each pack ran on the same 8 cases, 3 runs per case, with and without the
pack, in isolated OpenCode sessions. A blind judge from the other vendor
graded each run: Sol graded Opus runs and Opus graded Sol runs. The 8 cases are
the ones where some run failed in an earlier 20-case baseline. Opus is
`claude-agent/opus`; Sol is `openai/gpt-6.1-sol-fast#high`. Packs were pinned
to these commits: megapowers `b75e795`, mattpocock/skills `4ae2317`,
obra/superpowers `22f0b46`, pstack (`cursor/plugins`) `6b03002`,
addyosmani/agent-skills `301b02b`.

| Pack | Opus without → with (of 24) | Sol without → with (of 24) | Time with vs without (Opus / Sol) |
|---|---:|---:|---:|
| megapowers | 18 → 22 | 23 → 24 | −5% / +6% |
| mattpocock/skills | 17 → 22 | 24 → 23 | +6% / +19% |
| superpowers | 19 → 21 | 23 → 21 | +14% / +24% |
| pstack | 18 → 19 | 24 → 22 | −2% / +2% |
| addyosmani/agent-skills | 20 → 18 | 24 → 24 | +5% / +25% |

Not every row comes from a fresh run. The megapowers, mattpocock/skills and
superpowers Opus rows are the same 8 cases taken from earlier 20-case runs.
Those runs used the same skill content as listed above, and their saved runs
were graded again with the current grader. The pstack, addyosmani/agent-skills
and superpowers Sol rows ran fresh on the 8 cases.

Opus gained 4 to 5 passes with megapowers and with mattpocock/skills. This
small, selected sample does not establish reliable differences between packs.
Sol passes almost every case without any pack. On the full 20 cases, megapowers
scored 51 → 55 of 60 on Opus and 56 → 57 of 60 on Sol. Method, cases and
commands: [installed-ab study](./evals/studies/installed-ab/README.md).

### Earlier studies

The October 2026 A/B studies reported gains in judgment cases and ties on
their coding tasks. Their totals have not been revalidated after an October 12
grader correction, and the original result bundles are unavailable locally.
They do not establish current efficacy or a speed or token-cost benefit.
Historical results and limitations are retained in
[evals/RESULTS.md](./evals/RESULTS.md).

The evaluation suite includes a three-turn maintenance case that checks
whether later changes preserve earlier behavior. The scorer labels effort
levels and reports paired actor durations beside outcomes. The
[study protocol](./evals/studies/installed-ab/README.md) explains how to run
matched effort comparisons. Having these checks is not evidence that the
current plugin improves their results.

The study below measured earlier models with graders since found to fail
correct answers.

From an installed-plugin A/B study completed 2026-09-05: 1,080 valid trials
over 27 cases, with ten control/treatment pairs per case on each tool. The
control ran without the plugin and the treatment ran with it. Codex ran
`gpt-6-astra` on CLI `0.153.3`, and Claude ran `claude-fable-5-1` on Claude
Code `2.1.258`, both at high effort.

| Task checks passed | Codex without | Codex with | Claude without | Claude with |
|---|---:|---:|---:|---:|
| 18 development cases | 85/180 | 106/180 | 89/180 | 101/180 |
| 6 held-out cases, frozen before the run | 50/60 | 60/60 | 52/60 | 53/60 |

With the plugin installed, the skill a task needed loaded in 77 of 150 Codex
trials and 83 of 150 Claude trials, so skills help but do not always fire.
The study predates the `0.29.0` version stamp. Neither tool passes the
full benchmark, and the results do not show general model superiority. Full
results and method: [evals/RESULTS.md](./evals/RESULTS.md).

<details>
<summary>More results and caveats</summary>

| Development checks (18 cases) | Codex control | Codex + megapowers | Claude control | Claude + megapowers |
|---|---:|---:|---:|---:|
| Task outcome | 85/180 | 106/180 | 89/180 | 101/180 |
| Workflow | 163/180 | 170/180 | 164/180 | 169/180 |
| Outcome and exact activation profile | 85/180 | 74/180 | 89/180 | 73/180 |
| Required activation profile | n/a | 77/150 | n/a | 83/150 |

"Required activation profile" scores whether the skill a case requires
actually loaded, as shown in the trace. It is scored apart from task outcome.
"Outcome and exact activation profile" also required the exact expected set of
skill reads, so an extra successful skill read could fail it. On this row
treatment scores below control on both tools. The current runner permits
successful reads of other known skills unless explicitly forbidden; these
historical totals have not been regraded under that policy.
All ten Claude pending-review replies passed the task checks but did not load
`verify-and-finish`. The instruction-authoring case moved from 0/10 to 10/10
on Codex and 0/10 to 9/10 on Claude. Codex's pending-review case moved from
0/10 to 10/10. The separate autonomy/continuity diagnostics did not pass
(Codex 0/30 to 0/30, Claude 1/30 to 0/30).

The 2026-09-02 Codex trigger-recall run reported 0/117 implicit recall despite
a rendered catalog
([trigger-recall README](./evals/studies/trigger-recall/README.md)), so on
Codex the skill-trigger checks only report and never fail a run.
`scripts/validate.sh` and `evals/run-all.sh` prove repository mechanics, not
agent quality. Protocols are in [docs/advanced/evals.md](./docs/advanced/evals.md).

</details>

## Cost

The skill catalog loads per session; each skill body loads only when that
skill is used. Earlier token estimates included startup text that is no
longer shipped and do not describe this version.

<details>
<summary>Exactly what installs</summary>

- 13 skills under `skills/`, listed in `skills/catalog.json`.
- An HTTP skill catalog at `skills/index.json` for OpenCode.
- Guidance and passive metadata only: no executable code, hooks, or MCP entries.
  Optional helpers and setup skills live in repository-only
  [extras/](./extras/README.md).

The plugin does not edit global user configuration and supplies no model,
provider, or agent configuration. For non-trivial delegation, `orchestrating`
can read an optional personal registry at
`~/.config/megapowers/agent-capabilities.md`; it is advisory and grants no
access.

</details>

## Troubleshooting

Check registration with the plugin-list commands in
[docs/install.md](./docs/install.md), then ask the agent to load a named skill
and summarize it. For optional local diagnostics, see
[extras/](./extras/README.md).

| Symptom | Fix |
|---|---|
| Old `megapowers:Megapowers` style setting no longer resolves | Install the repository-only style manually from [extras](./extras/README.md#output-style), then select `Megapowers` |
| Wrong version after an upgrade | Compare the marketplace head with the approved release tag before registering it |

## Limits

- Skills are instructions, not enforcement. Model and tool behavior change.
- The repository-only Claude Code style cannot suppress tool-result rendering.
- The plugin supplies no destructive-command guard or command gates. Use
  harness permissions, OS sandboxing, and least privilege for enforcement.
  See [SECURITY.md](./SECURITY.md).
- Independent review guidance requires checking secrets and disclosure scope
  before sending material to the selected reviewer.
- The evaluation studies that call real models cost money and need a reviewed
  credential broker. Their self-tests do not replace a real run.
- Claude Code and Codex are the tested plugin targets; OpenCode uses skill sources. Exact boundaries are in
  [docs/harness-support.md](./docs/harness-support.md).

## Documentation

- [Install, pin, update, uninstall](./docs/install.md)
- [Harness support and freshness](./docs/harness-support.md)
- [Orchestration and task shapes](./docs/orchestration.md)
- [Independent review workflow](./docs/advanced/independent-review.md)
- [Evaluation and release evidence](./docs/advanced/evals.md)
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
