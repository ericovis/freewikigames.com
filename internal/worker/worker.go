// Package worker provides long-running background workers that tie together
// the scraper, database, and AI question generator. Two worker types exist:
//
//   - ScrapeWorker: given a list of search terms, continuously crawls Wikipedia
//     and persists pages to the database until the context is cancelled.
//
//   - QuestionWorker: polls for pages without questions, generates questions via
//     the AI, and stores them in the database.
//
// Both workers run until ctx is cancelled, making them suitable for use with
// signal.NotifyContext in long-running cmd binaries.
package worker

import (
	"context"
	"time"

	"github.com/ericovis/freewikigames.com/internal/db"
	"github.com/ericovis/freewikigames.com/internal/questions"
	"github.com/ericovis/freewikigames.com/internal/scraper"
)

// pageDAO is the subset of db.PageDAO used by workers.
type pageDAO interface {
	Upsert(ctx context.Context, url, language, title, summary, content string, datePublished, dateModified *time.Time) (*db.Page, error)
	FindWithoutQuestions(ctx context.Context, limit int) ([]db.Page, error)
}

// questionDAO is the subset of db.QuestionDAO used by workers.
type questionDAO interface {
	Insert(ctx context.Context, pageID int64, text string, choices []db.Choice) (*db.Question, error)
}

// scraperIface is the subset of scraper.Scraper used by ScrapeWorker.
type scraperIface interface {
	ScrapeURLs(ctx context.Context, urls []string) <-chan scraper.ScrapeResult
}

// questionGenerator is the interface for generating language-aware questions.
type questionGenerator interface {
	GenerateWithLanguage(ctx context.Context, title, language, summary, content string) ([]questions.Question, error)
}
