# WORKER.md — internal/worker

## Purpose

The `worker` package orchestrates the two long-running background processes that power the data pipeline:

1. **ScrapeWorker** — given a list of search terms, continuously crawls Wikipedia (via `SearchAndCrawl`) and persists every scraped page to the database.
2. **QuestionWorker** — polls for pages that have no questions yet, detects their language, calls the AI to generate trivia questions, and stores the results.

Both workers run until the provided `context.Context` is cancelled, making them suitable for use with `signal.NotifyContext` in `cmd` binaries.

## Key Types

### ScrapeWorker

```go
func NewScrapeWorker(terms []string, sc scraperIface, pages pageDAO, logger *slog.Logger) *ScrapeWorker
func (w *ScrapeWorker) Run(ctx context.Context) error
```

- Spawns one goroutine per search term.
- Each goroutine ranges over `scraper.SearchAndCrawl(ctx, term)` (unlimited depth/pages).
- Calls `PageDAO.Upsert` for each successfully scraped result.
- Scrape errors are logged and skipped; DB errors are also logged but do not stop the worker.
- `Run` blocks until all goroutines exit (when ctx is cancelled and all channels drain).

### QuestionWorker

```go
func NewQuestionWorker(pages pageDAO, questions questionDAO, generator questionGenerator, logger *slog.Logger) *QuestionWorker
func (w *QuestionWorker) Run(ctx context.Context) error
```

- Polls `PageDAO.FindWithoutQuestions` every 5 seconds (default) for a batch of up to 10 pages.
- For each page: detects language → generates questions → inserts each question.
- Sleeps `pollInterval` when no pages are found or on DB errors.
- Exits cleanly when ctx is cancelled.

## Language Detection

`detectLanguage(rawURL, rawHTML string) string` in `detect.go`:

1. Parses the URL hostname: `https://{lang}.wikipedia.org/...` → `lang`
2. Falls back to `<html lang="...">` regex on raw HTML
3. Final default: `"en"`

## Design Decisions

- **Interface-based dependencies**: `pageDAO`, `questionDAO`, `scraperIface`, and `questionGenerator` are defined locally in `worker.go`. This keeps the package testable with simple in-memory mocks and avoids importing concrete types just for the interface shape.
- **No real DB in tests**: Worker tests use mock implementations. DB correctness is already covered by `internal/db` integration tests.
- **Polling vs. LISTEN/NOTIFY**: Simple polling with a configurable interval. Sufficient for the current workload; avoids Postgres LISTEN/NOTIFY complexity.
- **One goroutine per term**: Natural parallelism for scraping; each term's channel is independent. `sync.WaitGroup` is used instead of `errgroup` to avoid the extra dependency (errors are logged, not aggregated).

## Testing Conventions

- All test files are in `package worker` (same package, not `_test`).
- Mocks are defined inline in `*_test.go` files.
- No real database or Ollama connection is used.
- Test names follow `Test<Type>_<Method>_<Scenario>`.
