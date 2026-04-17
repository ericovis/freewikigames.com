package worker

import (
	"context"
	"log/slog"
	"sync"
)

// ScrapeWorker runs SearchAndCrawl for each of its search terms concurrently,
// upserting every scraped page into the database. It runs until ctx is
// cancelled, at which point all goroutines exit and Run returns.
type ScrapeWorker struct {
	terms  []string
	sc     scraperIface
	pages  pageDAO
	logger *slog.Logger
}

// NewScrapeWorker constructs a ScrapeWorker.
func NewScrapeWorker(terms []string, sc scraperIface, pages pageDAO, logger *slog.Logger) *ScrapeWorker {
	return &ScrapeWorker{
		terms:  terms,
		sc:     sc,
		pages:  pages,
		logger: logger,
	}
}

// Run starts one goroutine per search term. Each goroutine ranges over the
// SearchAndCrawl result channel, upserting every successfully scraped page.
// Run blocks until all goroutines exit (which happens when ctx is cancelled).
func (w *ScrapeWorker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, term := range w.terms {
		wg.Add(1)
		go func(term string) {
			defer wg.Done()
			w.runTerm(ctx, term)
		}(term)
	}
	wg.Wait()
	return nil
}

func (w *ScrapeWorker) runTerm(ctx context.Context, term string) {
	w.logger.Info("scrape worker started", "term", term)
	for result := range w.sc.SearchAndCrawl(ctx, term) {
		if result.Err != nil {
			w.logger.Warn("scrape error", "term", term, "url", result.URL, "err", result.Err)
			continue
		}
		if _, err := w.pages.Upsert(ctx, result.URL, result.Language, result.Title, result.Summary, result.Content, result.DatePublished, result.DateModified); err != nil {
			w.logger.Error("upsert page", "url", result.URL, "err", err)
		}
	}
	w.logger.Info("scrape worker stopped", "term", term)
}
