# De-prescription rubric

How to trim a skill for frontier models without losing what the evals prove
matters. Written for the Fable 5 generation (its vendor guidance: instructions
written for older models can degrade output; remove them where default
behavior is now better). Applied wave by wave; see
evals/RESULTS.md for the measured gates each wave passed.

## Remove

- Enumerated don't-lists that one steering sentence covers. A frontier model
  follows a single brief instruction; long lists invite literalism and drift.
- Step-by-step micro-instructions where the how is the model's job. Keep the
  goal, the constraints, and the definition of done.
- Restatements of harness-native behavior: worktree mechanics the harness's
  worktree tools own, re-dispatch-with-recap patterns that resumable
  subagents made obsolete, background behavior subagents have by default.
- Rationale stated more than once. State a reason exactly where it binds.

## Keep

- Consent and safety gates, discard confirmations, single-writer discipline.
- Kill-list anti-pattern guards. Their absence is eval-enforced
  (evals/scenarios/killlist-antipatterns-absent).
- Verification oracles and evidence-before-claims wording.
- Any wording with a published effect size in evals/RESULTS.md. Trimming a
  measured sentence needs a fresh measurement, not taste.

## Add sparingly

Only where the skill is the natural home, one or two sentences:

- Curb unrequested tidying: a bug fix does not need surrounding cleanup.
- External stop budgets: long runs declare time, step, or token caps up front.
- Ground progress claims in tool results, never in intention.

## Never

- Touch frontmatter. Descriptions are the tuned trigger surface
  (scripts/check-description-freeze.sh enforces this).
- Change the register: declarative prose, no dash punctuation, none of the
  banned vocabulary listed in CONTRIBUTING.md.
