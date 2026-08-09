---
name: requesting-code-review
description: Use when finished work needs review before merge. Triggers on "review this", "ready to merge", or "check my work". Verify behavior first.
license: MIT
---

# Requesting Code Review

Dispatch a code reviewer to catch issues before they cascade. Give the reviewer
the requirements and a complete evaluation package.

**Core principle:** Review in proportion to risk, with independence where it
changes confidence.

- Low-risk work: focused tests plus self-review. One branch review is optional.
- Medium-risk work: one independent review at the task, milestone, or branch
  boundary.
- High-risk work: review each risky boundary and perform final independent
  verification. Auth, billing, security, concurrency, schema or data changes,
  and external side effects are high risk.

Do not stack task and branch reviews unless the risk warrants both. A stalled
or uncertain change may be reviewed earlier regardless of tier.

## The Review

**Scope the diff correctly.** BASE_SHA is the branch point, `git merge-base
<base-branch> HEAD`, or the exact commit you recorded before the work began;
HEAD_SHA is the current commit. Never default to `HEAD~1`: it silently drops
all but the last commit of a multi-commit task, so the reviewer sees a fraction
of the change and approves work it never read.

**Package the complete task surface (preferred).** Generate a review package
with committed range changes plus every staged, unstaged, and untracked change.
This is required even when commits are not authorized: an empty commit range
does not prove the task is empty. Use the `review-package` helper that ships
with `megapowers:subagent-driven-development`. Resolve that skill's installed
directory, then run its `scripts/review-package` helper with `BASE_SHA` and
`HEAD_SHA` as the two positional arguments.

It prints a file containing the commit list, diff stat, full committed diff,
staged diff, unstaged diff, and untracked file diffs. If the helper is
unavailable, build an equivalent file. A reviewer must not approve until every
section is present or explicitly empty.

**Dispatch the reviewer.** Fill the template at
[code-reviewer.md](code-reviewer.md) and dispatch it as a reviewer. It takes a
brief description of what you built, the plan or requirements, complete review
package path, BASE_SHA, and HEAD_SHA.

**Act on the findings.** Handle them per megapowers:receiving-code-review and
keep the axes distinct. A Specification Compliance Fail blocks proceeding
regardless of local finding severity: correct the implementation or obtain
explicit requirement-owner authorization, then re-review. For Engineering
Standards, fix Critical issues immediately, fix Important issues before
proceeding, and record Minor issues for later. Do not argue with valid
feedback; push back only when the reviewer is factually wrong, backed by the
code or tests that prove the behavior.

## Escalation

Changes touching billing, auth, concurrency, or security get an independent
review through mega-orchestration:cross-model-verification when available.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent),
https://github.com/obra/superpowers.
