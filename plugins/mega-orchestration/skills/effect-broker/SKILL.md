---
name: effect-broker
description: >-
  Use before any real-world side effect that leaves the working tree — a deploy,
  an email/notification send, a payment, a DB migration or destructive query, a
  DNS change, an external API write. Classify the action by how reversible it is,
  then simulate-then-commit the risky ones with approval scaled to the autonomy
  level. Triggers on "deploy", "send", "charge", "run this migration", "delete the
  production ...", "make this change live", or any irreversible external action.
---

# Effect Broker

Trust is the product: people delegate what they can trust not to do something
irreversible they didn't want. Guarding that by string-parsing shell commands is a
losing arms race (a parser can't see intent, and every parser has bypasses). The
honest place to gate irreversibility is on **what the action is** — so the caller
*declares* the action's class and the broker enforces the right protocol. This
composes with the autonomy dial from mega-orchestration:autonomous-run.

## Classify the effect

| Class | What it is | Examples |
|---|---|---|
| **reversible** | Undoable locally; a `git revert`/restore fixes it. | edit a file, read data, write to a scoped temp dir, create a branch |
| **staged** | Has a native dry-run / plan / preview and a defined undo. | `terraform apply` (has `plan`), `kubectl apply` (has `--dry-run`), a migration with up/down, a push to a non-default branch |
| **irreversible** | No real undo and real blast radius. | prod deploy, sending email/notifications, a payment/refund, `DROP`/`DELETE` without a backup, a DNS cutover, deleting a cloud resource |

When unsure between two classes, pick the more dangerous one.

Run `scripts/effect-broker <class> [--level <autonomy-level>]` to get the required
protocol as `KEY=VALUE` (DRY_RUN, IDEMPOTENCY, JOURNAL, APPROVAL, PROCEED).

## The protocol (simulate → commit)

- **reversible** — just do it, at any autonomy level. No dry-run, no approval; a
  journal line is optional.
- **staged** — **dry-run first, always.** Produce the plan/preview artifact
  (`terraform plan`, `--dry-run=server`, the migration's forward SQL), diff it
  against intent, and record it. Then commit. Use an **idempotency key** where the
  API supports it so a retry can't double-apply. A human gates this only at
  `in-the-loop`.
- **irreversible** — **stage a plan and get approval at every level, including
  `autonomous`.** It never auto-fires. Write the plan (what will happen, to what,
  and the blast radius), get explicit approval per the autonomy level, use an
  idempotency key, perform it, and record the commit in the run journal for replay.
  Respect the charter's blast-radius caps (e.g. "no prod", "no external sends") —
  a capped action is refused, not approved.

## Idempotency and blast radius

- **Idempotency:** for any staged/irreversible external write, derive a stable key
  (e.g. a hash of the request) and pass it so a retry after a timeout is a no-op,
  not a second charge/send/deploy.
- **Blast radius:** the charter (mega-orchestration:autonomous-run) declares what a
  run may not touch. Enforcing it is the caller's job — the broker helper prints
  `BLAST_RADIUS=caller-enforced-vs-charter`, and you (the agent following this
  protocol) refuse an action outside the charter's bounds rather than asking for
  approval to exceed them. The helper classifies and states the protocol; it does
  not itself read the charter.

## Honest failure reporting

If a committed effect fails or partially applies, say so plainly in the journal and
the run report — what succeeded, what didn't, and whether a compensating action is
needed. A half-applied irreversible effect is the case where honesty matters most.

## Relationship to the guardrail hooks

The `deny-destructive` PreToolUse hook (Claude Code only) is a thin, last-ditch
tripwire for a few unambiguous local catastrophes — not the irreversibility layer.
This skill is that layer, and it is portable: it works by declaration on every
runtime, with no dependency on a hook firing.

On Claude Code (v2.1.198+) the harness also enforces that no agent message counts
as a human approval or can change permissions or config; on Codex, OpenCode, and
Antigravity that guarantee is this skill's wording only. A scheduled or cloud
runner skips permission prompts entirely, so an irreversible action's approval
gate must live in the run prompt itself; mega-orchestration:autonomous-run
carries the full caveat.
