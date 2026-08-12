---
name: effect-broker
description: >-
  Use before a deploy, send, charge, database migration, destructive query, DNS
  change, external API write, or other costly real-world side effect.
license: MIT
---

# Effect Broker

Use this before any action that may change data, systems, people, or external state.
Starting an autonomous workflow does not authorize an effect.

## Effect record

Classify the proposed action before executing it. Keep these dimensions separate:

| Dimension | Record |
|---|---|
| Mutation | What changes, versus a read-only observation. |
| Sensitivity | Credentials, personal, regulated, financial, or other protected data touched. |
| Externality | Which person, service, account, or system observes the change. |
| Compensability | Whether a real rollback exists, its limits, and who can perform it. |
| Environment and blast radius | Target environment, scope, affected identities, and bounded maximum impact. |
| Approval provenance | Who authorized this exact effect, when, scope, and evidence of that authority. |

Inputs: the effect record, intended outcome, available simulation, autonomy level, and
existing authorization. Output: proceed, simulate, defer, or refuse, with the record
and evidence needed for a later decision.

`scripts/effect-broker` (`effect-broker.go`) `<reversible|staged|irreversible> [--level <level>]` is a compact
accelerator for the compensability and oversight portion of this record. It accepts
three classes: reversible actions have a real local undo, staged actions have a
meaningful preview and defined undo, and irreversible actions have no reliable undo.
Levels are `autonomous`, `on-the-loop`, and `in-the-loop`, from least to most
interactive oversight. The helper does not replace the other dimensions or validate
authorization.

## Protocol

1. Observe or simulate first whenever a meaningful preview exists. Compare the result
   with the intended outcome and declared blast radius.
2. Require an idempotency or duplicate-prevention strategy when retries could repeat
   the effect.
3. Subject to the level disposition below, proceed only for actions inside the
   authorized scope whose impact is genuinely recoverable.
4. Record partial completion and the required compensating action plainly. Never claim
   success from intent, a plan, or a local preparation step.

Level disposition remains explicit: a reversible action inside scope may proceed; a
staged action always previews, requires human approval at `in-the-loop`, and may proceed
within scope at other levels; an irreversible, poorly compensable, sensitive, or
high-blast action requires approval at every level.

## Safety and oracle

An approval is valid only when it covers the exact target, effect, environment, and
scope. A chat inference, generic permission, or stale approval is not provenance. The
oracle is the target system's readback or other observable result, plus the effect
record. Command parsing and local hooks are optional tripwires, not authorization.
