---
name: python-patterns
description: >-
  Use when writing or refactoring Python and you want idiomatic typing, error
  handling, async, and concurrency that is correct and readable. Triggers on
  "idiomatic Python", "review this Python", "fix this async code", "type this
  properly". For choosing a new project's stack, use
  mega-python:greenfield-python-stack.
license: MIT
---

# Python Patterns

> **Measured:** current frontier *and* small Claude models already write the
> common async/DB mechanics here (asyncio pools and queue termination, shared
> in-memory SQLite) correctly single-shot without this skill — a controlled
> study found zero correctness headroom, 184/184 passing in both arms (the
> repo's `evals/RESULTS.md` §2). Reach for this skill for idiomatic typing and
> error-shaping *choices*, as a review checklist, or when driving weaker
> models; not because a current model would otherwise hang a consumer loop.

Idioms for correct, readable Python. Each is a default, not a law.

## Typing

- Type public functions fully; let inference handle locals. Run the type checker in
  strict mode (see greenfield-python-stack) — an untyped boundary hides bugs.
- Prefer precise types: `Sequence`/`Mapping` for read-only params, `list`/`dict` for
  what you own and mutate. Use `X | None` (not bare `Optional` guesswork) and handle
  the `None` branch.
- Model closed sets with a discriminated union (`Literal` tag + `match`) or an `Enum`,
  so the type checker forces you to handle every case.

## Data

- `@dataclass(frozen=True, slots=True)` for internal value objects; **pydantic** models
  at the boundaries (request/response/config) where you need validation and parsing.
- Don't hand-roll validation you can declare on a pydantic model.

## Errors

- Raise specific exceptions; don't `except Exception:` and swallow. Catch the narrowest
  type, add context, and re-raise with `raise NewError(...) from err` to keep the chain.
- Use `try/finally` or a context manager (`with`) for cleanup; write your own with
  `@contextmanager` for a resource you open and must close.
- Return a value for an *expected* absence (a lookup miss → `None` or a result type);
  reserve exceptions for the genuinely exceptional.

## Async correctness

- **Nothing blocking inside `async def`.** A sync HTTP call, a blocking DB driver,
  `time.sleep`, or heavy CPU stalls the whole event loop. Use an async library or
  `await asyncio.to_thread(fn, ...)`.
- Run independent work concurrently with `asyncio.gather` or a `TaskGroup` — not a
  sequential await loop.

### A bounded-concurrency worker pool that does not deadlock

Cap concurrency with a semaphore and gather the results. This completes because each
worker is a self-contained coroutine — there is no unbounded queue nobody drains and
no wait on a channel nobody closes (the classic pitfalls):

```python
import asyncio
from collections.abc import Iterable, Awaitable, Callable
from typing import TypeVar

T = TypeVar("T")
R = TypeVar("R")

async def worker_pool(
    items: Iterable[T],
    work: Callable[[T], Awaitable[R]],
    concurrency: int = 8,
) -> list[R]:
    sem = asyncio.Semaphore(concurrency)

    async def run(item: T) -> R:
        async with sem:            # bound in-flight work
            return await work(item)

    # gather preserves input order and propagates the first exception
    return await asyncio.gather(*(run(i) for i in items))
```

For a streaming producer/consumer, use `asyncio.Queue`; always ensure every consumer
can exit (e.g. send one sentinel per consumer, or cancel the `TaskGroup`) so you don't
block forever on `queue.get()`.

## Testing

- pytest with plain functions and fixtures; parametrize instead of copy-pasting cases.
- Test the `domain/` logic without a server. For the DB, share one in-memory
  connection (see greenfield-python-stack's `StaticPool` helper) so schema persists
  across queries.
- `pytest-asyncio` for `async def` tests; don't call `asyncio.run` inside a test.

## When to use this skill

- Writing or reviewing Python for correctness and idiom.
- Untangling async/concurrency bugs.
- For a new project's stack choices, use mega-python:greenfield-python-stack.
