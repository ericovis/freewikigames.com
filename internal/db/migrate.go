package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrationStatus describes a single migration file and whether it has been applied.
type MigrationStatus struct {
	Version string
	Applied bool
}

// Migrate applies all pending migrations in order. It creates the schema_migrations
// tracking table if it does not exist. Each migration runs inside its own transaction.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	files, err := migrationFiles()
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	for _, file := range files {
		version := migrationVersion(file)
		if applied[version] {
			continue
		}

		sql, err := fs.ReadFile(migrationsFS, file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		if err := applyMigration(ctx, pool, version, string(sql)); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
	}

	return nil
}

// MigrateStatus returns the applied/pending status for every migration file.
func MigrateStatus(ctx context.Context, pool *pgxpool.Pool) ([]MigrationStatus, error) {
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return nil, err
	}

	files, err := migrationFiles()
	if err != nil {
		return nil, err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return nil, err
	}

	statuses := make([]MigrationStatus, 0, len(files))
	for _, file := range files {
		version := migrationVersion(file)
		statuses = append(statuses, MigrationStatus{
			Version: version,
			Applied: applied[version],
		})
	}

	return statuses, nil
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func migrationFiles() ([]string, error) {
	entries, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	return entries, nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, version, sql string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// migrationVersion extracts the base filename (without directory) from a path like
// "migrations/0001_create_raw_pages.sql" → "0001_create_raw_pages.sql".
func migrationVersion(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
