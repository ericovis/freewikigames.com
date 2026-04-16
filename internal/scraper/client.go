package scraper

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// rateLimiter enforces a maximum requests-per-second using a buffered token
// channel replenished by a background ticker goroutine.
type rateLimiter struct {
	tokens chan struct{}
	stop   chan struct{}
}

// newRateLimiter starts the background ticker and returns a rateLimiter.
// capacity is the burst size (tokens allowed before blocking).
func newRateLimiter(rps float64, capacity int) *rateLimiter {
	rl := &rateLimiter{
		tokens: make(chan struct{}, capacity),
		stop:   make(chan struct{}),
	}
	interval := time.Duration(float64(time.Second) / rps)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-rl.stop:
				return
			case <-ticker.C:
				// Non-blocking send: discard token if bucket is full.
				// This prevents unbounded accumulation during idle periods.
				select {
				case rl.tokens <- struct{}{}:
				default:
				}
			}
		}
	}()
	return rl
}

// Wait blocks until a token is available or ctx is cancelled.
func (r *rateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.tokens:
		return nil
	}
}

// Stop shuts down the background ticker goroutine.
func (r *rateLimiter) Stop() {
	close(r.stop)
}

// httpClient wraps net/http.Client with rate limiting.
type httpClient struct {
	client  *http.Client
	limiter *rateLimiter
}

// newHTTPClient constructs an httpClient with the given timeout and RPS.
// The burst capacity is set to ceil(rps) to allow short bursts.
func newHTTPClient(timeout time.Duration, rps float64) *httpClient {
	capacity := int(math.Ceil(rps))
	if capacity < 1 {
		capacity = 1
	}
	return &httpClient{
		client:  &http.Client{Timeout: timeout},
		limiter: newRateLimiter(rps, capacity),
	}
}

// Get performs a rate-limited GET request. The full response body is read and
// returned as a string. Non-2xx status codes are returned as errors.
func (c *httpClient) Get(ctx context.Context, url string) (string, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "freewikigames-scraper/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}
	return string(body), nil
}
