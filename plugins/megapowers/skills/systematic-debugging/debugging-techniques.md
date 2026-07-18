# Debugging Techniques

Three techniques the main skill references: trace to the source, validate at
every layer, wait on conditions instead of clocks.

## Root-Cause Tracing

Bugs often manifest deep in the call stack (git init in the wrong directory,
a file created in the wrong location). Fix where the bad value *originates*,
not where the error appears. Trace the chain: observe the symptom, find the
code that directly causes it, then keep asking "what called this, with what
value?" until you reach the original trigger. If you genuinely cannot trace
further, fixing at the symptom point is a fallback, and say so.

When manual tracing stalls, instrument before the dangerous operation:
capture `new Error().stack` plus the relevant context (directory, cwd,
environment) and log with `console.error` (test loggers may be suppressed).
To find which earlier test pollutes shared state, bisect with
`find-polluter.sh` in this directory.

## Defense-in-Depth Validation

After fixing the root cause, add validation at each layer the bad data
passed through, so the bug becomes structurally impossible rather than
merely fixed: entry-point validation at the API boundary, a business-logic
check where the value is used, and an environment guard for
context-specific dangers (for example, in tests refuse `git init` outside
the temp directory). Different layers catch different bypasses (other call
paths, refactors, mocks).

## Condition-Based Waiting

Arbitrary timeouts (`sleep 2`, `setTimeout(..., 500)`) make tests slow when
generous and flaky when tight. Poll for the actual condition instead:

```typescript
async function waitFor<T>(fn: () => T | undefined, timeoutMs = 5000): Promise<T> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const result = fn();
    if (result !== undefined) return result;
    await new Promise(r => setTimeout(r, 10));
  }
  throw new Error(`Timed out after ${timeoutMs}ms waiting for condition`);
}

const session = await waitFor(() => manager.getSession(id));
```

The timeout is a failure bound, not a pacing device: the wait returns the
moment the condition holds. Wait on process output, file existence, or
state transitions the same way.
