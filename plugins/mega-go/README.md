# mega-go (plugin)

An opinionated Go bundle: a greenfield stack picker for new projects plus a
reference of idiomatic Go patterns, so a fresh Go repo starts from a coherent
set of technology choices and is written in a consistent style.

## Skills

- `greenfield-go-stack`: an opinionated stack plus bootstrap-order picker for
  new Go projects. The default stack: the golang-standards project layout,
  GoFiber at the edge, gRPC with buf for internal services, templ/templui
  server-side rendering with Tailwind, SQLite via the pure-Go
  modernc.org/sqlite driver, the Bun ORM, Clerk for auth, Stripe for
  payments, Cloudflare for email, golangci-lint with the Uber style guide,
  and Wolfi-based Docker images.
- `golang-patterns`: a reference of idiomatic Go patterns: functional
  options, small interfaces, dependency injection, worker pools and context
  handling, error wrapping with sentinel and custom errors, and table-driven
  tests.

`greenfield-go-stack` covers the stack and bootstrap order; it delegates Go
idioms to `golang-patterns` rather than restating them.

## Prerequisites

`greenfield-go-stack` can use the context7 MCP server to fetch current library
documentation while scaffolding. This is optional: the skill degrades
gracefully and still works without it.

## Install

```
/plugin install mega-go@megapowers
```

## Attribution

The `golang-patterns` skill is vendored from Everything Claude Code
(MIT, (c) 2026 Affaan Mustafa),
https://github.com/affaan-m/everything-claude-code. See the repository
ATTRIBUTION.md for details.
