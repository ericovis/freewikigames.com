package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/googleapi"
)

// mockGeminiSession implements geminiSession for tests, bypassing the real API.
type mockGeminiSession struct {
	candidates []*genai.Candidate
	err        error
}

func (m *mockGeminiSession) SendMessage(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &genai.GenerateContentResponse{Candidates: m.candidates}, nil
}

func textCandidate(text string) []*genai.Candidate {
	return []*genai.Candidate{{
		Content: &genai.Content{
			Parts: []genai.Part{genai.Text(text)},
			Role:  "model",
		},
	}}
}

type testOutput struct {
	Value string `json:"value"`
}

func TestChat_Send_OK(t *testing.T) {
	chat := &Chat{session: &mockGeminiSession{candidates: textCandidate(`{"value":"hello"}`)}}
	var out testOutput
	if err := chat.Send(context.Background(), "prompt", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value != "hello" {
		t.Errorf("expected value 'hello', got %q", out.Value)
	}
}

func TestChat_Send_APIError(t *testing.T) {
	chat := &Chat{session: &mockGeminiSession{err: errors.New("api error")}}
	var out testOutput
	if err := chat.Send(context.Background(), "prompt", &out); err == nil {
		t.Fatal("expected error for API failure, got nil")
	}
}

func TestChat_Send_NoCandidates(t *testing.T) {
	chat := &Chat{session: &mockGeminiSession{candidates: nil}}
	var out testOutput
	if err := chat.Send(context.Background(), "prompt", &out); err == nil {
		t.Fatal("expected error for empty candidates, got nil")
	}
}

func TestChat_Send_EmptyContent(t *testing.T) {
	chat := &Chat{session: &mockGeminiSession{
		candidates: []*genai.Candidate{{Content: nil}},
	}}
	var out testOutput
	if err := chat.Send(context.Background(), "prompt", &out); err == nil {
		t.Fatal("expected error for nil content, got nil")
	}
}

func TestChat_Send_MalformedJSON(t *testing.T) {
	chat := &Chat{session: &mockGeminiSession{candidates: textCandidate("not-json")}}
	var out testOutput
	if err := chat.Send(context.Background(), "prompt", &out); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// callbackSession calls fn on each SendMessage, passing the attempt index.
type callbackSession struct {
	attempt int
	fn      func(attempt int) (*genai.GenerateContentResponse, error)
}

func (s *callbackSession) SendMessage(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
	resp, err := s.fn(s.attempt)
	s.attempt++
	return resp, err
}

func TestChat_Send_RetriesOn429(t *testing.T) {
	attempts := 0
	chat := &Chat{retryDelay: 0, session: &callbackSession{
		fn: func(attempt int) (*genai.GenerateContentResponse, error) {
			attempts++
			if attempt < 2 {
				return nil, &googleapi.Error{Code: 429, Message: "rate limit"}
			}
			return &genai.GenerateContentResponse{Candidates: textCandidate(`{"value":"ok"}`)}, nil
		},
	}}
	var out testOutput
	if err := chat.Send(context.Background(), "prompt", &out); err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if out.Value != "ok" {
		t.Errorf("expected 'ok', got %q", out.Value)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", attempts)
	}
}

func TestChat_Send_ExhaustsRetriesOn429(t *testing.T) {
	chat := &Chat{retryDelay: 0, session: &callbackSession{
		fn: func(int) (*genai.GenerateContentResponse, error) {
			return nil, &googleapi.Error{Code: 429, Message: "rate limit"}
		},
	}}
	var out testOutput
	if err := chat.Send(context.Background(), "prompt", &out); err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
}

func TestChat_Send_MultiTurn(t *testing.T) {
	calls := 0
	responses := []string{`{"value":"first"}`, `{"value":"second"}`}
	session := &mockGeminiSession{}

	chat := &Chat{}
	// Simulate multi-turn by swapping responses each call
	chat.session = &statefulMockSession{responses: responses}

	var out testOutput

	if err := chat.Send(context.Background(), "turn one", &out); err != nil {
		t.Fatalf("turn 1 error: %v", err)
	}
	if out.Value != "first" {
		t.Errorf("turn 1: expected 'first', got %q", out.Value)
	}

	if err := chat.Send(context.Background(), "turn two", &out); err != nil {
		t.Fatalf("turn 2 error: %v", err)
	}
	if out.Value != "second" {
		t.Errorf("turn 2: expected 'second', got %q", out.Value)
	}
	_ = calls
	_ = session
}

type statefulMockSession struct {
	responses []string
	idx       int
}

func (s *statefulMockSession) SendMessage(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
	if s.idx >= len(s.responses) {
		return nil, errors.New("no more mock responses")
	}
	resp := s.responses[s.idx]
	s.idx++
	return &genai.GenerateContentResponse{Candidates: textCandidate(resp)}, nil
}
