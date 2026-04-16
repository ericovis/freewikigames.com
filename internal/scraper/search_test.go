package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// newSearchServer creates a test server that handles both the Wikipedia search
// API (/w/api.php) and article pages (/wiki/*).
//
// searchResults maps search term → list of article titles.
// articleHTML is served for every /wiki/* request.
func newSearchServer(searchResults map[string][]string, articleHTML string) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/w/api.php", func(w http.ResponseWriter, r *http.Request) {
		term := r.URL.Query().Get("srsearch")
		titles := searchResults[term]

		items := make([]map[string]string, len(titles))
		for i, t := range titles {
			items[i] = map[string]string{"title": t}
		}
		resp := map[string]any{
			"query": map[string]any{"search": items},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/wiki/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, articleHTML)
	})

	return httptest.NewServer(mux)
}

func TestSearchWikipedia_ReturnsTitleURLs(t *testing.T) {
	titles := []string{"Black hole", "Hawking radiation", "Event horizon"}
	srv := newSearchServer(map[string][]string{
		"black hole": titles,
	}, "<html><body>article</body></html>")
	defer srv.Close()

	s := New(testConfig(srv.URL))
	ctx := context.Background()

	urls, err := s.searchWikipedia(ctx, "black hole")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != len(titles) {
		t.Fatalf("expected %d URLs, got %d", len(titles), len(urls))
	}
	for _, u := range urls {
		if u == "" {
			t.Error("got empty URL in results")
		}
		if !strings.Contains(u, "/wiki/") {
			t.Errorf("expected URL to contain /wiki/, got %q", u)
		}
	}
}

func TestSearchAndScrape_ScrapesTopResults(t *testing.T) {
	srv := newSearchServer(map[string][]string{
		"quantum": {"Quantum mechanics", "Quantum field theory"},
	}, "<html><body>content</body></html>")
	defer srv.Close()

	s := New(testConfig(srv.URL))
	ctx := context.Background()

	var results []ScrapeResult
	for r := range s.SearchAndScrape(ctx, "quantum") {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		results = append(results, r)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (one per search hit), got %d", len(results))
	}
}

func TestSearchAndScrape_SearchError_DeliveredAsResult(t *testing.T) {
	// Server returns invalid JSON for the search endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "api.php") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "<html></html>")
	}))
	defer srv.Close()

	s := New(testConfig(srv.URL))
	ctx := context.Background()

	var results []ScrapeResult
	for r := range s.SearchAndScrape(ctx, "anything") {
		results = append(results, r)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 error result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected non-nil Err for failed search, got nil")
	}
}

func TestSearchAndScrapeMultiple_MergesAllTerms(t *testing.T) {
	srv := newSearchServer(map[string][]string{
		"physics": {"Physics", "Classical mechanics"},
		"biology": {"Biology", "Cell (biology)"},
	}, "<html><body>article</body></html>")
	defer srv.Close()

	s := New(testConfig(srv.URL))
	ctx := context.Background()

	seen := make(map[string]bool)
	for r := range s.SearchAndScrapeMultiple(ctx, []string{"physics", "biology"}) {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		seen[r.URL] = true
	}

	// 2 results per term × 2 terms = 4 total
	if len(seen) != 4 {
		t.Errorf("expected 4 unique URLs (2 per term), got %d: %v", len(seen), sortedKeys(seen))
	}
}

func TestSearchAndCrawl_CrawlsFromSearchResults(t *testing.T) {
	// Search returns 1 result; that article links to 2 more pages.
	mux := http.NewServeMux()

	mux.HandleFunc("/w/api.php", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"query": map[string]any{
				"search": []map[string]string{{"title": "Gravity"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	mux.HandleFunc("/wiki/Gravity", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/wiki/Newton">Newton</a>
			<a href="/wiki/Einstein">Einstein</a>
		</body></html>`)
	})

	mux.HandleFunc("/wiki/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>leaf</body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxDepth = 1
	cfg.MaxPages = 20
	s := New(cfg)

	ctx := context.Background()
	seen := make(map[string]bool)
	for r := range s.SearchAndCrawl(ctx, "gravity") {
		if r.Err != nil {
			t.Errorf("unexpected error scraping %s: %v", r.URL, r.Err)
		}
		seen[r.URL] = true
	}

	// Gravity (seed from search) + Newton + Einstein = 3
	if len(seen) != 3 {
		t.Errorf("expected 3 URLs (Gravity + 2 crawled), got %d: %v", len(seen), sortedKeys(seen))
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
