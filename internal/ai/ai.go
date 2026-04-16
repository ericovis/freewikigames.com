package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is a thin HTTP wrapper around the Ollama /api/generate endpoint.
// Use New to construct one; pass OLLAMA_HOST and OLLAMA_MODEL as arguments.
type Client struct {
	host   string
	model  string
	client *http.Client
}

// New returns an AI Client targeting the given Ollama host with the given model.
func New(host, model string) *Client {
	return &Client{
		host:   host,
		model:  model,
		client: &http.Client{},
	}
}

// generateRequest is the body sent to Ollama.
type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format"`
}

// generateResponse is the shape of a completed (non-streamed) Ollama reply.
type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// GenerateJSON sends a prompt to Ollama with format:"json", reads the
// completed response, and unmarshals the JSON text into dst (must be a pointer).
func (c *Client) GenerateJSON(ctx context.Context, prompt string, dst any) error {
	reqBody := generateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/generate", bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, body)
	}

	var genResp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return fmt.Errorf("decode ollama response: %w", err)
	}

	if err := json.Unmarshal([]byte(genResp.Response), dst); err != nil {
		return fmt.Errorf("unmarshal model output: %w", err)
	}

	return nil
}
