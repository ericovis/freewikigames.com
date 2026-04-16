# Scraper Package Developer Guide

This guide is for contributors adding features or new crawler modes to `internal/scraper`.

## Overview

`internal/scraper` fetches Wikipedia pages and returns raw HTML — no parsing, no content extraction. That happens downstream in `internal/questions` and `internal/worker`. The package uses only Go's standard library (`net/http`, `regexp`, `encoding/json`, `sync`, `time`). Do not add third-party HTTP or HTML parsing dependencies.

## File Responsibilities

| File           | Responsibility                                                   |
|----------------|------------------------------------------------------------------|
| `scraper.go`   | `ScrapeResult`, `Config`, `Scraper` interface, `WikipediaScraper`, `ScrapeURL`/`ScrapeURLs` |
| `client.go`    | `httpClient` wrapper, `rateLimiter` (token bucket)              |
| `worker.go`    | `workerPool` (`Run`, `RunStream`), `mergeResults` fan-in helper |
| `crawler.go`   | `discoverLinks` regexp parser, `CrawlFromURL` BFS logic         |
| `search.go`    | Wikipedia search API client, `SearchAndScrape`/`Multiple`/`Crawl` |

## Key Types

### `ScrapeResult`
```go
type ScrapeResult struct {
    URL       string
    HTML      string
    Timestamp time.Time
    Err       error  // non-nil means the fetch failed; HTML will be empty
}
```

### `Config`
```go
type Config struct {
    MaxWorkers int           // goroutine concurrency
    RPS        float64       // requests/second across all workers
    MaxDepth   int           // 0 = unlimited depth
    MaxPages   int           // 0 = unlimited pages
    Timeout    time.Duration // per-request HTTP timeout
    BaseURL    string        // override Wikipedia origin (for tests only)
}
```

### `Scraper` interface
All six methods return `<-chan ScrapeResult`. The channel is **always closed** when the operation finishes (including on context cancellation). Callers use `range`:

```go
for result := range s.SearchAndScrape(ctx, "black hole") {
    if result.Err != nil {
        log.Printf("error: %v", result.Err)
        continue
    }
    // process result.HTML
}
```

## Worker Pool

`workerPool` has two entry points:

- **`Run(ctx, []string)`** — for a known list of URLs. Creates a fresh jobs channel buffered at `len(urls)`. The feed goroutine closes the jobs channel when all URLs are sent or ctx is cancelled, allowing workers to exit cleanly via `range jobs`.

- **`RunStream(ctx, <-chan string)`** — for progressively-fed workloads. Workers read from a jobs channel that is closed when the input `urls` channel is closed. Useful when the number of URLs is not known upfront.

`CrawlFromURL` uses `Run` per BFS level (the frontier is known before each level starts). `RunStream` is available for future streaming use cases.

Both methods return a `<-chan ScrapeResult` that is closed after all workers exit.

## Rate Limiter

The rate limiter uses a token-bucket pattern via a buffered channel:

- A background goroutine ticks at `time.Second / RPS` and sends one token into `tokens` on each tick.
- The send is **non-blocking** (`select { case tokens <- struct{}{}: default: }`). Tokens are discarded if the bucket is full, preventing accumulation during idle periods.
- Every call to `httpClient.Get` calls `limiter.Wait(ctx)` first, which blocks until a token is available or ctx is cancelled.
- The burst capacity (channel buffer size) is `ceil(RPS)`, allowing up to one second's worth of back-to-back requests before throttling.

The limiter is shared across all workers. There is no per-worker rate limiting.

## Channel Contract

All public methods follow these rules:

1. **Always closed**: the returned channel is always closed, even on ctx cancellation. Callers must not rely on it staying open.
2. **Errors in-band**: failures appear as `result.Err != nil`. They are never silently discarded.
3. **Backpressure**: output channels are buffered. If the caller reads slowly, workers will block briefly before sending — this is the correct backpressure behavior. Do not make output channels unbuffered.
4. **Context propagation**: every blocking operation (`limiter.Wait`, `pool.Run`, channel sends) selects on `ctx.Done()`. Cancellation terminates in-progress operations promptly.

## Adding a New Crawler Mode

1. Add the method signature to the `Scraper` interface in [scraper.go](scraper.go).
2. Implement it as a method on `*WikipediaScraper`:
   - If it crawls links: put it in [crawler.go](crawler.go).
   - If it uses the search API: put it in [search.go](search.go).
   - If it's a simple pool delegation: put it in [scraper.go](scraper.go).
3. Open a goroutine immediately and return the channel:
   ```go
   func (s *WikipediaScraper) MyMode(ctx context.Context, ...) <-chan ScrapeResult {
       out := make(chan ScrapeResult, 64)
       go func() {
           defer close(out)
           // ... work ...
       }()
       return out
   }
   ```
4. Use `pool.Run` or `pool.RunStream` — never call `client.Get` directly from a mode method.
5. Use `mergeResults(channels...)` to fan-in multiple sub-channels.
6. Add tests in a `_test.go` file using `httptest.NewServer` (see Testing section below).

## Wikipedia API Reference

**Search endpoint:**
```
GET /w/api.php?action=query&list=search&format=json&srlimit=10&srsearch=TERM
```

**Response shape:**
```json
{
  "query": {
    "search": [
      {"title": "Article Title"},
      ...
    ]
  }
}
```

**Article URL construction:**
```go
baseURL + "/wiki/" + url.PathEscape(title)
// e.g. "https://en.wikipedia.org/wiki/Black_hole"
```

**Link discovery regex:**
```go
regexp.MustCompile(`href="(/wiki/[^"#]+)"`)
```
Matches `href="/wiki/TITLE"` attributes. The `[^"#]+` excludes quote terminators and fragment identifiers (`#`), so `href="/wiki/Article#section"` does not match.

**Namespace denylist** (titles starting with these prefixes are skipped):
`File:`, `Help:`, `Wikipedia:`, `Special:`, `Talk:`, `Category:`, `Portal:`, `Template:`, `User:`, `Draft:`, `MediaWiki:`, `Module:`, `Book:`, `TimedText:`

## Testing Conventions

- **Use `httptest.NewServer`** — no real network calls in tests. Set `Config.BaseURL` to `srv.URL` to redirect all requests to the test server.
- **Use `testConfig(srv.URL)`** (defined in `scraper_test.go`) as the base config for tests: `RPS: 50`, `MaxWorkers: 2`, `Timeout: 5s`.
- **Table-driven tests** for pure functions like `discoverLinks`.
- **Test each mode independently**: don't rely on `SearchAndCrawl` working correctly to test `CrawlFromURL`.
- **Always drain the channel** in tests — use `for r := range ch { ... }`. Abandoning a channel causes goroutine leaks.

## Anti-Patterns

- **Don't use third-party HTTP libraries.** The `net/http` client is sufficient.
- **Don't collect results into a slice before returning.** Return a channel and stream results.
- **Don't swallow errors.** Set `ScrapeResult.Err` and send the result. Never `log.Printf` and continue silently.
- **Don't call `client.Get` directly from a mode method.** Always go through the worker pool to respect concurrency limits.
- **Don't share the jobs channel across concurrent `Run` calls.** Each `Run` / `RunStream` call creates its own fresh jobs channel.
- **Don't use `BaseURL` in production code.** It exists only for testing. Production callers use `DefaultConfig()` or set `BaseURL: ""`.
