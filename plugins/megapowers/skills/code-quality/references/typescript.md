# TypeScript Judgment

Use the repository's TypeScript target, module system, strictness, validator,
error convention, runtime lifecycle, and test runner.

- Type exported APIs and external-data boundaries. Accept untrusted values as
  `unknown`, then narrow or validate them before use; reserve `any` for a
  deliberate unchecked boundary.
- Use a discriminated union when callers must handle a closed set exhaustively,
  not as a default replacement for simple objects. Derive a static type from an
  existing runtime schema when that schema is the source of truth.
- Match the surrounding failure contract. Throw `Error` or an appropriate
  subclass for thrown failures, preserve causes when translating, and narrow
  caught values before inspection.
- Return, await, or deliberately attach every promise whose outcome matters.
  Use a runtime lifecycle hook for intentional background work and report its
  failures; a floating promise is not supervision.
- Start operations together only when they are independent and bounded. Use
  fail-fast aggregation when one failure invalidates the group, settled
  aggregation when every outcome matters, and a limiter for large fan-out.
