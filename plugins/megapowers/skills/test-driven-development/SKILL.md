---
name: test-driven-development
description: Use when implementing a feature or bugfix, before writing implementation code — write the failing test first. This includes ANY task that asks for new code plus its tests, in any order ("add a function with unit tests", "implement X and test it", "make sure the tests pass"). For diagnosing an existing failure, use systematic-debugging first, then return here to write the fix. Triggers on "TDD", "test-first", "write the test first", "red-green-refactor".
license: MIT
---

# Test-Driven Development (TDD)

## Overview

Write the test first. Watch it fail. Write minimal code to pass.

**Core principle:** if you didn't watch the test fail, you don't know whether it tests the right thing.

Follow the letter of these rules and you'll get the spirit of them for free.

## When to Use

Use TDD for:
- New features
- Bug fixes
- Refactoring
- Behavior changes

Reasonable exceptions — these are observable, so apply them yourself without stopping to ask:
- Throwaway prototypes (code you will delete, not ship)
- Generated code
- Configuration files (no behavior to assert)

Outside those cases, if you catch yourself thinking "I'll skip TDD just this once," treat that as a signal to slow down rather than a reason to skip.

## The Core Rule

Production code follows a failing test. Write the test, watch it fail, then write the code.

If you wrote implementation code before the test, delete it and start fresh from the test. Don't keep it as reference and don't adapt it while writing tests — code you keep around will shape the test toward what you already built, which is the thing TDD is meant to prevent. Reimplement from the test.

## Red-Green-Refactor

The cycle, in order:

1. Red — write one minimal failing test.
2. Verify red — run it and confirm it fails for the expected reason. If it passes, it's testing existing behavior; fix the test. If it errors (typo, missing import), fix the error and re-run until it fails cleanly.
3. Green — write the simplest code that makes the test pass.
4. Verify green — run it and confirm the test passes, the project's full suite still passes, and output is clean. If the test fails, fix the code (not the test). If other tests fail, fix them now.
5. Refactor — clean up while staying green. Don't add behavior.
6. Repeat — next failing test for the next behavior.

### Red: Write Failing Test

Write one minimal test showing what should happen.

Good:
```typescript
test('retries failed operations 3 times', async () => {
  let attempts = 0;
  const operation = () => {
    attempts++;
    if (attempts < 3) throw new Error('fail');
    return 'success';
  };

  const result = await retryOperation(operation);

  expect(result).toBe('success');
  expect(attempts).toBe(3);
});
```
Clear name, tests real behavior, one thing.

Avoid:
```typescript
test('retry works', async () => {
  const mock = jest.fn()
    .mockRejectedValueOnce(new Error())
    .mockRejectedValueOnce(new Error())
    .mockResolvedValueOnce('success');
  await retryOperation(mock);
  expect(mock).toHaveBeenCalledTimes(3);
});
```
Vague name, tests the mock instead of the code.

Requirements:
- One behavior
- Clear name
- Real code (no mocks unless unavoidable)

### Verify Red: Watch It Fail

Run the test and watch it fail. Don't skip this step.

```bash
npm test path/to/test.test.ts
```

Confirm:
- Test fails (not errors)
- Failure message is what you expected
- It fails because the feature is missing, not because of a typo

If the test passes, you're testing existing behavior — fix the test.

If the test errors, fix the error and re-run until it fails correctly.

### Green: Minimal Code

Write the simplest code that passes the test.

Good:
```typescript
async function retryOperation<T>(fn: () => Promise<T>): Promise<T> {
  for (let i = 0; i < 3; i++) {
    try {
      return await fn();
    } catch (e) {
      if (i === 2) throw e;
    }
  }
  throw new Error('unreachable');
}
```
Just enough to pass.

Avoid:
```typescript
async function retryOperation<T>(
  fn: () => Promise<T>,
  options?: {
    maxRetries?: number;
    backoff?: 'linear' | 'exponential';
    onRetry?: (attempt: number) => void;
  }
): Promise<T> {
  // YAGNI
}
```
Over-engineered.

Don't add features, refactor other code, or improve beyond what the test requires.

### Verify Green: Watch It Pass

Run the test and watch it pass.

```bash
npm test path/to/test.test.ts
```

Confirm:
- Test passes
- The full suite still passes — run the project's canonical test entrypoint
  (`./test.sh`, `make test`, `pytest`/`unittest discover`), not only the file
  you wrote. A green module over a red suite is how pre-existing failures get
  claimed as clean.
