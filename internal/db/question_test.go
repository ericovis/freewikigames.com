package db

import (
	"context"
	"testing"
	"time"
)

func TestQuestionDAO_Insert_OK(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &QuestionDAO{pool: testPool}
	pageDAO := &PageDAO{pool: testPool}

	page, err := pageDAO.Upsert(ctx, "https://en.wikipedia.org/wiki/Go", "<html/>", time.Now())
	if err != nil {
		t.Fatalf("upsert page: %v", err)
	}

	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}

	q, err := dao.Insert(ctx, page.ID, "What is Go?", choices)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}
	if q.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if q.PageID != page.ID {
		t.Errorf("expected page_id %d, got %d", page.ID, q.PageID)
	}
	if q.Text != "What is Go?" {
		t.Errorf("expected text 'What is Go?', got %q", q.Text)
	}
	if len(q.Choices) != 5 {
		t.Errorf("expected 5 choices, got %d", len(q.Choices))
	}
	if !q.Choices[0].Correct {
		t.Error("expected first choice to be correct")
	}
}

func TestQuestionDAO_FindByID_NotFound(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &QuestionDAO{pool: testPool}

	q, err := dao.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != nil {
		t.Errorf("expected nil for missing id, got %+v", q)
	}
}

func TestQuestionDAO_FindByPage(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &QuestionDAO{pool: testPool}
	pageDAO := &PageDAO{pool: testPool}

	page, err := pageDAO.Upsert(ctx, "https://en.wikipedia.org/wiki/Go", "<html/>", time.Now())
	if err != nil {
		t.Fatalf("upsert page: %v", err)
	}

	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}

	if _, err := dao.Insert(ctx, page.ID, "Question one", choices); err != nil {
		t.Fatalf("insert question 1: %v", err)
	}
	if _, err := dao.Insert(ctx, page.ID, "Question two", choices); err != nil {
		t.Fatalf("insert question 2: %v", err)
	}

	questions, err := dao.FindByPage(ctx, page.ID)
	if err != nil {
		t.Fatalf("find by page: %v", err)
	}
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
}

func TestQuestionDAO_Delete(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &QuestionDAO{pool: testPool}
	pageDAO := &PageDAO{pool: testPool}

	page, err := pageDAO.Upsert(ctx, "https://en.wikipedia.org/wiki/Go", "<html/>", time.Now())
	if err != nil {
		t.Fatalf("upsert page: %v", err)
	}

	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}

	q, err := dao.Insert(ctx, page.ID, "Delete me", choices)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}

	if err := dao.Delete(ctx, q.ID); err != nil {
		t.Fatalf("delete question: %v", err)
	}

	found, err := dao.FindByID(ctx, q.ID)
	if err != nil {
		t.Fatalf("find after delete: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil after delete, got %+v", found)
	}
}
