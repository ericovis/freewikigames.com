package scraper

import (
	"strings"
	"testing"
	"time"
)

func TestExtractLanguage_WikipediaSubdomain(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://en.wikipedia.org/wiki/Go", "en"},
		{"https://pt.wikipedia.org/wiki/Brasil", "pt"},
		{"https://de.wikipedia.org/wiki/Berlin", "de"},
	}
	for _, tc := range cases {
		got := extractLanguage(tc.url, "")
		if got != tc.want {
			t.Errorf("extractLanguage(%q, \"\") = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestExtractLanguage_HTMLFallback(t *testing.T) {
	html := `<html lang="fr"><body>Bonjour</body></html>`
	got := extractLanguage("https://example.com/page", html)
	if got != "fr" {
		t.Errorf("extractLanguage: got %q, want %q", got, "fr")
	}
}

func TestExtractLanguage_DefaultsEN(t *testing.T) {
	got := extractLanguage("https://example.com/page", "<html><body>hi</body></html>")
	if got != "en" {
		t.Errorf("extractLanguage: got %q, want %q", got, "en")
	}
}

func TestExtractJSONLD_ValidScript(t *testing.T) {
	rawHTML := `<html><head>
<script type="application/ld+json">{"@type":"Article","name":"Nynetjer","datePublished":"2005-03-08T11:13:01Z","dateModified":"2026-04-16T23:20:34Z"}</script>
</head><body></body></html>`

	title, published, modified := extractJSONLD(rawHTML)

	if title != "Nynetjer" {
		t.Errorf("title = %q, want %q", title, "Nynetjer")
	}
	if published == nil {
		t.Fatal("expected non-nil datePublished")
	}
	wantPub := time.Date(2005, 3, 8, 11, 13, 1, 0, time.UTC)
	if !published.Equal(wantPub) {
		t.Errorf("datePublished = %v, want %v", *published, wantPub)
	}
	if modified == nil {
		t.Fatal("expected non-nil dateModified")
	}
	wantMod := time.Date(2026, 4, 16, 23, 20, 34, 0, time.UTC)
	if !modified.Equal(wantMod) {
		t.Errorf("dateModified = %v, want %v", *modified, wantMod)
	}
}

func TestExtractJSONLD_NoScript(t *testing.T) {
	title, published, modified := extractJSONLD("<html><body>no ld+json here</body></html>")
	if title != "" || published != nil || modified != nil {
		t.Errorf("expected zero values, got title=%q published=%v modified=%v", title, published, modified)
	}
}

func TestExtractJSONLD_InvalidJSON(t *testing.T) {
	rawHTML := `<script type="application/ld+json">not valid json</script>`
	title, published, modified := extractJSONLD(rawHTML)
	if title != "" || published != nil || modified != nil {
		t.Errorf("expected zero values for invalid JSON, got title=%q", title)
	}
}

func TestExtractBodyContent_FindsDiv(t *testing.T) {
	rawHTML := `<html><body>
<div id="mw-head">navigation</div>
<div id="bodyContent"><p>Article text here.</p></div>
<div id="footer">footer</div>
</body></html>`

	result := extractBodyContent(rawHTML)

	if !strings.Contains(result, "Article text here.") {
		t.Errorf("expected body content, got: %q", result)
	}
	if strings.Contains(result, "navigation") {
		t.Errorf("should not contain navigation content, got: %q", result)
	}
	if strings.Contains(result, "footer") {
		t.Errorf("should not contain footer content, got: %q", result)
	}
}

func TestExtractBodyContent_MissingDiv_ReturnsInput(t *testing.T) {
	rawHTML := "<html><body><p>no bodyContent id here</p></body></html>"
	result := extractBodyContent(rawHTML)
	if result != rawHTML {
		t.Errorf("expected original HTML back, got: %q", result)
	}
}

func TestConvertToMarkdown_HeadingsAndParagraphs(t *testing.T) {
	rawHTML := `<h1>Title</h1><p>Some paragraph text.</p>`
	result := convertToMarkdown(rawHTML)

	if !strings.Contains(result, "Title") {
		t.Errorf("expected heading in markdown, got: %q", result)
	}
	if !strings.Contains(result, "Some paragraph text.") {
		t.Errorf("expected paragraph text in markdown, got: %q", result)
	}
}
