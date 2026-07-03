# Trigger-recall study

Every effect-size study in this repo *prepends* a skill's wording to the
task. This study measures the step before that, the delivery assumption
everything else rests on: with the plugin installed, does an agent organically
reach for the right skill when the task matches (recall), and stay quiet when
it doesn't (precision)?

## Protocol

1. One plugin-installed config home is built per invocation (fresh
   `CLAUDE_CONFIG_DIR`, credentials only, `claude plugin marketplace add` +
   `claude plugin install megapowers@megapowers`), then **copied per run** so
   parallel sessions share no mutable state.
2. Each run is `claude -p` in a fresh seeded mini-project. Tasks never name a
   skill:
   - `on-tdd` → should trigger test-driven-development
   - `on-debug` (fixture has a planted failing suite) → systematic-debugging
   - `on-brainstorm` → brainstorming
   - `on-plans` → writing-plans
   - `held-*` → the same four intents in held-out paraphrases. The `on-*`
     prompts were the tuning set for the description fixes, so their recall
     can overfit to phrase echo; report `on-*` (in-sample) and `held-*`
     (held-out) separately.
   - `orch-*` → mega-orchestration positives (autonomous-run,
     cross-model-verification, best-of-n, orchestrating). The template now
     installs `mega-orchestration@megapowers` alongside `megapowers`, which
     also raises the precision bar on every negative.
   - `neg-*` (rename a parameter / explain code / convert JSON→YAML / describe
     the repo) → should trigger **no** domain skill
   - `neg-mention-*` → adversarial negatives: the prompt *contains* trigger
     words ("test-driven development", "parallelize") but only asks for an
     explanation. Firing a domain skill here is a precision failure.
3. Oracle: `Skill` tool invocations extracted from the stream-json transcript.
   HIT = expected skill invoked; FALSE_FIRE = any domain skill on a negative.
   Dispatcher invocations (the `using-megapowers` meta-skill, which fires by
   design on every task) are excluded from false-fire and reported
   separately. Triggering happens early in a run, so turn-capped runs still
   count; only an empty transcript is indeterminate.

```bash
evals/studies/trigger-recall/run-recall.sh --out /tmp/recall --n 6
evals/studies/trigger-recall/oracle.sh /tmp/recall
```

## Why this matters

Recall below 100% is the tuning loop: the skill *description* is the
interface that decides triggering, so a low-recall skill gets its description
sharpened and re-measured. Precision matters equally: a suite that fires on
everything taxes every task. Published numbers in `../../RESULTS.md`.

## Scope and limits

- Claude Code subjects only for now (the Skill-tool invocation is the clean,
  deterministic trigger signal; other harnesses surface skills differently).
  A Codex arm needs an equivalent deterministic signal (a skill-file read in
  the transcript) and is future work.
- Recall here is single-turn `-p` recall on short tasks; interactive sessions
  with a human nudging ("use your skills") will sit higher.
- The published numbers in `../../RESULTS.md` predate the `held-*`, `orch-*`,
  and `neg-mention-*` extensions and were measured with only `megapowers`
  installed. The extended protocol has no published numbers yet; it needs a
  keyed run first.
