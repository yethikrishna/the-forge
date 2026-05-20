package aisdk_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forge/sword/internal/aisdk"
)

func TestNewClient(t *testing.T) {
	client := aisdk.NewClient(aisdk.ModelConfig{
		Provider: aisdk.ProviderOpenAI,
		Model:    "gpt-5-mini",
		APIKey:   "test-key",
	})
	if client == nil {
		t.Fatal("client should not be nil")
	}
}

func TestChatRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req aisdk.ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Stream {
			t.Error("non-streaming request should have stream=false")
		}

		resp := aisdk.ChatResponse{
			ID:    "chat-1",
			Model: req.Model,
			Choices: []aisdk.Choice{{
				Index: 0,
				Message: aisdk.ChatMessage{
					Role:    "assistant",
					Content: "Hello from the forge",
				},
				FinishReason: "stop",
			}},
			Usage: aisdk.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := aisdk.NewClient(aisdk.ModelConfig{
		Provider: aisdk.ProviderCustom,
		Model:    "test-model",
		APIKey:   "test",
		BaseURL:  server.URL + "/v1",
	})

	resp, err := client.Chat(context.Background(), []aisdk.ChatMessage{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("chat error: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello from the forge" {
		t.Errorf("unexpected response: %s", resp.Choices[0].Message.Content)
	}
}

func TestEstimateCost(t *testing.T) {
	cost := aisdk.EstimateCost(aisdk.ProviderAnthropic, "claude-sonnet-4-20250514", 1000, 500)
	if cost <= 0 {
		t.Error("cost should be positive for known model")
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	cost := aisdk.EstimateCost(aisdk.ProviderCustom, "unknown-model", 1000, 500)
	if cost != 0 {
		t.Error("cost should be 0 for unknown model")
	}
}

func TestKnownModels(t *testing.T) {
	models := aisdk.KnownModels()
	if len(models) == 0 {
		t.Error("should have known models")
	}
}

func TestChatURL(t *testing.T) {
	tests := []struct {
		provider aisdk.Provider
		contains string
	}{
		{aisdk.ProviderOpenAI, "openai.com"},
		{aisdk.ProviderAnthropic, "anthropic.com"},
		{aisdk.ProviderXAI, "x.ai"},
	}

	for _, tt := range tests {
		client := aisdk.NewClient(aisdk.ModelConfig{Provider: tt.provider})
		// Just verify the client was created without panic
		if client == nil {
			t.Errorf("client for %s should not be nil", tt.provider)
		}
	}
}
