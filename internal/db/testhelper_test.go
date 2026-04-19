package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool holds the shared connection pool for the entire test binary.
var testPool *pgxpool.Pool

// TestMain sets up a real database connection, runs all migrations once, then
// executes all tests in the package. Tests must call truncateTables to start
// from a clean state.
func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://freewiki:freewiki@localhost:5432/freewiki?sslmode=disable"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic("connect to test database: " + err.Error())
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		panic("ping test database: " + err.Error())
	}

	if err := Migrate(ctx, pool); err != nil {
		panic("run migrations: " + err.Error())
	}

	testPool = pool
	os.Exit(m.Run())
}

// truncateTables clears test data between individual tests. It also resets the
// schema_migrations table so migration tests start with a clean slate.
func truncateTables(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`TRUNCATE game_answers, game_participants, game_sessions, users, pages, questions, schema_migrations RESTART IDENTITY CASCADE`,
	)
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}
