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

func (m *mockAI) GenerateJSONSchema(ctx context.Context, prompt string, schema any, dst any) error {
	return m.fn(ctx, prompt, dst)
}

func fiveChoices(correctIdx int) []Choice {
	choices := make([]Choice, 5)
	for i := range choices {
		choices[i] = Choice{Text: string(rune('A' + i)), Correct: i == correctIdx}
	}
	return choices
}

func TestGenerator_GenerateWithLanguage_ValidResponse(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		switch d := dst.(type) {
		case *llmResponse:
			d.Questions = []Question{
				{Text: "Q1", Choices: fiveChoices(0)},
				{Text: "Q2", Choices: fiveChoices(2)},
			}
		case *reviewResponse:
			d.Verdict = "accept"
		}
		return nil
	}}

	g := New(ai)
	questions, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "## Overview\nGo is open source.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
}

func TestGenerator_GenerateWithLanguage_SkipsInvalidChoiceCount(t *testing.T) {
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		switch d := dst.(type) {
		case *llmResponse:
			d.Questions = []Question{
				{Text: "Bad Q", Choices: []Choice{{Text: "A", Correct: true}}},
				{Text: "Good Q", Choices: fiveChoices(1)},
			}
		case *reviewResponse:
			d.Verdict = "accept"
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

	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		resp := dst.(*llmResponse)
		resp.Questions = []Question{
			{Text: "Two correct", Choices: choices},
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

	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		resp := dst.(*llmResponse)
		resp.Questions = []Question{
			{Text: "No correct", Choices: choices},
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
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		return errors.New("ollama unavailable")
	}}

	g := New(ai)
	_, err := g.GenerateWithLanguage(context.Background(), "Go", "en", "content")
	if err == nil {
		t.Fatal("expected error from AI client, got nil")
	}
}

func TestGenerator_GenerateWithLanguage_PromptContainsStructuredFields(t *testing.T) {
	var capturedPrompt string
	ai := &mockAI{fn: func(ctx context.Context, prompt string, dst any) error {
		capturedPrompt = prompt
		return nil
	}}

	g := New(ai)
	g.GenerateWithLanguage(context.Background(), "Go (programming language)", "pt", "## Overview\nGo is open source.")

	for _, want := range []string{"pt", "Subject: Go (programming language)", "## Overview"} {
		if !strings.Contains(capturedPrompt, want) {
			t.Errorf("prompt missing %q\nfull prompt:\n%s", want, capturedPrompt)
		}
	}
}

func TestSplitSections_MultipleHeadings(t *testing.T) {
	content := "## History\nGo was designed at Google in 2007 by Robert Griesemer, Rob Pike, and Ken Thompson to improve programming productivity.\n\n## Features\nGo includes garbage collection, limited structural typing, memory safety, and CSP-style concurrent programming features."
	chunks := splitSections(content)
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if !strings.HasPrefix(chunks[0], "## History") {
		t.Errorf("expected first chunk to start with '## History', got %q", chunks[0])
	}
	if !strings.HasPrefix(chunks[1], "## Features") {
		t.Errorf("expected second chunk to start with '## Features', got %q", chunks[1])
	}
}

func TestSplitSections_NoHeadingsFallback(t *testing.T) {
	content := "Just plain content without any section headings."
	chunks := splitSections(content)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk (fallback), got %d", len(chunks))
	}
	if chunks[0] != content {
		t.Errorf("expected chunk to equal content, got %q", chunks[0])
	}
}

func TestSplitSections_SkipsShortChunks(t *testing.T) {
	// "## Tiny" alone is < 80 chars and has no body — should be skipped
	content := "## Tiny\n\n## Real section\nThis section has enough text to be included as a valid chunk in the output."
	chunks := splitSections(content)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk (short section skipped), got %d: %v", len(chunks), chunks)
	}
	if !strings.HasPrefix(chunks[0], "## Real section") {
		t.Errorf("expected surviving chunk to be '## Real section', got %q", chunks[0])
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
