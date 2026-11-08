---
name: verify-and-finish
description: "Before claiming completion, run acceptance checks on the current artifact and await required reviews; local checks do not prove external outcomes. Load before commit, push, PR, merge, publish, or deploy, for release/handoff gates, or uncertain verification boundaries. A routine local edit, isolated test run, or status-only answer needs evidence, not this full workflow."
---

# Verify and Finish

Check whether the outcome already holds with the cheapest fresh oracle. Report
a verified no-op before broad checks only if no work, review, or manual gate
remains; an external no-op needs external readback.

Run each acceptance oracle fresh against the current artifact. Read the whole
result; claim only what it proves. An empty search proves absence only after
finding a known positive. Never weaken a test, baseline, threshold, or predicate
to obtain a pass. After any code change that touches tests, baselines,
thresholds, or predicates, run the [bar guard](references/bar-guard.md) and fix
or report each finding. Inspect the diff for missing criteria and scope changes.
Preserve exit status and output.
Use [run receipts](references/run-receipt.md) for checks that can outlive the
tool timeout or lose output. Block on completion, not a guessed wait.

Label inspection and inference, not as executed checks. Rerun relevant checks
after a rebase onto a changed base. Distinguish declared configuration from
runtime behavior. Exercise the claimed boundary, recording environment and
correlation identity, or mark it unverified. A local build cannot prove external
or user-visible outcomes. Recurrence claims need schedule, hook, or config readback.

Run affected user journeys through the user's surface; name agreed substitutes'
limits. After driving one journey twice, offer a repeatable
[verification driver](references/verification-driver.md). Map each requirement
to fresh evidence or a gap. Join every requested review before finishing;
follow user rules for [independent review](../independent-review/SKILL.md).

Before handoff, commit, PR, merge, or release, run canonical checks and inspect
the diff and workspace. Remove generated excess; report scope creep. Keep
review packages and evidence under `$TMPDIR` unless asked otherwise. Report
review provenance. Confirm target branch and authority before commits,
publication, merges, or branch/worktree deletion. For cross-session resumption,
use the [handoff guide](references/handoff.md); for PRs, the
[PR body guide](references/pr-body.md).

Open with `VERIFIED: <claim>` only when every required criterion has fresh oracle
evidence; state what that evidence does not cover. Otherwise use
`NOT VERIFIED. Remaining: <gap>`, citing the rule or result behind each gap.
