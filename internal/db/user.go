package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User represents a row in the users table.
type User struct {
	ID        int64
	Username  string
	CreatedAt time.Time
}

// UserDAO provides data-access methods for the users table.
// Each method maps to a single SQL statement.
type UserDAO struct {
	pool *pgxpool.Pool
}

// Insert creates a new user and returns the persisted row.
func (d *UserDAO) Insert(ctx context.Context, username string) (*User, error) {
	row := d.pool.QueryRow(ctx, `
		INSERT INTO users (username)
		VALUES ($1)
		RETURNING id, username, created_at
	`, username)
	return scanUser(row)
}

// FindByUsername returns the user with the given username, or (nil, nil) if not found.
func (d *UserDAO) FindByUsername(ctx context.Context, username string) (*User, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, username, created_at
		FROM users
		WHERE username = $1
	`, username)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// FindByID returns the user with the given id, or (nil, nil) if not found.
func (d *UserDAO) FindByID(ctx context.Context, id int64) (*User, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, username, created_at
		FROM users
		WHERE id = $1
	`, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}
