package worker

import (
	"context"
	"log/slog"
)

// ScrapeWorker scrapes each of its URLs once and upserts the results into the
// database. It runs until all URLs are processed or ctx is cancelled.
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

// Run scrapes all URLs sequentially, upserting each successfully scraped page.
// It blocks until all URLs are processed or ctx is cancelled.
func (w *ScrapeWorker) Run(ctx context.Context) error {
	w.logger.Info("scrape worker started", "urls", len(w.urls))
	for result := range w.sc.ScrapeURLs(ctx, w.urls) {
		if result.Err != nil {
			w.logger.Warn("scrape error", "url", result.URL, "err", result.Err)
			continue
		}
		if _, err := w.pages.Upsert(ctx, result.URL, result.Language, result.Title, result.Summary, result.Content, result.DatePublished, result.DateModified); err != nil {
			w.logger.Error("upsert page", "url", result.URL, "err", err)
		}
	}
	w.logger.Info("scrape worker stopped")
	return nil
}
