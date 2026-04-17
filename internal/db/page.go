package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Page represents a row in the raw_pages table.
type Page struct {
	ID            int64
	URL           string
	RawHTML       string
	LastScrapedAt time.Time
	CreatedAt     time.Time
}

// PageDAO provides data-access methods for the raw_pages table.
// Each method maps to a single SQL statement.
type PageDAO struct {
	pool *pgxpool.Pool
}

// Upsert inserts a new page or updates raw_html and last_scraped_at if the URL
// already exists. It returns the full persisted row.
func (d *PageDAO) Upsert(ctx context.Context, url, rawHTML string, scrapedAt time.Time) (*Page, error) {
	row := d.pool.QueryRow(ctx, `
		INSERT INTO raw_pages (url, raw_html, last_scraped_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (url) DO UPDATE
			SET raw_html        = EXCLUDED.raw_html,
			    last_scraped_at = EXCLUDED.last_scraped_at
		RETURNING id, url, raw_html, last_scraped_at, created_at
	`, url, rawHTML, scrapedAt)

	return scanPage(row)
}

// FindByURL returns the page with the given URL, or (nil, nil) if not found.
func (d *PageDAO) FindByURL(ctx context.Context, url string) (*Page, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, url, raw_html, last_scraped_at, created_at
		FROM raw_pages
		WHERE url = $1
	`, url)

	page, err := scanPage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return page, err
}

// FindOldestScraped returns up to limit pages ordered by last_scraped_at ascending
// (i.e. the pages that were scraped longest ago come first).
func (d *PageDAO) FindOldestScraped(ctx context.Context, limit int) ([]Page, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, url, raw_html, last_scraped_at, created_at
		FROM raw_pages
		ORDER BY last_scraped_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectPages(rows)
}

// List returns a paginated slice of pages ordered by id ascending.
func (d *PageDAO) List(ctx context.Context, limit, offset int) ([]Page, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, url, raw_html, last_scraped_at, created_at
		FROM raw_pages
		ORDER BY id ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectPages(rows)
}

// FindWithoutQuestions returns up to limit pages that have no rows in the
// questions table, ordered by id ascending (oldest first).
func (d *PageDAO) FindWithoutQuestions(ctx context.Context, limit int) ([]Page, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT p.id, p.url, p.raw_html, p.last_scraped_at, p.created_at
		FROM raw_pages p
		WHERE NOT EXISTS (
			SELECT 1 FROM questions q WHERE q.page_id = p.id
		)
		ORDER BY p.id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectPages(rows)
}

// Delete removes the page with the given URL. It is a no-op if the URL does not exist.
func (d *PageDAO) Delete(ctx context.Context, url string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM raw_pages WHERE url = $1`, url)
	return err
}

// scanPage scans a single row into a Page.
func scanPage(row pgx.Row) (*Page, error) {
	var p Page
	err := row.Scan(&p.ID, &p.URL, &p.RawHTML, &p.LastScrapedAt, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// collectPages drains a pgx.Rows into a slice of Page.
func collectPages(rows pgx.Rows) ([]Page, error) {
	var pages []Page
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.URL, &p.RawHTML, &p.LastScrapedAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}
