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

## Choose from one personal registry

For non-trivial delegation, `orchestrating` looks for the optional personal
registry at `~/.config/megapowers/agent-capabilities.md`. This is one editable
source across local harnesses. It stays outside repositories and the installed
plugin.

The file is Markdown so the lead can read it directly. A compact structured
block keeps the vocabulary consistent:

```yaml
version: 1
refreshed_at: 2026-08-18
expires_at: 2026-09-01
policy:
  capability_floor: strong
  optimize: [speed, cost]
  escalate_on: [oracle_failure, high_risk, unresolved_ambiguity]
profiles:
  balanced-build:
    roles: [writer, challenger]
    reasoning: strong
    speed: balanced
    cost: medium
    write: true
bindings:
  <harness>:
    balanced-build:
      model: <native-model-id>
      effort: high
      agent_type: <native-agent-type>
      family: <opaque-family>
      access: native
      status: available
      rankable: true
```

`reasoning`, `speed`, and `cost` are relative operator judgments, not measured
facts. A binding is eligible for capability ranking only when `rankable: true`,
its model and effort are known, it is available to the active harness, fits the
lane's role and write boundary, meets the reasoning floor, and has native
access. Keep ambient or otherwise unverified bindings unranked; use them only
through an explicit task-shape route. For independent review, its opaque
`family` must differ from every artifact author's family. Among eligible
bindings, prefer the fastest, then the cheapest. Escalate only at a declared
trigger.

Missing, expired, malformed, or unreadable data falls back to native defaults.
This is model-readable guidance, not parser-enforced validation: if the lead
cannot establish that the required fields and expiry are usable, it ignores the
registry.
`manual` describes something the operator can run; `approved-external` still
requires the explicit disclosure workflow. Neither is a native agent. The
registry is advisory, not authority: it cannot grant permissions, authorize
source disclosure, or approve writes and side effects. Do not put credentials,
account identifiers, command lines, or private source paths in it.

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
