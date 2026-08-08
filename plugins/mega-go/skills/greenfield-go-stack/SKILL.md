---
name: greenfield-go-stack
description: >
  Use to start or scaffold a Go project, service, SaaS, backend, or
  server-rendered app, or choose its framework, database, auth, payments,
  hosting, and
  architecture.
license: MIT
---

# Greenfield Go Stack

Choose the project shape before choosing a framework or provider. Establish the
deliverable, interfaces, runtime constraints, persistence, deployment target,
and integrations. If an answer affects the choice and is unknown, ask before
scaffolding.

## Select a shape

| Need | Start with |
|---|---|
| Importable library | A root package; add `internal/` only for implementation packages callers must not import. |
| Small CLI | A root `main` package. |
| Multiple commands or a mixed library and CLI module | `cmd/<command>/` entry points and shared `internal/` packages. |
| HTTP service | A thin command entry point and `internal/` service packages. Choose `net/http` unless a framework solves a stated need. |
| Server-rendered web app | The HTTP service shape; choose templates and client-side behavior from the interaction requirements. |
| Service-to-service RPC | A separately defined contract and an RPC transport when independent clients, streaming, or generated clients justify it. |
| Edge or platform function | The host's Go runtime and deployment constraints first; do not assume containers or a long-lived server. |

The [official Go module layout guide](https://go.dev/doc/modules/layout) is the
layout authority. Keep packages private in `internal/` until external callers
need a supported API; split a package into its own module only when it needs an
independent versioning and release boundary.

## Add capabilities only when required

- Select a database and driver from durability, concurrency, query, and hosting
  constraints. Keep migrations and production backup/restore requirements in
  the initial design.
- Select auth, payment, email, and observability providers only for stated
  product requirements. Keep provider-specific calls behind an application
  boundary where replacement is plausible.
- Keep credentials out of source and checked-in configuration. Load them from
  the deployment's secret store or ignored local configuration.
- Add a container image only when the target runs one. Match build settings,
  base image, user, and health checks to the selected runtime.
- Consult current official documentation before adding any dependency.

## Optional recipes

Read a recipe only after selecting its technology. They record dated integration
details and should be rechecked against the linked official documentation.

- [references/fiber-baseline.md](references/fiber-baseline.md): Fiber v3 HTTP
  middleware and trusted proxies.
- [references/sqlite-bun.md](references/sqlite-bun.md): SQLite and Bun.
- [references/docker-wolfi.md](references/docker-wolfi.md): static Go binary
  in a Wolfi image.

For context, errors, and goroutine lifecycles, use mega-go:golang-patterns.
