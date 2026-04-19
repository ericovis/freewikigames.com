package db

import (
	"context"
	"testing"
)

func TestQuestionDAO_FindNextForSession_ReturnsQuestion(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	pageDAO := &PageDAO{pool: testPool}
	questionDAO := &QuestionDAO{pool: testPool}
	sessionDAO := &GameSessionDAO{pool: testPool}
	answerDAO := &GameAnswerDAO{pool: testPool}
	userDAO := &UserDAO{pool: testPool}

	page := upsertTestPage(t, ctx, pageDAO, "https://en.wikipedia.org/wiki/Rust")
	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}
	q, err := questionDAO.Insert(ctx, page.ID, "What is Rust?", choices)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}

	s, err := sessionDAO.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	next, err := questionDAO.FindNextForSession(ctx, "en", nil, s.ID)
	if err != nil {
		t.Fatalf("find next: %v", err)
	}
	if next == nil {
		t.Fatal("expected a question, got nil")
	}
	if next.ID != q.ID {
		t.Errorf("expected question %d, got %d", q.ID, next.ID)
	}

	// After the question is answered it should no longer be returned.
	alice := insertTestUser(t, ctx, userDAO, "alice")
	if _, err := answerDAO.Insert(ctx, s.ID, alice.ID, q.ID, 0, true, false); err != nil {
		t.Fatalf("insert answer: %v", err)
	}

	exhausted, err := questionDAO.FindNextForSession(ctx, "en", nil, s.ID)
	if err != nil {
		t.Fatalf("find next after answer: %v", err)
	}
	if exhausted != nil {
		t.Errorf("expected nil after pool exhausted, got %+v", exhausted)
	}
}

func TestQuestionDAO_FindNextForSession_FiltersByLanguage(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	pageDAO := &PageDAO{pool: testPool}
	questionDAO := &QuestionDAO{pool: testPool}
	sessionDAO := &GameSessionDAO{pool: testPool}

	// Insert a Portuguese page with a question.
	ptPage, err := pageDAO.Upsert(ctx, "https://pt.wikipedia.org/wiki/Go", "pt", "Go", "Go.", "Go.", nil, nil)
	if err != nil {
		t.Fatalf("upsert pt page: %v", err)
	}
	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}
	if _, err := questionDAO.Insert(ctx, ptPage.ID, "Pergunta?", choices); err != nil {
		t.Fatalf("insert pt question: %v", err)
	}

	// An English session should not see the Portuguese question.
	s, err := sessionDAO.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	next, err := questionDAO.FindNextForSession(ctx, "en", nil, s.ID)
	if err != nil {
		t.Fatalf("find next: %v", err)
	}
	if next != nil {
		t.Errorf("expected nil for English session with no English questions, got %+v", next)
	}
}
