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

## Fiber middleware baseline (perf + security out of the box)

Wire these in order on every app. CORS is explicit origins, never `*`.

```go
app.Use(recover.New())
app.Use(requestid.New())
app.Use(logger.New())                              // early: a request rejected by
                                                   // a later middleware (429, CORS)
                                                   // still gets logged
app.Use(helmet.New())                              // security headers
app.Use(cors.New(cors.Config{AllowOrigins: origins})) // explicit, not "*"
app.Use(compress.New())
app.Use(etag.New())
app.Use(limiter.New(limiter.Config{               // explicit budget — the zero
    Max:        120,                               // value (~5/min) breaks real
    Expiration: 1 * time.Minute,                   // pages (each pulls many assets)
}))
```

The limiter keys on `c.IP()` by default. Behind a reverse proxy every request
arrives from the proxy's IP, so **all users share one bucket** unless you make
Fiber trust the forwarded client IP. Set that on the app config, not the
middleware:

```go
app := fiber.New(fiber.Config{
    ProxyHeader:             fiber.HeaderXForwardedFor,
    EnableTrustedProxyCheck: true,
    TrustedProxies:          []string{"10.0.0.0/8"}, // your proxy's CIDR
})
```

Only enable this when you actually sit behind a trusted proxy — trusting
`X-Forwarded-For` from untrusted clients lets them spoof their IP and evade the
limiter.

**Do not put `cache.New()` in the global chain.** Fiber's cache keys on request
path by default, with no user/session in the key — so on an app with auth + SSR it
serves one user's rendered `GET /dashboard` to the next user for the whole TTL: a
cross-user data leak. Response-cache only *explicitly public, non-personalized* routes,
and only with a key that includes everything that varies the response:

```go
public := app.Group("/assets") // or a public, auth-free route group
public.Use(cache.New(cache.Config{
    Expiration:   10 * time.Minute,
    CacheControl: true,
    // KeyGenerator MUST include anything that changes the body (path is not enough
    // once cookies/headers/query vary the response). Never mount this on authed routes.
}))
```
For authenticated pages, rely on `etag` + per-handler `Cache-Control` instead of a
shared response cache.

## SQLite + Bun (CGO-free)

Use `modernc.org/sqlite` so the binary is fully static and builds on minimal
Wolfi/static images. Bun over a `database/sql` handle with `sqlitedialect`.
Set pragmas on open: `_pragma=journal_mode(WAL)`, `_pragma=foreign_keys(ON)`,
`_pragma=busy_timeout(5000)`.

Test databases:

```go
// Import the pure-Go driver: _ "modernc.org/sqlite" (registers as "sqlite").
// Do NOT use "sqlite3" (mattn/go-sqlite3) — it needs CGO and breaks
// CGO_ENABLED=0 static builds.
func testDB(t *testing.T) *sql.DB {
    t.Helper()
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("failed to open test db: %v", err)
    }
    // A ":memory:" database is private to a single connection, and database/sql
    // is a lazy pool: without this, each pooled connection gets its OWN empty
    // database, so a table created on one connection is missing on the next.
    // Pinning the pool to one connection makes the whole test share one
    // in-memory DB. Caveat: with a single connection, while a tx is open
    // (db.Begin) that tx holds the only connection — route ALL work through
    // the tx until Commit/Rollback; a concurrent db.Query on the pool would
    // block waiting for it. If a test genuinely needs multiple live
    // connections, use a shared-cache DSN instead
    // ("file:testdb?mode=memory&cache=shared") and keep >=1 conn open so the
    // in-memory DB isn't dropped.
    db.SetMaxOpenConns(1)
    t.Cleanup(func() { db.Close() })
    return db
}
```

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

## Docker / Wolfi

Multi-stage: build on `golang` with `CGO_ENABLED=0`, run on
`cgr.dev/chainguard/wolfi-base` (or `static`). Non-root, healthcheck. Use
`docker compose` for local (app + sidecars).

```dockerfile
FROM golang:1-bookworm AS build
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server

FROM cgr.dev/chainguard/wolfi-base
RUN apk add --no-cache ca-certificates
COPY --from=build /app /app
USER nonroot
ENTRYPOINT ["/app"]
```

## Bootstrap order

1. `go mod init`; scaffold golang-standards layout.
2. `.golangci.yml` (Uber style); make lint a CI gate.
3. Fiber app + middleware baseline above.
4. `templ` + `templui init`; Tailwind.
5. SQLite (`modernc.org/sqlite`) + Bun; migrations.
6. Add only the needed integrations: Clerk / Stripe / gRPC / Cloudflare email.
7. Dockerfile (Wolfi) + `docker compose`.

## Caveats

- Cloudflare Email Service is beta — verify current limits before relying on it
  for critical transactional mail.
- Confirm current majors using available official documentation (Fiber v2/v3,
  `stripe-go`, `clerk-sdk-go`); use Context7 if it is installed.
