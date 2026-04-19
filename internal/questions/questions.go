package questions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ericovis/freewikigames.com/internal/ai"
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
	NewChat(systemPrompt string) ai.Session
}

// Generator generates trivia questions from raw Wikipedia article data.
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

// systemPromptTemplate is the system instruction sent to the model at session start.
// Parameter: language (ISO 639-1 code).
const systemPromptTemplate = `You are a trivia writer for a Wikipedia-based quiz game.
Generate multiple-choice trivia questions from the Wikipedia article content provided.

Rules:
- Each question must have exactly 5 answer choices.
- Exactly 1 choice must be marked correct ("correct": true); the other 4 must be incorrect ("correct": false).
- Write questions in %s (ISO 639-1 language code).
- Questions must be about specific, verifiable facts from the article.
- Return ONLY a JSON object in this exact format:
{
  "questions": [
    {
      "text": "question text",
      "choices": [
        {"text": "answer", "correct": true},
        {"text": "answer", "correct": false},
        {"text": "answer", "correct": false},
        {"text": "answer", "correct": false},
        {"text": "answer", "correct": false}
      ]
    }
  ]
}`

// maxValidationRetries is the number of follow-up turns allowed to fix
// structurally invalid questions before giving up and keeping only the valid ones.
const maxValidationRetries = 3

// GenerateWithLanguage generates trivia questions from a full Wikipedia article.
// It sends the entire content in one request, validates the output, and follows
// up in the same session if any questions fail structural validation.
// language is an ISO 639-1 code (e.g. "en", "pt", "de").
func (g *Generator) GenerateWithLanguage(ctx context.Context, title, language, content string) ([]Question, error) {
	session := g.ai.NewChat(fmt.Sprintf(systemPromptTemplate, language))

	userMessage := fmt.Sprintf("Article title: %s\n\nContent:\n%s\n\nGenerate as many trivia questions as possible from this article.", title, content)

	var resp llmResponse
	if err := session.Send(ctx, userMessage, &resp); err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	// Follow up if any questions are structurally invalid.
	for retry := 0; retry < maxValidationRetries && hasInvalid(resp.Questions); retry++ {
		if ctx.Err() != nil {
			break
		}
		followUp := buildFollowUp(resp.Questions)
		var fixed llmResponse
		if err := session.Send(ctx, followUp, &fixed); err != nil {
			g.logger.Warn("validation follow-up failed", "title", title, "retry", retry+1, "err", err)
			break
		}
		resp = fixed
		g.logger.Debug("follow-up response", "title", title, "retry", retry+1, "raw", len(resp.Questions))
	}

	var valid []Question
	for _, q := range resp.Questions {
		if err := validate(q); err != nil {
			g.logger.Debug("question discarded", "title", title, "reason", err)
			continue
		}
		valid = append(valid, q)
	}

	g.logger.Info("questions generated", "title", title, "valid", len(valid))
	return valid, nil
}

// hasInvalid reports whether any question in qs fails structural validation.
func hasInvalid(qs []Question) bool {
	for _, q := range qs {
		if validate(q) != nil {
			return true
		}
	}
	return false
}

// buildFollowUp constructs a follow-up message describing validation failures
// so the model can fix them within the same session.
func buildFollowUp(qs []Question) string {
	var sb strings.Builder
	sb.WriteString("Some questions in your response have validation issues. Please fix them and return ALL questions (both valid and fixed) in the same JSON format:\n\n")
	for _, q := range qs {
		if err := validate(q); err != nil {
			fmt.Fprintf(&sb, "- Question %q: %s\n", q.Text, err)
		}
	}
	return sb.String()
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
