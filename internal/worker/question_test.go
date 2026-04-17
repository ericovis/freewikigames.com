package worker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ericovis/freewikigames.com/internal/db"
	"github.com/ericovis/freewikigames.com/internal/questions"
)

// mockQuestionDAO satisfies questionDAO for tests.
type mockQuestionDAO struct {
	inserted []db.Question
}

func (m *mockQuestionDAO) Insert(ctx context.Context, pageID int64, text, language string, choices []db.Choice) (*db.Question, error) {
	q := db.Question{ID: int64(len(m.inserted) + 1), PageID: pageID, Text: text, Language: language, Choices: choices}
	m.inserted = append(m.inserted, q)
	return &q, nil
}

// mockGenerator satisfies questionGenerator for tests.
type mockGenerator struct {
	fn func(ctx context.Context, rawHTML, language string) ([]questions.Question, error)
}

func (m *mockGenerator) GenerateWithLanguage(ctx context.Context, rawHTML, language string) ([]questions.Question, error) {
	return m.fn(ctx, rawHTML, language)
}

func fiveQChoices(correctIdx int) []questions.Choice {
	choices := make([]questions.Choice, 5)
	for i := range choices {
		choices[i] = questions.Choice{Text: string(rune('A' + i)), Correct: i == correctIdx}
	}
	return choices
}

func TestQuestionWorker_Run_GeneratesAndStoresQuestions(t *testing.T) {
	page := db.Page{ID: 1, URL: "https://en.wikipedia.org/wiki/Go", RawHTML: "<html/>"}

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	pages := &mockPageDAO{
		findWithoutQuestionsFn: func(ctx context.Context, limit int) ([]db.Page, error) {
			calls++
			if calls == 1 {
				return []db.Page{page}, nil
			}
			// Cancel after the first batch has been processed.
			cancel()
			return nil, nil
		},
	}
	qDAO := &mockQuestionDAO{}
	gen := &mockGenerator{fn: func(ctx context.Context, rawHTML, language string) ([]questions.Question, error) {
		return []questions.Question{
			{Text: "Q1", Choices: fiveQChoices(0)},
			{Text: "Q2", Choices: fiveQChoices(1)},
		}, nil
	}}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewQuestionWorker(pages, qDAO, gen, logger)
	w.pollInterval = time.Millisecond

	w.Run(ctx) //nolint:errcheck

	if len(qDAO.inserted) != 2 {
		t.Errorf("expected 2 inserted questions, got %d", len(qDAO.inserted))
	}
	if qDAO.inserted[0].Language != "en" {
		t.Errorf("expected language 'en', got %q", qDAO.inserted[0].Language)
	}
}

func TestQuestionWorker_Run_PollesWhenNoPagesFound(t *testing.T) {
	pollCount := 0
	pages := &mockPageDAO{
		findWithoutQuestionsFn: func(ctx context.Context, limit int) ([]db.Page, error) {
			pollCount++
			return nil, nil
		},
	}
	qDAO := &mockQuestionDAO{}
	gen := &mockGenerator{fn: func(ctx context.Context, rawHTML, language string) ([]questions.Question, error) {
		return nil, nil
	}}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewQuestionWorker(pages, qDAO, gen, logger)
	w.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	w.Run(ctx) //nolint:errcheck

	if pollCount < 2 {
		t.Errorf("expected at least 2 polls (got %d) to confirm polling behaviour", pollCount)
	}
}

func TestQuestionWorker_Run_ExitsOnContextCancel(t *testing.T) {
	pages := &mockPageDAO{
		findWithoutQuestionsFn: func(ctx context.Context, limit int) ([]db.Page, error) {
			return nil, nil
		},
	}
	qDAO := &mockQuestionDAO{}
	gen := &mockGenerator{fn: func(ctx context.Context, rawHTML, language string) ([]questions.Question, error) {
		return nil, nil
	}}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewQuestionWorker(pages, qDAO, gen, logger)
	w.pollInterval = time.Hour // long poll so exit is driven only by ctx

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected nil error on cancel, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestQuestionWorker_Run_LogsGeneratorError(t *testing.T) {
	page := db.Page{ID: 1, URL: "https://en.wikipedia.org/wiki/Go", RawHTML: "<html/>"}
	calls := 0
	pages := &mockPageDAO{
		findWithoutQuestionsFn: func(ctx context.Context, limit int) ([]db.Page, error) {
			calls++
			if calls == 1 {
				return []db.Page{page}, nil
			}
			return nil, nil
		},
	}
	qDAO := &mockQuestionDAO{}
	gen := &mockGenerator{fn: func(ctx context.Context, rawHTML, language string) ([]questions.Question, error) {
		return nil, errors.New("ollama unavailable")
	}}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewQuestionWorker(pages, qDAO, gen, logger)
	w.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	w.Run(ctx) //nolint:errcheck

	// No questions should have been inserted despite a page being found.
	if len(qDAO.inserted) != 0 {
		t.Errorf("expected 0 inserted questions on generator error, got %d", len(qDAO.inserted))
	}
}
