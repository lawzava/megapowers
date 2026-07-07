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

Any change with behavior to assert: new features, bug fixes, refactoring, behavior changes. The exceptions are observable, so apply them yourself without stopping to ask: throwaway prototypes (code you will delete, not ship), generated code, and configuration files with no behavior to assert. Outside those, the thought "I'll skip TDD just this once" is a signal to slow down, not a reason to skip.

## The Core Rule

Production code follows a failing test. Write the test, watch it fail, then write the code.

If you wrote implementation code before the test, delete it and start fresh from the test. Don't keep it as reference and don't adapt it while writing tests; code you keep around will shape the test toward what you already built, which is the thing TDD is meant to prevent. The hours already spent are spent either way; the real choice is between a rewrite you can trust and code you can't. Reimplement from the test.

## Red-Green-Refactor

1. Red: write one minimal failing test that shows what should happen. One behavior per test, a name that describes that behavior, real code rather than mocks wherever possible. A test that exercises a mock proves only the mock.
2. Verify red: run the test and confirm it fails for the expected reason, because the feature is missing. If it passes, it is testing existing behavior; fix the test. If it errors (typo, missing import), fix the error and re-run until it fails cleanly. Don't skip this step.
3. Green: write the simplest code that makes the test pass. No speculative options, no features the test doesn't demand, and no tidying of surrounding code the task didn't ask for.
4. Verify green: confirm the new test passes with clean output and the full suite still passes. Run the project's canonical test entrypoint (`./test.sh`, `make test`, `pytest`/`unittest discover`), not only the file you wrote. A green module over a red suite is how pre-existing failures get claimed as clean. If the test fails, fix the code, not the test. If other tests fail, fix them now.
5. Refactor: remove duplication, improve names, extract helpers, staying green throughout. Add no behavior, and leave code the change didn't touch alone; a bug fix does not need surrounding cleanup.
6. Repeat: next failing test for the next behavior.

## Pressure and Rationalizations

Every argument for writing the code first ("too simple to test", "I'll test after to verify", "already manually tested", "this is urgent", "deleting hours of work is wasteful") fails the same way: a test written after the code passes immediately, and passing immediately proves nothing. You never saw it catch anything, and it is biased toward what you built rather than what was required. When a test is hard to write, treat that as design feedback: hard to test usually means hard to use, so simplify the interface instead of skipping the test. If the codebase has no tests, you are improving it; add them as you go.

## Bug Fixes

Fix bugs with a test, not without one. Write a failing test that reproduces the bug, then follow the cycle; the test proves the fix and guards against regression. If the cause is still unknown, diagnose with systematic-debugging first, then return here.

## Testing Anti-Patterns

When adding mocks or test utilities, read [testing-anti-patterns.md](testing-anti-patterns.md) first. It covers testing mock behavior instead of real behavior, adding test-only methods to production classes, and mocking without understanding dependencies.

## Summary

Production code has a test that existed and failed first. Anything else isn't TDD. Beyond the observable exceptions above, skip the process only with your human partner's agreement.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
