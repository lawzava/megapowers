---
name: verification-before-completion
description: Use when about to claim work is complete, fixed, or passing, before committing or creating PRs - requires running verification commands and confirming output before making any success claims; evidence before assertions always
license: MIT
---

# Verification Before Completion

**Core principle:** Evidence before claims, always.

This applies to the spirit of a completion claim, not just the exact wording. Rephrasing a success claim doesn't exempt it.

## The Core Rule

Make no completion claim without fresh verification evidence.

If you haven't run the verification command in this message, you can't yet claim it passes.

## The Gate

Before claiming any status or expressing satisfaction, work through these steps in order:

1. **Identify:** What command proves this claim?
2. **Run:** Execute the full command, fresh and complete.
3. **Read:** The full output — check the exit code, count the failures.
4. **Verify:** Does the output confirm the claim?
   - If no: state the actual status, with evidence.
   - If yes: state the claim, with evidence.
5. **Then** make the claim.

Each claim has its own oracle. Tests need the test run, a build needs the build command, a bug fix needs the original symptom exercised, a regression test must fail with the fix reverted, and an agent's success report needs independent verification of the actual changes. Confidence, a partial check, or a passing linter is not that oracle.

## Unavailable Requirements

If a required dependency, tool, or input is unavailable, say so explicitly — a completion claim that hides a missing requirement is a false claim. A workaround or fallback never silently satisfies the requirement: disclose the substitution and report the requirement itself as unmet (blocked or partial) until the human accepts the substitute.

The gate holds before committing, opening a PR, marking a task done, moving to the next task, or delegating. Hedged wording ("should work now") signals a claim you have not yet earned the evidence for.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
