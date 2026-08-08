---
name: greenfield-python-stack
description: >-
  Use to start or scaffold a Python project or service, or choose its package
  manager, tooling, web framework, validation, async, database, middleware, and
  layout.
license: MIT
---

# Greenfield Python Stack

Choose the project shape before choosing dependencies. Keep the first version
small enough to replace a tool when its real constraints become clear.

## Start with the artifact

| Artifact | Start with | Add when needed |
| --- | --- | --- |
| Library | `src/` package, public API, tests against installed package | compatibility policy, optional extras |
| CLI | package entry point, argument parsing, stderr/stdout contract | rich output or shell completion |
| Batch job | explicit input, idempotency boundary, structured logs | queue, scheduler, checkpointing |
| Frontend helper | build integration and browser-compatible boundary | server only if it owns server work |
| Edge function | platform runtime and small dependency graph | shared library for portable logic |
| Service | app entry point, configuration boundary, health behavior | framework, datastore, background work |

Use the package manager already required by the target platform or team. For a
new standalone project, `uv` is a reasonable default because it manages the
environment and lockfile together. Choose a formatter, linter, type checker,
and test runner that fit the repository or deployment; record the commands in
`pyproject.toml` or the project documentation. `ruff` and `pytest` are common
small defaults, not requirements.

## Layout and boundaries

Use a `src/` layout when packaging or import behavior matters. Keep transport,
CLI, scheduler, and framework wiring at the edge; put business rules in ordinary
modules that tests can call directly. A tiny CLI or one-shot script may not need
the service-shaped directory tree.

```text
project/
├── pyproject.toml
├── src/package_name/
│   ├── __init__.py
│   ├── cli.py or app.py
│   └── domain/
└── tests/
```

Validate and parse untrusted input at the boundary. A schema library is useful
when it reduces duplicated parsing, while a small library can use explicit
constructors and standard-library types. Select a database access layer that the
team can operate. Add migrations only for persistent schema that must evolve;
ensure the deploy path applies and can roll back or recover from them.
Keep credentials out of source and checked-in configuration. Load them from the
deployment's secret store or ignored local configuration.

## Async and service decisions

Choose an async runtime only when the framework and required dependencies expose
awaitable work; keep a synchronous project synchronous otherwise. For
`TaskGroup`, `gather`, thread, process, queue, and cancellation choices, use
`mega-python:python-patterns`.

Expose CORS, proxy trust, rate limits, and middleware only when the deployment
has those boundaries. Configure each from the actual clients, proxies, and
threat model rather than copying a universal middleware order.

## Test shape

Run the project's configured test and type-check commands. For a packaged
library, make at least one test import the installed package rather than a
same-named checkout module.

## When to use this skill

- Starting a Python library, CLI, batch job, frontend helper, edge function, or service.
- Choosing the first project shape and its tooling.
- For idioms inside an existing project, use mega-python:python-patterns.
