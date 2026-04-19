package db

import (
	"context"
	"testing"
)

func TestGameAnswerDAO_Insert_And_Exists(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	userDAO := &UserDAO{pool: testPool}
	pageDAO := &PageDAO{pool: testPool}
	questionDAO := &QuestionDAO{pool: testPool}
	sessionDAO := &GameSessionDAO{pool: testPool}
	answerDAO := &GameAnswerDAO{pool: testPool}

	alice := insertTestUser(t, ctx, userDAO, "alice")
	page := upsertTestPage(t, ctx, pageDAO, "https://en.wikipedia.org/wiki/Go")
	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}
	q, err := questionDAO.Insert(ctx, page.ID, "Test question?", choices)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}

	s, err := sessionDAO.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	exists, err := answerDAO.ExistsForQuestion(ctx, s.ID, alice.ID, q.ID)
	if err != nil {
		t.Fatalf("exists check: %v", err)
	}
	if exists {
		t.Error("expected false before answer inserted")
	}

	if _, err := answerDAO.Insert(ctx, s.ID, alice.ID, q.ID, 0, true, false); err != nil {
		t.Fatalf("insert answer: %v", err)
	}

	exists, err = answerDAO.ExistsForQuestion(ctx, s.ID, alice.ID, q.ID)
	if err != nil {
		t.Fatalf("exists check after insert: %v", err)
	}
	if !exists {
		t.Error("expected true after answer inserted")
	}
}

func TestGameAnswerDAO_ListBySession(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	userDAO := &UserDAO{pool: testPool}
	pageDAO := &PageDAO{pool: testPool}
	questionDAO := &QuestionDAO{pool: testPool}
	sessionDAO := &GameSessionDAO{pool: testPool}
	answerDAO := &GameAnswerDAO{pool: testPool}

	alice := insertTestUser(t, ctx, userDAO, "alice")
	page := upsertTestPage(t, ctx, pageDAO, "https://en.wikipedia.org/wiki/Python")
	choices := []Choice{
		{Text: "A", Correct: true},
		{Text: "B", Correct: false},
		{Text: "C", Correct: false},
		{Text: "D", Correct: false},
		{Text: "E", Correct: false},
	}

	q1, err := questionDAO.Insert(ctx, page.ID, "Q1?", choices)
	if err != nil {
		t.Fatalf("insert q1: %v", err)
	}
	q2, err := questionDAO.Insert(ctx, page.ID, "Q2?", choices)
	if err != nil {
		t.Fatalf("insert q2: %v", err)
	}

	s, err := sessionDAO.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if _, err := answerDAO.Insert(ctx, s.ID, alice.ID, q1.ID, 0, true, false); err != nil {
		t.Fatalf("insert answer 1: %v", err)
	}
	if _, err := answerDAO.Insert(ctx, s.ID, alice.ID, q2.ID, 1, false, false); err != nil {
		t.Fatalf("insert answer 2: %v", err)
	}

	answers, err := answerDAO.ListBySession(ctx, s.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(answers) != 2 {
		t.Errorf("expected 2 answers, got %d", len(answers))
	}
}
