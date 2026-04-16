package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"
)

// --- discoverLinks unit tests ---

func TestDiscoverLinks_FindsWikiLinks(t *testing.T) {
	html := `<a href="/wiki/Go">Go</a><a href="/wiki/Rust">Rust</a>`
	got := discoverLinks(html)
	sort.Strings(got)
	want := []string{"/wiki/Go", "/wiki/Rust"}
	assertStringSliceEqual(t, want, got)
}

func TestDiscoverLinks_Deduplicates(t *testing.T) {
	html := `<a href="/wiki/Go">link1</a><a href="/wiki/Go">link2</a>`
	got := discoverLinks(html)
	if len(got) != 1 || got[0] != "/wiki/Go" {
		t.Errorf("expected [/wiki/Go], got %v", got)
	}
}

func TestDiscoverLinks_ExcludesNamespaces(t *testing.T) {
	html := `
		<a href="/wiki/File:Image.jpg">img</a>
		<a href="/wiki/Special:Search">search</a>
		<a href="/wiki/Wikipedia:About">about</a>
		<a href="/wiki/Talk:Go">talk</a>
		<a href="/wiki/Help:Contents">help</a>
		<a href="/wiki/Category:Languages">cat</a>
		<a href="/wiki/Portal:Science">portal</a>
		<a href="/wiki/Template:Infobox">tmpl</a>
		<a href="/wiki/Go">ok</a>
	`
	got := discoverLinks(html)
	if len(got) != 1 || got[0] != "/wiki/Go" {
		t.Errorf("expected only [/wiki/Go], got %v", got)
	}
}

func TestDiscoverLinks_ExcludesFragmentLinks(t *testing.T) {
	// The regex pattern [^"#]+ stops at '#', so href="/wiki/Go#History"
	// does not match. Only the clean href="/wiki/Go" matches.
	html := `<a href="/wiki/Go#History">history</a><a href="/wiki/Go">ok</a>`
	got := discoverLinks(html)
	if len(got) != 1 || got[0] != "/wiki/Go" {
		t.Errorf("expected [/wiki/Go], got %v", got)
	}
}

func TestDiscoverLinks_EmptyHTML(t *testing.T) {
	got := discoverLinks(`<html><body>no links here</body></html>`)
	if len(got) != 0 {
		t.Errorf("expected no links, got %v", got)
	}
}

// --- CrawlFromURL integration tests ---

// crawlTestServer creates a test server with a known link structure:
//
//	Seed → A, B, C (excluded: Special:Random, File:Img.png)
//	A    → A_child
//	B    → B_child
//	C    → C_child
//	*_child → (no links)
func crawlTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/wiki/Seed", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/wiki/A">A</a>
			<a href="/wiki/B">B</a>
			<a href="/wiki/C">C</a>
			<a href="/wiki/Special:Random">skip</a>
			<a href="/wiki/File:Img.png">skip</a>
		</body></html>`)
	})

	for _, name := range []string{"A", "B", "C"} {
		n := name
		mux.HandleFunc("/wiki/"+n, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `<html><body><a href="/wiki/%s_child">child</a></body></html>`, n)
		})
	}

	// Leaf pages: children and anything else.
	mux.HandleFunc("/wiki/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>leaf page</body></html>`)
	})

	return httptest.NewServer(mux)
}

func TestCrawlFromURL_RespectsMaxDepthAndMaxPages(t *testing.T) {
	srv := crawlTestServer()
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxDepth = 1
	cfg.MaxPages = 5
	s := New(cfg)

	ctx := context.Background()
	var results []ScrapeResult
	for r := range s.CrawlFromURL(ctx, srv.URL+"/wiki/Seed") {
		results = append(results, r)
		if r.Err != nil {
			t.Errorf("unexpected error scraping %s: %v", r.URL, r.Err)
		}
	}

	if len(results) > cfg.MaxPages {
		t.Errorf("expected at most %d results (MaxPages), got %d", cfg.MaxPages, len(results))
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result (seed page)")
	}

	urls := make(map[string]bool, len(results))
	for _, r := range results {
		urls[r.URL] = true
	}
	if !urls[srv.URL+"/wiki/Seed"] {
		t.Error("seed URL missing from results")
	}
}

func TestCrawlFromURL_UnlimitedDepth_StopsAtMaxPages(t *testing.T) {
	srv := crawlTestServer()
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxDepth = 0 // unlimited
	cfg.MaxPages = 3
	s := New(cfg)

	ctx := context.Background()
	var results []ScrapeResult
	for r := range s.CrawlFromURL(ctx, srv.URL+"/wiki/Seed") {
		results = append(results, r)
	}

	if len(results) != 3 {
		t.Errorf("expected exactly 3 results (MaxPages=3), got %d", len(results))
	}
}

func TestCrawlFromURL_UnlimitedPages_StopsAtMaxDepth(t *testing.T) {
	srv := crawlTestServer()
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxDepth = 1
	cfg.MaxPages = 0 // unlimited
	s := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var results []ScrapeResult
	for r := range s.CrawlFromURL(ctx, srv.URL+"/wiki/Seed") {
		results = append(results, r)
	}

	// depth=0: Seed (1 page)
	// depth=1: A, B, C (3 pages)
	// depth=2: would be children, but MaxDepth=1 stops here
	// Total: 4
	if len(results) != 4 {
		t.Errorf("expected 4 results (seed + 3 linked), got %d", len(results))
	}
}

func TestCrawlFromURL_NoDuplicates(t *testing.T) {
	// Server where multiple pages link to the same target.
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/Seed", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/wiki/Shared">shared</a>
		</body></html>`)
	})
	mux.HandleFunc("/wiki/Shared", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/wiki/Seed">back to seed</a>
		</body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxDepth = 3
	cfg.MaxPages = 20
	s := New(cfg)

	ctx := context.Background()
	seen := make(map[string]int)
	for r := range s.CrawlFromURL(ctx, srv.URL+"/wiki/Seed") {
		seen[r.URL]++
	}

	for url, count := range seen {
		if count > 1 {
			t.Errorf("URL %s scraped %d times, expected 1", url, count)
		}
	}
}

// assertStringSliceEqual compares two sorted string slices for equality.
func assertStringSliceEqual(t *testing.T, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("slice length mismatch: want %v, got %v", want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("index %d: want %q, got %q", i, want[i], got[i])
		}
	}
}
