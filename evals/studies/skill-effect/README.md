# Skill effect-size study

A reproducible protocol for measuring whether a skill's guidance changes the
**correctness** of what an agent produces, the question the eval spine exists
to answer. The published run is in `../../RESULTS.md` (§2). This directory
holds the protocol and scoring tooling, so you can re-run it with any agent
and any model.

## What it measures

For a task where correctness is decidable by running the output, generate the
output many times with the skill's guidance (**skill** mode) and without it
(**control** mode), then compile-and-run every result. The effect size is
`pass%(skill) − pass%(control)`, with a two-proportion z-score.

## Protocol

1. **Pick tasks with a runnable oracle.** The published run used five, each
   printing a task-specific success token the oracle checks (see
   `expected_token()` in `oracle.sh`):
   - `go-worker-pool`, `py-async-pool`: a worker/async pool that must
     terminate (`SUM=30`).
   - `py-queue-terminate`: an asyncio.Queue producer/3-consumers that must
     terminate (`RESULT=110`); the naive "consumer loops on `queue.get()`"
     hangs.
   - `py-sqlite-memory`: an in-memory SQLite table visible across
     `get_conn()` calls (`COUNT=5`); a fresh `:memory:` connection per call
     loses the table.
   - `go-pipeline`: a 3-stage channel pipeline (`SUM=110`); an unclosed stage
     deadlocks.

   The last three are deliberately error-prone (a naive-but-plausible
   solution fails deterministically) so the study has dynamic range on models
   that make those mistakes.

2. **Two prompt modes per task.** skill mode = the skill's actual guidance +
   the task; control = the task only. (The published prompts + inlined skill
   guidance are in the workflow that generated them; the essential control
   task is: "write a complete, self-contained program defining
   `workerPool`/`worker_pool`, feeding it 1..5, and printing `SUM=<sum of
   doubled values>`; it must terminate.")

3. **Generate N per (task × mode).** N = 8 in the published run. Any agent
   works: `claude -p`, `codex exec --json`, an SDK loop, or a Claude Code
   dynamic-workflow run. Collect the programs into a JSON file shaped like:

   ```json
   { "results": [ { "scenario": "go-worker-pool", "lang": "go", "mode": "skill", "program": "<source>" }, ... ] }
   ```
   (The oracle also accepts `{ "result": { "results": [ ... ] } }`, the shape
   a dynamic-workflow run emits.)

4. **Score.** The oracle compiles and runs each program under a timeout and
   tallies:

   ```bash
   ./oracle.sh results.json          # ORACLE_TIMEOUT=25 by default
   ```
   PASS = compiles, terminates within the timeout, prints `SUM=30`. Output is
   a markdown scorecard with per-cell pass rates, Δ, and z, plus a failure
   breakdown.

## Requirements

`jq`, plus the toolchains for the languages under test (`go`, `python3`).
Behavior generation runs a real agent, so run it outside a
credential-blocking sandbox.

## Note on the published null

Across five tasks (including three deliberately error-prone ones) and two
models (frontier + Haiku), 184/184 programs passed in both skill and control
mode: a clean null, Δ = 0% everywhere. Current models are at
ceiling on these common patterns, so a pattern-skill has no single-shot
headroom. The tasks still give the study dynamic range for weaker or older
models and for harder variants: swap `--model` or add a task. See
`../../RESULTS.md` §2 for the full matrix and what a discriminating eval for
these skills actually needs to measure.
