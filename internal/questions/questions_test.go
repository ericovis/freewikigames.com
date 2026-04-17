package questions

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mockAI implements aiClient for tests.
type mockAI struct {
	fn func(ctx context.Context, prompt string, dst any) error
}

func (m *mockAI) GenerateJSON(ctx context.Context, prompt string, dst any) error {
	return m.fn(ctx, prompt, dst)
}

func fiveChoices(correctIdx int) []Choice {
	choices := make([]Choice, 5)
	for i := range choices {
		choices[i] = Choice{Text: string(rune('A' + i)), Correct: i == correctIdx}
	}
	return choices
}

func TestGenerator_Generate_ValidResponse(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		resp := dst.(*llmResponse)
		resp.Questions = []Question{
			{Text: "Q1", Choices: fiveChoices(0)},
			{Text: "Q2", Choices: fiveChoices(2)},
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.Generate(context.Background(), "<html><p>some article</p></html>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
}

func TestGenerator_Generate_SkipsInvalidChoiceCount(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		resp := dst.(*llmResponse)
		resp.Questions = []Question{
			{Text: "Bad Q", Choices: []Choice{{Text: "A", Correct: true}}},        // only 1 choice
			{Text: "Good Q", Choices: fiveChoices(1)},                             // valid
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.Generate(context.Background(), "<p>text</p>")
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

func TestGenerator_Generate_SkipsTwoCorrect(t *testing.T) {
	choices := fiveChoices(0)
	choices[1].Correct = true // now 2 correct

	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		resp := dst.(*llmResponse)
		resp.Questions = []Question{
			{Text: "Two correct", Choices: choices},
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.Generate(context.Background(), "<p>text</p>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 0 {
		t.Errorf("expected 0 valid questions, got %d", len(questions))
	}
}

func TestGenerator_Generate_SkipsZeroCorrect(t *testing.T) {
	choices := make([]Choice, 5)
	for i := range choices {
		choices[i] = Choice{Text: string(rune('A' + i)), Correct: false}
	}

	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		resp := dst.(*llmResponse)
		resp.Questions = []Question{
			{Text: "No correct", Choices: choices},
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.Generate(context.Background(), "<p>text</p>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 0 {
		t.Errorf("expected 0 valid questions, got %d", len(questions))
	}
}

func TestGenerator_Generate_PropagatesAIError(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		return errors.New("ollama unavailable")
	}}

	g := New(ai)
	_, err := g.Generate(context.Background(), "<p>text</p>")
	if err == nil {
		t.Fatal("expected error from AI client, got nil")
	}
}

func TestGenerator_GenerateWithLanguage_ValidResponse(t *testing.T) {
	var capturedPrompt string
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		capturedPrompt = prompt
		resp := dst.(*llmResponse)
		resp.Questions = []Question{
			{Text: "Q1", Choices: fiveChoices(0)},
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.GenerateWithLanguage(context.Background(), "<html><p>some article</p></html>", "pt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 1 {
		t.Errorf("expected 1 question, got %d", len(questions))
	}
	if !strings.Contains(capturedPrompt, `"pt"`) {
		t.Errorf("expected prompt to contain language code \"pt\", got: %s", capturedPrompt)
	}
}

func TestStripHTML(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"<p>Hello world</p>", "Hello world"},
		{"<div><span>foo</span></div>", "foo"},
		{"no tags here", "no tags here"},
		{"  extra   spaces  ", "extra spaces"},
	}
	for _, tc := range cases {
		got := stripHTML(tc.input)
		if got != tc.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tc.input, got, tc.want)
		}
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
