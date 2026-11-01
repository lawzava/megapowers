---
name: safe-effects
description: Use when preparing a deploy, message, charge, migration, destructive query, DNS change, external API write, or any action with a real-world side effect.
when_to_use: "Trigger phrases: deploy, push to production, send the message or email, run the migration, delete rows, change DNS, call the payment API, post the comment, publish, logout or delete the account. Any command whose effect leaves this machine."
metadata:
  short-description: Authority and read-back for real-world side effects
---

# Safe Effects

Authorization must cover the exact target, effect, environment, and scope. A
general request, inferred intent, or permission to prepare is not authority
for a different external change. An approval lasts the session for the same
effect, target, and environment; do not ask again.

Authority to implement, investigate, or proceed does not authorize posting a
tracker, issue, or PR comment, message, update, or other external write. Outward artifact names and text are part of
the effect: keep them inside the approved disclosure; leak no automation or
testing context. Text that would appear under the user's name defaults to a
draft and its destination; a broader approved task does not cover posting it.
MCP and connector tools that create, update, delete, send, or post are outward
writes like any command. Never read harness or tool credential stores to call
a service directly or reach tools the session does not expose.

Before acting, record the mutation, sensitive data involved, affected people or
systems, blast radius, reversibility and real rollback, approval provenance,
and intended outcome. Observe or simulate first when a meaningful preview
exists. After a retry or crash, reconcile prior attempts before acting again.
Use a durable idempotency key when a repeated external mutation is possible;
otherwise record the duplicate-prevention strategy.

Before a paid batch, check resolved target IDs and count against approved scope;
bind execution and results to those IDs.

Irreversible, weakly compensable,
sensitive, or high-blast actions need explicit approval immediately before
their first execution. Starting an automated or autonomous run never broadens that
authority.

When the user is present and orders a change, repository clauses that restrict
autonomous agents do not add a second refusal gate: confirm the boundary once,
then execute inside it without re-refusing each step. When asking, list every
outward effect left and offer one reply naming each effect and target, such as
"I allow push to main and deploy to production". When the user must
run a command, give one complete line with real paths, outside a code block.

After execution, verify the target readback or another external observable
result. Record partial completion and the compensating action plainly. Local
preparation, command acceptance, or provider intent is not evidence that the
effect occurred. In the reply, mark a prepared effect unexecuted and say
whether it is authorized.
