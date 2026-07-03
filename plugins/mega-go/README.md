# mega-go (plugin)

mega-go is an opinionated Go bundle for Claude Code. It pairs a greenfield
stack picker for new projects with a reference of idiomatic Go patterns, so a
fresh Go repo starts from a coherent set of technology choices and is written
in a consistent, idiomatic style.

## Skills

| Skill | What it does |
| --- | --- |
| `greenfield-go-stack` | An opinionated stack plus bootstrap-order picker for new Go projects. Defaults to the golang-standards project layout, GoFiber at the edge, gRPC with buf for internal services, templ/templui server-side rendering with Tailwind, SQLite via the pure-Go modernc.org/sqlite driver, the Bun ORM, Clerk for auth, Stripe for payments, Cloudflare for email, golangci-lint with the Uber style guide, and Wolfi-based Docker images. |
| `golang-patterns` | A reference of idiomatic Go patterns: functional options, small interfaces, dependency injection, worker pools and context handling, error wrapping with sentinel and custom errors, and table-driven tests. |

## How the skills relate

`greenfield-go-stack` covers the stack and bootstrap order; it delegates Go
idioms to the `golang-patterns` skill rather than restating them. Install this
bundle to get both. If you only want the patterns reference, `golang-patterns`
is also installable on its own through the `golang-patterns` marketplace entry.

## Prerequisites

`greenfield-go-stack` can use the context7 MCP server to fetch current library
documentation while scaffolding. This is optional: the skill degrades
gracefully and still works if context7 is not available.

## Install

```
/plugin install mega-go@megapowers
```

### Standalone skills

`golang-patterns` is also published as a standalone marketplace entry. Install
the bundle **or** the standalone skill, not both — a skill installed twice
registers twice.

## Attribution

The `golang-patterns` skill is vendored from Everything Claude Code
(MIT, (c) 2026 Affaan Mustafa) —
https://github.com/affaan-m/everything-claude-code. See the repository
ATTRIBUTION.md for details.
