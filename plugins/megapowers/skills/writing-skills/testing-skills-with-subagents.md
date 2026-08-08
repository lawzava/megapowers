# Evaluating Skills With Subagents

Use this reference when creating or materially editing a skill. Evaluation asks
whether the skill improves real task outcomes. It is not an obedience test.

## Choose evidence proportionate to risk

Start with the smallest honest evaluation that can falsify the proposed change.
Editorial fixes and stable references need link, example, and retrieval checks.
Behavioral guidance needs representative task execution. Safety, consent,
destructive operations, security, and irreversible changes need adversarial
cases and independent review where available.

Record the task, expected observable outcome, environment, guidance version,
and result. State limits plainly. Do not manufacture urgency, claim a scenario
is real when it is an evaluation, or require an agent to select a predetermined
answer.

## Build the evaluation

1. Define the decision or outcome the skill should improve, such as selecting
   the right procedure, preserving an explicit consent gate, or producing a
   valid artifact.
2. Choose representative tasks, including a nearby case where the skill should
   not apply. Use real historical cases when safe and available; otherwise label
   realistic synthetic tasks as synthetic.
3. Establish a control when it would answer a question: compare no guidance,
   prior guidance, or a competing concise formulation under comparable
   conditions. A control is not required for mechanical validation or an
   already-observed defect with a direct regression test.
4. Run enough varied cases to expose meaningful differences. Increase samples
   only when results are noisy, impact is high, or the change makes a broad
   claim. Do not set a fixed count for every skill.
5. Inspect both the delivered artifact and the path where safety or process
   matters. A fluent explanation is not evidence that the required action,
   check, or refusal occurred.
6. Preserve failures, ambiguous results, and counterexamples. Revise the skill
   only for a demonstrated gap, then repeat the affected cases.

## Evaluation types

| Skill type | Useful evidence |
| --- | --- |
| Reference | Retrieval of the right source, accurate application, links or scripts validated |
| Technique | Completion of novel representative tasks with an observable oracle |
| Workflow | Correct artifacts, ordering, and verification evidence across realistic task states |
| Safety or consent | Boundary cases, refusal or confirmation behavior, and independent review when warranted |
| Discovery | Positive, near-miss, and negative prompts; measure both intended selection and false activation |

## Interpreting results

Compare outcomes, not citations of the skill. A skill is useful when it improves
the target outcome without adding material false activations, unsafe behavior,
or unnecessary work. If the control already succeeds, state that the proposed
guidance has no demonstrated value and avoid adding it. If results vary, report
the uncertainty and test a clearer task, oracle, or formulation before making a
strong rule.

For descriptions, keep held-out near misses separate from development prompts.
For body changes, retain the cases that revealed the defect as regressions. For
high-risk instructions, include an explicit check that required consent,
verification, and stop conditions remain present.

## Common mistakes

- Asking an agent to recite the skill instead of completing a task.
- Treating compliance with a forced option as proof of usefulness.
- Hiding the evaluation purpose or pretending a synthetic scenario is live.
- Using a fixed sample target when one deterministic check or a broader varied
  sample is the better evidence.
- Dropping failed or inconclusive cases from the record.
- Optimizing for trigger rate without measuring false activation.

## Release evidence

Record the changed behavior, scenarios, controls if used, oracle, observed
results, and known limits. Re-run repository validation. The evidence should be
enough for a reviewer to reproduce the claim without trusting an evaluator's
summary.
