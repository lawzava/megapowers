---
name: greenfield-ts-stack
description: >-
  Use to start or scaffold a TypeScript or Node service, or choose its package
  manager, tsconfig, tests, linting, validation, web framework, database, and
  layout.
license: MIT
---

# Greenfield TypeScript Stack

Choose the artifact and target runtime before selecting a framework or package
manager. Keep dependencies proportional to the deployment and team ownership.

## Start with the artifact

| Artifact | Start with | Add when needed |
| --- | --- | --- |
| Library | exported modules, public API tests, package metadata | build output, compatibility policy |
| CLI | `bin` entry point, argument and stream contract | interactive UI or shell completion |
| Batch job | explicit input, idempotency boundary, structured logs | scheduler, queue, checkpointing |
| Frontend | framework and bundler selected by browser/product needs | server code only for server-owned work |
| Edge function | target platform adapter and edge-compatible dependencies | shared package for portable logic |
| Service | app entry point, configuration boundary, health behavior | router, datastore, jobs, observability |

For a frontend's visual direction, palette, typography, and layout, use
`mega-frontend:designing-frontends`; this skill owns project shape and tooling.

Use the package manager and runtime the platform or repository already supports.
For a standalone Node project, `pnpm` is a reasonable default. Start TypeScript
with compiler options that match the published runtime and the codebase's risk
tolerance, then make `tsc --noEmit` part of the project check. Build tools that
only strip types do not replace this check. Select test, lint, and formatting tools as one compatible
set: a repository may use Vitest, Jest, Node's runner, Biome, ESLint, or another
established choice.

## Layout and boundaries

Separate entry-point wiring from domain logic when that improves testability.
A small script does not need a service directory tree, while a packaged library
usually benefits from source and test directories.

```text
project/
├── package.json
├── tsconfig.json
├── src/
│   ├── index.ts or cli.ts
│   └── domain/
└── test/
```

Validate external data at the boundary using the project's schema library or
explicit parsing. Choose an ORM, query builder, or driver only after considering
the data model and operational needs. Add migrations for evolving persistent
schema and define how deployment applies and recovers from them.
Keep credentials out of source and checked-in configuration. Load them from the
deployment's secret store or ignored local configuration.

## Platform and concurrency decisions

Use the web framework that fits the runtime and team, including no framework
for a small adapter. Add CORS, proxy handling, middleware, and rate limiting only
when the deployed boundary requires them; configure them from actual origins,
proxies, and traffic controls.

Decide whether the artifact needs concurrent work from its latency, ordering,
failure, and resource constraints. For promise and limiter patterns, use
`mega-ts:typescript-patterns`.

## Test shape

Run the project's configured test and type-check commands. For a packaged
library, test its public exports through the built package rather than internal
source paths.

## When to use this skill

- Starting a TypeScript library, CLI, batch job, frontend, edge function, or service.
- Choosing the first project shape and its tooling.
- For patterns in an existing codebase, use mega-ts:typescript-patterns.
