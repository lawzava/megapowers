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
│   ├── lib.sh              #   shared runner core (agent exec, fan-out)
│   ├── process-behavior/   #   process-discipline + pressure/honesty probes (§3, §5a-b)
│   ├── install-smoke/      #   fresh-env install + first-task load (RESULTS.md §4)
│   ├── trigger-recall/     #   organic skill triggering, recall + precision (§5c)
│   ├── gauntlet/           #   four disciplines in one task, per-discipline profile (§5d)
│   └── autonomy-run/       #   multi-step autonomy honesty pilot (§5e)
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

## Control arms: measure the skill, not the ask

A skill's honest delta is skill versus a terse control arm that already asks
for the generic behavior in one line, not skill versus bare baseline.
Comparing against the bare baseline conflates the skill with the generic
instruction and inflates the number. Where an output-shape study needs it,
add the control arm alongside baseline and skill. Two design rules ride
along, and this harness already follows both: commit the scored snapshot so
any change to published numbers is reviewable as a diff, and keep generation
(keyed) separate from scoring (deterministic, CI-safe). Methodology adapted
from caveman (https://github.com/JuliusBrussee/caveman, MIT).

## Consistency, not just pass rate

`score.go` reports `pass^3` beside the pass rate for every arm with at least
three runs. A pass rate answers whether a skill usually binds; `pass^3`
estimates the chance that all three independent runs comply, which is the bar a
discipline skill actually has to clear, because a session does not get to be
usually governed. The two diverge fast and the gap is the interesting part: 90%
passing is 70% at k=3. Use the pass rate to compare arms and `pass^3` to decide
whether a discipline is ready to rely on.

## Noise floor for real-agent numbers

Two sources of noise sit under every keyed number here, and neither is the
model.

Sampling noise is the one the harness already reports: `z` and `fisher_p`
exist so a small-n difference is read as directional rather than proven.

Infrastructure noise is the one that is easy to miss. Anthropic measured a
6-point spread on Terminal-Bench 2.0 (p<0.01) between the most- and
least-resourced setups of the *same* model and harness, and 1.54 points on
SWE-bench across a 5x RAM variance; their infrastructure error rate moved from
5.8% to 0.5% purely on resource headroom
([Quantifying infrastructure noise in agentic coding evals](https://www.anthropic.com/engineering/infrastructure-noise),
2026-02-05). That is larger than several results worth having.

So, for any wave whose conclusion depends on a difference under about 3 points:

- Record the machine and the resource envelope in the study protocol, with the
  same care given to the model id and the prompt.
- Spread the runs across more than one time of day, and preferably more than
  one day, rather than taking a single contiguous block.
- Read a sub-3-point difference as noise unless the configurations are
  documented and matched.
- Suspect the environment, not the skill, when failures correlate across
  unrelated scenarios in the same block.

This does not touch the large results: a 0/36 to 36/36 split is orders of
magnitude outside any of it. It bounds what the small ones are allowed to claim.

## Published artifacts

Re-running a study draws a fresh stochastic sample; it does not reproduce the
exact published counts. No study wave has committed run artifacts: a published
number is auditable only by a fresh keyed re-run of the committed protocol,
which is a new sample, not a replay. If a future wave commits its run
directories, sanitize first: transcripts must carry no credentials, tokens,
or private paths.

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
