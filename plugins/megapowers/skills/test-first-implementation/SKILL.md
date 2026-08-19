---
name: test-first-implementation
description: Use when adding or changing behavior, fixing a confirmed bug, refactoring executable code, or implementing a feature with tests.
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

Use mocks only after understanding the real dependency and preserve the side
effects the test needs. Do not add production APIs used only by tests. Run the
repository's broader suite at the task or milestone boundary and before a
completion claim.

Generated output, throwaway prototypes that will be deleted, and prose-only
configuration may not have an executable behavior to test. Apply the nearest
deterministic correctness check instead; do not turn that exception into a way
around testing product behavior. When a direct executable oracle is stronger
than a brittle characterization test, record the exception. Confirm the
pre-change failure or deficiency, then run that oracle before and after the
change.
