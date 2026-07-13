---
name: requesting-code-review
description: Use when finished work needs review before merge. Triggers on "review this", "ready to merge", or "check my work". Verify behavior first.
license: MIT
---

# Requesting Code Review

Dispatch a code reviewer subagent to catch issues before they cascade. The reviewer gets crafted context for evaluation, never your session history. That keeps it focused on the work product, not your thought process, and preserves your own context for continued work.

**Core principle:** Review early, review often.

Review after each task in subagent-driven development, after completing a major feature, and before merging. It also pays off when you are stuck, before a refactor, and after fixing a complex bug. A change that looks simple still deserves review; that is often where a quiet bug hides.

## The Review

**Scope the diff correctly.** BASE_SHA is the branch point, `git merge-base <base-branch> HEAD`, or the exact commit you recorded before the work began; HEAD_SHA is the current commit. Never default to `HEAD~1`: it silently drops all but the last commit of a multi-commit task, so the reviewer sees a fraction of the change and approves work it never read.

**Package the diff (preferred).** Generate a review package so the diff never enters your own context and the reviewer reads one file instead of re-deriving it. Use the `review-package` helper that ships with megapowers:subagent-driven-development, resolved from that skill's installed `scripts/` directory (the path depends on how megapowers was installed; never assume it is repo-root-relative):

```bash
<subagent-driven-development skill>/scripts/review-package "$BASE_SHA" "$HEAD_SHA"
```

It prints the path of a unique file containing the commit list, the diff stat, and the full diff with context. If you cannot run bash or resolve the helper, build an equivalent single file yourself from `git log --oneline`, `git diff --stat`, and `git diff -U10` for the range.

**Dispatch the reviewer.** Fill the template at [code-reviewer.md](code-reviewer.md) and dispatch it as a `general-purpose` subagent. It takes a brief description of what you built, the plan or requirements, the review package path (leave it blank to have the reviewer derive the diff from the SHAs), BASE_SHA, and HEAD_SHA.

**Act on the findings.** Handle them per megapowers:receiving-code-review: verify each finding against the code before implementing it, fix Critical issues immediately, fix Important issues before proceeding, and record Minor issues for later. Do not argue with valid feedback; push back only when the reviewer is factually wrong, backed by the code or tests that prove the behavior.

## Escalation

Changes touching billing, auth, concurrency, or security get an independent different-vendor pass via mega-orchestration:cross-model-verification (if installed); a same-model review shares the author's blind spots. On Claude Code, `/code-review ultra` runs a deeper same-vendor fleet review; being same-vendor, it complements the cross-model pass rather than replacing it.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
