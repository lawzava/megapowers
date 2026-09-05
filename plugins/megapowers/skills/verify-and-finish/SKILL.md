---
name: verify-and-finish
description: Use to verify a completed outcome before claiming success, handing off, committing, merging, publishing, deploying, or closing a task. Do not use for an intermediate update, an isolated test run, or merely naming an oracle command.
when_to_use: "Trigger phrases: prove this is done, commit, push, open the PR, merge, publish, deploy, ship it, wrap up, hand off, final completion check."
metadata:
  short-description: Verification and honest status before any completion claim
---

# Verify and Finish

Start with the cheapest fresh state or oracle that can establish whether the
requested outcome already holds. If it shows no change or effect remains and no
review or manual gate is pending, report a verified no-op and stop before broad
checks. An external no-op needs an external readback; local proof cannot
establish an external state or effect.

Evidence precedes every load-bearing claim. Run each acceptance oracle fresh
against the current artifact, read the whole result, and state only what it
proves. A focused test does not prove the full suite; artifact inspection does
not prove runtime behavior.

Label artifact inspection and inference instead of presenting either as an
executed check. Bind stale-prone results to the artifact identity and commit.
Separate declared configuration from effective runtime behavior.

Separate local verification from external proof. A local build cannot prove a
deployment, published package, API effect, or user-visible result. When the
criterion requires an external oracle, exercise the real boundary and record
the environment and correlation identity. If a required tool or environment is
unavailable, report the criterion as unverified rather than substituting a
nearby check.

For a user-facing product, run the real journey. If an agreed substitute is
necessary, name what it cannot prove.

Reconcile each affected repository-owned specification with verified behavior.
Resolve every requirement to fresh evidence or a remaining gap; a plan is not
implementation evidence.

Account for every requested review, including queued and running requests.
Join them and resolve credible findings. One approval does not cancel another
pending review. Bind checks and gates to the current artifact, and reassess
after changes. A pending review remains open work.

Before a handoff, commit, PR, merge, or release, run canonical checks and inspect
the diff and workspace. Remove generated excess. Confirm a named target branch
before committing. Do not commit, publish, merge, delete a branch, or remove a
worktree without authority for that destination. Destructive cleanup requires
target confirmation and ownership evidence.

Open the report with `VERIFIED: <claim>` only when every required criterion has
fresh oracle evidence. Otherwise use `NOT VERIFIED. Remaining: <gap>`.
