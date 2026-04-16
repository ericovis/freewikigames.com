package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// searchResponse mirrors the Wikipedia search API JSON response structure.
type searchResponse struct {
	Query struct {
		Search []struct {
			Title string `json:"title"`
		} `json:"search"`
	} `json:"query"`
}

// searchAPIURL returns the Wikipedia search API URL for the given term.
func (s *WikipediaScraper) searchAPIURL(term string) string {
	return s.baseURL() + "/w/api.php?action=query&list=search&format=json&srlimit=10&srsearch=" + url.QueryEscape(term)
}

// articleURL returns the full Wikipedia article URL for the given title.
func (s *WikipediaScraper) articleURL(title string) string {
	return s.baseURL() + "/wiki/" + url.PathEscape(title)
}

// searchWikipedia queries the Wikipedia search API for term and returns the
// article URLs of the top results.
func (s *WikipediaScraper) searchWikipedia(ctx context.Context, term string) ([]string, error) {
	body, err := s.client.Get(ctx, s.searchAPIURL(term))
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}

	var resp searchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}

	urls := make([]string, 0, len(resp.Query.Search))
	for _, item := range resp.Query.Search {
		urls = append(urls, s.articleURL(item.Title))
	}
	return urls, nil
}

// SearchAndScrape queries Wikipedia for term, then scrapes the top results.
func (s *WikipediaScraper) SearchAndScrape(ctx context.Context, term string) <-chan ScrapeResult {
	out := make(chan ScrapeResult, 16)
	go func() {
		defer close(out)

		urls, err := s.searchWikipedia(ctx, term)
		if err != nil {
			out <- ScrapeResult{Err: fmt.Errorf("search %q: %w", term, err)}
			return
		}

		for result := range s.pool.Run(ctx, urls) {
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// SearchAndScrapeMultiple runs SearchAndScrape for each term concurrently
// and merges all results into a single channel.
func (s *WikipediaScraper) SearchAndScrapeMultiple(ctx context.Context, terms []string) <-chan ScrapeResult {
	channels := make([]<-chan ScrapeResult, 0, len(terms))
	for _, term := range terms {
		channels = append(channels, s.SearchAndScrape(ctx, term))
	}
	return mergeResults(channels...)
}

// SearchAndCrawl queries Wikipedia for term, then crawls from each result page
// following links up to cfg.MaxDepth and cfg.MaxPages.
func (s *WikipediaScraper) SearchAndCrawl(ctx context.Context, term string) <-chan ScrapeResult {
	out := make(chan ScrapeResult, 64)
	go func() {
		defer close(out)

		urls, err := s.searchWikipedia(ctx, term)
		if err != nil {
			out <- ScrapeResult{Err: fmt.Errorf("search %q: %w", term, err)}
			return
		}

		channels := make([]<-chan ScrapeResult, 0, len(urls))
		for _, u := range urls {
			channels = append(channels, s.CrawlFromURL(ctx, u))
		}

		for result := range mergeResults(channels...) {
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
