package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testOutput struct {
	Value string `json:"value"`
}

func TestClient_GenerateJSON_OK(t *testing.T) {
	inner, _ := json.Marshal(testOutput{Value: "hello"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected /api/generate, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(generateResponse{Response: string(inner), Done: true})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-model")
	var out testOutput
	if err := c.GenerateJSON(context.Background(), "prompt", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value != "hello" {
		t.Errorf("expected value 'hello', got %q", out.Value)
	}
}

func TestClient_GenerateJSON_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-model")
	var out testOutput
	err := c.GenerateJSON(context.Background(), "prompt", &out)
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

func TestClient_GenerateJSON_MalformedOuterJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-model")
	var out testOutput
	err := c.GenerateJSON(context.Background(), "prompt", &out)
	if err == nil {
		t.Fatal("expected error for malformed outer JSON, got nil")
	}
}

func TestClient_GenerateJSON_MalformedInnerJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(generateResponse{Response: "not-json", Done: true})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-model")
	var out testOutput
	err := c.GenerateJSON(context.Background(), "prompt", &out)
	if err == nil {
		t.Fatal("expected error for malformed inner JSON, got nil")
	}
}
