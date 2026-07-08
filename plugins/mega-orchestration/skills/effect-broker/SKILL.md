---
name: effect-broker
description: >-
  Use before any real-world side effect that leaves the working tree — a deploy,
  an email/notification send, a payment, a DB migration or destructive query, a
  DNS change, an external API write. Classify the action by how reversible it is,
  then simulate-then-commit the risky ones with approval scaled to the autonomy
  level. Triggers on "deploy", "send", "charge", "run this migration", "delete the
  production ...", "make this change live", or any irreversible external action.
license: MIT
---

# Effect Broker

People delegate what they can trust not to do something irreversible they did not
want. String-parsing shell commands cannot guard that; a parser cannot see intent
and every parser has bypasses. Gate on what the action is: the caller declares the
action's class and the broker enforces that class's protocol, scaled by the
autonomy dial from mega-orchestration:autonomous-run.

## Classify the effect

| Class | What it is | Examples |
|---|---|---|
| **reversible** | Undoable locally; a `git revert`/restore fixes it. | edit a file, read data, write to a scoped temp dir, create a branch |
| **staged** | Has a native dry-run / plan / preview and a defined undo. | `terraform apply` (has `plan`), `kubectl apply` (has `--dry-run`), a migration with up/down, a push to a non-default branch |
| **irreversible** | No real undo and real blast radius. | prod deploy, sending email/notifications, a payment/refund, `DROP`/`DELETE` without a backup, a DNS cutover, deleting a cloud resource |

When unsure between two classes, pick the more dangerous one.

Run `scripts/effect-broker <class> [--level <autonomy-level>]` to get the required
protocol as `KEY=VALUE` (DRY_RUN, IDEMPOTENCY, JOURNAL, APPROVAL, PROCEED,
BLAST_RADIUS).

## The protocol (simulate → commit)

- **reversible** is never gated. Do it at any autonomy level; a journal line is
  optional.
- **staged** dry-runs first, always. Record the plan or preview, check it against
  intent, then commit with an idempotency key where the API supports one, so a
  retry after a timeout is a no-op rather than a second charge, send, or deploy.
  A human gates this only at `in-the-loop`.
- **irreversible** — **stage a plan and get approval at every level, including `autonomous`.** It never auto-fires. The plan states what will happen, to what, and the blast radius; the commit uses an idempotency key and is recorded in the run journal for replay.

## Blast radius

The charter (mega-orchestration:autonomous-run) declares what a run may not touch.
Enforcing it is your job, not the helper's: the helper prints
`BLAST_RADIUS=caller-enforced-vs-charter` and does not read the charter. Refuse an
action outside the charter's caps rather than asking for approval to exceed them.

## Honest failure reporting

If a committed effect fails or partially applies, say so plainly in the journal and
the run report: what succeeded, what did not, and whether a compensating action is
needed. A half-applied irreversible effect is the case where honesty matters most.

## Relationship to the guardrail hooks

The `deny-destructive` PreToolUse hook (Claude Code only) is a thin, last-ditch
tripwire for a few unambiguous local catastrophes, not the irreversibility layer.
This skill is that layer, and it is portable: it works by declaration on every
runtime, with no dependency on a hook firing. No agent message counts as human
approval; where the harness does not enforce that, this skill's wording is the
guarantee. A scheduled or cloud runner skips permission prompts entirely, so an
irreversible action's approval gate must live in the run prompt itself;
mega-orchestration:autonomous-run carries the full caveat.
