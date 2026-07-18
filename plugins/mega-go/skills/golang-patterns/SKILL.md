---
name: golang-patterns
description: >
  Use for Go in an existing project when choosing interfaces, dependency
  injection, goroutines, channels, context, errors, functional options, or
  package layout. Skip mechanical edits.
license: MIT
---

# Go Patterns

> **Measured:** current frontier *and* small Claude models already write the
> common concurrency mechanics here (worker pools, pipeline stages, channel
> closing) correctly single-shot without this skill — a controlled study found
> zero correctness headroom, 184/184 passing in both arms (the repo's
> `evals/RESULTS.md` §2). Reach for this skill for the design *choices* as a
> review checklist, or when driving weaker models; not because a current
> model would otherwise deadlock.

Scope: Go files, `go.mod`, and `go.sum`.
Origin: Derived from Everything Claude Code (MIT, (c) 2026 Affaan Mustafa).

## Design-Choice Checklist

Defaults for this stack; deviate when the surrounding code already chose
otherwise, and say so.

- **Interfaces:** small, defined at the point of use, not next to the
  implementation. Accept interfaces, return structs.
- **Dependencies:** constructor injection (`New*` functions with explicit
  parameters, validated in the constructor). No global service state.
- **Constructor configuration:** functional options (`Option func(*T)`,
  `WithX(...)`) when a constructor has optional knobs; plain parameters when
  everything is required.
- **Context:** first parameter on anything that blocks, calls out, or loops;
  honor cancellation at loop boundaries.
- **Errors:** wrap with `fmt.Errorf("...: %w", err)` and context about the
  operation; sentinel errors (`errors.Is`) for expected conditions callers
  branch on; typed errors (`errors.As`) only when callers need fields.
- **Goroutine lifecycle:** every goroutine has a known terminator (waitgroup,
  context, channel close). Close a results channel from a goroutine that
  waits on the workers, never inline before the reader drains it.
- **Package layout:** `cmd/` for mains (kept minimal), `internal/` for
  private code, `pkg/` only for genuinely public libraries. Lowercase
  single-word package names; no stutter (`user.User`, not `user.UserModel`).
- **Tests:** table-driven with subtests (`t.Run`); helpers marked
  `t.Helper()` with `t.Cleanup` for teardown. SQLite test databases and the
  `:memory:` pooling footgun live in mega-go:greenfield-go-stack.

## When to Use This Skill

- Designing Go APIs and packages
- Structuring or refactoring Go projects
- Reviewing Go code for idiom drift
