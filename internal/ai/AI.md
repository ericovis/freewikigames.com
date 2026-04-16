# AI Package Developer Guide

This guide is for contributors working with or extending `internal/ai`.

## Overview

`internal/ai` is a thin HTTP wrapper around the [Ollama](https://ollama.com) `/api/generate` endpoint. Its only job is to send a prompt to a running Ollama instance, request JSON output, and unmarshal the model's response into a caller-supplied Go value. No prompt engineering, retries, or business logic lives here.

## File Responsibilities

| File       | Responsibility                                          |
|------------|---------------------------------------------------------|
| `ai.go`    | `Client` struct, `New()`, `GenerateJSON()`              |
| `ai_test.go` | Unit tests using `httptest.NewServer`                 |

## Key Types

### `Client`
```go
type Client struct { /* unexported */ }

func New(host, model string) *Client
func (c *Client) GenerateJSON(ctx context.Context, prompt string, dst any) error
```

- `host` — full base URL of the Ollama server (e.g. `http://localhost:11434`).
- `model` — model tag to pass in every request (e.g. `gemma4:e2b`).

Callers read `OLLAMA_HOST` and `OLLAMA_MODEL` from the environment and pass them to `New`. The package itself never reads environment variables.

## `GenerateJSON`

Sends a single non-streaming request to Ollama:

**Request body:**
```json
{
  "model":  "<model>",
  "prompt": "<caller-supplied prompt>",
  "stream": false,
  "format": "json"
}
```

**Response shape (Ollama):**
```json
{ "response": "<JSON string produced by the model>", "done": true }
```

The `response` field is then `json.Unmarshal`'d into `dst`. If the model produces invalid JSON, or if Ollama returns a non-200 status, `GenerateJSON` returns a descriptive error.

## Environment Variables

| Variable      | Purpose                          | Default (cmd/test) |
|---------------|----------------------------------|--------------------|
| `OLLAMA_HOST` | Base URL of the Ollama server    | `http://localhost:11434` |
| `OLLAMA_MODEL`| Model tag                        | `gemma4:e2b` |

These are read by binaries (`cmd/worker`, etc.) and passed to `New`; they are not accessed inside this package.

## Testing Conventions

- Use `httptest.NewServer` to stub Ollama — never call a real Ollama instance in tests.
- Pass `srv.URL` as the `host` argument to `New`.
- Test the four key paths: successful generation, non-200 HTTP status, malformed outer JSON, malformed inner (model-produced) JSON.

## Anti-Patterns

- **Don't add third-party HTTP libraries.** `net/http` is sufficient.
- **Don't enable streaming.** All callers expect a complete, single response. `stream` is always `false`.
- **Don't put prompt logic here.** Prompts belong in the package that knows the domain (e.g. `internal/questions`).
- **Don't read env vars inside this package.** Accept `host` and `model` as constructor arguments so callers control configuration.
