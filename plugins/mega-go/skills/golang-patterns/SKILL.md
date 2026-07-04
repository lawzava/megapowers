---
name: golang-patterns
description: >
  Use when writing or refactoring Go and a design choice is in play — interfaces
  vs structs, dependency wiring, concurrency (goroutines/channels/context), error
  shaping, functional options, or package layout. Triggers on "idiomatic Go", "how
  should I structure this", "functional options", "dependency injection",
  "goroutine/channel pattern". Skip for trivial or mechanical edits.
license: MIT
---

# Go Patterns

> **Measured:** current frontier *and* small Claude models already write the
> common concurrency mechanics here (worker pools, pipeline stages, channel
> closing) correctly single-shot without this skill — a controlled study found
> zero correctness headroom, 184/184 passing in both arms (the repo's
> `evals/RESULTS.md` §2). Reach for this skill for the design *choices* —
> interfaces, dependency wiring, error shaping, package layout — as a review
> checklist, or when driving weaker models; not because a current model would
> otherwise deadlock.

Scope: Go files, `go.mod`, and `go.sum`.
Origin: Derived from Everything Claude Code (MIT, (c) 2026 Affaan Mustafa).

> This skill provides comprehensive Go patterns extending common design principles with Go-specific idioms.

## Functional Options

Use the functional options pattern for flexible constructor configuration:

```go
type Option func(*Server)

func WithPort(port int) Option {
    return func(s *Server) { s.port = port }
}

func NewServer(opts ...Option) *Server {
    s := &Server{port: 8080}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

**Benefits:**
- Backward compatible API evolution
- Optional parameters with defaults
- Self-documenting configuration

## Small Interfaces

Define interfaces where they are used, not where they are implemented.

**Principle:** Accept interfaces, return structs

```go
// Good: Small, focused interface defined at point of use
type UserStore interface {
    GetUser(id string) (*User, error)
}

func ProcessUser(store UserStore, id string) error {
    user, err := store.GetUser(id)
    // ...
}
```

**Benefits:**
- Easier testing and mocking
- Loose coupling
- Clear dependencies

## Dependency Injection

Use constructor functions to inject dependencies:

```go
func NewUserService(repo UserRepository, logger Logger) *UserService {
    return &UserService{
        repo:   repo,
        logger: logger,
    }
}
```

**Pattern:**
- Constructor functions (New* prefix)
- Explicit dependencies as parameters
- Return concrete types
- Validate dependencies in constructor

## Concurrency Patterns

### Worker Pool

```go
func workerPool(jobs <-chan Job, results chan<- Result, workers int) {
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                results <- processJob(job)
            }
        }()
    }
    // Close results once all workers finish — in a separate goroutine so this
    // function returns immediately and the caller can range over results while
    // workers are still producing. An inline wg.Wait() here deadlocks: the
    // workers block sending on results until a reader drains it, but the reader
    // only runs after workerPool returns.
    go func() {
        wg.Wait()
        close(results)
    }()
}
```

### Context Propagation

Always pass context as first parameter:

```go
func FetchUser(ctx context.Context, id string) (*User, error) {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ... fetch logic
}
```

## Error Handling

### Error Wrapping

```go
if err != nil {
    return fmt.Errorf("failed to fetch user %s: %w", id, err)
}
```

### Custom Errors

```go
type ValidationError struct {
    Field string
    Msg   string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}
```

### Sentinel Errors

```go
var (
    ErrNotFound = errors.New("not found")
    ErrInvalid  = errors.New("invalid input")
)

// Check with errors.Is
if errors.Is(err, ErrNotFound) {
    // handle not found
}
```

## Package Organization

### Structure

```
project/
├── cmd/              # Main applications
│   └── server/
│       └── main.go
├── internal/         # Private application code
│   ├── domain/       # Business logic
│   ├── handler/      # HTTP handlers
│   └── repository/   # Data access
└── pkg/              # Public libraries
```

### Naming Conventions

- Package names: lowercase, single word
- Avoid stutter: `user.User` not `user.UserModel`
- Use `internal/` for private code
- Keep `main` package minimal

## Testing Patterns

### Table-Driven Tests

```go
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "test@example.com", false},
        {"invalid", "not-an-email", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Helpers

```go
// Import the pure-Go driver: _ "modernc.org/sqlite" (registers as "sqlite").
// Do NOT use "sqlite3" (mattn/go-sqlite3) — it needs CGO and breaks CGO_ENABLED=0
// static builds. The greenfield-go-stack bundle mandates pure-Go/CGO-free.
func testDB(t *testing.T) *sql.DB {
    t.Helper()
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("failed to open test db: %v", err)
    }
    // A ":memory:" database is private to a single connection, and database/sql
    // is a lazy pool: without this, each pooled connection gets its OWN empty
    // database, so a table created on one connection is missing on the next.
    // Pinning the pool to one connection makes the whole test share one in-memory
    // DB. Caveat: with a single connection, while a tx is open (db.Begin) that tx
    // holds the only connection — route ALL work through the tx until Commit/
    // Rollback; a concurrent db.Query on the pool would block waiting for it. If a
    // test genuinely needs multiple live connections, use a shared-cache DSN
    // instead ("file:testdb?mode=memory&cache=shared") and keep >=1 conn open so
    // the in-memory DB isn't dropped.
    db.SetMaxOpenConns(1)
    t.Cleanup(func() { db.Close() })
    return db
}
```

## When to Use This Skill

- Designing Go APIs and packages
- Implementing concurrent systems
- Structuring Go projects
- Writing idiomatic Go code
- Refactoring Go codebases
