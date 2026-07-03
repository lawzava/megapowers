# Root Cause Tracing

## Overview

Bugs often manifest deep in the call stack (git init in the wrong directory, a file created in the wrong location, a database opened with the wrong path). The instinct is to fix where the error appears, but that treats a symptom.

Core principle: trace backward through the call chain until you find the original trigger, then fix at the source.

## When to Use

When a bug appears deep in the stack, decide based on whether you can trace it:
- If you can trace backward, follow the chain to the original trigger, then also add defense-in-depth.
- If you hit a dead end and can't trace further, fix at the symptom point as a fallback.

**Use when:**
- The error happens deep in execution, not at the entry point
- The stack trace shows a long call chain
- It's unclear where the invalid data originated
- You need to find which test or code triggers the problem

## The Tracing Process

### 1. Observe the Symptom
```
Error: git init failed in ~/project/packages/core
```

### 2. Find the Immediate Cause
What code directly causes this?
```typescript
await execFileAsync('git', ['init'], { cwd: projectDir });
```

### 3. Ask What Called This
```typescript
WorktreeManager.createSessionWorktree(projectDir, sessionId)
  → called by Session.initializeWorkspace()
  → called by Session.create()
  → called by test at Project.create()
```

### 4. Keep Tracing Up
What value was passed?
- `projectDir = ''` (empty string)
- Empty string as `cwd` resolves to `process.cwd()`
- That's the source code directory

### 5. Find the Original Trigger
Where did the empty string come from?
```typescript
const context = setupCoreTest(); // Returns { tempDir: '' }
Project.create('name', context.tempDir); // Accessed before beforeEach!
```

## Adding Stack Traces

When you can't trace manually, add instrumentation:

```typescript
// Before the problematic operation
async function gitInit(directory: string) {
  const stack = new Error().stack;
  console.error('DEBUG git init:', {
    directory,
    cwd: process.cwd(),
    nodeEnv: process.env.NODE_ENV,
    stack,
  });

  await execFileAsync('git', ['init'], { cwd: directory });
}
```

Use `console.error()` in tests; a logger may be suppressed.

Run and capture:
```bash
npm test 2>&1 | grep 'DEBUG git init'
```

Analyze the stack traces:
- Look for test file names
- Find the line number triggering the call
- Identify the pattern (same test? same parameter?)

## Finding Which Test Causes Pollution

If something appears during tests but you don't know which test, use the bisection script `find-polluter.sh` in this directory:

```bash
./find-polluter.sh '.git' 'src/**/*.test.ts'
```

It runs tests one by one and stops at the first polluter. See the script for usage.

## Real Example: Empty projectDir

Symptom: `.git` created in `packages/core/` (source code)

Trace chain:
1. `git init` runs in `process.cwd()` ← empty cwd parameter
2. WorktreeManager called with empty projectDir
3. Session.create() passed empty string
4. Test accessed `context.tempDir` before beforeEach
5. setupCoreTest() returns `{ tempDir: '' }` initially

Root cause: top-level variable initialization accessing an empty value

Fix: made tempDir a getter that throws if accessed before beforeEach

Also added defense-in-depth:
- Layer 1: Project.create() validates directory
- Layer 2: WorkspaceManager validates not empty
- Layer 3: NODE_ENV guard refuses git init outside tmpdir
- Layer 4: Stack trace logging before git init

## Key Principle

Trace back to the original trigger rather than fixing where the error appears:

1. Start from the immediate cause.
2. Can you trace one level up? If not, this is where you fix — but you're still only patching a symptom, so prefer to keep tracing.
3. If yes, trace backward and ask whether that level is the source.
4. Keep going until you reach the source, then fix there.
5. Add validation at each layer the data passes through, which makes the bug structurally impossible.

## Stack Trace Tips

- In tests, use `console.error()` rather than a logger, which may be suppressed.
- Log before the dangerous operation, not after it fails.
- Include context: directory, cwd, environment variables, timestamps.
- Capture the stack with `new Error().stack` to see the complete call chain.
