---
name: greenfield-python-stack
description: >-
  Use when starting a new Python project or service and choosing its stack and
  layout — dependency manager, linter/formatter, type checker, test runner, web
  framework, and the async/DB/middleware baseline. Triggers on "new Python
  project", "scaffold a Python service", "set up a FastAPI app", "which Python
  stack". For idioms within existing code, use mega-python:python-patterns.
---

# Greenfield Python Stack

Opinionated defaults for a new Python service in 2026. Time-stamped: these reflect
today's tools and will go stale — treat them as a starting point to adapt.

For a non-trivial new project, run megapowers:brainstorming and
megapowers:writing-plans first (if installed) — this skill supplies the stack,
not the process.

## Toolchain (pin these)

- **uv** — dependency + venv + lockfile manager. `uv init`, `uv add`, `uv run`,
  `uv sync`. One tool, fast, a real lockfile (`uv.lock`). Commit the lock.
- **ruff** — lint *and* format (replaces black + flake8 + isort). `ruff check --fix`
  and `ruff format`. Enable a strict ruleset in `pyproject.toml`.
- **pyright** (or **ty**) in **strict** mode — types are a correctness tool, not
  decoration. CI fails on a type error.
- **pytest** — tests. `pytest-asyncio` for async tests.
- **pydantic v2** — validation and settings (`pydantic-settings` for config from env).

## Layout

`src/` layout so tests import the installed package, not the working dir:

```
myservice/
├── pyproject.toml        # deps, ruff, pyright, pytest config all here
├── uv.lock
├── src/myservice/
│   ├── __init__.py
│   ├── main.py           # app wiring only
│   ├── api/              # routes
│   ├── domain/           # logic, no framework imports
│   └── db.py
└── tests/
```

Keep framework imports out of `domain/` — logic you can test without a server.

## Web: FastAPI + uvicorn

FastAPI + pydantic v2 for typed request/response. Prefer `async def` handlers, but
see async correctness below.

### Middleware order matters (and it's the reverse of what you'd guess)

In Starlette/FastAPI, **the middleware added *last* runs *outermost*** (first on the
way in, last on the way out). So the logging middleware must be added **last** to see
*every* request — including ones a later (inner) middleware rejects with a 4xx. Put it
outermost, or rate-limited/blocked requests never get logged.

```python
# inner-most first ... outer-most last
app.add_middleware(GZipMiddleware)
app.add_middleware(CORSMiddleware, allow_origins=settings.cors_origins)  # explicit, never "*"
app.add_middleware(RateLimitMiddleware)        # rejects with 429
app.add_middleware(RequestLoggingMiddleware)   # LAST = outermost => logs the 429 too
```

### Rate limiting behind a proxy

A limiter keyed on the client IP must derive that IP correctly, or **every request
behind a reverse proxy shares the proxy's IP and one bucket**. Trust the forwarded
header only from a known proxy:

```python
# Uvicorn's ProxyHeadersMiddleware (uvicorn.middleware.proxy_headers) sets
# request.client.host from X-Forwarded-For ONLY for trusted proxies. Enable it with
#   uvicorn app:app --proxy-headers --forwarded-allow-ips=<proxy-cidr>
# Never trust X-Forwarded-For from arbitrary clients — they can spoof their IP.
```

Set `--forwarded-allow-ips` to your proxy's address, never `*`.

## Database & the in-memory test footgun

A SQLite `:memory:` database is **per-connection**: a pooled app opens several
connections and each gets its own empty database, so a table created on one is missing
on the next. For tests, force a single shared connection:

```python
from sqlalchemy import create_engine
from sqlalchemy.pool import StaticPool

def make_test_engine():
    # StaticPool reuses ONE connection, so the whole test shares one in-memory DB.
    return create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
```

For a real DB use a driver + connection pool sized to the deployment; keep migrations
(alembic) with a forward and a down path.

## Async correctness

- **Never block the event loop.** A synchronous call (a `requests` call, a blocking DB
  driver, `time.sleep`, heavy CPU) inside `async def` stalls *every* concurrent
  request. Use an async library, or push the blocking call to a thread:
  `await asyncio.to_thread(blocking_fn, ...)`.
- Run independent awaitables concurrently with `asyncio.gather(...)` or a
  `TaskGroup` (3.11+) — not a sequential `await` loop — when they don't depend on
  each other.
- Don't create fire-and-forget tasks you never await; a `TaskGroup` gives structured
  cancellation and surfaces exceptions.

## When to use this skill

- Standing up a new Python project or service.
- Choosing or justifying the stack.
- For idioms and refactors inside existing Python, use mega-python:python-patterns.
