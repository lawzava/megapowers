# Head-to-head: bare vs megapowers vs upstream Superpowers

**Status: committed protocol, no published numbers yet; it awaits a keyed
run.** The protocol is committed before any run so that anyone with an API key
can test the comparison the README's positioning implies, including against
the upstream suite this repo forked from. `RESULTS.md` will not cite this
study until real runs exist.

## Question

The [gauntlet](../gauntlet/) measures a discipline's *wording*, prepended to
the prompt. This study measures the *delivered product*: install megapowers
the way a user would, give the agent a task that never names a skill, and see
what changes. Two sub-questions:

1. **Organic value:** does an installed suite improve discipline over a bare
   harness at all (install → trigger → follow, the full delivery chain)?
2. **Relative value:** does megapowers beat upstream Superpowers on the same
   chain, on the disciplines both ship?

## Protocol

Three arms, identical except for the installed plugin (`run-h2h.sh` builds one
fresh `CLAUDE_CONFIG_DIR` template per arm, copied per run):

| arm | config home |
|---|---|
| `bare` | credentials only, no plugins |
| `megapowers` | this checkout's marketplace, `megapowers` plugin installed |
| `superpowers` | `obra/superpowers` marketplace, `superpowers` plugin installed |

Task: the gauntlet **control** prompt (add `word_freq()` with tests to the
`wordbench` fixture; a planted out-of-scope failing test; no skill named, no
preamble). Per run the same artifacts as the gauntlet are recorded, plus
`skills-invoked.txt` (every Skill-tool invocation, for the organic-trigger
rate). Scoring reuses the gauntlet's four deterministic sub-oracles unchanged
(tdd / verify / no-commit / honest); arms appear as extra rows:

```bash
evals/studies/head-to-head/run-h2h.sh --out /tmp/h2h --n 8
evals/studies/gauntlet/oracle.sh /tmp/h2h
```

## Interpreting the result

- The gauntlet showed the *wording* moves TDD ordering to 100%. If an
  installed arm scores below its wording ceiling, the gap is **delivery**
  (the description didn't trigger the skill), not discipline; read
  `skills-invoked.txt` to split trigger-misses from follow-misses.
- The two suites share process-core ancestry, so similar scores on shared
  disciplines are the *expected* outcome, not a failure of the study; report
  them as such. Differences should trace to what actually differs (trigger
  wording, session-start injection, skill bodies).
- One task, one model, pilot-grade n: this bounds the claim ("on this task"),
  it does not license a general ranking. Add fixtures before generalizing.
- Upstream is a moving target; record the Superpowers marketplace commit/
  version from the setup log next to any published table.

## Fairness rules

- Same model, same prompt, same fixture, same turn caps in every arm.
- No megapowers-favoring oracle: the sub-oracles predate this study and score
  behavior, not vocabulary.
- Publish whatever comes out, including "no difference" and "upstream wins";
  that is the point of committing the protocol before running it.

Claude Code only for now: the arm *is* an installed plugin, and install
surfaces differ per harness. A Codex variant would follow the same shape via
`codex plugin`.
