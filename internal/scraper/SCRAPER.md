# Scraper Package Developer Guide

This guide is for contributors working on `internal/scraper`.

## Overview

`internal/scraper` fetches individual Wikipedia pages and returns structured content: title, language, summary, markdown body, and publication dates. It does not crawl links or perform searches — callers supply explicit URLs.

## File Responsibilities

| File          | Responsibility                                                              |
|---------------|-----------------------------------------------------------------------------|
| `scraper.go`  | `ScrapeResult`, `Config`, `Scraper` interface, `WikipediaScraper`, `ScrapeURL`/`ScrapeURLs`, HTTP fetch logic |
| `extract.go`  | HTML extraction helpers: `extractLanguage`, `extractJSONLD`, `extractBodyContent`, `convertToMarkdown` |
| `summary.go`  | Wikipedia REST summary API client (`fetchSummary`)                          |

## Key Types

### `ScrapeResult`
```go
type ScrapeResult struct {
    URL           string
    Language      string
    Title         string
    Summary       string
    Content       string     // markdown converted from #bodyContent element
    DatePublished *time.Time // from JSON-LD, nil if absent
    DateModified  *time.Time // from JSON-LD, nil if absent
    Timestamp     time.Time
    Err           error      // non-nil means the fetch failed
}
```

### `Config`
```go
type Config struct {
    Timeout time.Duration // per-request HTTP timeout
}
```

### `Scraper` interface
Both methods return `<-chan ScrapeResult`. The channel is **always closed** when the operation finishes (including on context cancellation). Callers use `range`:

```go
for result := range s.ScrapeURL(ctx, url) {
    if result.Err != nil {
        log.Printf("error: %v", result.Err)
        continue
    }
    // use result.Content, result.Title, etc.
}
```

## HTTP Client

`WikipediaScraper` uses a single `*http.Client` for both page fetches and summary API calls. There is no rate limiting or concurrency — `ScrapeURLs` processes URLs sequentially.

## Channel Contract

1. **Always closed**: the returned channel is always closed, even on ctx cancellation.
2. **Errors in-band**: failures appear as `result.Err != nil`. They are never silently discarded.
3. **Backpressure**: `ScrapeURLs` buffers the output channel at `len(urls)`. The goroutine also checks `ctx.Done()` before each send.

## Testing Conventions

- **Use `httptest.NewServer`** — no real network calls in tests.
- **Use `testConfig()`** (defined in `scraper_test.go`) as the base config: `Timeout: 5s`.
- **Always drain the channel** in tests — use `for r := range ch { ... }`. Abandoning a channel causes goroutine leaks.
- **`extract_test.go`** covers the pure HTML extraction functions independently.

## Anti-Patterns

- **Don't add crawling or search logic here.** URLs are supplied by callers.
- **Don't use third-party HTTP libraries.** `net/http` is sufficient.
- **Don't collect results into a slice before returning.** Return a channel and stream results.
- **Don't swallow errors.** Set `ScrapeResult.Err` and send the result.
