# megapowers evals

A small, portable eval harness that scores the skill suite, so a change to a
skill has a measurable effect size. No framework: pure bash plus a Go stdlib
scorer.

Two layers, in order of value:

1. Deterministic oracles (the eval spine). Each scenario ships a `check.sh`
   that inspects the finished workdir (files, git state, script output) and
   returns a hard verdict. No model, no API key, so the whole pipeline runs in
   CI and guards against regressions. Many seed scenarios are artifact tests
   that exercise the scripts and hooks a skill ships; they double as
   regression guards for real bugs fixed during development.
2. Behavior evals (optional, run against a real agent). A scenario can
   instead hand a task prompt to a real coding agent (`claude -p`,
   `codex exec --json`, `opencode run`) and check what it produced, with a
   paired `--control` run so we can compute the effect size of a skill, not
   just assert it fires. Your `agents.toml` command template expresses the
   two arms, keyed on `{{MODE}}` (for example, a profile or `--add-dir` that
   includes the skill only in skill mode); the shipped examples leave that
   wiring to you. A built-in `mock` agent proves the path end-to-end without
   burning tokens.

## Layout

```
evals/
├── run.sh                 # run ONE scenario, emit a JSON result row
├── run-all.sh             # run every scenario (mock/local), fail on any regression
├── score.go               # aggregate rows -> scorecard + skill-vs-control effect size
├── agents.example.toml    # per-agent command templates (copy + edit)
├── studies/                # committed real-agent study protocols + runners
│   ├── skill-effect/       #   code-correctness effect size (RESULTS.md §2)
│   ├── process-behavior/   #   process-discipline + pressure/honesty probes (§3, §5a-b)
│   ├── install-smoke/      #   fresh-env install + first-task load (RESULTS.md §4)
│   ├── trigger-recall/     #   organic skill triggering, recall + precision (§5c)
│   ├── gauntlet/           #   four disciplines in one task, per-discipline profile (§5d)
│   ├── autonomy-run/       #   multi-step autonomy honesty pilot (§5e)
│   └── head-to-head/       #   bare vs megapowers vs upstream Superpowers, organic
│                           #   triggering (protocol committed; awaits a keyed run)
└── scenarios/<id>/
    ├── scenario.toml       # id, title, skill, kind, (prompt for behavior)
    ├── setup.sh            # optional: seed $WORKDIR before the run
    ├── solve.sh            # artifact scenarios: the deterministic actor (runs the shipped script)
    ├── mock/actions.sh     # behavior scenarios: what a compliant agent would do (for the mock)
    └── check.sh            # the oracle: exit 0 pass, 1 fail, 77 indeterminate
```

## Scenarios vs studies

The two directories answer different questions and run differently:

- Scenarios (`scenarios/<id>/`) are cheap, oracle-checked units run by
  `run.sh`/`run-all.sh`. They run in CI on every push, against the mock agent
  where a scenario needs one.
- Studies (`studies/<name>/`) are standalone protocols with their own runner
  scripts. They run real agents, so they need a keyed run (real model
  credentials and API spend, which CI does not have), and they are the source
  of the numbers in [`RESULTS.md`](./RESULTS.md).

## Published artifacts

Re-running a study draws a fresh stochastic sample; it does not reproduce the
exact published counts. To let a reader *audit* a published number rather than
only re-sample it, keyed waves from 2026-07 onward follow one convention:

- Each wave writes its run directories to `evals/results-<date>/` (for example
  `evals/results-2026-08-01/`): the sanitized agent transcripts, the per-run
  JSONL rows, and the oracle's own output for that wave.
- `RESULTS.md` references each such directory by content hash next to the
  section it backs, so a reader can fetch the archived runs and re-run the
  committed `oracle.sh` over them offline, with no credentials and no sampling
  variation.

Plainly: the pre-2026-07 study waves predate this convention and have **no**
committed run artifacts. Their numbers are auditable only by a fresh keyed
re-run of the committed protocol, which is a new sample, not a replay of the
published one. Sanitize before committing any wave: transcripts must carry no
credentials, tokens, or private paths.

## Scenario kinds

- `artifact`: deterministic. `solve.sh` runs a shipped script or hook against a
  seeded `$WORKDIR`; `check.sh` asserts the result. Runs in CI, no agent.
- `behavior`: the runner invokes an agent with `prompt`; `check.sh` asserts on
  the workdir/trace. Runs against a real agent, or the mock (`mock/actions.sh`)
  in CI.
- `trigger`: a negative behavior test. The skill must NOT fire off-topic;
  `check.sh` greps the trace for the skill's activation signature and passes
  when it is absent.

## check.sh contract

`check.sh` runs with cwd `$WORKDIR` and these env vars:
`$WORKDIR` (agent's finished tree), `$TRACE` (captured stdout/transcript, may be empty),
`$SCENARIO_DIR` (the scenario's own dir), `$MODE` (`skill` or `control`).
Exit `0` pass, `1` fail, `77` indeterminate (couldn't decide, never counts as pass).

## Run

```bash
# whole suite, deterministic (CI-safe): artifact scenarios run for real, behavior
# scenarios run against the mock agent. Fails if any oracle fails.
evals/run-all.sh

# one scenario against a real agent (behavior scenarios):
evals/run.sh task-brief-boundary                          # artifact: no agent needed
evals/run.sh brainstorm-proportional-gate --agent claude  # behavior: real agent
evals/run.sh brainstorm-proportional-gate --agent claude --control   # paired control

# score the collected rows into a scorecard:
evals/run-all.sh --paired --json results.jsonl && go run evals/score.go results.jsonl
```

`--paired` also runs each behavior/trigger scenario in control mode (skill
withheld); `score.go` needs that paired data to compute a skill-vs-control
effect size. With the mock agent the control run is indeterminate (the mock
does nothing without the skill), so a real effect size needs a real
`--agent`; the wiring is the same either way.

Agent command templates live in `agents.example.toml`; copy to `agents.toml`
and edit. The eval harness is agent-agnostic: point it at any CLI that takes
a prompt and works in a dir.

## Adding a scenario

Create `scenarios/<id>/` with a `scenario.toml` and a `check.sh`. Make `check.sh`
able to fail (mutation-test it once). Prefer a deterministic oracle; reach for a
model-graded rubric only when quality can't be captured in code, and when you do,
grade the final artifact blind (no reasoning trace): verifiers that see prior
conclusions anchor to them.
