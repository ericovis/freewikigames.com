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

func TestExtractBodyContent_ExcludesFiguresAndTables(t *testing.T) {
	rawHTML := `<html><body>
<div id="bodyContent">
  <p>Keep this text.</p>
  <figure><img src="photo.jpg"/><figcaption>A caption</figcaption></figure>
  <table><tr><td>row data</td></tr></table>
  <p>And this text.</p>
</div>
</body></html>`

	result := extractBodyContent(rawHTML)

	if !strings.Contains(result, "Keep this text.") {
		t.Errorf("expected body text to be kept, got: %q", result)
	}
	if !strings.Contains(result, "And this text.") {
		t.Errorf("expected second paragraph to be kept, got: %q", result)
	}
	if strings.Contains(result, "<figure") || strings.Contains(result, "figcaption") || strings.Contains(result, "photo.jpg") {
		t.Errorf("expected <figure> to be stripped, got: %q", result)
	}
	if strings.Contains(result, "<table") || strings.Contains(result, "row data") {
		t.Errorf("expected <table> to be stripped, got: %q", result)
	}
}

func TestExtractBodyContent_MissingDiv_ReturnsInput(t *testing.T) {
	rawHTML := "<html><body><p>no bodyContent id here</p></body></html>"
	result := extractBodyContent(rawHTML)
	if result != rawHTML {
		t.Errorf("expected original HTML back, got: %q", result)
	}
}

func TestStripBoilerplateSections_RemovesKnownSections(t *testing.T) {
	content := "## History\nGo was designed at Google in 2007 by Robert Griesemer, Rob Pike, and Ken Thompson.\n\n## See also\nSome links here.\n\n## References\nLots of citations.\n\n## Features\nGo includes garbage collection and CSP-style concurrent programming features."
	result := stripBoilerplateSections(content)

	if strings.Contains(result, "## See also") || strings.Contains(result, "Some links here") {
		t.Errorf("expected 'See also' section to be stripped, got: %q", result)
	}
	if strings.Contains(result, "## References") || strings.Contains(result, "Lots of citations") {
		t.Errorf("expected 'References' section to be stripped, got: %q", result)
	}
	if !strings.Contains(result, "## History") {
		t.Errorf("expected 'History' section to be kept, got: %q", result)
	}
	if !strings.Contains(result, "## Features") {
		t.Errorf("expected 'Features' section to be kept, got: %q", result)
	}
}

func TestStripBoilerplateSections_PortugueseSections(t *testing.T) {
	content := "## Biografia\nTexto sobre a vida.\n\n## Ver também\nOutros artigos.\n\n## Referências\nCitações aqui."
	result := stripBoilerplateSections(content)

	if strings.Contains(result, "Ver também") || strings.Contains(result, "Outros artigos") {
		t.Errorf("expected 'Ver também' to be stripped, got: %q", result)
	}
	if strings.Contains(result, "Referências") || strings.Contains(result, "Citações aqui") {
		t.Errorf("expected 'Referências' to be stripped, got: %q", result)
	}
	if !strings.Contains(result, "## Biografia") {
		t.Errorf("expected 'Biografia' section to be kept, got: %q", result)
	}
}

func TestStripBoilerplateSections_NoBoilerplate(t *testing.T) {
	content := "## History\nSome facts.\n\n## Features\nMore facts."
	result := stripBoilerplateSections(content)
	if result != content {
		t.Errorf("expected content unchanged, got: %q", result)
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
