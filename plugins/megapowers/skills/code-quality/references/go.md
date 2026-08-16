# Go Judgment

Use the repository's Go version, package boundaries, constructors, error
conventions, and test style.

- Add `context.Context` only to contracts that need cancellation, deadlines, or
  request-scoped propagation. Pass it explicitly and do not store it in a
  long-lived struct.
- Add operation context when returning failures. Wrap when callers need to
  inspect a cause; define a sentinel or typed error only for a stable
  caller-facing condition. Prefer `errors.Is` and `errors.As` to message
  matching.
- Give every goroutine a termination path. The owner coordinates shutdown and
  closes shared result channels after all producers finish, never from an
  arbitrary producer. If a receiver can stop early, cancellation must keep
  producers from blocking forever.
- Bound fan-out and decide whether one failure cancels siblings, is collected,
  or is reported independently before starting concurrent work.
- Keep interfaces at the consumer boundary and small enough to express an
  actual substitution. Prefer concrete types when only one implementation
  exists.
