package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// fetchSummary calls the Wikipedia REST summary API and returns the plain-text
// extract for the article. Returns an empty string on any error so callers
// degrade gracefully.
func (s *WikipediaScraper) fetchSummary(ctx context.Context, lang, title string) string {
	if title == "" {
		return ""
	}
	apiURL := fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1/page/summary/%s",
		lang, url.PathEscape(title))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "freewikigames-scraper/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var data struct {
		Extract string `json:"extract"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	return data.Extract
}
