# SQLite + Bun (CGO-free)

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
