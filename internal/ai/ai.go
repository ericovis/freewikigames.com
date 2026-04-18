package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Session is a multi-turn conversation that returns structured JSON responses.
// *Chat implements Session; this interface is exported so callers can mock it.
type Session interface {
	Send(ctx context.Context, message string, dst any) error
}

// Client wraps Google Generative AI for structured JSON chat conversations.
// Use New to construct one; pass GEMINI_API_KEY and model name as arguments.
type Client struct {
	inner     *genai.Client
	modelName string
}

// New returns a Client backed by the given Google AI model and API key.
// The caller must call Close when done to release the underlying connection.
func New(ctx context.Context, apiKey, model string) (*Client, error) {
	inner, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}
	return &Client{inner: inner, modelName: model}, nil
}

// Close releases the underlying gRPC/HTTP connection.
func (c *Client) Close() {
	c.inner.Close()
}

// geminiSession is the subset of genai.ChatSession used by Chat.
// Extracted as an interface so tests can substitute a mock without HTTP.
type geminiSession interface {
	SendMessage(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error)
}

// Chat is a stateful multi-turn conversation that returns JSON on every turn.
type Chat struct {
	session    geminiSession
	retryDelay time.Duration // base delay for 429 backoff; overridden in tests
}

// NewChat starts a new JSON conversation with the given system instruction.
// The model is instructed to return JSON on every turn.
func (c *Client) NewChat(systemPrompt string) Session {
	model := c.inner.GenerativeModel(c.modelName)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	model.ResponseMIMEType = "application/json"
	return &Chat{session: model.StartChat(), retryDelay: retryBaseDelay}
}

const (
	maxSendRetries = 4
	retryBaseDelay = 2 * time.Second
)

// Send sends a user message in the conversation and unmarshals the JSON
// response into dst (must be a pointer). On HTTP 429 it retries up to
// maxSendRetries times with exponential backoff before giving up.
func (ch *Chat) Send(ctx context.Context, message string, dst any) error {
	for attempt := 0; attempt <= maxSendRetries; attempt++ {
		err := ch.send(ctx, message, dst)
		if err == nil {
			return nil
		}
		var apiErr *googleapi.Error
		if errors.As(err, &apiErr) && apiErr.Code == 429 && attempt < maxSendRetries {
			delay := ch.retryDelay * time.Duration(1<<attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		return err
	}
	return fmt.Errorf("send: max retries exceeded")
}

func (ch *Chat) send(ctx context.Context, message string, dst any) error {
	resp, err := ch.session.SendMessage(ctx, genai.Text(message))
	if err != nil {
		return fmt.Errorf("gemini send: %w", err)
	}
	if len(resp.Candidates) == 0 {
		return fmt.Errorf("gemini returned no candidates")
	}
	cand := resp.Candidates[0]
	if cand.Content == nil || len(cand.Content.Parts) == 0 {
		return fmt.Errorf("gemini candidate has no content")
	}
	text, ok := cand.Content.Parts[0].(genai.Text)
	if !ok {
		return fmt.Errorf("gemini response part is not text")
	}
	if err := json.Unmarshal([]byte(text), dst); err != nil {
		return fmt.Errorf("unmarshal gemini response: %w", err)
	}
	return nil
}
