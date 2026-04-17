package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Page represents a row in the pages table.
type Page struct {
	ID            int64
	URL           string
	Language      string
	Title         string
	Summary       string
	Content       string     // markdown
	DatePublished *time.Time // nullable
	DateModified  *time.Time // nullable
}

// PageDAO provides data-access methods for the pages table.
// Each method maps to a single SQL statement.
type PageDAO struct {
	pool *pgxpool.Pool
}

// Upsert inserts a new page or updates all structured fields if the URL already
// exists. It returns the full persisted row.
func (d *PageDAO) Upsert(ctx context.Context, url, language, title, summary, content string, datePublished, dateModified *time.Time) (*Page, error) {
	row := d.pool.QueryRow(ctx, `
		INSERT INTO pages (url, language, title, summary, content, date_published, date_modified)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (url) DO UPDATE
			SET language       = EXCLUDED.language,
			    title          = EXCLUDED.title,
			    summary        = EXCLUDED.summary,
			    content        = EXCLUDED.content,
			    date_published = EXCLUDED.date_published,
			    date_modified  = EXCLUDED.date_modified
		RETURNING id, url, language, title, summary, content, date_published, date_modified
	`, url, language, title, summary, content, datePublished, dateModified)

	return scanPage(row)
}

// FindByURL returns the page with the given URL, or (nil, nil) if not found.
func (d *PageDAO) FindByURL(ctx context.Context, url string) (*Page, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, url, language, title, summary, content, date_published, date_modified
		FROM pages
		WHERE url = $1
	`, url)

	page, err := scanPage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return page, err
}

// List returns a paginated slice of pages ordered by id ascending.
func (d *PageDAO) List(ctx context.Context, limit, offset int) ([]Page, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, url, language, title, summary, content, date_published, date_modified
		FROM pages
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
		SELECT p.id, p.url, p.language, p.title, p.summary, p.content, p.date_published, p.date_modified
		FROM pages p
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
	_, err := d.pool.Exec(ctx, `DELETE FROM pages WHERE url = $1`, url)
	return err
}

// scanPage scans a single row into a Page.
func scanPage(row pgx.Row) (*Page, error) {
	var p Page
	err := row.Scan(&p.ID, &p.URL, &p.Language, &p.Title, &p.Summary, &p.Content, &p.DatePublished, &p.DateModified)
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
		if err := rows.Scan(&p.ID, &p.URL, &p.Language, &p.Title, &p.Summary, &p.Content, &p.DatePublished, &p.DateModified); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}
