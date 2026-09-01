---
name: test-first-implementation
description: Use when adding or changing behavior, fixing a confirmed bug, refactoring executable code, or implementing a feature with tests.
when_to_use: Trigger phrases: implement, add the feature, fix the confirmed bug, refactor, change behavior in code, make the tests pass, add tests for this.
metadata:
  short-description: Failing test first, then minimal implementation
---

# Test-First Implementation

Production code follows a failing test. A test written after implementation
can describe the code but cannot prove it would have caught the missing
behavior.

Before the first test, inspect the entry point, callers, nearest convention,
and existing coverage. If the cause of a failure is unknown, use
`systematic-debugging` first.

## Red, green, refactor

1. Write one small test at a stable observable boundary. Prefer real behavior
   over assertions about mocks.
2. Verify red: run it and confirm a clean failure for the expected missing
   behavior, not a typo or setup error.
3. Add the minimum implementation that can satisfy that test. Avoid new
   options, abstractions, and adjacent cleanup.
4. Verify green: run the focused test and read its full result. Fix production
   code when it fails.
5. Refactor only the changed code while keeping the test green. Repeat for the
   next behavior.

Use mocks only after understanding the real dependency. Do not add production APIs used only by tests. Run the
repository's broader suite at the task or milestone boundary and before a
completion claim.

## Code judgment

Repository instructions, existing code, and configured project tools are authoritative; skills supply defaults only where the repository is silent.
Match local idioms, package boundaries, and public contracts unless they
violate the requested behavior or a clear safety property. Mechanical style
stays with the formatter, linter, and tests.

Type boundaries and validate untrusted data once at entry. Make error
ownership explicit: expected, retryable, or terminal. Give every concurrent
task a bounded lifecycle, cancellation, failure policy, and resource limit.
When repeated state branches describe one concept, model the domain instead of
synchronized booleans. Make retryable lifecycle operations idempotent. If a
failure recurs, move the invariant into types, tests, lint, or a canonical
helper.

Load exactly one language reference only when a maintenance, review, refactor,
architecture, API, concurrency, or debugging decision needs language-specific
judgment that local code does not settle:

- Go: [references/go.md](references/go.md)
- Python: [references/python.md](references/python.md)
- TypeScript: [references/typescript.md](references/typescript.md)

Generated output, throwaway prototypes that will be deleted, and prose-only
configuration may not have an executable behavior to test. Apply the nearest
deterministic correctness check instead; do not turn that exception into a way
around testing product behavior. When a direct executable oracle is stronger
than a brittle characterization test, record the exception. Confirm the
pre-change failure or deficiency, then run that oracle before and after the
change.
