---
name: typescript-patterns
description: >-
  Use for TypeScript in an existing project when choosing idiomatic types,
  discriminated unions, typed errors, promises, concurrency, or async design.
license: MIT
---

# TypeScript Patterns

Follow the repository's TypeScript target, module system, validation approach,
error conventions, and test tooling before adding a new abstraction.

## Types and boundaries

Type exported APIs and external-data boundaries. Prefer `unknown` to `any` for
untrusted values, then narrow or validate before use. Let inference carry local
details. Use a discriminated union when callers must branch exhaustively over a
closed set, not as a default replacement for simple objects.

```ts
type Shape = { kind: 'circle'; radius: number } | { kind: 'rect'; width: number; height: number }

export function area(shape: Shape): number {
  switch (shape.kind) {
    case 'circle': return Math.PI * shape.radius ** 2
    case 'rect': return shape.width * shape.height
  }
}
```

Derive static types from a validator or literal configuration when the project
already has a single source of truth. Otherwise keep the boundary parser and its
type close enough to change together.

## Errors

Use the surrounding API's failure style. An expected lookup miss may be `null`,
`undefined`, a result-like value, or a domain error depending on the caller's
contract. For thrown failures, throw `Error` or an appropriate subclass and
preserve a cause when translating another error. Narrow caught values before
using them.

## Promises and concurrency

Return, await, or deliberately observe promises whenever their outcome affects
correctness. Some runtimes expose a lifecycle hook for intentional background
work, such as `ctx.waitUntil`. Use that hook and attach error reporting; an
unattached floating promise remains a defect. Do not rely on `await` inside `forEach`; use `for...of`
for ordered work or map into promises for concurrent work.

```ts
const [profile, settings] = await Promise.all([loadProfile(id), loadSettings(id)])
```

Start work concurrently only when it is independent, desired, and bounded.
Choose `Promise.all` for fail-fast work, `Promise.allSettled` when every outcome
is required, and a limiter for large fan-out.

Test domain behavior with the repository's selected runner and test conventions.
Avoid adding a validation, result, or lint library solely to follow this skill.

## When to use this skill

- Writing or reviewing TypeScript in an existing project.
- Choosing types, error contracts, promise handling, or concurrency patterns.
- For a new project's shape and tooling, use mega-ts:greenfield-ts-stack.
