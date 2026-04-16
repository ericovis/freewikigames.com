package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool connection pool and exposes DAO accessors.
type DB struct {
	pool *pgxpool.Pool
}

// New opens a connection pool using the given DSN, verifies connectivity with a
// ping, and returns a ready-to-use DB. Call Close when the application exits.
func New(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close releases all connections in the pool. It is safe to call multiple times.
func (d *DB) Close() {
	d.pool.Close()
}

// Pages returns a PageDAO bound to this DB's connection pool.
func (d *DB) Pages() *PageDAO {
	return &PageDAO{pool: d.pool}
}
