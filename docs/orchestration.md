# Orchestration

megapowers adds decision rules around native harness features. It does not add a
second scheduler or agent runtime.

A lane is one independent piece of work that a single agent can own.

## Start with task shape

| Task shape | Route |
|---|---|
| One clear, bounded change | Work inline. Load the task skill that supplies the missing discipline. |
| One bounded output-only investigation | Use one fresh-context agent. Return one JSON object with `verdict`, `evidence`, `uncertainty`, and `next`, not the raw payload. |
| Unclear behavior, interface, risk, or acceptance check | Use `design-and-plan`. |
| Several disjoint deliverables | Use native agents with explicit, non-overlapping ownership. |
| Four or more durable lanes, dependencies, or repeated follow-ups | Use native team or task coordination when the harness provides it. Otherwise use staged waves. |
| Ordinary handoff, takeover, or harness switch | Inspect current state inline and use `verify-and-finish`; earlier approvals do not transfer. |
| An approved goal that must survive interruption | Use a native goal plus `autonomous-run` checkpoints. |
| Historical rationale or contested evidence | Use `evidence-research`; research does not authorize a change or publication. |
| Residual high-stakes uncertainty after executable checks | Use `independent-review`. |
| Deploy, message, migration, charge, destructive query, or external write | Use `safe-effects` before execution. |

`orchestrating` starts work that matches the table with a short lane scan. It
dispatches independent read-heavy lanes before the lead reads them and sends
bounded output-only work to one fresh-context agent. Keeping a qualifying lane
inline is a workflow failure unless native agents are unavailable. Keep one
bounded or sequential dependency path inline. Repeat the scan after scope or
context changes.

## Choose from one personal registry

For non-trivial delegation, `orchestrating` looks for the optional personal
registry at `~/.config/megapowers/agent-capabilities.md`, one editable file
shared across local harnesses. It stays outside repositories and the installed
plugin.

The file is Markdown so the lead can read it directly. Start from the
[template](../plugins/megapowers/skills/orchestrating/assets/agent-capabilities.md),
which defines the version 2 fields: `policy`, `profiles`, `lead_preferences`,
`bindings`, and `fallbacks`. `policy` is optional; without
`capability_floor`, no floor applies.

`intelligence`, `speed`, and `cost` are relative operator judgments, not
measured facts. Version 1 files that use `reasoning` and `optimize` remain
readable; treat `reasoning` as `intelligence`.

A binding can be ranked by capability only when `rankable: true`, its model
and effort are known, it is available to the active harness, it fits the
lane's role and write boundary, it meets the capability floor, and it has
native access. Keep ambient or otherwise unverified bindings unranked; use
them only through an explicit task-shape route. For independent review, the
binding's opaque `family` must differ from every artifact author's family.
Among eligible bindings, prefer the fastest, then the cheapest. Escalate only
at a declared trigger.

An unavailable binding is reported, not silently replaced. A `fallbacks`
entry applies only when access, disclosure, and permissions for it already
exist. A failed task check calls for diagnosis, not a provider switch.

Missing, expired, malformed, or unreadable data falls back to native defaults.
This is model-readable guidance, not parser-enforced validation: if the lead
cannot establish that the required fields and expiry are usable, it ignores the
registry. `megapowers-doctor` warns when the file is expired or has no
readable `expires_at`.

`manual` describes something the operator can run; `approved-external` still
requires the explicit disclosure workflow. Neither is a native agent. The
registry is advisory. It cannot grant permissions, authorize source
disclosure, or approve writes and side effects. Do not put credentials,
account identifiers, command lines, or private source paths in it.

## Delegate safely

Use direct agents for one to three independent lanes. For larger or durable
work, prefer native team or task state that records ownership, dependencies, and
completion. If the harness lacks it, dispatch staged waves and synthesize before
the next wave. Use milestone checkpoints or a fresh-lead handoff before the
lead context becomes a program log.

Before dispatch, confirm the child has the required tools, MCP access,
authentication, network, permissions, and write access. Read the personal
registry once per session, then reuse it until its identity changes. Use fresh
or bounded child context for self-contained work. Use full history only when a
brief cannot carry the required context.

A useful task brief names:

- one outcome;
- exact file or module ownership;
- relevant interfaces and constraints;
- the acceptance check;
- what the worker must not change;
- the return condition and whether nested delegation is allowed.

The default return contains a verdict, evidence references, uncertainty, and
the next decision. An output-only task returns those fields as one JSON object.
Keep raw payloads outside the lead context. Require an artifact path only when
bulky evidence cannot fit this bounded return. Send delta-only follow-ups with
only new or changed facts.

Parallel ownership must be disjoint. Keep shared interfaces and dependent tasks
sequential. The lead remains the single writer for integration and Git, reads
the returned artifacts, resolves conflicts, and reruns the real check.

Continue lead work while agents run. Then prefer completion events or
asynchronous waits within tool deadlines and the required update cadence;
avoid repeated reads of unchanged status and keep a final readback. Batch
eligible agents before waiting. Spawning agents one at a time and waiting on
each is not parallel fan-out.

Same-provider agents provide parallelism and context separation. They do not
provide vendor independence. Use the trusted review path only when another
provider materially reduces residual risk.

Set the review scope and correction-round budget before dispatch. Track queued
and running review requests until completion or confirmed cancellation. Target
follow-up reviews at fixes and affected boundaries. Repeat a full review only
when new evidence or changes justify it. A budget limit leaves unresolved
findings open; an approval does not settle another pending review.

## Keep durable runs small

Prefer the harness's native goal and wait mechanisms and follow their current
tool contracts for status changes, budgets, and continuation. `autonomous-run`
is experimental until resume and compaction are proven in executed runs. Add
ignored `.megapowers/run/<id>/` files only when work must resume after context
or process loss under a currently approved autonomous goal:

- `charter.md` freezes the outcome, boundaries, approvals, and cap.
- `checkpoint.md` records workspace and artifact identity, current milestone,
  evidence, delegate ownership, remaining approved side effects, blocker, and
  next safe command.
- `journal.jsonl` records observed transitions and their evidence.

Update durable state at real transitions, not every turn. On resume, reconcile
repository, worktree, branch, HEAD, runtime, and external state before acting.
Report unresolved blockers and stop affected work on missing evidence or a
workspace mismatch. Use read-only inspection to resolve uncertainty. Compaction
does not revoke existing approvals. Verify scope and approvals after a crash,
handoff, or harness change; native goals do not transfer automatically across
harnesses. Portable checkpoint labels do not change native goal status. A
journal proves only what its check proved.

## Stop rules

Set a bounded check before expensive work: a passing test, decision criterion,
candidate count, time budget, or retry limit. Three failed fixes on one approach
require a new diagnosis, not more retries. Side effects outside the machine
still require exact approval even inside an autonomous goal.
