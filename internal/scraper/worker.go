package scraper

import (
	"context"
	"sync"
	"time"
)

type job struct {
	ctx context.Context
	url string
}

// workerPool manages a fixed set of goroutines that process scrape jobs.
type workerPool struct {
	workers int
	client  *httpClient
}

func newWorkerPool(workers int, client *httpClient) *workerPool {
	return &workerPool{workers: workers, client: client}
}

// Run dispatches all urls as jobs across N worker goroutines and returns a
// channel that receives one ScrapeResult per URL. The channel is closed after
// all results are delivered. ctx cancellation stops feeding new jobs; any
// in-flight request still delivers a result (with Err set).
func (p *workerPool) Run(ctx context.Context, urls []string) <-chan ScrapeResult {
	jobs := make(chan job, len(urls))
	out := make(chan ScrapeResult, len(urls))

	var wg sync.WaitGroup
	for range p.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				html, err := p.client.Get(j.ctx, j.url)
				result := ScrapeResult{
					URL:       j.url,
					HTML:      html,
					Timestamp: time.Now(),
					Err:       err,
				}
				select {
				case out <- result:
				case <-j.ctx.Done():
					return
				}
			}
		}()
	}

	// Feed goroutine: closes jobs channel when done or ctx cancelled.
	go func() {
		defer close(jobs)
		for _, u := range urls {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{ctx: ctx, url: u}:
			}
		}
	}()

	// Closer goroutine: closes output channel once all workers finish.
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// RunStream is like Run but accepts URLs via a channel for progressively-fed
// workloads (e.g. BFS crawl). The output channel is closed once all workers
// finish, which happens when the urls channel is closed.
func (p *workerPool) RunStream(ctx context.Context, urls <-chan string) <-chan ScrapeResult {
	jobs := make(chan job, p.workers)
	out := make(chan ScrapeResult, 64)

	var wg sync.WaitGroup
	for range p.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				html, err := p.client.Get(j.ctx, j.url)
				result := ScrapeResult{
					URL:       j.url,
					HTML:      html,
					Timestamp: time.Now(),
					Err:       err,
				}
				select {
				case out <- result:
				case <-j.ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for {
			select {
			case <-ctx.Done():
				return
			case u, ok := <-urls:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case jobs <- job{ctx: ctx, url: u}:
				}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// mergeResults fans multiple result channels into one. The returned channel
// is closed once all input channels are drained.
func mergeResults(channels ...<-chan ScrapeResult) <-chan ScrapeResult {
	out := make(chan ScrapeResult, 64)
	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(c <-chan ScrapeResult) {
			defer wg.Done()
			for r := range c {
				out <- r
			}
		}(ch)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
