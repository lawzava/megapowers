# Orchestration

megapowers adds decision rules around native harness features. It does not add a
second scheduler or agent runtime.

## Start with task shape

| Task shape | Route |
|---|---|
| One clear, bounded change | Work inline. Load the task skill that supplies the missing discipline. |
| Unclear behavior, interface, risk, or acceptance oracle | Use `design-and-plan`. |
| Several disjoint deliverables | Use native agents with explicit, non-overlapping ownership. |
| Work that must survive interruption | Use a native goal plus `autonomous-run` checkpoints. |
| Residual high-stakes uncertainty after executable checks | Use `independent-review`. |
| Deploy, message, migration, charge, destructive query, or external write | Use `safe-effects` before execution. |

`orchestrating` applies this routing to non-trivial work. Inline work remains the
default because every dispatch has briefing, integration, and review cost.

## Delegate safely

A useful task brief names:

- one outcome;
- exact file or module ownership;
- relevant interfaces and constraints;
- the acceptance oracle;
- what the worker must not change.

Parallel ownership must be disjoint. Keep shared interfaces and dependent tasks
sequential. The lead remains the single writer for integration and Git, reads
the returned artifacts, resolves conflicts, and reruns the real oracle.

Same-provider agents provide parallelism and context separation. They do not
provide vendor independence. Use the trusted review path only when another
provider materially reduces residual risk.

## Keep durable runs small

Prefer the harness's native goal and wait mechanisms. Add ignored
`.megapowers/run/<id>/` files only when work must resume after context or process
loss:

- `charter.md` freezes outcome, boundaries, authority, and cap.
- `checkpoint.md` records the current milestone, evidence, blocker, and next
  command.
- `journal.jsonl` records observed transitions and their evidence.

Update durable state at real transitions, not every turn. On resume, reconcile
it with fresh repository state before acting. A journal proves only what its
recorded oracle proved.

## Stop rules

Set a bounded oracle before expensive work: a passing test, decision criterion,
candidate count, time budget, or retry limit. Three failed fixes on one approach
require a new diagnosis, not more retries. External effects still require exact
approval even inside an autonomous goal.
