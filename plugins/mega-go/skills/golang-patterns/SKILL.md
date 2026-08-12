---
name: golang-patterns
description: >
  Use for Go in an existing project when behavior depends on context
  cancellation, error inspection, or goroutine lifecycle. Skip mechanical edits.
license: MIT
---

# Go Patterns

Follow the repository's existing package boundaries, constructors, and test
style. Use this skill only when local code does not settle a concurrency,
context, or error-handling choice.

- **Context:** use it when a contract needs cancellation, a deadline, or
  request-scoped propagation. Check cancellation at loop boundaries; do not add
  it to purely in-memory helpers without one of those needs.
- **Errors:** add operation context when returning an error. Use wrapping when
  callers need to inspect the cause; use a sentinel or typed error only for a
  stable caller-facing condition.
- **Goroutines:** give every goroutine a termination path. Have producers signal
  completion with `sync.WaitGroup`, or `errgroup.Group` when errors propagate.
  Close a shared results channel from one goroutine that waits for every
  producer, never from a producer. Receivers range until close; if one stops
  early, cancellation must prevent producers from blocking forever.

For a new module's layout, select a project shape with
mega-go:greenfield-go-stack rather than imposing one here. For a throwaway
`go run` helper, use mega-go:scripting-in-go.
