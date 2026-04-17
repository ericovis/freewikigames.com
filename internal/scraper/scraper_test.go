package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// testConfig returns a Config with a short timeout suitable for local tests.
func testConfig() Config {
	return Config{Timeout: 5 * time.Second}
}

func TestScrapeURL_ReturnsResultAndTimestamp(t *testing.T) {
	const body = `<html lang="en"><body>hello world</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	s := New(testConfig())
	ctx := context.Background()

	var results []ScrapeResult
	for r := range s.ScrapeURL(ctx, srv.URL+"/wiki/Test") {
		results = append(results, r)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if r.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if r.URL != srv.URL+"/wiki/Test" {
		t.Errorf("URL mismatch: got %q, want %q", r.URL, srv.URL+"/wiki/Test")
	}
}

func TestScrapeURL_ExtractsLanguageFromHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html lang="pt"><body>olá</body></html>`)
	}))
	defer srv.Close()

	s := New(testConfig())
	var results []ScrapeResult
	for r := range s.ScrapeURL(context.Background(), srv.URL+"/wiki/Brasil") {
		results = append(results, r)
	}

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("unexpected result: %+v", results)
	}
	if results[0].Language != "pt" {
		t.Errorf("Language = %q, want %q", results[0].Language, "pt")
	}
}

func TestScrapeURLs_ReturnsOneResultPerURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><body>%s</body></html>", r.URL.Path)
	}))
	defer srv.Close()

	paths := []string{"/wiki/A", "/wiki/B", "/wiki/C"}
	urls := make([]string, len(paths))
	for i, p := range paths {
		urls[i] = srv.URL + p
	}

	s := New(testConfig())
	ctx := context.Background()

	results := make(map[string]ScrapeResult)
	for r := range s.ScrapeURLs(ctx, urls) {
		results[r.URL] = r
	}

	if len(results) != len(urls) {
		t.Fatalf("expected %d results, got %d", len(urls), len(results))
	}
	for _, u := range urls {
		r, ok := results[u]
		if !ok {
			t.Errorf("missing result for %s", u)
			continue
		}
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", u, r.Err)
		}
	}
}

func TestScrapeURL_ContextCancellation_ChannelAlwaysClosed(t *testing.T) {
	var once sync.Once
	ready := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(ready) })
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer srv.Close()

	s := New(testConfig())
	ctx, cancel := context.WithCancel(context.Background())

	ch := s.ScrapeURL(ctx, srv.URL+"/wiki/Slow")

	<-ready
	cancel()

	var results []ScrapeResult
	for r := range ch {
		results = append(results, r)
	}

	for _, r := range results {
		if r.Err == nil {
			t.Error("expected error due to context cancellation, got nil")
		}
	}
}

func TestScrapeURL_HTTPError_SurfacedAsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := New(testConfig())
	ctx := context.Background()

	var results []ScrapeResult
	for r := range s.ScrapeURL(ctx, srv.URL+"/wiki/Broken") {
		results = append(results, r)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected non-nil Err for HTTP 500, got nil")
	}
}

func TestScrapeURLs_EmptyList_ChannelClosedImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "should not be called")
	}))
	defer srv.Close()

	s := New(testConfig())
	ctx := context.Background()

	var count int
	for range s.ScrapeURLs(ctx, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 results for empty URL list, got %d", count)
	}
}
