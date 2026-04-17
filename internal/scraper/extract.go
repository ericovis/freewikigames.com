package scraper

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"
)

var htmlLangRegexp = regexp.MustCompile(`(?i)<html\s[^>]*lang="([^"]+)"`)

// extractLanguage returns the ISO 639-1 language code for a Wikipedia page.
//
// Primary strategy: parse the URL hostname. Wikipedia URLs follow the pattern
// https://{lang}.wikipedia.org/wiki/... so the subdomain is the language code.
//
// Fallback: scan rawHTML for the <html lang="..."> attribute.
//
// Final default: "en".
func extractLanguage(rawURL, rawHTML string) string {
	if u, err := url.Parse(rawURL); err == nil {
		host := u.Hostname()
		if dot := strings.Index(host, "."); dot > 0 {
			lang := host[:dot]
			rest := host[dot+1:]
			if rest == "wikipedia.org" && len(lang) >= 2 {
				return lang
			}
		}
	}
	if m := htmlLangRegexp.FindStringSubmatch(rawHTML); len(m) == 2 {
		return m[1]
	}
	return "en"
}

var jsonLDRegexp = regexp.MustCompile(`(?i)<script[^>]+type="application/ld\+json"[^>]*>([\s\S]*?)</script>`)

// extractJSONLD finds the first application/ld+json script block and returns
// the article title and publication/modification timestamps. All return values
// are zero/nil if the script is absent or cannot be parsed.
func extractJSONLD(rawHTML string) (title string, published, modified *time.Time) {
	m := jsonLDRegexp.FindStringSubmatch(rawHTML)
	if len(m) < 2 {
		return
	}
	var data struct {
		Name          string `json:"name"`
		DatePublished string `json:"datePublished"`
		DateModified  string `json:"dateModified"`
	}
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		return
	}
	title = data.Name
	if t, err := time.Parse(time.RFC3339, data.DatePublished); err == nil {
		published = &t
	}
	if t, err := time.Parse(time.RFC3339, data.DateModified); err == nil {
		modified = &t
	}
	return
}

// extractBodyContent parses rawHTML and returns the inner HTML of the element
// with id="bodyContent". Returns rawHTML unchanged if the element is not found.
func extractBodyContent(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return rawHTML
	}
	node := findNodeByID(doc, "bodyContent")
	if node == nil {
		return rawHTML
	}
	var buf strings.Builder
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return rawHTML
		}
	}
	return buf.String()
}

// findNodeByID does a depth-first search for the first HTML element whose id
// attribute equals id.
func findNodeByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val == id {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNodeByID(c, id); found != nil {
			return found
		}
	}
	return nil
}

// convertToMarkdown converts an HTML fragment to Markdown. Returns rawHTML
// unchanged if conversion fails.
func convertToMarkdown(rawHTML string) string {
	md, err := htmltomarkdown.ConvertString(rawHTML)
	if err != nil {
		return rawHTML
	}
	return md
}