- Output is clean (no errors, warnings)

If the test fails, fix the code, not the test.

If other tests fail, fix them now.

### Refactor: Clean Up

Once green:
- Remove duplication
- Improve names
- Extract helpers

Keep tests green. Don't add behavior.

### Repeat

Write the next failing test for the next feature.

## Good Tests

- Minimal: one behavior per test. If the name contains "and" (`test('validates email and domain and whitespace')`), split it.
- Clear: the name describes the behavior, not `test('test1')`.
- Shows intent: the test demonstrates the desired API rather than obscuring what the code should do.

## Rationalizations to Watch For

When one of these thoughts shows up, it usually means it's time to write the test first:

- "Too simple to test." Simple code still breaks, and the test takes 30 seconds.
- "I'll test after, to verify it works." Tests written after the code pass immediately, and passing immediately proves nothing — the test might check the wrong thing, test the implementation rather than the behavior, or miss the edge cases you forgot. You never saw it catch a bug. Test-first forces you to see the test fail, which proves it tests something real.
- "Tests after achieve the same goals — it's spirit, not ritual." Tests-after answer "what does this do?"; tests-first answer "what should this do?" Tests-after are biased by your implementation: you test what you built and the cases you remembered, not what's required and the ones you'd discover.
- "Already manually tested." Ad-hoc isn't systematic — no record of what you tested, can't re-run when the code changes, easy to forget cases under pressure.
- "Manual testing feels faster." It does not prove edge cases and you will re-test on every change — write the automated test.
- "Deleting X hours is wasteful." That time is spent either way (sunk cost); the real choice is rewriting with TDD (high confidence) vs keeping code you can't fully trust. Working code without real tests is technical debt.
- "Keep it as reference, write tests first." You'll end up adapting it, which is testing after.
- "Need to explore first." Fine — throw the exploration away and start with TDD.
- "Test is hard to write." Hard to test usually means hard to use; listen to the test and simplify the design.
- "TDD will slow me down." / "TDD is dogmatic; pragmatism means adapting." TDD is the pragmatic choice: bugs surface before commit, refactoring stays safe, and it beats debugging in production later.
- "Existing code has no tests." You're improving it — add tests as you go.

## Signs to Stop and Restart with TDD

If you notice any of these, delete the untested code and restart with the test first:

- Code written before the test
- Test written after implementation
- Test passes immediately
- You can't explain why the test failed
- Tests deferred to "later"

## Example: Bug Fix

Bug: empty email is accepted.

RED
```typescript
test('rejects empty email', async () => {
  const result = await submitForm({ email: '' });
  expect(result.error).toBe('Email required');
});
```

Verify RED
```bash
$ npm test
FAIL: expected 'Email required', got undefined
```

GREEN
```typescript
function submitForm(data: FormData) {
  if (!data.email?.trim()) {
    return { error: 'Email required' };
  }
  // ...
}
```

Verify GREEN
```bash
$ npm test
PASS
```

REFACTOR
Extract validation for multiple fields if needed.

## Verification Checklist

Before marking work complete:

- [ ] Every new function/method has a test
- [ ] Watched each test fail before implementing
- [ ] Each test failed for the expected reason (feature missing, not a typo)
- [ ] Wrote minimal code to pass each test
- [ ] All tests pass
- [ ] Output is clean (no errors, warnings)
- [ ] Tests use real code (mocks only if unavoidable)
- [ ] Edge cases and errors covered

If you can't check every box, some of the work wasn't done test-first — go back and redo those parts with TDD.

## When Stuck

- Don't know how to test it: write the wished-for API, write the assertion first, or ask your human partner.
- Test is too complicated: the design is likely too complicated — simplify the interface.
- Must mock everything: the code is too coupled — use dependency injection.
- Test setup is huge: extract helpers; if it's still complex, simplify the design.

## Debugging Integration

Found a bug? Write a failing test that reproduces it, then follow the TDD cycle. The test proves the fix and guards against regression. Fix bugs with a test, not without one.

## Testing Anti-Patterns

When adding mocks or test utilities, read [testing-anti-patterns.md](testing-anti-patterns.md) to avoid common pitfalls:
- Testing mock behavior instead of real behavior
- Adding test-only methods to production classes
- Mocking without understanding dependencies

## Summary

Production code has a test that existed and failed first. Anything else isn't TDD. Beyond the observable exceptions above, skip the process only with your human partner's agreement.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
