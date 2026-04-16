# Questions Package Developer Guide

This guide is for contributors working with or extending `internal/questions`.

## Overview

`internal/questions` generates multiple-choice trivia questions from raw Wikipedia HTML. It calls a local Ollama instance (via `internal/ai`) with a structured prompt, then parses and validates the JSON response. Only structurally valid questions — exactly 5 choices, exactly 1 correct — are returned to the caller.

This package has no database dependency. Persisting questions to PostgreSQL is the responsibility of callers (e.g. `cmd/worker`) using `internal/db`.

## File Responsibilities

| File               | Responsibility                                                |
|--------------------|---------------------------------------------------------------|
| `questions.go`     | `Choice`, `Question`, `Generator`, `Generate()`, `validate()`, `stripHTML()` |
| `questions_test.go`| Unit tests using a mock `aiClient`; no real network or DB     |
| `QUESTIONS.md`     | This developer guide                                          |

## Key Types

### `Question`
```go
type Question struct {
    Text    string   `json:"text"`
    Choices []Choice `json:"choices"`
}

type Choice struct {
    Text    string `json:"text"`
    Correct bool   `json:"correct"`
}
```

This mirrors the `db.Question` / `db.Choice` types but is decoupled from the database layer.

### `Generator`
```go
func New(ai aiClient) *Generator
func (g *Generator) Generate(ctx context.Context, rawHTML string) ([]Question, error)
```

## LLM Response Format

The prompt instructs the model to return **only** a JSON object matching this schema:

```json
{
  "questions": [
    {
      "text": "<question text>",
      "choices": [
        { "text": "<answer>", "correct": true },
        { "text": "<answer>", "correct": false },
        { "text": "<answer>", "correct": false },
        { "text": "<answer>", "correct": false },
        { "text": "<answer>", "correct": false }
      ]
    }
  ]
}
```

The `format: "json"` flag in the Ollama request instructs the model to output valid JSON. The `response` field from Ollama is then unmarshalled into `llmResponse`.

## Validation Rules

`validate(q Question) error` enforces:

1. **Exactly 5 choices** — questions with more or fewer are silently skipped.
2. **Exactly 1 correct choice** — questions with 0 or 2+ correct answers are silently skipped.

Invalid questions are dropped rather than returned as errors, because partial output from the LLM should not abort the entire generation run.

## HTML Stripping

Before building the prompt, `stripHTML` removes all HTML tags using a regexp and collapses whitespace. This keeps prompts compact and avoids sending thousands of bytes of markup to the model.

## Adding a New Prompt Strategy

1. Define a new prompt template as a package-level `const`.
2. Add a new method on `*Generator` (e.g. `GenerateFromTitle`) that builds the prompt differently.
3. Reuse `validate` for all strategies — the output contract is always the same.

## Testing Conventions

- **No real Ollama or database in tests.** Use the `mockAI` struct (defined in `questions_test.go`) which implements `aiClient`.
- Test the golden path (all valid questions), the skip paths (wrong choice count, 0 or 2+ correct), and the error path (AI client failure).
- Test `stripHTML` and `validate` as pure functions with table-driven cases.

## Anti-Patterns

- **Don't import `internal/db` from this package.** Question types are defined here; mapping to DB types is the caller's job.
- **Don't swallow AI errors.** If `GenerateJSON` returns an error, propagate it — don't return an empty slice silently.
- **Don't return invalid questions.** Always run `validate` before adding to the result set.
- **Don't put prompt logic in `internal/ai`.** The AI package is a dumb transport; prompts belong here.
