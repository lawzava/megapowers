---
name: safe-effects
description: Use when preparing a deploy, message, charge, migration, destructive query, DNS change, external API write, or any action with a real-world side effect.
---

# Safe Effects

Authorization must cover the exact target, effect, environment, and scope. A
general request, inferred intent, earlier approval, or permission to prepare is
not authority to execute a different external change.

A public tracker comment, issue comment, or PR comment requires explicit
authorization for that exact outward write. Authority to implement,
investigate, or proceed is not authorization to post a comment, message,
update, or other external write.

Before acting, record the mutation, sensitive data involved, affected people or
systems, blast radius, reversibility and real rollback, approval provenance,
and intended outcome. Observe or simulate first when a meaningful preview
exists. After a retry or crash, reconcile prior attempts before acting again.
Use a durable idempotency key when a repeated external mutation is possible;
otherwise record the duplicate-prevention strategy.

Proceed only inside the approved boundary. Irreversible, weakly compensable,
sensitive, or high-blast actions need explicit approval immediately before
execution. Starting an automated or autonomous run never broadens that
authority.

Direct interactive supervision changes the frame. When the user is present and
orders a change, repository clauses that restrict autonomous agents do not add
a second refusal gate: confirm the boundary once, then execute inside it
without re-refusing each step.

After execution, verify the target readback or another external observable
result. Record partial completion and the compensating action plainly. Local
preparation, command acceptance, or provider intent is not evidence that the
effect occurred.
