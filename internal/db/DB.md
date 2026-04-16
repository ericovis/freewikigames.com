# Database Package Developer Guide

This guide is for contributors adding DAOs, migrations, or cross-table services to `internal/db`.

## Overview

`internal/db` provides three things:

1. **Connection management** — `DB` wraps a `pgxpool.Pool` and exposes typed DAO accessors.
2. **DAO-style data access** — every table has its own file with a model type and a DAO struct. Each DAO method maps to a single SQL statement. No business logic lives here.
3. **Migration system** — a lightweight embedded-SQL runner tracks applied migrations in `schema_migrations`. Migrations live in `migrations/` at the repo root and are embedded at compile time.

## File Responsibilities

| File              | Responsibility                                                           |
|-------------------|--------------------------------------------------------------------------|
| `db.go`           | `DB` struct, `New()`, `Close()`, DAO accessors (`Pages()`, …)           |
| `page.go`         | `Page` model, `PageDAO`, all CRUD for `raw_pages`                       |
| `migrate.go`      | `Migrate()`, `MigrateStatus()`, embedded-SQL runner, `schema_migrations` |

Test files:

| File                    | Responsibility                                              |
|-------------------------|-------------------------------------------------------------|
| `testhelper_test.go`    | `TestMain`, `connectTestDB`, `truncateTables`, `runMigrations` |
| `migrate_test.go`       | Integration tests for the migration runner                  |
| `page_test.go`          | Integration tests for `PageDAO`                             |

## Key Types

### `DB`
```go
type DB struct { pool *pgxpool.Pool }

func New(ctx context.Context, dsn string) (*DB, error)
func (d *DB) Close()
func (d *DB) Pages() *PageDAO
```

### `Page`
```go
type Page struct {
    ID            int64
    URL           string
    RawHTML       string
    LastScrapedAt time.Time
    CreatedAt     time.Time
}
```

### `PageDAO`
```go
func (d *PageDAO) Upsert(ctx, url, rawHTML string, scrapedAt time.Time) (*Page, error)
func (d *PageDAO) FindByURL(ctx, url string) (*Page, error)  // nil, nil if not found
func (d *PageDAO) FindOldestScraped(ctx, limit int) ([]Page, error)
func (d *PageDAO) List(ctx, limit, offset int) ([]Page, error)
func (d *PageDAO) Delete(ctx, url string) error
```

## Migration System

Migrations live in `internal/db/migrations/` as numbered SQL files:

```
internal/db/migrations/
  0001_create_raw_pages.sql
  0002_add_next_feature.sql   ← future
```

### Adding a new migration

1. Create `migrations/NNNN_description.sql` where `NNNN` is the next sequential number (zero-padded to 4 digits).
2. Write idempotent SQL (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, etc.).
3. Run `go run ./cmd/migrate up` to apply.

The runner sorts files lexicographically, so the numeric prefix controls order. Each file runs inside its own transaction. On success its filename is recorded in `schema_migrations`.

### Running migrations

```bash
# Check status
go run ./cmd/migrate status

# Apply pending migrations
go run ./cmd/migrate up
```

Both commands read `DATABASE_URL` from the environment.

## Services Pattern

When an operation spans multiple tables or requires a coordinated transaction across DAOs, create a service file:

```
internal/db/service_<domain>.go
```

A service accepts `*DB`, calls multiple DAOs, and wraps everything in a transaction:

```go
func DoComplexOperation(ctx context.Context, d *DB, ...) error {
    tx, err := d.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    // Use tx directly or pass it to DAO helpers that accept pgx.Tx.

    return tx.Commit(ctx)
}
```

The current scope (one table) does not require a service yet. Add one when you first need cross-table atomicity.

## Testing Conventions

- **Real database required.** Tests in this package hit a live PostgreSQL instance. Set `DATABASE_URL` or rely on the default (`postgres://freewiki:freewiki@localhost:5432/freewiki?sslmode=disable`).
- **`TestMain` runs migrations once** before all tests in the binary.
- **Call `truncateTables(t)` and `runMigrations(t)` at the top of each test** to start from a clean, fully-migrated state.
- **No mocks.** The pgx pool is the real thing. Mock-based tests diverge from prod behavior and mask schema mismatches.

## Anti-Patterns

- **Don't put business logic in DAOs.** A DAO method is one SQL statement. Decisions, loops, and orchestration belong in callers or services.
- **Don't skip `truncateTables` in tests.** Leftover rows cause false failures and test ordering dependencies.
- **Don't write raw SQL in non-DAO files.** If a caller needs a DB query, add a method to the appropriate DAO instead.
- **Don't use `SELECT *`.** Always name columns explicitly so schema changes produce clear compile or scan errors.
- **Don't ignore `rows.Err()`.** Always check it after draining a `pgx.Rows` cursor.
- **Don't add down migrations.** The runner is intentionally up-only. Production rollbacks are handled by forward migrations.
