package worker

import (
	"net/url"
	"regexp"
	"strings"
)

var htmlLangRegexp = regexp.MustCompile(`(?i)<html\s[^>]*lang="([^"]+)"`)

// detectLanguage extracts the language code for a scraped page.
//
// Primary strategy: parse the Wikipedia URL hostname. Wikipedia URLs follow
// the pattern https://{lang}.wikipedia.org/wiki/... so the subdomain is the
// ISO 639-1 language code.
//
// Fallback: scan rawHTML for the <html lang="..."> attribute.
//
// Final default: "en".
func detectLanguage(rawURL, rawHTML string) string {
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
