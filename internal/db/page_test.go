package db

import (
	"context"
	"testing"
	"time"
)

func TestPageDAO_Upsert_Insert(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}
	now := time.Now().UTC().Truncate(time.Millisecond)

	page, err := dao.Upsert(ctx, "https://en.wikipedia.org/wiki/Go", "<html>Go</html>", now)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if page.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if page.URL != "https://en.wikipedia.org/wiki/Go" {
		t.Errorf("URL = %q, want %q", page.URL, "https://en.wikipedia.org/wiki/Go")
	}
	if page.RawHTML != "<html>Go</html>" {
		t.Errorf("RawHTML = %q, want %q", page.RawHTML, "<html>Go</html>")
	}
	if !page.LastScrapedAt.Equal(now) {
		t.Errorf("LastScrapedAt = %v, want %v", page.LastScrapedAt, now)
	}
	if page.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestPageDAO_Upsert_Update(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	first := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	original, err := dao.Upsert(ctx, "https://en.wikipedia.org/wiki/Go", "<html>old</html>", first)
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	second := time.Now().UTC().Truncate(time.Millisecond)
	updated, err := dao.Upsert(ctx, "https://en.wikipedia.org/wiki/Go", "<html>new</html>", second)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	if updated.RawHTML != "<html>new</html>" {
		t.Errorf("RawHTML = %q, want updated value", updated.RawHTML)
	}
	if !updated.LastScrapedAt.Equal(second) {
		t.Errorf("LastScrapedAt not updated: got %v, want %v", updated.LastScrapedAt, second)
	}
	if !updated.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt changed on update: got %v, want %v", updated.CreatedAt, original.CreatedAt)
	}
}

func TestPageDAO_FindByURL_Found(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}
	now := time.Now().UTC().Truncate(time.Millisecond)

	inserted, _ := dao.Upsert(ctx, "https://en.wikipedia.org/wiki/Rust", "<html>Rust</html>", now)

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

func TestPageDAO_FindOldestScraped(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	base := time.Now().UTC().Truncate(time.Millisecond)
	urls := []string{
		"https://en.wikipedia.org/wiki/A",
		"https://en.wikipedia.org/wiki/B",
		"https://en.wikipedia.org/wiki/C",
	}
	for i, u := range urls {
		// Insert with descending scraped times so A is newest, C is oldest.
		scrapedAt := base.Add(-time.Duration(i) * time.Hour)
		if _, err := dao.Upsert(ctx, u, "<html/>", scrapedAt); err != nil {
			t.Fatalf("Upsert %s: %v", u, err)
		}
	}

	pages, err := dao.FindOldestScraped(ctx, 2)
	if err != nil {
		t.Fatalf("FindOldestScraped: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	if pages[0].URL != "https://en.wikipedia.org/wiki/C" {
		t.Errorf("first page = %q, want C (oldest)", pages[0].URL)
	}
	if pages[1].URL != "https://en.wikipedia.org/wiki/B" {
		t.Errorf("second page = %q, want B", pages[1].URL)
	}
}

func TestPageDAO_List(t *testing.T) {
	truncateTables(t)
	runMigrations(t)

	ctx := context.Background()
	dao := &PageDAO{pool: testPool}

	now := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 5; i++ {
		url := "https://en.wikipedia.org/wiki/Page" + string(rune('A'+i))
		if _, err := dao.Upsert(ctx, url, "<html/>", now); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
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
	now := time.Now().UTC().Truncate(time.Millisecond)

	if _, err := dao.Upsert(ctx, "https://en.wikipedia.org/wiki/Python", "<html/>", now); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

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

// runMigrations applies all pending migrations within a test. It is called after
// truncateTables so the schema_migrations table is also empty and migrations re-run.
func runMigrations(t *testing.T) {
	t.Helper()
	if err := Migrate(context.Background(), testPool); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}
