# Testing Anti-Patterns

Load this reference when writing or changing tests, adding mocks, or considering test-only methods on production code.

## Overview

Tests should verify real behavior, not mock behavior. Mocks are a way to isolate the code under test, not the thing being tested.

Core principle: test what the code does, not what the mocks do.

Following TDD closely tends to prevent these anti-patterns on its own.

## The Core Rules

1. Don't test mock behavior.
2. Don't add test-only methods to production classes.
3. Don't mock without understanding the dependencies.

## Anti-Pattern 1: Testing Mock Behavior

The violation:
```typescript
// Testing that the mock exists
test('renders sidebar', () => {
  render(<Page />);
  expect(screen.getByTestId('sidebar-mock')).toBeInTheDocument();
});
```

Why this is wrong:
- You're verifying the mock works, not that the component works
- The test passes when the mock is present and fails when it's not
- It tells you nothing about real behavior

A useful check from your human partner: "Are we testing the behavior of a mock?"

The fix:
```typescript
// Test the real component, or don't mock it
test('renders sidebar', () => {
  render(<Page />);  // Don't mock sidebar
  expect(screen.getByRole('navigation')).toBeInTheDocument();
});

// OR if sidebar must be mocked for isolation:
// Don't assert on the mock - test Page's behavior with sidebar present
```

Gate: before asserting on any mock element, ask whether you're testing real component behavior or just mock existence. If it's mock existence, delete the assertion or unmock the component, and test real behavior instead.

## Anti-Pattern 2: Test-Only Methods in Production

The violation:
```typescript
// destroy() only used in tests
class Session {
  async destroy() {  // Looks like production API!
    await this._workspaceManager?.destroyWorkspace(this.id);
    // ... cleanup
  }
}

// In tests
afterEach(() => session.destroy());
```

Why this is wrong:
- The production class is polluted with test-only code
- It's dangerous if accidentally called in production
- It violates YAGNI and separation of concerns
- It confuses object lifecycle with entity lifecycle

The fix:
```typescript
// Test utilities handle test cleanup
// Session has no destroy() - it's stateless in production

// In test-utils/
export async function cleanupSession(session: Session) {
  const workspace = session.getWorkspaceInfo();
  if (workspace) {
    await workspaceManager.destroyWorkspace(workspace.id);
  }
}

// In tests
afterEach(() => cleanupSession(session));
```

Gate: before adding any method to a production class, ask whether it's only used by tests. If so, put it in test utilities instead. Also ask whether the class actually owns this resource's lifecycle; if not, it's the wrong class for the method.

## Anti-Pattern 3: Mocking Without Understanding

The violation:
```typescript
// Mock breaks test logic
test('detects duplicate server', () => {
  // Mock prevents config write that test depends on!
  vi.mock('ToolCatalog', () => ({
    discoverAndCacheTools: vi.fn().mockResolvedValue(undefined)
  }));

  await addServer(config);
  await addServer(config);  // Should throw - but won't!
});
```

Why this is wrong:
- The mocked method had a side effect the test depended on (writing config)
- Over-mocking to "be safe" breaks the actual behavior
- The test passes for the wrong reason, or fails mysteriously

The fix:
```typescript
// Mock at the correct level
test('detects duplicate server', () => {
  // Mock the slow part, preserve behavior test needs
  vi.mock('MCPServerManager'); // Just mock slow server startup

  await addServer(config);  // Config written
  await addServer(config);  // Duplicate detected
});
```

Gate: before mocking any method, work through it first. Ask what side effects the real method has, whether the test depends on any of those side effects, and whether you fully understand what the test needs. If the test depends on side effects, mock at a lower level (the actual slow or external operation) or use a test double that preserves the necessary behavior — not the high-level method the test relies on. If you're unsure what the test depends on, run it against the real implementation first, observe what needs to happen, then add minimal mocking at the right level.

Rationalizations to watch for: "I'll mock this to be safe," "this might be slow, better mock it," and mocking without knowing the dependency chain.

## Anti-Pattern 4: Incomplete Mocks

The violation:
```typescript
// Partial mock - only fields you think you need
const mockResponse = {
  status: 'success',
  data: { userId: '123', name: 'Alice' }
  // Missing: metadata that downstream code uses
};

// Later: breaks when code accesses response.metadata.requestId
```

Why this is wrong:
- Partial mocks hide structural assumptions — you only mocked fields you know about
- Downstream code may depend on fields you didn't include, causing silent failures
- Tests pass but integration fails, because the mock is incomplete and the real API isn't
- It gives false confidence — the test proves nothing about real behavior

Rule: mock the complete data structure as it exists in reality, not just the fields your immediate test uses.

The fix:
```typescript
// Mirror real API completeness
const mockResponse = {
  status: 'success',
  data: { userId: '123', name: 'Alice' },
  metadata: { requestId: 'req-789', timestamp: 1234567890 }
  // All fields real API returns
};
```

Gate: before creating a mock response, check what fields the real API response contains. Examine an actual response from docs or examples, include every field the system might consume downstream, and verify the mock matches the real schema completely. If you're creating a mock, you need to understand the entire structure — partial mocks fail silently when code depends on omitted fields. When uncertain, include all documented fields.

## Anti-Pattern 5: Integration Tests as Afterthought

The violation:
```
Implementation complete
No tests written
"Ready for testing"
```

Why this is wrong:
- Testing is part of implementation, not an optional follow-up
- TDD would have caught this
- Work isn't complete without tests

The fix:
```
TDD cycle:
1. Write failing test
2. Implement to pass
3. Refactor
4. THEN claim complete
```

## When Mocks Become Too Complex

Warning signs:
- Mock setup is longer than the test logic
- You're mocking everything to make the test pass
- Mocks are missing methods the real components have
- The test breaks when the mock changes

A useful check from your human partner: "Do we need to be using a mock here?"

Integration tests with real components are often simpler than complex mocks.

## TDD Prevents These Anti-Patterns

Why TDD helps:
1. Write the test first — forces you to think about what you're actually testing.
2. Watch it fail — confirms the test checks real behavior, not mocks.
3. Minimal implementation — keeps test-only methods from creeping in.
4. Real dependencies — you see what the test actually needs before mocking.

If you're testing mock behavior, TDD got skipped somewhere: you added mocks without first watching the test fail against real code.

## Quick Reference

| Anti-Pattern | Fix |
|--------------|-----|
| Assert on mock elements | Test real component or unmock it |
| Test-only methods in production | Move to test utilities |
| Mock without understanding | Understand dependencies first, mock minimally |
| Incomplete mocks | Mirror real API completely |
| Tests as afterthought | TDD - tests first |
| Over-complex mocks | Consider integration tests |

## Signs Something Is Off

- Assertions checking for `*-mock` test IDs
- Methods only called in test files
- Mock setup that's more than half the test
- A test that fails when you remove the mock
- Not being able to explain why the mock is needed
- Mocking "just to be safe"

## The Bottom Line

Mocks are tools to isolate, not things to test.

If TDD reveals that you're testing mock behavior, something went wrong. Test real behavior, or reconsider why you're mocking at all.
