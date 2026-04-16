# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**freewikigames.com** is a game platform that scrapes Wikipedia, stores raw HTML in PostgreSQL, and uses AI to generate trivia questions and other game content. Players can play solo or multiplayer. The backend is entirely Go.

Go module: `github.com/ericovis/freewikigames.com`

## Common Commands

```bash
# Build all binaries
go build ./...

# Run all tests (verbose)
./cmd/test

# Run a specific package
./cmd/test ./internal/db/...

# Run a single test
./cmd/test -run TestMigrate ./internal/db/...

# Run linter (golangci-lint expected)
golangci-lint run ./...

# Format code
goimports -w .
```

## Architecture

The project follows a monorepo layout with distinct binaries and internal shared packages:

```
cmd/
  api/        — HTTP API server (game sessions, user-facing endpoints)
  migrate/    — Migration CLI (up, status)
  scraper/    — Wikipedia scraping worker
  worker/     — Background jobs (AI question generation, dedup analysis)
internal/
  db/         — Database access layer (PostgreSQL via pgx)
    migrations/ — Embedded SQL migration files
  scraper/    — Wikipedia fetch + HTML storage logic
  questions/  — AI-driven question generation and similarity/dedup logic
  game/       — Game session, lobby, and multiplayer state management
  ai/         — Ollama client integration for content generation (Gemma4)
```

### Key Architectural Decisions

**Scraper pipeline**: Scrapers fetch Wikipedia article HTML and store the full raw HTML in PostgreSQL (not just parsed content). Downstream workers read from this raw store to extract and analyze content.

**AI question generation**: A worker reads scraped articles and calls a local Ollama instance (Gemma4 model) to produce trivia questions. Before inserting new questions, a similarity check (via pgvector embeddings stored in Postgres) deduplicates against existing content.

**Multiplayer**: Real-time game sessions are managed server-side; the API binary handles both REST endpoints and WebSocket connections for live gameplay.

**Database**: PostgreSQL is the single store for raw HTML, structured game data, embeddings (pgvector), and user/session state.

## Database Package

The `internal/db` package implements connection management, DAO-style data access, and migration execution. See [`internal/db/DB.md`](internal/db/DB.md) for the full developer guide, including how to add DAOs, write migrations, and create cross-table services.

```bash
# Apply pending migrations
go run ./cmd/migrate up

# Check migration status
go run ./cmd/migrate status
```

## Scraper Package

The `internal/scraper` package implements all Wikipedia fetch logic. See [`internal/scraper/SCRAPER.md`](internal/scraper/SCRAPER.md) for the full developer guide, including how to add new crawler modes, the worker pool and rate-limiter design, and testing conventions.

## AI Package

The `internal/ai` package is a thin HTTP client for the local Ollama instance. See [`internal/ai/AI.md`](internal/ai/AI.md) for the developer guide, including the request/response shape, environment variable conventions, and testing patterns.

## Questions Package

The `internal/questions` package generates trivia questions from raw Wikipedia HTML using the local Ollama instance. See [`internal/questions/QUESTIONS.md`](internal/questions/QUESTIONS.md) for the developer guide, including the LLM prompt strategy, response validation rules, and testing conventions.

## Package Conventions

Every `internal/` package must include an architecture reference file (e.g. `PACKAGE.md`) that documents its purpose, key types, design decisions, and testing conventions. Existing examples:

- [`internal/db/DB.md`](internal/db/DB.md)
- [`internal/scraper/SCRAPER.md`](internal/scraper/SCRAPER.md)
- [`internal/ai/AI.md`](internal/ai/AI.md)
- [`internal/questions/QUESTIONS.md`](internal/questions/QUESTIONS.md)

When adding a new `internal/` package, create the architecture file alongside the first `.go` file.

## Testing

**Every change must have tests and must pass `./cmd/test` before it is considered done.**

### Running tests

Use `./cmd/test` rather than `go test ./...` directly. The script sources `.env` (if present) and exports the required env vars with safe defaults so database-backed tests connect correctly.

```bash
# Run all tests (verbose)
./cmd/test

# Run a specific package
./cmd/test ./internal/db/...

# Run a single test
./cmd/test -run TestMigrate ./internal/db/...
```

### Writing tests

- **Package**: place test files in the same package as the code under test (`package foo`, not `package foo_test`) so unexported internals are accessible.
- **Database tests**: use `TestMain` + `truncateTables` + `runMigrations` (see `internal/db/testhelper_test.go`) to share one real connection pool per package. Never mock the database — integration tests must hit a real Postgres instance.
- **HTTP/network tests**: use `net/http/httptest` servers instead of real external endpoints.
- **Test names**: `Test<Type>_<Method>_<Scenario>` (e.g. `TestPageDAO_Upsert_Insert`).
- **No test helpers in non-test files**: helper functions used only in tests go in `*_test.go` files.
- **Env vars in tests**: rely on the defaults already set by `./cmd/test`; never hard-code credentials.

## Local Development

Infrastructure runs via Docker Compose; the Go API is run directly on the host.

```bash
# Start infrastructure
docker compose up -d

# Stop infrastructure
docker compose down
```

Expected `.env` variables (never committed):

```
DATABASE_URL=postgres://freewiki:freewiki@localhost:5432/freewiki?sslmode=disable
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=gemma4:e2b
```
