package worker

import (
	"context"
	"log/slog"
	"sync"
)

// ScrapeWorker runs CrawlFromURL for each of its start URLs concurrently,
// upserting every scraped page into the database. It runs until ctx is
// cancelled, at which point all goroutines exit and Run returns.
type ScrapeWorker struct {
	urls   []string
	sc     scraperIface
	pages  pageDAO
	logger *slog.Logger
}

// NewScrapeWorker constructs a ScrapeWorker.
func NewScrapeWorker(urls []string, sc scraperIface, pages pageDAO, logger *slog.Logger) *ScrapeWorker {
	return &ScrapeWorker{
		urls:   urls,
		sc:     sc,
		pages:  pages,
		logger: logger,
	}
}

// Run starts one goroutine per URL. Each goroutine ranges over the
// CrawlFromURL result channel, upserting every successfully scraped page.
// Run blocks until all goroutines exit (which happens when ctx is cancelled).
func (w *ScrapeWorker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, url := range w.urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			w.runURL(ctx, url)
		}(url)
	}
	wg.Wait()
	return nil
}

func (w *ScrapeWorker) runURL(ctx context.Context, url string) {
	w.logger.Info("scrape worker started", "url", url)
	for result := range w.sc.CrawlFromURL(ctx, url) {
		if result.Err != nil {
			w.logger.Warn("scrape error", "url", result.URL, "err", result.Err)
			continue
		}
		if _, err := w.pages.Upsert(ctx, result.URL, result.Language, result.Title, result.Summary, result.Content, result.DatePublished, result.DateModified); err != nil {
			w.logger.Error("upsert page", "url", result.URL, "err", err)
		}
	}
	w.logger.Info("scrape worker stopped", "url", url)
}
