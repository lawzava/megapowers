---
name: verify-and-finish
description: Use to verify a completed outcome before claiming success, handing off, committing, merging, publishing, deploying, or closing a task. Do not use for an intermediate update, an isolated test run, or merely naming an oracle command.
when_to_use: "Trigger phrases: prove this is done, commit, push, open the PR, merge, publish, deploy, ship it, wrap up, hand off, final completion check."
metadata:
  short-description: Verification and honest status before any completion claim
---

# Verify and Finish

First check, with the cheapest fresh oracle, whether the outcome already holds.
If nothing remains and no review or manual gate is pending, report a verified
no-op before broad checks; an external no-op needs an external readback.

Evidence precedes every load-bearing claim. Run each acceptance oracle fresh
against the current artifact, read the whole result, and state only what it
proves. Never weaken a test, baseline, threshold, or predicate to obtain a
pass; after a code change, run the [bar guard](references/bar-guard.md) and fix
or report each finding. Run long or verbose checks through the
[run receipt tool](references/run-receipt.md); block on a running check and
never poll with `sleep`.

Label inspection and inference; neither is an executed check. Rerun relevant
checks after a rebase onto a changed base. Separate declared configuration from
effective runtime behavior. A local build cannot prove a deployment, published
package, API effect, or user-visible result; exercise the real boundary and
record the environment and correlation identity, or report the criterion
unverified. A claim that something will recur ("from now on", "every workday")
names its schedule, hook, or config entry and cites a readback of it.

When the change can affect a user journey, run that journey through the surface
the user will open; name any agreed substitute's limits. After driving one
journey by hand twice, offer a repeatable
[verification driver](references/verification-driver.md). Resolve each affected
specification requirement to fresh evidence or a remaining gap, and join every
requested review before finishing.

Before a handoff, commit, PR, merge, or release, run canonical checks and
inspect the diff and workspace. Remove generated excess and report unrequested
behavior as scope creep. Keep review packages, receipts, and evidence records
under `$TMPDIR`, outside the deliverable, unless the user asks otherwise; review
provenance belongs in the report, not in project files.
Confirm a named target branch and authority before
committing, publishing, merging, or deleting a branch or worktree. For a
handoff, write a [handoff note](references/handoff.md); for a PR, use the
[PR body guide](references/pr-body.md).

Open the report with `VERIFIED: <claim>` only when every required criterion has
fresh oracle evidence, and state what that evidence does not cover. Otherwise
use `NOT VERIFIED. Remaining: <gap>`, citing each gap's rule or command result.
