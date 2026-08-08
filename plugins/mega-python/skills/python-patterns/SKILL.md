---
name: python-patterns
description: >-
  Use for Python in an existing project when choosing idiomatic typing, Pydantic
  models, error handling, asyncio, concurrency, or async design.
license: MIT
---

# Python Patterns

Follow the repository's supported Python version, formatter, type checker,
validation library, and test conventions before introducing a new pattern.

## Boundaries and data

Type exported APIs and data that crosses module boundaries. Let local inference
carry obvious implementation details. Use the collection abstraction matching
the contract, such as `Sequence` for read-only input and `list` for owned,
mutable data. Model a closed set with an `Enum`, `Literal`, or tagged structure
when callers must handle every case.

Use dataclasses, Pydantic, attrs, or plain classes according to the surrounding
code and boundary needs. Immutability, slots, and schema models solve specific
problems, they are not universal defaults. Parse or validate untrusted input at
the boundary once, then pass domain values inward.

## Errors and resources

Use the project's error convention. Return an ordinary value such as `None` for
an expected absence when that makes the call site clear; raise a specific
exception for failures that should interrupt normal flow. Preserve a caught
exception as the cause when translating it, and use context managers or
`try`/`finally` for resources that must close.

## Async choices

Write `async def` only when its framework and dependencies expose awaitable
work. A synchronous function is simpler and correct when its work is
synchronous. Within async code, use an async library for network or database
I/O where available. `asyncio.to_thread` is typically appropriate for blocking
I/O. It does not make CPU-bound Python parallel on the default GIL-enabled
build; free-threaded builds differ, so confirm the interpreter before relying
on either behavior.

Start independent awaitable operations together only when their concurrency
improves the required behavior and their resource use is bounded. Choose
`TaskGroup`, `gather`, a semaphore, or a queue based on cancellation, failure,
streaming, and backpressure needs, rather than applying a worker-pool recipe to
every fan-out.

```python
import asyncio
from collections.abc import Awaitable
from pathlib import Path
from typing import Protocol

class Response(Protocol):
    text: str

class AsyncClient(Protocol):
    def get(self, path: str) -> Awaitable[Response]: ...

async def read_pair(client: AsyncClient) -> tuple[str, str]:
    first, second = await asyncio.gather(client.get("/first"), client.get("/second"))
    return first.text, second.text

def read_file(path: Path) -> str:
    return path.read_text()

async def load_config(path: Path) -> str:
    return await asyncio.to_thread(read_file, path)
```

Test the domain behavior directly. Use the repository's test runner and async
test support; do not add an async test framework merely because a function has
`async def`.

## When to use this skill

- Writing or reviewing Python in an existing project.
- Choosing typing, error, data-boundary, or async patterns.
- For a new project's shape and tooling, use mega-python:greenfield-python-stack.
