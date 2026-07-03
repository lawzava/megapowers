---
name: greenfield-ts-stack
description: >-
  Use when starting a new TypeScript project or service and choosing its stack and
  layout — package manager, tsconfig, test runner, lint/format, web framework, and
  validation. Triggers on "new TypeScript project", "scaffold a Node service", "set
  up a Hono/Fastify app", "which TS stack". For idioms within existing code, use
  mega-ts:typescript-patterns.
---

# Greenfield TypeScript Stack

Opinionated defaults for a new TypeScript service in 2026. Time-stamped: these
reflect today's tools and will go stale — adapt them.

For a non-trivial new project, run megapowers:brainstorming and
megapowers:writing-plans first (if installed) — this skill supplies the stack,
not the process.

## Toolchain (pin these)

- **pnpm** — package manager (fast, strict, content-addressed store, a real
  lockfile). Commit `pnpm-lock.yaml`.
- **TypeScript, strict** — `"strict": true` plus `"noUncheckedIndexedAccess": true`,
  `"exactOptionalPropertyTypes": true`, `"noImplicitOverride": true`. Types are a
  correctness tool; CI fails on a type error (`tsc --noEmit`).
- **vitest** — tests (fast, TS-native, ESM-first).
- **Biome** (lint + format in one) *or* eslint + prettier — pick one and commit its
  config. Biome is the fewer-moving-parts default.
- **zod** — runtime validation at the boundaries; infer static types from schemas
  (`z.infer`) so the validator and the type never drift.
- Target **ESM** (`"type": "module"`) and a current Node LTS.

## Web: Hono (or Fastify)

Hono for a small, fast, edge-portable service; Fastify when you want its plugin
ecosystem. Validate every request body/query with zod and derive the handler's types
from the schema.

### Middleware order

Middleware wraps in the order registered: the **first** registered is the
**outermost** (runs first inbound, last outbound). So register the **logger first**
so it sees every request — including ones a later middleware rejects with a 4xx/429.
(Hono's `app.use(logger())` first; then CORS with explicit origins, then rate limit.)

```ts
app.use('*', logger())                       // outermost: logs even the 429 below
app.use('*', cors({ origin: env.CORS_ORIGINS }))  // explicit, never '*'
app.use('*', rateLimit())                    // rejects with 429
```

### Rate limiting behind a proxy

A limiter keyed on the client IP must read the *real* client IP, or every request
behind a reverse proxy shares the proxy's IP and one bucket. Derive the IP from the
forwarded header **only** for a trusted proxy (never trust `X-Forwarded-For` from
arbitrary clients — they can spoof it). Configure your platform's trusted-proxy list.

## Layout

```
myservice/
├── package.json          # "type": "module"
├── pnpm-lock.yaml
├── tsconfig.json         # strict
├── src/
│   ├── index.ts          # wiring only
│   ├── routes/
│   ├── domain/           # logic, no framework imports
│   └── db.ts
└── test/
```

Keep framework imports out of `domain/` — logic you can test without a server.

## Database & test setup

Use a typed query builder or ORM (Drizzle, Prisma, or Kysely) with migrations that
have an up and a down. For tests against SQLite, an in-memory DB is tied to its
connection — share one connection/instance across the test (e.g. a single
better-sqlite3 instance) so schema created in setup is visible to every query, rather
than opening a fresh in-memory DB per query.

## Async & concurrency (see typescript-patterns for detail)

- **Never leave a promise floating** — every promise is awaited or explicitly handled;
  enable `no-floating-promises`. An unhandled rejection can crash the process.
- Run independent async work with `Promise.all` (or `Promise.allSettled` when you
  need every result regardless of failures) — not a sequential `await` loop.

## When to use this skill

- Standing up a new TypeScript/Node project or service.
- Choosing or justifying the stack.
- For idioms and refactors inside existing code, use mega-ts:typescript-patterns.
