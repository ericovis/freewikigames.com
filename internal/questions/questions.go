package questions

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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

// Generator generates trivia questions from raw Wikipedia HTML.
type Generator struct {
	ai aiClient
}

// New returns a Generator backed by the given AI client.
func New(ai aiClient) *Generator {
	return &Generator{ai: ai}
}

var tagRegexp = regexp.MustCompile(`<[^>]+>`)

// stripHTML removes HTML tags and collapses whitespace so the prompt sent to
// the LLM is compact plain text.
func stripHTML(html string) string {
	plain := tagRegexp.ReplaceAllString(html, " ")
	// Collapse runs of whitespace into a single space and trim.
	fields := strings.Fields(plain)
	return strings.Join(fields, " ")
}

const promptTemplate = `You are a trivia question generator. Given the following Wikipedia article text, generate as many multiple-choice trivia questions as you can.

Rules:
- Each question must have exactly 5 answer choices.
- Exactly 1 choice must be correct (set "correct": true); the other 4 must be incorrect (set "correct": false).
- Return ONLY a JSON object with this exact structure, no other text:
{"questions": [{"text": "<question text>", "choices": [{"text": "<answer>", "correct": <true|false>}, ...]}, ...]}

Article text:
%s`

// Generate sends the stripped article HTML to the AI and returns all questions
// that pass structural validation. Questions with the wrong number of choices
// or an invalid number of correct answers are silently skipped.
// Returns an error only if the AI call itself fails or the response cannot be
// parsed at all.
func (g *Generator) Generate(ctx context.Context, rawHTML string) ([]Question, error) {
	text := stripHTML(rawHTML)
	prompt := fmt.Sprintf(promptTemplate, text)

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

const promptTemplateWithLanguage = `You are a trivia question generator. Given the following Wikipedia article text, generate as many multiple-choice trivia questions as you can.

Rules:
- Each question must have exactly 5 answer choices.
- Exactly 1 choice must be correct (set "correct": true); the other 4 must be incorrect (set "correct": false).
- Generate all questions in the "%s" language (ISO 639-1 code).
- Return ONLY a JSON object with this exact structure, no other text:
{"questions": [{"text": "<question text>", "choices": [{"text": "<answer>", "correct": <true|false>}, ...]}, ...]}

Article text:
%s`

// GenerateWithLanguage is like Generate but instructs the LLM to produce
// questions in the specified language (ISO 639-1 code, e.g. "en", "pt", "de").
func (g *Generator) GenerateWithLanguage(ctx context.Context, rawHTML, language string) ([]Question, error) {
	text := stripHTML(rawHTML)
	prompt := fmt.Sprintf(promptTemplateWithLanguage, language, text)

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
