package worker

import "testing"

func TestDetectLanguage_WikipediaURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://en.wikipedia.org/wiki/Go", "en"},
		{"https://pt.wikipedia.org/wiki/Programação", "pt"},
		{"https://de.wikipedia.org/wiki/Golang", "de"},
		{"https://zh.wikipedia.org/wiki/Go", "zh"},
	}
	for _, tc := range cases {
		got := detectLanguage(tc.url, "")
		if got != tc.want {
			t.Errorf("detectLanguage(%q, \"\") = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestDetectLanguage_HTMLFallback(t *testing.T) {
	// Non-Wikipedia URL falls back to <html lang="...">
	got := detectLanguage("https://example.com/page", `<html lang="fr"><body>content</body></html>`)
	if got != "fr" {
		t.Errorf("expected 'fr' from HTML fallback, got %q", got)
	}
}

func TestDetectLanguage_DefaultsToEN(t *testing.T) {
	got := detectLanguage("https://example.com/page", "<html><body>no lang attr</body></html>")
	if got != "en" {
		t.Errorf("expected default 'en', got %q", got)
	}
}

func TestDetectLanguage_InvalidURL(t *testing.T) {
	got := detectLanguage("://invalid-url", "")
	if got != "en" {
		t.Errorf("expected default 'en' for invalid URL, got %q", got)
	}
}
