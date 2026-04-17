package questions

import (
	"context"
	"fmt"
	"log/slog"
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
	GenerateJSONSchema(ctx context.Context, prompt string, schema any, dst any) error
}

// responseSchema is the JSON Schema passed to Ollama as the format constraint.
// It forces the model to produce a valid questions array with exactly 5 choices
// per question, each having text and correct fields.
var responseSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"questions": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{"type": "string"},
					"choices": map[string]any{
						"type":     "array",
						"minItems": 5,
						"maxItems": 5,
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"text":    map[string]any{"type": "string"},
								"correct": map[string]any{"type": "boolean"},
							},
							"required": []string{"text", "correct"},
						},
					},
				},
				"required": []string{"text", "choices"},
			},
		},
	},
	"required": []string{"questions"},
}

// Generator generates trivia questions from structured Wikipedia article data.
type Generator struct {
	ai     aiClient
	logger *slog.Logger
}

// New returns a Generator backed by the given AI client.
// An optional logger can be provided; if omitted, slog.Default() is used.
func New(ai aiClient, loggers ...*slog.Logger) *Generator {
	logger := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return &Generator{ai: ai, logger: logger}
}

// sectionPromptTemplate is the per-section prompt sent to the LLM.
// Parameters: language, title, summary, section content.
const sectionPromptTemplate = `You are a trivia writer. Read the Wikipedia section below and write 1 to 2 multiple-choice questions about specific facts in it.

Rules:
- Each question must have exactly 5 answer choices.
- Exactly 1 choice must be correct ("correct": true); the other 4 must be incorrect ("correct": false).
- Write questions in %s (ISO 639-1 language code).

Article: %s
Summary: %s

Section:
%s`

// GenerateWithLanguage generates trivia questions from structured article data
// by splitting the content into sections and querying the LLM once per section.
// language is an ISO 639-1 code (e.g. "en", "pt", "de").
func (g *Generator) GenerateWithLanguage(ctx context.Context, title, language, summary, content string) ([]Question, error) {
	chunks := splitSections(content)
	g.logger.Info("generating questions", "title", title, "sections", len(chunks))

	var allQuestions []Question
	var lastErr error

	for i, chunk := range chunks {
		if ctx.Err() != nil {
			break
		}
		qs, err := g.generateForSection(ctx, title, language, summary, chunk)
		if err != nil {
			g.logger.Warn("section generation failed", "title", title, "section", i+1, "err", err)
			lastErr = err
			continue
		}
		g.logger.Debug("section done", "title", title, "section", i+1, "valid", len(qs))
		allQuestions = append(allQuestions, qs...)
	}

	g.logger.Info("questions generated", "title", title, "total", len(allQuestions))

	if len(allQuestions) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return allQuestions, nil
}

func (g *Generator) generateForSection(ctx context.Context, title, language, summary, chunk string) ([]Question, error) {
	prompt := fmt.Sprintf(sectionPromptTemplate, language, title, summary, chunk)

	var resp llmResponse
	if err := g.ai.GenerateJSONSchema(ctx, prompt, responseSchema, &resp); err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	g.logger.Debug("llm raw response", "title", title, "questions_raw", len(resp.Questions))

	var valid []Question
	for _, q := range resp.Questions {
		if err := validate(q); err != nil {
			g.logger.Debug("question discarded", "title", title, "reason", err)
			continue
		}
		valid = append(valid, q)
	}
	g.logger.Debug("questions validated", "title", title, "valid", len(valid), "raw", len(resp.Questions))
	return valid, nil
}

// boilerplateSections is a set of lowercased section titles that never contain
// trivia-worthy facts and should be skipped during question generation.
var boilerplateSections = map[string]bool{
	// English
	"see also": true, "references": true, "external links": true,
	"notes": true, "bibliography": true, "further reading": true,
	"footnotes": true, "sources": true, "citations": true,
	// Portuguese
	"ver também": true, "referências": true, "ligações externas": true,
	"notas": true, "bibliografia": true, "leitura adicional": true,
	"leitura complementar": true, "notas de rodapé": true, "fontes": true,
}

// splitSections splits markdown content into chunks on ## and ### headings.
// Content before the first heading is included as a chunk if long enough.
// Sections whose titles appear in boilerplateSections are skipped entirely.
// Chunks shorter than minChunkLen characters are skipped (just a heading, no body).
// If no sections are found, the entire content is returned as a single chunk.
func splitSections(content string) []string {
	const minChunkLen = 80
	lines := strings.Split(content, "\n")
	var chunks []string
	var current []string
	skipCurrent := false

	flush := func() {
		if !skipCurrent {
			chunk := strings.TrimSpace(strings.Join(current, "\n"))
			if len(chunk) >= minChunkLen {
				chunks = append(chunks, chunk)
			}
		}
		current = current[:0]
		skipCurrent = false
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			flush()
			title := strings.ToLower(strings.TrimLeft(line, "# "))
			skipCurrent = boilerplateSections[title]
		}
		current = append(current, line)
	}
	flush()

	if len(chunks) == 0 {
		if trimmed := strings.TrimSpace(content); len(trimmed) > 0 {
			return []string{trimmed}
		}
	}
	return chunks
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
