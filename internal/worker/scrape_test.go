package worker

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ericovis/freewikigames.com/internal/db"
	"github.com/ericovis/freewikigames.com/internal/scraper"
)

// mockScraper satisfies scraperIface for tests.
type mockScraper struct {
	results []scraper.ScrapeResult
}

func (m *mockScraper) ScrapeURLs(ctx context.Context, urls []string) <-chan scraper.ScrapeResult {
	ch := make(chan scraper.ScrapeResult, len(m.results))
	for _, r := range m.results {
		ch <- r
	}
	close(ch)
	return ch
}

// mockPageDAO satisfies pageDAO for tests.
type mockPageDAO struct {
	upserted               []string
	findWithoutQuestionsFn func(ctx context.Context, limit int) ([]db.Page, error)
}

func (m *mockPageDAO) Upsert(ctx context.Context, url, language, title, summary, content string, datePublished, dateModified *time.Time) (*db.Page, error) {
	m.upserted = append(m.upserted, url)
	return &db.Page{ID: 1, URL: url, Language: language}, nil
}

func (m *mockPageDAO) FindWithoutQuestions(ctx context.Context, limit int) ([]db.Page, error) {
	if m.findWithoutQuestionsFn != nil {
		return m.findWithoutQuestionsFn(ctx, limit)
	}
	return nil, nil
}

func TestScrapeWorker_Run_StoresResults(t *testing.T) {
	sc := &mockScraper{results: []scraper.ScrapeResult{
		{URL: "https://en.wikipedia.org/wiki/A", Language: "en", Title: "A", Summary: "A summary.", Content: "## A"},
		{URL: "https://en.wikipedia.org/wiki/B", Language: "en", Title: "B", Summary: "B summary.", Content: "## B"},
	}}
	pages := &mockPageDAO{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w := NewScrapeWorker([]string{"https://en.wikipedia.org/wiki/A", "https://en.wikipedia.org/wiki/B"}, sc, pages, logger)
	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(pages.upserted) != 2 {
		t.Errorf("expected 2 upserted pages, got %d", len(pages.upserted))
	}
}

func TestScrapeWorker_Run_SkipsErrors(t *testing.T) {
	sc := &mockScraper{results: []scraper.ScrapeResult{
		{URL: "https://en.wikipedia.org/wiki/A", Err: context.DeadlineExceeded},
		{URL: "https://en.wikipedia.org/wiki/B", Language: "en", Title: "B", Summary: "B summary.", Content: "## B"},
	}}
	pages := &mockPageDAO{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w := NewScrapeWorker([]string{"https://en.wikipedia.org/wiki/A", "https://en.wikipedia.org/wiki/B"}, sc, pages, logger)
	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(pages.upserted) != 1 {
		t.Errorf("expected 1 upserted page (error skipped), got %d", len(pages.upserted))
	}
	if pages.upserted[0] != "https://en.wikipedia.org/wiki/B" {
		t.Errorf("expected B to be upserted, got %q", pages.upserted[0])
	}
}

func TestScrapeWorker_Run_ExitsOnContextCancel(t *testing.T) {
	blocked := make(chan scraper.ScrapeResult)
	sc := &blockingScraper{ch: blocked}
	pages := &mockPageDAO{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		w := NewScrapeWorker([]string{"https://en.wikipedia.org/wiki/Test"}, sc, pages, logger)
		done <- w.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected nil error on cancel, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

// blockingScraper returns a channel that is closed when ctx is done.
type blockingScraper struct {
	ch chan scraper.ScrapeResult
}

func (b *blockingScraper) ScrapeURLs(ctx context.Context, urls []string) <-chan scraper.ScrapeResult {
	out := make(chan scraper.ScrapeResult)
	go func() {
		<-ctx.Done()
		close(out)
	}()
	return out
}
