package questions

import (
	"context"
	"strings"
	"testing"

	"github.com/ericovis/freewikigames.com/internal/ai"
)

func TestReviewQuestion_AcceptVerdict(t *testing.T) {
	q := Question{Text: "When was Go released?", Choices: fiveChoices(1)}
	mock := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		dst.(*reviewResponse).Verdict = "accept"
		dst.(*reviewResponse).Reason = "looks good"
		return nil
	}}
	g := New(mock)
	got, keep, err := g.reviewQuestion(context.Background(), "Go", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !keep {
		t.Fatal("expected keep=true for accept verdict")
	}
	if got.Text != q.Text {
		t.Errorf("expected original question text, got %q", got.Text)
	}
}

func TestReviewQuestion_RejectVerdict(t *testing.T) {
	q := Question{Text: "Bad question?", Choices: fiveChoices(0)}
	mock := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		dst.(*reviewResponse).Verdict = "reject"
		dst.(*reviewResponse).Reason = "factually wrong"
		return nil
	}}
	g := New(mock)
	_, keep, err := g.reviewQuestion(context.Background(), "Go", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keep {
		t.Fatal("expected keep=false for reject verdict")
	}
}

func TestReviewQuestion_ImproveVerdict_WithValidQuestion(t *testing.T) {
	original := Question{Text: "Original?", Choices: fiveChoices(0)}
	improved := Question{Text: "Improved?", Choices: fiveChoices(2)}
	mock := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		r := dst.(*reviewResponse)
		r.Verdict = "improve"
		r.Reason = "typo fixed"
		r.Question = &improved
		return nil
	}}
	g := New(mock)
	got, keep, err := g.reviewQuestion(context.Background(), "Go", original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !keep {
		t.Fatal("expected keep=true for improve verdict")
	}
	if got.Text != improved.Text {
		t.Errorf("expected improved question text %q, got %q", improved.Text, got.Text)
	}
}

func TestReviewQuestion_ImproveVerdict_NilQuestion_FallsBackToOriginal(t *testing.T) {
	q := Question{Text: "Original?", Choices: fiveChoices(0)}
	mock := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		dst.(*reviewResponse).Verdict = "improve"
		dst.(*reviewResponse).Reason = "needs work"
		// Question intentionally nil
		return nil
	}}
	g := New(mock)
	got, keep, err := g.reviewQuestion(context.Background(), "Go", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !keep {
		t.Fatal("expected keep=true when improved question is nil")
	}
	if got.Text != q.Text {
		t.Errorf("expected original question text, got %q", got.Text)
	}
}

func TestReviewQuestion_ImproveVerdict_InvalidQuestion_FallsBackToOriginal(t *testing.T) {
	q := Question{Text: "Original?", Choices: fiveChoices(0)}
	bad := Question{Text: "Bad improved?", Choices: fiveChoices(0)[:3]} // only 3 choices — invalid
	mock := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		r := dst.(*reviewResponse)
		r.Verdict = "improve"
		r.Reason = "needs work"
		r.Question = &bad
		return nil
	}}
	g := New(mock)
	got, keep, err := g.reviewQuestion(context.Background(), "Go", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !keep {
		t.Fatal("expected keep=true when improved question fails validation")
	}
	if got.Text != q.Text {
		t.Errorf("expected original question text, got %q", got.Text)
	}
}

func TestReviewQuestion_AIError_KeepsOriginal(t *testing.T) {
	q := Question{Text: "When was Go released?", Choices: fiveChoices(1)}
	mock := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		return errTestFailure("gemini unavailable")
	}}
	g := New(mock)
	got, keep, err := g.reviewQuestion(context.Background(), "Go", q)
	if err == nil {
		t.Fatal("expected error from AI client, got nil")
	}
	if !keep {
		t.Fatal("expected keep=true on review error")
	}
	if got.Text != q.Text {
		t.Errorf("expected original question on error, got %q", got.Text)
	}
}

func TestFormatQuestionForReview_MarkesCorrectAnswer(t *testing.T) {
	q := Question{
		Text:    "What is the capital of France?",
		Choices: fiveChoices(2), // C is correct
	}
	formatted := formatQuestionForReview(q)
	if !strings.Contains(formatted, "C) C  ← marked as correct") {
		t.Errorf("expected correct answer marked with arrow, got:\n%s", formatted)
	}
	if strings.Contains(formatted, "A) A  ← marked as correct") {
		t.Errorf("unexpected correct marker on wrong choice:\n%s", formatted)
	}
}

func TestReviewQuestion_PromptContainsQuestionText(t *testing.T) {
	q := Question{Text: "What year did Go debut?", Choices: fiveChoices(0)}
	var capturedMessage string
	mock := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		if _, ok := dst.(*reviewResponse); ok {
			capturedMessage = message
		}
		dst.(*reviewResponse).Verdict = "accept"
		return nil
	}}
	g := New(mock)
	g.reviewQuestion(context.Background(), "Go", q) //nolint:errcheck
	if !strings.Contains(capturedMessage, "What year did Go debut?") {
		t.Errorf("expected question text in prompt, got:\n%s", capturedMessage)
	}
}

// errTestFailure is a simple error type for tests.
type errTestFailure string

func (e errTestFailure) Error() string { return string(e) }

// ensure mockAI satisfies aiClient (compile-time check)
var _ aiClient = (*mockAI)(nil)
var _ ai.Session = (*mockSession)(nil)
