package questions

import (
	"context"
	"fmt"
	"strings"
)

// reviewResponse is the JSON envelope the LLM returns when evaluating a question.
type reviewResponse struct {
	// Verdict is one of "accept", "improve", or "reject".
	Verdict string `json:"verdict"`
	// Reason explains the ruling (used for debug logging).
	Reason string `json:"reason"`
	// Question is the revised question returned when Verdict == "improve".
	// It is absent for "accept" and "reject".
	Question *Question `json:"question,omitempty"`
}

// reviewSystemPrompt is the system instruction for quality review sessions.
const reviewSystemPrompt = `You are a trivia quality reviewer. A question has been automatically generated and one choice has been marked as correct.

Your tasks:
1. Verify that the marked correct answer is factually accurate.
2. Check that the question text is clear and unambiguous.
3. Check that the four incorrect choices are plausible but clearly distinguishable from the correct answer.

Return one of three verdicts:
- "accept": the question is good as-is.
- "improve": the question has fixable issues; return the corrected version in the "question" field with all 5 choices and exactly 1 marked correct.
- "reject": the question is fundamentally flawed (factually wrong correct answer, genuinely ambiguous, or no clear correct answer exists).

Respond with JSON only:
{
  "verdict": "accept" | "improve" | "reject",
  "reason": "brief explanation",
  "question": { "text": "...", "choices": [...] }
}`

// reviewQuestion asks the LLM to evaluate q for factual accuracy and quality.
// It returns the (possibly improved) question and true if the question should be
// kept, or the zero Question and false if it should be rejected.
// On LLM error the original question is returned with kept=true so a transient
// failure does not silently drop content.
func (g *Generator) reviewQuestion(ctx context.Context, title string, q Question) (Question, bool, error) {
	session := g.ai.NewChat(reviewSystemPrompt)
	prompt := formatQuestionForReview(q)

	var resp reviewResponse
	if err := session.Send(ctx, prompt, &resp); err != nil {
		return q, true, fmt.Errorf("review question: %w", err)
	}

	g.logger.Debug("question review", "title", title, "question", q.Text, "verdict", resp.Verdict, "reason", resp.Reason)

	switch resp.Verdict {
	case "reject":
		return Question{}, false, nil
	case "improve":
		if resp.Question == nil {
			return q, true, nil
		}
		improved := *resp.Question
		if err := validate(improved); err != nil {
			g.logger.Debug("improved question failed validation, keeping original", "title", title, "reason", err)
			return q, true, nil
		}
		return improved, true, nil
	default: // "accept" or any unrecognised value
		return q, true, nil
	}
}

// formatQuestionForReview renders q as a human-readable block with the correct
// answer labelled, suitable for inclusion in the review prompt.
func formatQuestionForReview(q Question) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Question: %s\n\nChoices:\n", q.Text)
	for i, c := range q.Choices {
		label := string(rune('A' + i))
		if c.Correct {
			fmt.Fprintf(&sb, "%s) %s  ← marked as correct\n", label, c.Text)
		} else {
			fmt.Fprintf(&sb, "%s) %s\n", label, c.Text)
		}
	}
	return sb.String()
}
