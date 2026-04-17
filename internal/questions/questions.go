package questions

import (
	"context"
	"fmt"
)

// Choice is a single answer option for a question.
type Choice struct {
	Text    string `json:"text"`
	Correct bool   `json:"correct"`
}

// Question is a generated multiple-choice question with exactly 5 choices,
// one of which is correct.
type Question struct {
	Text    string   `json:"text"`
	Choices []Choice `json:"choices"`
}

// llmResponse is the JSON envelope the LLM must return.
type llmResponse struct {
	Questions []Question `json:"questions"`
}

// aiClient is the interface the Generator uses to call the AI backend.
// *ai.Client satisfies this interface.
type aiClient interface {
	GenerateJSON(ctx context.Context, prompt string, dst any) error
}

// Generator generates trivia questions from structured Wikipedia article data.
type Generator struct {
	ai aiClient
}

// New returns a Generator backed by the given AI client.
func New(ai aiClient) *Generator {
	return &Generator{ai: ai}
}

const promptTemplate = `You are a trivia question generator. Given the following Wikipedia article, generate as many multiple-choice trivia questions as you can.

Rules:
- Each question must have exactly 5 answer choices.
- Exactly 1 choice must be correct (set "correct": true); the other 4 must be incorrect (set "correct": false).
- Generate all questions in the "%s" language (ISO 639-1 code).
- Return ONLY a JSON object with this exact structure, no other text:
{"questions": [{"text": "<question text>", "choices": [{"text": "<answer>", "correct": <true|false>}, ...]}, ...]}

Title: %s

Summary:
%s

Article:
%s`

// GenerateWithLanguage generates trivia questions from structured article data.
// language is an ISO 639-1 code (e.g. "en", "pt", "de").
func (g *Generator) GenerateWithLanguage(ctx context.Context, title, language, summary, content string) ([]Question, error) {
	prompt := fmt.Sprintf(promptTemplate, language, title, summary, content)

	var resp llmResponse
	if err := g.ai.GenerateJSON(ctx, prompt, &resp); err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	var valid []Question
	for _, q := range resp.Questions {
		if err := validate(q); err != nil {
			continue
		}
		valid = append(valid, q)
	}
	return valid, nil
}

// validate checks that q has exactly 5 choices with exactly 1 marked correct.
func validate(q Question) error {
	if len(q.Choices) != 5 {
		return fmt.Errorf("question has %d choices, want 5", len(q.Choices))
	}
	correct := 0
	for _, c := range q.Choices {
		if c.Correct {
			correct++
		}
	}
	if correct != 1 {
		return fmt.Errorf("question has %d correct choices, want exactly 1", correct)
	}
	return nil
}
