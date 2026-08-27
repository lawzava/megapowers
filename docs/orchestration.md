# Orchestration

megapowers adds decision rules around native harness features. It does not add a
second scheduler or agent runtime.

## Start with task shape

| Task shape | Route |
|---|---|
| One clear, bounded change | Work inline. Load the task skill that supplies the missing discipline. |
| One bounded output-only investigation | Use one fresh-context agent. Return one JSON object with `verdict`, `evidence`, `uncertainty`, and `next`, not the raw payload. |
| Unclear behavior, interface, risk, or acceptance oracle | Use `design-and-plan`. |
| Several disjoint deliverables | Use native agents with explicit, non-overlapping ownership. |
| Four or more durable lanes, dependencies, or repeated follow-ups | Use native team or task coordination when the harness provides it. Otherwise use staged waves. |
| Ordinary handoff, takeover, or harness switch | Inspect current state inline and use `verify-and-finish`; prior authority does not transfer. |
| An approved goal that must survive interruption | Use a native goal plus `autonomous-run` checkpoints. |
| Historical rationale or contested evidence | Use `evidence-research`; research does not authorize a change or publication. |
| Residual high-stakes uncertainty after executable checks | Use `independent-review`. |
| Deploy, message, migration, charge, destructive query, or external write | Use `safe-effects` before execution. |

`orchestrating` starts non-trivial work with a short lane scan. Dispatch
independent read-heavy lanes and bounded output-only work before deep local work.
Keep one bounded dependency path inline. Repeat the scan after scope changes or
context compaction.

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

Use direct agents for one to three independent lanes. For larger or durable
work, prefer native team or task state that records ownership, dependencies, and
completion. If the harness lacks it, dispatch staged waves and synthesize before
the next wave. Use milestone checkpoints or a fresh-lead handoff before the
lead context becomes a program log.

Before dispatch, confirm the child has the required tools, MCP access,
authentication, network, permissions, and write authority. Read the personal
registry once per session, then reuse it until its identity changes. Use fresh
or bounded child context for self-contained work. Use full history only when a
brief cannot carry the required context.

A useful task brief names:

- one outcome;
- exact file or module ownership;
- relevant interfaces and constraints;
- the acceptance oracle;
- what the worker must not change;
- the return condition and whether nested delegation is allowed.

The default return contains a verdict, evidence references, uncertainty, and
the next decision. An output-only task returns those fields as one JSON object.
Keep raw payloads outside the lead context. Require an
artifact path only when bulky evidence cannot fit this bounded return. Send
delta-only follow-ups with only new or changed facts.

Parallel ownership must be disjoint. Keep shared interfaces and dependent tasks
sequential. The lead remains the single writer for integration and Git, reads
the returned artifacts, resolves conflicts, and reruns the real oracle.

Continue lead work while agents run. Then use one longest supported wait and
avoid short polling. Batch eligible agents before waiting. Do not accept serial
spawn-complete interleaving as parallel fan-out.

Same-provider agents provide parallelism and context separation. They do not
provide vendor independence. Use the trusted review path only when another
provider materially reduces residual risk.

## Keep durable runs small

Prefer the harness's native goal and wait mechanisms. Add ignored
`.megapowers/run/<id>/` files only when work must resume after context or process
loss under a currently approved autonomous goal:

- `charter.md` freezes outcome, boundaries, authority, and cap.
- `checkpoint.md` records workspace and artifact identity, current milestone,
  evidence, delegate ownership, remaining effect authority, blocker, and next
  safe command.
- `journal.jsonl` records observed transitions and their evidence.

Update durable state at real transitions, not every turn. On resume, reconcile
repository, worktree, branch, HEAD, runtime, and external state before acting.
Stop on missing or contradictory evidence. A handoff or harness switch does not
inherit prior authority. A journal proves only what its oracle proved.

## Stop rules

Set a bounded oracle before expensive work: a passing test, decision criterion,
candidate count, time budget, or retry limit. Three failed fixes on one approach
require a new diagnosis, not more retries. External effects still require exact
approval even inside an autonomous goal.
