---
name: greenfield-go-stack
description: >
  Use to start or scaffold a Go project, service, SaaS, backend, or server
  rendered app, or choose its framework, database, auth, payments, hosting, and
  architecture.
license: MIT
---

# Greenfield Go Stack

Scope: greenfield Go repositories and projects with `go.mod`.

For a non-trivial new project, run megapowers:brainstorming and
megapowers:writing-plans first (if installed) — this skill supplies the stack,
not the process.

One predictable, opinionated, secure-by-default stack for new Go projects.
**Pick only the layers the project needs from the fixed menu below — do not
substitute unfamiliar libraries for these defaults.** Performance and security
come from the baseline middleware and minimal images, not bolted on later.

## Core principles

- **SSR by default** — templ + templui. Add client JS only where SSR genuinely
  can't do it (HTMX-style partials before reaching for a SPA).
- **gRPC for internal/service-to-service APIs**, HTTP (Fiber) for the
  browser-facing edge. Prefer gRPC over hand-rolled REST between services.
- **Pure-Go everything** so binaries stay static (CGO off → tiny, secure Wolfi
  images, no glibc).
- **Consult current official docs before wiring each library** using the docs or
  browser tools available in the active harness. Use Context7 if it is installed.

## The stack (fixed menu — include only what's needed)

| Layer | Default | Notes |
|---|---|---|
| Language / layout | Go + golang-standards/project-layout | `cmd/ internal/ pkg/` |
| Web edge | GoFiber | fasthttp; SSR + REST |
| RPC | gRPC (grpc-go + buf) | internal & service-to-service |
| Templating | templ (`github.com/a-h/templ`) | typed, compiled SSR components |
| Components | templui (shadcn-style) | `templui add <c>` copies source in |
| CSS | Tailwind (templui dependency) | |
| DB | SQLite, `modernc.org/sqlite` | **pure Go, no CGO** |
| ORM | Bun (`github.com/uptrace/bun`) | SQL-first; `sqlitedialect` |
| Auth | Clerk (`github.com/clerk/clerk-sdk-go/v2`) | |
| Payments | Stripe (`github.com/stripe/stripe-go/v86`) | Checkout/Elements + webhooks |
| Email | Cloudflare | see Email section (send + receive) |
| Hosting | Docker + docker compose | Wolfi base images |
| Lint | golangci-lint | Uber Go Style Guide config |

## Conventions

- **Style:** Uber Go Style Guide, enforced by golangci-lint (`.golangci.yml`).
- **Layout:** golang-standards/project-layout. Keep `main` thin in
  `cmd/<app>/`; all logic in `internal/`.
- **SQL:** follow the SQL Style Guide (https://www.sqlstyle.guide/) — UPPERCASE
  keywords, snake_case identifiers, consistent layout. Applies to schema,
  migrations, and any raw queries (incl. Bun `bun.Raw`).
- For idiomatic errors / interfaces / functional options / concurrency, use the
  `golang-patterns` skill — don't restate it here.

## Reference recipes

Read the file for the layer you are wiring:

- [references/fiber-baseline.md](references/fiber-baseline.md): the ordered
  middleware chain, explicit rate-limit budget, trusted-proxy config for
  `c.IP()`, and why a global `cache.New()` leaks one user's rendered page to the
  next.
- [references/sqlite-bun.md](references/sqlite-bun.md): pure-Go driver choice,
  open pragmas, and the single-connection in-memory test database.
- [references/docker-wolfi.md](references/docker-wolfi.md): the multi-stage
  `CGO_ENABLED=0` build on a Wolfi base.

## gRPC + SSR split

- **Browser:** Fiber + templ/templui SSR.
- **Internal/API:** gRPC, codegen with `buf`. Fiber is fasthttp (HTTP/1.x) — run
  gRPC on its own listener. If a browser must call gRPC, use Connect or
  grpc-gateway / grpc-web.

## Email (Cloudflare)

- **Receive:** Cloudflare Email Routing → Email Worker → webhook into the app.
- **Send:** Cloudflare Email Service / Worker `send_email` binding (SPF+DKIM).
  Note: Email Service is **beta** — keep a provider (Resend) behind an interface
  as the production-stable fallback. The old MailChannels free integration is
  deprecated; do not use it.

## Bootstrap order

1. `go mod init`; scaffold golang-standards layout.
2. `.golangci.yml` (Uber style); make lint a CI gate.
3. Fiber app + middleware baseline (references/fiber-baseline.md).
4. `templ` + `templui init`; Tailwind.
5. SQLite (`modernc.org/sqlite`) + Bun; migrations (references/sqlite-bun.md).
6. Add only the needed integrations: Clerk / Stripe / gRPC / Cloudflare email.
7. Dockerfile (Wolfi) + `docker compose` (references/docker-wolfi.md).

## Caveats

- Cloudflare Email Service is beta — verify current limits before relying on it
  for critical transactional mail.
- Confirm current majors using available official documentation (Fiber v2/v3,
  `stripe-go`, `clerk-sdk-go`); use Context7 if it is installed.
