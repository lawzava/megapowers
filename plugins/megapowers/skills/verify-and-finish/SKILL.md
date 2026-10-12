---
name: verify-and-finish
description: Use to verify a completed outcome before claiming success, handing off, committing, merging, publishing, deploying, or closing a task. Do not use for an intermediate update, an isolated test run, or merely naming an oracle command.
when_to_use: "Trigger phrases: prove this is done, commit, push, open the PR, merge, publish, deploy, ship it, wrap up, hand off, final completion check."
metadata:
  short-description: Verification and honest status before any completion claim
---

# Verify and Finish

First check, with the cheapest fresh oracle, whether the outcome already
holds. If nothing remains and no review or manual gate is pending, report a
verified no-op before broad checks; an external no-op needs an external
readback.

Evidence precedes every load-bearing claim. Run each acceptance oracle fresh
against the current artifact, read the whole result, and state only what it
proves. A focused test does not prove the full suite; artifact inspection does
not prove runtime behavior. Never weaken a test, baseline, threshold, or
predicate to obtain a pass.

Run long or verbose checks through the
[run receipt tool](references/run-receipt.md), which keeps full output, exit
status, and commit; a filtered or truncated summary proves nothing.

Label inspection and inference; neither is an executed check. Bind stale-prone
results to the artifact identity and commit. Rerun relevant checks after a
rebase onto a changed base; an identical patch does not establish the same
tested tree or dependencies. Separate declared configuration from effective
runtime behavior.

A local build cannot prove a deployment, published package, API effect, or
user-visible result; for those, exercise the real boundary and record the
environment and correlation identity. If a required tool or environment is
unavailable, report the criterion unverified instead of substituting a nearby
check.

When the change can affect a user journey, run that journey for real through
the surface the user will open; name any agreed substitute's limits. After
driving one journey by hand twice, offer a repeatable
[verification driver](references/verification-driver.md).

Resolve each affected specification requirement to fresh evidence or a
remaining gap; a plan is not implementation evidence.

Account for every requested review, including queued and running requests.
Join them and resolve credible findings.

Before a handoff, commit, PR, merge, or release, run canonical checks and inspect
the diff and workspace. Remove generated excess and report unrequested
behavior as scope creep. Confirm a named target branch before committing. Do
not commit, publish, merge, delete a branch, or remove a worktree without
authority for that destination. Destructive cleanup requires target
confirmation and ownership evidence. For a handoff to another harness,
directory, or person, write a [handoff note](references/handoff.md). Write a
pull request description from the [PR body guide](references/pr-body.md).

Open the report with `VERIFIED: <claim>` only when every required criterion has
fresh oracle evidence, and state what that evidence does not cover, such as
deployment or publication. Otherwise use `NOT VERIFIED. Remaining: <gap>`. Each listed gap or blocker
cites a rule's file and line or a command result; drop uncited ones.
