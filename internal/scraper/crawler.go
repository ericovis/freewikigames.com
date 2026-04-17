package scraper

import (
	"context"
	"regexp"
	"strings"
	"sync"
)

// linkRegexp matches href="/wiki/TITLE" attributes in raw HTML.
// Compiled once at package init to avoid per-call overhead.
var linkRegexp = regexp.MustCompile(`href="(/wiki/[^"#]+)"`)

// excludedPrefixes lists Wikipedia namespace prefixes whose pages should not
// be followed during crawling. These are meta-pages, not article content.
var excludedPrefixes = []string{
	"File:", "Help:", "Wikipedia:", "Special:", "Talk:",
	"Category:", "Portal:", "Template:", "User:", "Draft:",
	"MediaWiki:", "Module:", "Book:", "TimedText:",
}

// discoverLinks parses raw HTML and returns unique Wikipedia article paths
// (e.g. "/wiki/Black_hole") found in href attributes. Namespace pages and
// duplicate paths are excluded.
func discoverLinks(html string) []string {
	matches := linkRegexp.FindAllStringSubmatch(html, -1)
	seen := make(map[string]struct{}, len(matches))
	result := make([]string, 0, len(matches))

	for _, m := range matches {
		path := m[1] // e.g. "/wiki/Black_hole"
		title := path[len("/wiki/"):]

		excluded := false
		for _, prefix := range excludedPrefixes {
			if strings.HasPrefix(title, prefix) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		if _, dup := seen[path]; dup {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

// CrawlFromURL starts at startURL, discovers Wikipedia links from each scraped
// page, and recursively scrapes them using breadth-first search.
//
// Depth and page limits from Config apply:
//   - MaxDepth 0 → no depth cap; crawl until MaxPages or ctx cancellation.
//   - MaxPages 0 → no page cap; crawl until depth exhausted or ctx cancelled.
func (s *WikipediaScraper) CrawlFromURL(ctx context.Context, startURL string) <-chan ScrapeResult {
	out := make(chan ScrapeResult, 64)
	go func() {
		defer close(out)

		visited := &sync.Map{}
		visited.Store(startURL, true)

		frontier := []string{startURL}
		depth := 0
		total := 0

		for len(frontier) > 0 {
			// Depth limit check (0 = unlimited).
			if s.cfg.MaxDepth != 0 && depth > s.cfg.MaxDepth {
				break
			}

			// Page limit: trim frontier if it would exceed MaxPages.
			toProcess := frontier
			if s.cfg.MaxPages != 0 {
				remaining := s.cfg.MaxPages - total
				if remaining <= 0 {
					break
				}
				if len(toProcess) > remaining {
					toProcess = toProcess[:remaining]
				}
			}
			frontier = nil

			for result := range s.pool.Run(ctx, toProcess) {
				select {
				case out <- result:
				case <-ctx.Done():
					return
				}
				total++

				if result.Err == nil {
					for _, path := range discoverLinks(result.rawHTML) {
						link := s.baseURL() + path
						if _, seen := visited.LoadOrStore(link, true); !seen {
							frontier = append(frontier, link)
						}
					}
				}
			}

			depth++
		}
	}()
	return out
}
