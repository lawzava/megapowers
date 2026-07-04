---
name: requesting-code-review
description: Use when you have finished a task or feature and want the changes reviewed before merging. Triggers on "review this", "is this ready to merge", "check my work". Distinct from verification-before-completion (which runs the code to prove it works) and finishing-a-development-branch (which chooses how to integrate).
---

# Requesting Code Review

Dispatch a code reviewer subagent to catch issues before they cascade. The reviewer gets precisely crafted context for evaluation — never your session's history. This keeps the reviewer focused on the work product, not your thought process, and preserves your own context for continued work.

**Core principle:** Review early, review often.

## When to Request Review

Request review after each task in subagent-driven development, after completing a major feature, and before merging to main.

It also pays off when you're stuck (a fresh perspective helps), before a refactor (baseline check), and after fixing a complex bug.

## How to Request

**1. Get the git range:**
```bash
BASE_SHA=$(git merge-base main HEAD)   # the commit the work branched from
HEAD_SHA=$(git rev-parse HEAD)
```
Use the branch point (`git merge-base <base-branch> HEAD`) as BASE, or the exact
commit you recorded before the work began. **Do not default to `HEAD~1`** — it
silently drops all but the last commit of a multi-commit task, so the reviewer
sees a fraction of the change and approves work it never read.

**2. (Preferred) Hand the reviewer its diff as a file.** So the diff never enters
your own context and the reviewer reads one file instead of re-deriving it, generate
a review package and pass its path. Use the `review-package` helper that ships with
the subagent-driven-development skill (resolve it from that skill's installed
`scripts/` directory; the path depends on how you installed megapowers, so don't
assume a repo-root-relative path):
```bash
<subagent-driven-development skill>/scripts/review-package "$BASE_SHA" "$HEAD_SHA"
# prints a unique file path containing: commit list + git diff --stat + full diff -U10
```
Without bash — or if you can't resolve the helper's path — redirect `git log --oneline`,
`git diff --stat`, and `git diff -U10` for the range into one uniquely named file yourself.

**3. Dispatch code reviewer subagent:**

Dispatch a `general-purpose` subagent, filling the template at [code-reviewer.md](code-reviewer.md)

**Placeholders:**
- `[DESCRIPTION]` - Brief summary of what you built
- `[PLAN_OR_REQUIREMENTS]` - What it should do
- `[REVIEW_PACKAGE_PATH]` - Path to the pre-generated diff file (preferred); leave blank to have the reviewer derive the diff from the SHAs
- `[BASE_SHA]` - Starting commit (branch point, not `HEAD~1`)
- `[HEAD_SHA]` - Ending commit

**4. Act on feedback:**
- Evaluate findings per megapowers:receiving-code-review — verify each against
  the code before implementing it
- Fix Critical issues immediately
- Fix Important issues before proceeding
- Note Minor issues for later
- Push back if the reviewer is wrong (with reasoning)

**Escalate risky diffs beyond same-model review.** For changes touching
billing, auth, concurrency, or security, add an independent different-vendor
pass via mega-orchestration:cross-model-verification (if installed) — a
same-model review shares the author's blind spots.

**Harness-native deep review.** On Claude Code, `/code-review ultra` (alias
`/ultrareview`; `claude ultrareview --json` for CI) runs a cloud fleet of
reviewer agents that independently reproduce their findings, a deeper
same-vendor pass than the subagent above. Being same-vendor, it complements
rather than replaces the cross-model verification, which is what catches a
single vendor's blind spots.

## Example

```
[Just completed Task 2: Add verification function]

You: Let me request code review before proceeding.

BASE_SHA=<the commit recorded before Task 2 began>   # not HEAD~1
HEAD_SHA=$(git rev-parse HEAD)
[Generate the review package for BASE_SHA..HEAD_SHA; note the printed path]

[Dispatch code reviewer subagent]
  DESCRIPTION: Added verifyIndex() and repairIndex() with 4 issue types
  PLAN_OR_REQUIREMENTS: Task 2 from docs/megapowers/plans/deployment-plan.md
  REVIEW_PACKAGE_PATH: .megapowers/sdd/review-task-2.md
  BASE_SHA: a7981ec
  HEAD_SHA: 3df7661

[Subagent returns]:
  Strengths: Clean architecture, real tests
  Issues:
    Important: Missing progress indicators
    Minor: Magic number (100) for reporting interval
  Assessment: Ready to proceed

You: [Fix progress indicators]
[Continue to Task 3]
```

## Integration with Workflows

**Subagent-Driven Development:**
- Review after each task
- Catch issues before they compound
- Fix before moving to the next task

**Executing Plans:**
- Review after each task or at natural checkpoints
- Get feedback, apply, continue

**Ad-Hoc Development:**
- Review before merge
- Review when stuck

## Keep the Discipline

Review is still worth it when the change looks simple — that's often where a quiet bug hides. Address Critical issues before you continue, and resolve Important issues before proceeding rather than deferring them.

Do not argue with valid feedback. Push back only when the reviewer is factually wrong, and back it with technical reasoning: show the code or tests that prove the behavior, and ask for clarification where the feedback is ambiguous.
