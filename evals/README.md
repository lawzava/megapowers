# megapowers evals

A small, portable harness that scores the suite, so a change to a skill is a
measured effect rather than an opinion. No framework: pure bash plus a Go
stdlib scorer.

Two layers, in order of value:

1. Deterministic oracles (the spine). Each scenario ships a `check.sh` that
   inspects the finished workdir (files, git state, script output) and returns a
   hard verdict. No model, no API key, so the whole pipeline runs in CI and guards
   against regressions. Many seed scenarios are artifact tests that exercise the
   scripts and hooks a skill ships; they double as regression guards for real bugs
   fixed during development.
2. Behavior evals (optional, per-harness). A scenario can instead hand a task
   prompt to a real coding agent (`claude -p`, `codex exec --json`, `opencode run`)
   and check what it produced, with a paired `--control` run (skill withheld) so we
   can compute the effect size of a skill, not just assert it fires. A shipped
   `mock` agent proves this path end-to-end without burning tokens.

## Layout

```
evals/
├── run.sh                 # run ONE scenario, emit a JSON result row
├── run-all.sh             # run every scenario (mock/local), fail on any regression
├── score.go               # aggregate rows -> scorecard + skill-vs-control effect size
├── agents.example.toml    # per-harness agent command templates (copy + edit)
├── lib/
│   └── mock-agent.sh       # deterministic stand-in agent for behavior scenarios
├── studies/                # committed real-agent study protocols + harnesses
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

# score the collected rows into a scorecard. Add --paired so behavior/trigger
# scenarios also run in CONTROL mode (skill withheld); that paired data is what
# score.go needs to compute a skill-vs-control effect size. With the mock agent the
# control run is indeterminate (the mock does nothing without the skill), so a real
# effect size needs a real --agent; the wiring is the same either way:
evals/run-all.sh --paired --json results.jsonl && go run evals/score.go results.jsonl
```

Agent command templates live in `agents.example.toml`; copy to `agents.toml` and edit.
The harness is agent-agnostic: point it at any CLI that takes a prompt and works in a dir.

## Adding a scenario

Create `scenarios/<id>/` with a `scenario.toml` and a `check.sh`. Make `check.sh`
able to fail (mutation-test it once). Prefer a deterministic oracle; reach for a
model-graded rubric only when quality can't be captured in code, and when you do,
grade the final artifact blind (no reasoning trace): verifiers that see prior
conclusions anchor to them.
