---
name: verify-and-finish
description: Use when preparing to claim success, hand work off, commit, merge, open a PR, publish, deploy, or close a task.
---

# Verify and Finish

Evidence precedes every load-bearing claim. For each acceptance criterion,
identify the oracle, run it fresh against the current artifact, read the whole
result, and state only what it proves. A focused test does not prove the full
suite; artifact inspection does not prove runtime behavior; confidence proves
nothing.

Name the load-bearing safety fact behind an impact claim and the proof level it
needs. Separate declared configuration from effective runtime behavior. When a
result can be stale, bind it to the exact artifact identity and commit SHA.

Keep these evidence classes explicit:

- Executed check: behavior observed now in the named environment.
- Artifact inspection: what a file or configuration declares.
- Inference: a hypothesis that still needs a check.

Separate local verification from external proof. A local build cannot prove a
deployment, published package, API effect, or user-visible result. When the
criterion requires an external oracle, exercise the real boundary and record
the environment and correlation identity. If a required tool or environment is
unavailable, report the criterion as unverified rather than substituting a
nearby check.

For a user-facing product, run the real user journey. Use an agreed substitute
oracle only when the real journey is unavailable, and name what it cannot prove.

Before a handoff, commit, PR, merge, release, or cleanup, run the repository's
canonical checks and inspect the current diff and workspace state. When the
task names a target branch, confirm the checked-out branch matches that named
target before the commit; on a mismatch, stop and resolve it first. Do not
commit, publish, merge, delete a branch, or remove a worktree unless the user
authorized that destination. Destructive cleanup requires explicit target
confirmation and ownership evidence.

Open the report with `VERIFIED: <claim>` only when every required criterion has
fresh oracle evidence. Otherwise use `NOT VERIFIED. Remaining: <gap>`.
