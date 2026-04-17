package db

import (
	"context"
	"testing"
)

// upsertTestPage is a helper that inserts a page with sensible defaults.
func upsertTestPage(t *testing.T, ctx context.Context, dao *PageDAO, url string) *Page {
	t.Helper()
	p, err := dao.Upsert(ctx, url, "en", "Test Title", "Test summary.", "## Content\nBody text.", nil, nil)
	if err != nil {
		t.Fatalf("upsert page %s: %v", url, err)
	}
	return p
}

func TestPageDAO_Upsert_Insert(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	page, err := dao.Upsert(ctx,
		"https://en.wikipedia.org/wiki/Go",
		"en", "Go (programming language)", "Go is a language.", "## Overview\nGo is open source.",
		nil, nil,
	)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if page.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if page.URL != "https://en.wikipedia.org/wiki/Go" {
		t.Errorf("URL = %q, want %q", page.URL, "https://en.wikipedia.org/wiki/Go")
	}
	if page.Language != "en" {
		t.Errorf("Language = %q, want %q", page.Language, "en")
	}
	if page.Title != "Go (programming language)" {
		t.Errorf("Title = %q, want %q", page.Title, "Go (programming language)")
	}
	if page.Summary != "Go is a language." {
		t.Errorf("Summary = %q, want %q", page.Summary, "Go is a language.")
	}
	if page.Content != "## Overview\nGo is open source." {
		t.Errorf("Content = %q", page.Content)
	}
	if page.DatePublished != nil || page.DateModified != nil {
		t.Error("expected nil dates")
	}
}

func TestPageDAO_Upsert_Update(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	original, err := dao.Upsert(ctx,
		"https://en.wikipedia.org/wiki/Go",
		"en", "Go", "Old summary.", "Old content.",
		nil, nil,
	)
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	updated, err := dao.Upsert(ctx,
		"https://en.wikipedia.org/wiki/Go",
		"en", "Go (programming language)", "New summary.", "New content.",
		nil, nil,
	)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	if updated.ID != original.ID {
		t.Errorf("ID changed on update: got %d, want %d", updated.ID, original.ID)
	}
	if updated.Title != "Go (programming language)" {
		t.Errorf("Title not updated: got %q", updated.Title)
	}
	if updated.Content != "New content." {
		t.Errorf("Content not updated: got %q", updated.Content)
	}
}

func TestPageDAO_FindByURL_Found(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	inserted := upsertTestPage(t, ctx, dao, "https://en.wikipedia.org/wiki/Rust")

	found, err := dao.FindByURL(ctx, "https://en.wikipedia.org/wiki/Rust")
	if err != nil {
		t.Fatalf("FindByURL: %v", err)
	}
	if found == nil {
		t.Fatal("expected page, got nil")
	}
	if found.ID != inserted.ID {
		t.Errorf("ID mismatch: got %d, want %d", found.ID, inserted.ID)
	}
}

func TestPageDAO_FindByURL_NotFound(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	page, err := dao.FindByURL(ctx, "https://en.wikipedia.org/wiki/DoesNotExist")
	if err != nil {
		t.Fatalf("FindByURL: %v", err)
	}
	if page != nil {
		t.Errorf("expected nil, got page with ID %d", page.ID)
	}
}

func TestPageDAO_List(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	for i := 0; i < 5; i++ {
		url := "https://en.wikipedia.org/wiki/Page" + string(rune('A'+i))
		upsertTestPage(t, ctx, dao, url)
	}

	page1, err := dao.List(ctx, 3, 0)
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(page1) != 3 {
		t.Fatalf("got %d rows, want 3", len(page1))
	}

	page2, err := dao.List(ctx, 3, 3)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("got %d rows, want 2", len(page2))
	}

	// Verify no overlap between pages.
	ids := make(map[int64]bool)
	for _, p := range append(page1, page2...) {
		if ids[p.ID] {
			t.Errorf("duplicate ID %d across pages", p.ID)
		}
		ids[p.ID] = true
	}
}

func TestPageDAO_Delete(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	upsertTestPage(t, ctx, dao, "https://en.wikipedia.org/wiki/Python")

	if err := dao.Delete(ctx, "https://en.wikipedia.org/wiki/Python"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	page, err := dao.FindByURL(ctx, "https://en.wikipedia.org/wiki/Python")
	if err != nil {
		t.Fatalf("FindByURL after delete: %v", err)
	}
	if page != nil {
		t.Error("expected nil after delete, got a page")
	}
}

func TestPageDAO_FindWithoutQuestions_ReturnsUnprocessedPages(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	upsertTestPage(t, ctx, dao, "https://en.wikipedia.org/wiki/A")
	upsertTestPage(t, ctx, dao, "https://en.wikipedia.org/wiki/B")

	pages, err := dao.FindWithoutQuestions(ctx, 10)
	if err != nil {
		t.Fatalf("FindWithoutQuestions: %v", err)
	}
	if len(pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(pages))
	}
}

func TestPageDAO_FindWithoutQuestions_ExcludesPagesWithQuestions(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}
	qDAO := &QuestionDAO{pool: testPool}

	pageA := upsertTestPage(t, ctx, dao, "https://en.wikipedia.org/wiki/A")
	upsertTestPage(t, ctx, dao, "https://en.wikipedia.org/wiki/B")

	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}
	if _, err := qDAO.Insert(ctx, pageA.ID, "Q?", choices); err != nil {
		t.Fatalf("Insert question: %v", err)
	}

	pages, err := dao.FindWithoutQuestions(ctx, 10)
	if err != nil {
		t.Fatalf("FindWithoutQuestions: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page without questions, got %d", len(pages))
	}
	if pages[0].URL != "https://en.wikipedia.org/wiki/B" {
		t.Errorf("expected page B, got %q", pages[0].URL)
	}
}

func TestPageDAO_FindWithoutQuestions_RespectsLimit(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	for _, u := range []string{"A", "B", "C", "D", "E"} {
		upsertTestPage(t, ctx, dao, "https://en.wikipedia.org/wiki/"+u)
	}

	pages, err := dao.FindWithoutQuestions(ctx, 3)
	if err != nil {
		t.Fatalf("FindWithoutQuestions: %v", err)
	}
	if len(pages) != 3 {
		t.Errorf("expected 3 pages (limit), got %d", len(pages))
	}
}

// runMigrations applies all pending migrations within a test. It is called after
// truncateTables so the schema_migrations table is also empty and migrations re-run.
func runMigrations(t *testing.T) {
	t.Helper()
	if err := Migrate(context.Background(), testPool); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}
