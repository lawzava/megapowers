# mega-go (plugin)

An opinionated Go bundle: a greenfield stack picker for new projects, a
reference of idiomatic Go patterns, and the pattern for agent-authored
scripts, so glue is `go run` even inside a non-Go repository.

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
- `scripting-in-go`: stdlib `go run` helpers for agent glue, probes, and
  one-off tool calls. Use this instead of bash, Python, or Node scripts,
  including inside a Python or TypeScript repository.

`greenfield-go-stack` covers the stack and bootstrap order; it delegates Go
idioms to `golang-patterns` rather than restating them. Agent glue is
`scripting-in-go`.

## Prerequisites

`greenfield-go-stack` can use the context7 MCP server to fetch current library
documentation while scaffolding. This is optional: the skill degrades
gracefully and still works without it.

## Install

```
/plugin install mega-go@megapowers
```

For Codex installation and OpenCode's portable-skills-only setup, use the
canonical [setup guide](../../docs/setup.md). OpenCode does not load this as a
native plugin bundle.

## Attribution

The `golang-patterns` skill is vendored from Everything Claude Code
(MIT, (c) 2026 Affaan Mustafa),
https://github.com/affaan-m/everything-claude-code. See the repository
ATTRIBUTION.md for details.
