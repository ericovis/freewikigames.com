package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ericovis/freewikigames.com/internal/db"
)

const defaultPollInterval = 5 * time.Second
const defaultBatchSize = 10

// QuestionWorker polls for pages that have no questions yet, generates
// questions for each one via the AI, and stores them in the database.
// It runs until ctx is cancelled.
type QuestionWorker struct {
	pages        pageDAO
	questions    questionDAO
	generator    questionGenerator
	batchSize    int
	pollInterval time.Duration
	logger       *slog.Logger
}

// NewQuestionWorker constructs a QuestionWorker with default batch size and
// poll interval.
func NewQuestionWorker(pages pageDAO, questions questionDAO, generator questionGenerator, logger *slog.Logger) *QuestionWorker {
	return &QuestionWorker{
		pages:        pages,
		questions:    questions,
		generator:    generator,
		batchSize:    defaultBatchSize,
		pollInterval: defaultPollInterval,
		logger:       logger,
	}
}

// Run polls for unprocessed pages in a loop until ctx is cancelled.
func (w *QuestionWorker) Run(ctx context.Context) error {
	w.logger.Info("question worker started")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("question worker stopped")
			return nil
		default:
		}

		pages, err := w.pages.FindWithoutQuestions(ctx, w.batchSize)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil
			}
			w.logger.Error("find pages without questions", "err", err)
			if !w.sleep(ctx) {
				return nil
			}
			continue
		}

		if len(pages) == 0 {
			if !w.sleep(ctx) {
				return nil
			}
			continue
		}

		for _, page := range pages {
			if ctx.Err() != nil {
				return nil
			}
			w.processPage(ctx, page)
		}
	}
}

func (w *QuestionWorker) processPage(ctx context.Context, page db.Page) {
	qs, err := w.generator.GenerateWithLanguage(ctx, page.Title, page.Language, page.Content)
	if err != nil {
		w.logger.Error("generate questions", "page_id", page.ID, "url", page.URL, "err", err)
		return
	}

	for _, q := range qs {
		choices := make([]db.Choice, len(q.Choices))
		for i, c := range q.Choices {
			choices[i] = db.Choice{Text: c.Text, Correct: c.Correct}
		}
		if _, err := w.questions.Insert(ctx, page.ID, q.Text, choices); err != nil {
			w.logger.Error("insert question", "page_id", page.ID, "err", err)
		}
	}
	w.logger.Info("processed page", "page_id", page.ID, "url", page.URL, "language", page.Language, "questions", len(qs))
}

// sleep waits for pollInterval or ctx cancellation. Returns false if ctx was
// cancelled.
func (w *QuestionWorker) sleep(ctx context.Context) bool {
	select {
	case <-time.After(w.pollInterval):
		return true
	case <-ctx.Done():
		return false
	}
}
