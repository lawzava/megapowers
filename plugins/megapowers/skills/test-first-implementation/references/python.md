# Python Judgment

Use the repository's supported Python version, formatter, type checker,
validation library, async framework, and test conventions.

- Type exported APIs and values crossing module boundaries; let inference
  carry obvious local details. Choose collection abstractions by ownership,
  such as `Sequence` for read-only input and `list` for owned mutation.
- Validate untrusted input at the boundary with the project's existing model or
  parser, then pass domain values inward. Do not add a modeling library solely
  for this guidance.
- Use ordinary absence values for expected misses when call sites stay clear;
  raise a specific exception for failures that interrupt normal flow. Preserve
  the original cause when translating an exception.
- Use context managers or `try`/`finally` for resource ownership. A resource's
  creator should make its close or transfer contract explicit.
- Write async code only for an async contract. Bound concurrent tasks and choose
  task groups, gathering, semaphores, or queues based on cancellation, failure,
  streaming, and backpressure needs. Moving blocking I/O to a thread does not
  make CPU-bound work parallel.
