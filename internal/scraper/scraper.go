// Package scraper provides a Wikipedia scraping library with configurable
// concurrency, rate limiting, link discovery, and search-based crawling.
//
// All methods return a receive-only channel that is always eventually closed.
// Errors are embedded in ScrapeResult.Err; the caller checks them inline:
//
//	for result := range s.ScrapeURL(ctx, url) {
//	    if result.Err != nil { ... }
//	}
package scraper

import (
	"context"
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
	rawHTML       string // unexported; used by CrawlFromURL for link discovery
}

// Config holds all tunable parameters for a WikipediaScraper.
type Config struct {
	// MaxWorkers controls how many goroutines fetch URLs concurrently.
	MaxWorkers int
	// RPS is the maximum requests per second across all workers combined.
	RPS float64
	// MaxDepth controls crawl depth in CrawlFromURL and SearchAndCrawl.
	// 0 means unlimited depth (crawl until MaxPages or context cancellation).
	MaxDepth int
	// MaxPages caps the total number of pages fetched in a crawl operation.
	// 0 means unlimited pages (crawl until depth exhausted or ctx cancelled).
	MaxPages int
	// Timeout is the per-request HTTP timeout.
	Timeout time.Duration
	// BaseURL overrides the Wikipedia base URL. Defaults to
	// "https://en.wikipedia.org" when empty. Intended for testing.
	BaseURL string
}

// DefaultConfig returns a Config with sensible production defaults.
func DefaultConfig() Config {
	return Config{
		MaxWorkers: 5,
		RPS:        2.0,
		MaxDepth:   3,
		MaxPages:   100,
		Timeout:    15 * time.Second,
	}
}

// Scraper defines all supported scraping operations.
// Every method returns a read-only channel of ScrapeResult.
// The caller must drain the channel; the implementation closes it when done.
type Scraper interface {
	// ScrapeURL fetches a single URL and sends one result.
	ScrapeURL(ctx context.Context, url string) <-chan ScrapeResult

	// ScrapeURLs fetches each URL in the list concurrently.
	ScrapeURLs(ctx context.Context, urls []string) <-chan ScrapeResult

	// CrawlFromURL starts at startURL, discovers Wikipedia links in HTML,
	// and recursively scrapes them up to cfg.MaxDepth and cfg.MaxPages.
	CrawlFromURL(ctx context.Context, startURL string) <-chan ScrapeResult

	// SearchAndScrape queries Wikipedia for term, then scrapes the top results.
	SearchAndScrape(ctx context.Context, term string) <-chan ScrapeResult

	// SearchAndScrapeMultiple runs SearchAndScrape for each term concurrently.
	SearchAndScrapeMultiple(ctx context.Context, terms []string) <-chan ScrapeResult

	// SearchAndCrawl queries Wikipedia for term, then crawls from each result page.
	SearchAndCrawl(ctx context.Context, term string) <-chan ScrapeResult
}

// WikipediaScraper is the standard implementation of Scraper.
type WikipediaScraper struct {
	cfg           Config
	client        *httpClient
	pool          *workerPool
	summaryClient *http.Client
}

// New constructs a WikipediaScraper with the given Config.
func New(cfg Config) *WikipediaScraper {
	c := newHTTPClient(cfg.Timeout, cfg.RPS)
	s := &WikipediaScraper{
		cfg:           cfg,
		client:        c,
		summaryClient: &http.Client{Timeout: cfg.Timeout},
	}
	s.pool = newWorkerPool(cfg.MaxWorkers, c, s.buildResult)
	return s
}

// buildResult extracts all structured fields from raw HTML and fetches the summary.
func (s *WikipediaScraper) buildResult(ctx context.Context, rawURL, html string) ScrapeResult {
	lang := extractLanguage(rawURL, html)
	title, published, modified := extractJSONLD(html)
	bodyHTML := extractBodyContent(html)
	content := convertToMarkdown(bodyHTML)
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
		rawHTML:       html,
	}
}

// baseURL returns the Wikipedia origin to use, applying the BaseURL override.
func (s *WikipediaScraper) baseURL() string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL
	}
	return "https://en.wikipedia.org"
}

// ScrapeURL fetches a single URL and sends one result.
func (s *WikipediaScraper) ScrapeURL(ctx context.Context, url string) <-chan ScrapeResult {
	return s.pool.Run(ctx, []string{url})
}

// ScrapeURLs fetches each URL in the list concurrently.
func (s *WikipediaScraper) ScrapeURLs(ctx context.Context, urls []string) <-chan ScrapeResult {
	return s.pool.Run(ctx, urls)
}
