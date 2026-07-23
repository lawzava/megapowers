---
name: requesting-code-review
description: Use when finished work needs review before merge. Triggers on "review this", "ready to merge", or "check my work". Verify behavior first.
license: MIT
---

# Requesting Code Review

Dispatch a code reviewer subagent to catch issues before they cascade. Request
fresh context explicitly, then give the reviewer a crafted evaluation package.
Do not let a fork default inherit the author's reasoning: that biases the
review toward the implementation it should challenge.

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

**Scope the diff correctly.** BASE_SHA is the branch point, `git merge-base <base-branch> HEAD`, or the exact commit you recorded before the work began; HEAD_SHA is the current commit. Never default to `HEAD~1`: it silently drops all but the last commit of a multi-commit task, so the reviewer sees a fraction of the change and approves work it never read.

**Package the diff (preferred).** Generate a review package so the diff never enters your own context and the reviewer reads one file instead of re-deriving it. Use the `review-package` helper that ships with megapowers:subagent-driven-development, resolved from that skill's installed `scripts/` directory (the path depends on how megapowers was installed; never assume it is repo-root-relative):

```bash
<subagent-driven-development skill>/scripts/review-package "$BASE_SHA" "$HEAD_SHA"
```

It prints the path of a unique file containing the commit list, the diff stat, and the full diff with context. If you cannot run bash or resolve the helper, build an equivalent single file yourself from `git log --oneline`, `git diff --stat`, and `git diff -U10` for the range.

**Dispatch the reviewer.** Fill the template at [code-reviewer.md](code-reviewer.md) and dispatch it as a `general-purpose` subagent. It takes a brief description of what you built, the plan or requirements, the review package path (leave it blank to have the reviewer derive the diff from the SHAs), BASE_SHA, and HEAD_SHA.

**Act on the findings.** Handle them per megapowers:receiving-code-review and
keep the axes distinct. A Specification Compliance Fail blocks proceeding
regardless of local finding severity: correct the implementation or obtain
explicit requirement-owner authorization, then re-review. For Engineering
Standards, fix Critical issues immediately, fix Important issues before
proceeding, and record Minor issues for later. Do not argue with valid feedback;
push back only when the reviewer is factually wrong, backed by the code or tests
that prove the behavior.

## Escalation

Changes touching billing, auth, concurrency, or security get an independent different-vendor pass via mega-orchestration:cross-model-verification (if installed); a same-model review shares the author's blind spots. On Claude Code, `/code-review ultra` runs a deeper same-vendor fleet review; being same-vendor, it complements the cross-model pass rather than replacing it.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
