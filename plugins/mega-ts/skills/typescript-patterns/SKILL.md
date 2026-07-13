---
name: typescript-patterns
description: >-
  Use for TypeScript in an existing project when choosing idiomatic types,
  discriminated unions, typed errors, promises, concurrency, or async design.
license: MIT
---

# TypeScript Patterns

Idioms for correct, readable TypeScript. Each is a default, not a law.

> **Measured:** in this repo's skill-effect study, current frontier *and* small
> Claude models already write correct versions of common concurrency/data
> patterns without pattern skills (zero correctness headroom; see
> `evals/RESULTS.md`). The value here is consistency of *design choices*
> (types at boundaries, error shape, module layout), not single-shot
> correctness.

## Types

- Turn on `strict` and `noUncheckedIndexedAccess`. Let inference handle locals; type
  the public boundaries (exported functions, request/response, module APIs).
- Prefer `unknown` over `any` at a boundary, then narrow. `any` disables the checker
  and hides bugs; if you must, isolate it behind a typed function.
- Model closed sets with a **discriminated union** (a literal `kind` field) and switch
  on it exhaustively — add a `never` default so the compiler flags a missing case:

```ts
type Shape = { kind: 'circle'; r: number } | { kind: 'rect'; w: number; h: number }
function area(s: Shape): number {
  switch (s.kind) {
    case 'circle': return Math.PI * s.r ** 2
    case 'rect':   return s.w * s.h
    default: { const _exhaustive: never = s; return _exhaustive }
  }
}
```

- Derive types from a single source of truth: `z.infer<typeof Schema>` for validated
  data, `as const` + `typeof` for literal config. Don't hand-maintain a parallel type.

## Errors

- For an *expected* failure (a lookup miss, a parse failure), return a **result** the
  caller must handle rather than throwing — a
  `type Result<T, E> = { ok: true; value: T } | { ok: false; error: E }` union, or a
  library's Result. Reserve `throw` for the genuinely exceptional.
- When you do throw, throw `Error` (or a subclass), never a string; preserve the cause
  with `new Error(msg, { cause: err })`.
- Narrow a caught `unknown`: `catch (e) { if (e instanceof SomeError) ... }`.

## Promises & async correctness

- **No floating promises.** Every promise is `await`ed, `return`ed, or explicitly
  handled with `.catch(...)`. Turn on `@typescript-eslint/no-floating-promises`. An
  unhandled rejection can take down the process.
- Run independent async work concurrently, not sequentially:

```ts
// sequential (slow): each await blocks the next
const seqA = await fetchA()
const seqB = await fetchB()
// concurrent: start both, then await together
const [a, b] = await Promise.all([fetchA(), fetchB()])
// need every outcome even if some reject:
const results = await Promise.allSettled([fetchA(), fetchB()])
```

- Don't mix `await` inside `.forEach` (it won't wait) — use a `for...of` loop with
  `await`, or `Promise.all(items.map(...))` for concurrency.
- Bound concurrency for large fan-out (a small pool / `p-limit`) so you don't open
  ten thousand sockets at once.

## Testing

- vitest with plain functions; `test.each` to parametrize instead of copy-paste.
- Test `domain/` logic without a server. Share one in-memory DB connection across a
  test so schema persists (see greenfield-ts-stack).
- Prefer real modules to mocks; mock only at true boundaries (network, clock).

## When to use this skill

- Writing or reviewing TypeScript for correctness and idiom.
- Untangling promise/async bugs.
- For a new project's stack choices, use mega-ts:greenfield-ts-stack.
