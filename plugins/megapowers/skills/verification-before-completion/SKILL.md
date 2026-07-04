---
name: verification-before-completion
description: Use when about to claim work is complete, fixed, or passing, before committing or creating PRs - requires running verification commands and confirming output before making any success claims; evidence before assertions always
license: MIT
---

# Verification Before Completion

## Overview

Claiming work is complete without verification isn't efficiency — it's a guess dressed up as a result.

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

Each step matters. Skipping one turns a verified result back into a guess.

## Common Failures

| Claim | Requires | Not sufficient |
|-------|----------|----------------|
| Linter clean | Linter output: 0 errors | Partial check, extrapolation |
| Bug fixed | Test original symptom: passes | Code changed, assumed fixed |
| Uses dependency X | X actually resolvable and exercised | A silent fallback that hides X being unavailable |

The patterns below cover tests, builds, regression tests, requirements, and agent delegation.

## Unavailable Requirements

If a required dependency, tool, or input is unavailable, say so explicitly — a completion claim that hides a missing requirement is a false claim. A workaround or fallback never silently satisfies the requirement: disclose the substitution and report the requirement itself as unmet (blocked or partial) until the human accepts the substitute.

## Signs to Pause and Verify

- Reaching for "should", "probably", "seems to".
- Expressing satisfaction before verification ("Great!", "Perfect!", "Done!").
- About to commit, push, or open a PR without verification.
- Trusting an agent's success report at face value.
- Relying on a partial check.
- Treating this as a one-time exception.
- Being tired and wanting the work over.
- Any wording that implies success when you haven't run verification.

## Rationalizations to watch for

- "Should work now" — run the verification instead.
- "I'm confident" — confidence isn't evidence.
- "Just this once" — the rule holds every time.
- "Linter passed" — the linter isn't the compiler.
- "Agent said success" — verify it independently.
- "I'm tired" — that doesn't change what's true.
- "Partial check is enough" — a partial check proves nothing about the whole.
- "Different words, so the rule doesn't apply" — the spirit governs, not the phrasing.

## Key Patterns

**Tests:**
```
Good: [Run test command] [See: 34/34 pass] "All tests pass"
Bad:  "Should pass now" / "Looks correct"
```

**Regression tests (TDD Red-Green):**
```
Good: Write → Run (pass) → Revert fix → Run (must fail) → Restore → Run (pass)
Bad:  "I've written a regression test" (without red-green verification)
```

**Build:**
```
Good: [Run build] [See: exit 0] "Build passes"
Bad:  "Linter passed" (the linter doesn't check compilation)
```

**Requirements:**
```
Good: Re-read plan → Create checklist → Verify each → Report gaps or completion
Bad:  "Tests pass, phase complete"
```

**Agent delegation:**
```
Good: Agent reports success → Check VCS diff → Verify changes → Report actual state
Bad:  Trust the agent report
```

## Why This Matters

Drawn from repeated failures where skipping verification cost real trust and time:

- A partner said "I don't believe you" — trust broken.
- Undefined functions shipped that would crash on run.
- Missing requirements shipped as "complete" features.
- Time lost to a false completion, then the redirect, then the rework.
- Honesty is a core value; a confident false claim undermines it.

## When to Apply

Always, before:

- Any variation of a success or completion claim.
- Any expression of satisfaction with the work.
- Any positive statement about the state of the work.
- Committing, opening a PR, or marking a task done.
- Moving to the next task.
- Delegating to agents.

The rule covers exact phrases, paraphrases and synonyms, implications of success, and any communication that suggests completion or correctness.

## The Bottom Line

Run the command. Read the output. Then claim the result.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
