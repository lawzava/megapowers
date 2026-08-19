---
name: code-quality
description: Use when implementation, maintenance, review, refactor, architecture, API, error, concurrency, or debugging work requires a code-quality judgment.
---

# Code Quality

Repository instructions, existing code, and configured project tools are authoritative; skills supply defaults only where the repository is silent.

Inspect the supported language version, package boundaries, neighboring code,
public contracts, formatter, linter, type checker, and tests before introducing
a pattern. Match local idioms unless they violate the requested behavior or a
clear safety property.

Keep boundaries typed and validate untrusted data once at entry. Make error
ownership explicit: callers should know which failures are expected,
inspectable, retryable, or terminal. Every concurrent task needs a bounded
lifecycle, cancellation or shutdown path, failure policy, and backpressure or
resource limit. Reduce reader state load by keeping related policy together and
making transitions explicit.

When repeated state branches describe one concept, model the domain instead of
adding synchronized booleans. Make retryable lifecycle operations idempotent.
Establish separate ownership before serialization or concurrency. If a failure
recurs, move the invariant into types, tests, lint, or a canonical helper.
Prefer the smallest abstraction that clarifies a current contract; do not add
configurability for imagined use. Delete only code made unused by the change.

Mechanical style belongs to the repository's formatter, linter, and tests.
Load exactly one language reference only when a maintenance, review, refactor,
architecture, API, concurrency, or debugging decision needs language-specific
judgment that local code does not settle:

- Go: [references/go.md](references/go.md)
- Python: [references/python.md](references/python.md)
- TypeScript: [references/typescript.md](references/typescript.md)

Do not load a language reference for mechanical edits or when repository code
already answers the decision.
