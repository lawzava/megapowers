---
name: test-first-implementation
description: Use when adding or changing behavior, fixing a confirmed bug, refactoring executable code, or implementing a feature with tests.
when_to_use: "Trigger phrases: implement, add the feature, fix the confirmed bug, refactor, change behavior in code, make the tests pass, add tests for this."
metadata:
  short-description: Failing test first, then minimal implementation
---

# Test-First Implementation

Production code follows a failing test.

First inspect the entry point, callers, nearest convention, and coverage.
Use `systematic-debugging` for unknown causes.

## Red, green, refactor

1. Test real behavior at a stable observable boundary, not through mock
   assertions or a side-channel query. Take expected values from the
   specification or a worked example, never from a recomputation of the code.
2. Verify red: confirm failure for the missing behavior, excluding setup errors
   and typos. When you force red by mutating code, diff against the pristine
   file to prove the mutation landed.
3. Implement only what satisfies that test. Avoid speculative options,
   abstractions, and adjacent cleanup.
4. Verify green: run the focused test, read its full result, and fix production
   code on failure.
5. Refactor changed code while green. Repeat for the next behavior.

Understand dependencies before mocking. Do not add production APIs only for
tests. Run the broader suite at task or milestone completion, before claiming success.

## Code judgment

Before writing custom code, check existing code, the standard library, native
platform features, and installed dependencies. Require behavioral fit; justify
new dependencies by missing capability or maintenance benefit.

When simplifying changed code, preserve public contracts, trust-boundary
validation, data-loss error handling, security, accessibility, and required
tests. Do not sacrifice readability or correctness for line count. One caller
or implementation does not make a boundary unnecessary.

For a deliberate simplification with a known limit, comment its ceiling and
revisit condition using local conventions. Ordinary code needs no debt marker.

Validate untrusted data once at entry and make error ownership explicit:
expected, retryable, or terminal.

Load exactly one language reference when local code cannot settle a
language-specific engineering decision:

- Go: [references/go.md](references/go.md)
- Python: [references/python.md](references/python.md)
- TypeScript: [references/typescript.md](references/typescript.md)

For generated output, prototypes destined for deletion, or prose-only
configuration without executable behavior, use the nearest deterministic check.
A reversible, low-impact edit such as copy, styling, or a config value needs
the existing checks, not a new test; a test that restates the implementation
proves nothing. Never exempt product behavior. If a direct executable oracle beats a brittle
characterization test, record the exception. Confirm the pre-change deficiency
and run that oracle before and after the change.
