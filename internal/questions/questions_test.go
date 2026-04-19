package questions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ericovis/freewikigames.com/internal/ai"
)

// mockSession implements ai.Session for tests.
type mockSession struct {
	fn func(ctx context.Context, message string, dst any) error
}

func (m *mockSession) Send(ctx context.Context, message string, dst any) error {
	return m.fn(ctx, message, dst)
}

// mockAI implements aiClient for tests.
// Every NewChat call returns a session backed by the same fn.
type mockAI struct {
	fn func(ctx context.Context, message string, dst any) error
}

func (m *mockAI) NewChat(systemPrompt string) ai.Session {
	return &mockSession{fn: m.fn}
}

func fiveChoices(correctIdx int) []Choice {
	choices := make([]Choice, 5)
	for i := range choices {
		choices[i] = Choice{Text: string(rune('A' + i)), Correct: i == correctIdx}
	}
	return choices
}

func TestGenerator_GenerateWithLanguage_ValidResponse(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		if d, ok := dst.(*llmResponse); ok {
			d.Questions = []Question{
				{Text: "Q1", Choices: fiveChoices(0)},
				{Text: "Q2", Choices: fiveChoices(2)},
			}
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "Go is open source.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
}

func TestGenerator_GenerateWithLanguage_SkipsInvalidChoiceCount(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		if d, ok := dst.(*llmResponse); ok {
			d.Questions = []Question{
				{Text: "Bad Q", Choices: []Choice{{Text: "A", Correct: true}}},
				{Text: "Good Q", Choices: fiveChoices(1)},
			}
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 1 {
		t.Errorf("expected 1 valid question, got %d", len(questions))
	}
	if questions[0].Text != "Good Q" {
		t.Errorf("expected 'Good Q', got %q", questions[0].Text)
	}
}

func TestGenerator_GenerateWithLanguage_SkipsTwoCorrect(t *testing.T) {
	choices := fiveChoices(0)
	choices[1].Correct = true

	ai := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		if resp, ok := dst.(*llmResponse); ok {
			resp.Questions = []Question{
				{Text: "Two correct", Choices: choices},
			}
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 0 {
		t.Errorf("expected 0 valid questions, got %d", len(questions))
	}
}

func TestGenerator_GenerateWithLanguage_SkipsZeroCorrect(t *testing.T) {
	choices := make([]Choice, 5)
	for i := range choices {
		choices[i] = Choice{Text: string(rune('A' + i)), Correct: false}
	}

	ai := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		if resp, ok := dst.(*llmResponse); ok {
			resp.Questions = []Question{
				{Text: "No correct", Choices: choices},
			}
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 0 {
		t.Errorf("expected 0 valid questions, got %d", len(questions))
	}
}

func TestGenerator_GenerateWithLanguage_PropagatesAIError(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		return errors.New("gemini unavailable")
	}}

	g := New(ai)
	_, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "content")
	if err == nil {
		t.Fatal("expected error from AI client, got nil")
	}
}

func TestGenerator_GenerateWithLanguage_PromptContainsArticleFields(t *testing.T) {
	var capturedMessage string
	ai := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		if _, ok := dst.(*llmResponse); ok && capturedMessage == "" {
			capturedMessage = message
		}
		return nil
	}}

	g := New(ai)
	g.GenerateWithLanguage(context.Background(), "Go (programming language)", "pt", "Go is open source and fast.")

	for _, want := range []string{"Go (programming language)", "Go is open source"} {
		if !strings.Contains(capturedMessage, want) {
			t.Errorf("prompt missing %q\nfull message:\n%s", want, capturedMessage)
		}
	}
}

func TestGenerator_GenerateWithLanguage_SystemPromptContainsLanguage(t *testing.T) {
	var capturedSystem string
	mockClient := &captureSystemMockAI{
		fn: func(ctx context.Context, message string, dst any) error {
			if resp, ok := dst.(*llmResponse); ok {
				resp.Questions = []Question{{Text: "Q", Choices: fiveChoices(0)}}
			}
			return nil
		},
		onNewChat: func(systemPrompt string) {
			if capturedSystem == "" {
				capturedSystem = systemPrompt
			}
		},
	}

	g := New(mockClient)
	g.GenerateWithLanguage(context.Background(), "Go", "pt", "content")

	if !strings.Contains(capturedSystem, "pt") {
		t.Errorf("system prompt missing language 'pt', got:\n%s", capturedSystem)
	}
}

// captureSystemMockAI captures the system prompt passed to NewChat.
type captureSystemMockAI struct {
	fn        func(ctx context.Context, message string, dst any) error
	onNewChat func(systemPrompt string)
}

func (m *captureSystemMockAI) NewChat(systemPrompt string) ai.Session {
	if m.onNewChat != nil {
		m.onNewChat(systemPrompt)
	}
	return &mockSession{fn: m.fn}
}

func TestGenerator_GenerateWithLanguage_FollowsUpOnInvalidQuestions(t *testing.T) {
	callCount := 0
	ai := &mockAI{fn: func(ctx context.Context, message string, dst any) error {
		if d, ok := dst.(*llmResponse); ok {
			callCount++
			if callCount == 1 {
				d.Questions = []Question{
					{Text: "Bad Q", Choices: fiveChoices(0)[:3]},
				}
			} else {
				d.Questions = []Question{
					{Text: "Fixed Q", Choices: fiveChoices(0)},
				}
			}
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 1 || questions[0].Text != "Fixed Q" {
		t.Errorf("expected 1 fixed question, got %v", questions)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 LLM calls (initial + follow-up), got %d", callCount)
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		if err := validate(Question{Text: "Q", Choices: fiveChoices(0)}); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
	t.Run("too few choices", func(t *testing.T) {
		if err := validate(Question{Text: "Q", Choices: fiveChoices(0)[:3]}); err == nil {
			t.Error("expected error for too few choices")
		}
	})
	t.Run("too many choices", func(t *testing.T) {
		choices := append(fiveChoices(0), Choice{Text: "F"})
		if err := validate(Question{Text: "Q", Choices: choices}); err == nil {
			t.Error("expected error for too many choices")
		}
	})
	t.Run("no correct", func(t *testing.T) {
		choices := make([]Choice, 5)
		if err := validate(Question{Text: "Q", Choices: choices}); err == nil {
			t.Error("expected error for zero correct choices")
		}
	})
	t.Run("two correct", func(t *testing.T) {
		choices := fiveChoices(0)
		choices[1].Correct = true
		if err := validate(Question{Text: "Q", Choices: choices}); err == nil {
			t.Error("expected error for two correct choices")
		}
	})
}
