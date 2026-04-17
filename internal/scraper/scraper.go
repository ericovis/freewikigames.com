// Package scraper fetches Wikipedia pages and extracts structured content
// (title, summary, markdown body, language, dates) from them.
//
// The single entry points are ScrapeURL (one page) and ScrapeURLs (many pages,
// sequential). Both return a channel that is always closed when the operation
// finishes, even on context cancellation:
//
//	for result := range s.ScrapeURL(ctx, url) {
//	    if result.Err != nil { ... }
//	}
package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ScrapeResult holds the structured outcome of scraping one Wikipedia page.
type ScrapeResult struct {
	URL           string
	Language      string
	Title         string
	Summary       string
	Content       string     // markdown converted from #bodyContent element
	DatePublished *time.Time // from JSON-LD, nil if absent
	DateModified  *time.Time // from JSON-LD, nil if absent
	Timestamp     time.Time
	Err           error
}

// Config holds tunable parameters for a WikipediaScraper.
type Config struct {
	// Timeout is the per-request HTTP timeout.
	Timeout time.Duration
}

// DefaultConfig returns a Config with sensible production defaults.
func DefaultConfig() Config {
	return Config{
		Timeout: 15 * time.Second,
	}
}

// Scraper defines the supported scraping operations.
// Both methods return a read-only channel of ScrapeResult that is always closed.
type Scraper interface {
	// ScrapeURL fetches a single URL and sends one result.
	ScrapeURL(ctx context.Context, url string) <-chan ScrapeResult

	// ScrapeURLs fetches each URL sequentially and sends one result per URL.
	ScrapeURLs(ctx context.Context, urls []string) <-chan ScrapeResult
}

// WikipediaScraper is the standard implementation of Scraper.
type WikipediaScraper struct {
	client *http.Client
}

// New constructs a WikipediaScraper with the given Config.
func New(cfg Config) *WikipediaScraper {
	return &WikipediaScraper{
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// fetch performs a single HTTP GET and returns a fully populated ScrapeResult.
func (s *WikipediaScraper) fetch(ctx context.Context, rawURL string) ScrapeResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ScrapeResult{URL: rawURL, Err: fmt.Errorf("creating request: %w", err), Timestamp: time.Now()}
	}
	req.Header.Set("User-Agent", "freewikigames-scraper/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return ScrapeResult{URL: rawURL, Err: fmt.Errorf("executing request: %w", err), Timestamp: time.Now()}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ScrapeResult{URL: rawURL, Err: fmt.Errorf("unexpected status %d for %s", resp.StatusCode, rawURL), Timestamp: time.Now()}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ScrapeResult{URL: rawURL, Err: fmt.Errorf("reading body: %w", err), Timestamp: time.Now()}
	}
	return s.buildResult(ctx, rawURL, string(body))
}

// buildResult extracts all structured fields from raw HTML.
func (s *WikipediaScraper) buildResult(ctx context.Context, rawURL, html string) ScrapeResult {
	lang := extractLanguage(rawURL, html)
	title, published, modified := extractJSONLD(html)
	bodyHTML := extractBodyContent(html)
	content := stripBoilerplateSections(convertToMarkdown(bodyHTML))
	summary := s.fetchSummary(ctx, lang, title)
	return ScrapeResult{
		URL:           rawURL,
		Language:      lang,
		Title:         title,
		Summary:       summary,
		Content:       content,
		DatePublished: published,
		DateModified:  modified,
		Timestamp:     time.Now(),
	}
}

// ScrapeURL fetches a single URL and sends one result.
func (s *WikipediaScraper) ScrapeURL(ctx context.Context, url string) <-chan ScrapeResult {
	out := make(chan ScrapeResult, 1)
	go func() {
		defer close(out)
		out <- s.fetch(ctx, url)
	}()
	return out
}

// ScrapeURLs fetches each URL sequentially and sends one result per URL.
// It stops early if ctx is cancelled.
func (s *WikipediaScraper) ScrapeURLs(ctx context.Context, urls []string) <-chan ScrapeResult {
	out := make(chan ScrapeResult, len(urls))
	go func() {
		defer close(out)
		for _, u := range urls {
			result := s.fetch(ctx, u)
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
