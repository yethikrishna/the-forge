// Package aisdk provides AI SDK streaming utilities for LLM interactions.
// Stream the forge's wisdom in real-time.
package aisdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider identifies an LLM provider.
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderGoogle    Provider = "google"
	ProviderXAI       Provider = "xai"
	ProviderAzure     Provider = "azure"
	ProviderCustom    Provider = "custom"
)

// ModelConfig configures which model to use.
type ModelConfig struct {
	Provider    Provider `json:"provider"`
	Model       string   `json:"model"`
	APIKey      string   `json:"-"` // Never serialized
	BaseURL     string   `json:"base_url,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
}

// ChatMessage represents a message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatRequest is a request to the LLM.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatResponse is a non-streaming response from the LLM.
type ChatResponse struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	Choices   []Choice  `json:"choices"`
	Usage     Usage     `json:"usage"`
	CreatedAt time.Time `json:"created_at"`
}

// Choice represents a response choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	ID      string        `json:"id"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice represents a choice in a streaming chunk.
type ChunkChoice struct {
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

// Delta represents a content delta in a streaming chunk.
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// Client is an AI SDK client.
type Client struct {
	config     ModelConfig
	httpClient *http.Client
}

// NewClient creates a new AI SDK client.
func NewClient(config ModelConfig) *Client {
	return &Client{
		config:     config,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

// Chat sends a chat request and returns the full response.
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	req := ChatRequest{
		Model:     c.config.Model,
		Messages:  messages,
		MaxTokens: c.config.MaxTokens,
		Stream:    false,
	}

	if c.config.Temperature > 0 {
		req.Temperature = &c.config.Temperature
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("aisdk: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("aisdk: create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aisdk: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("aisdk: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("aisdk: decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatStream sends a chat request and returns a channel of streaming chunks.
func (c *Client) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamChunk, error) {
	req := ChatRequest{
		Model:     c.config.Model,
		Messages:  messages,
		MaxTokens: c.config.MaxTokens,
		Stream:    true,
	}

	if c.config.Temperature > 0 {
		req.Temperature = &c.config.Temperature
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("aisdk: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("aisdk: create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aisdk: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("aisdk: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			ch <- chunk
		}
	}()

	return ch, nil
}

func (c *Client) chatURL() string {
	base := c.config.BaseURL
	if base == "" {
		switch c.config.Provider {
		case ProviderOpenAI:
			base = "https://api.openai.com/v1"
		case ProviderAnthropic:
			base = "https://api.anthropic.com/v1"
		case ProviderGoogle:
			base = "https://generativelanguage.googleapis.com/v1beta"
		case ProviderXAI:
			base = "https://api.x.ai/v1"
		default:
			base = "http://localhost:11434/v1"
		}
	}

	return strings.TrimRight(base, "/") + "/chat/completions"
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	switch c.config.Provider {
	case ProviderAnthropic:
		req.Header.Set("x-api-key", c.config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
}

// ModelPricing contains pricing info for models.
type ModelPricing struct {
	Provider      Provider `json:"provider"`
	Model         string   `json:"model"`
	InputPer1M    float64  `json:"input_per_1m"`   // Price per 1M input tokens
	OutputPer1M   float64  `json:"output_per_1m"`  // Price per 1M output tokens
	ContextWindow int      `json:"context_window"` // Max context tokens
}

// KnownModels returns pricing for known models.
func KnownModels() []ModelPricing {
	return []ModelPricing{
		{ProviderAnthropic, "claude-sonnet-4-20250514", 3.0, 15.0, 200000},
		{ProviderAnthropic, "claude-opus-4-20250514", 15.0, 75.0, 200000},
		{ProviderAnthropic, "claude-haiku-3-20240307", 0.25, 1.25, 200000},
		{ProviderOpenAI, "gpt-5-mini", 1.5, 6.0, 128000},
		{ProviderOpenAI, "o3", 10.0, 40.0, 200000},
		{ProviderOpenAI, "o4-mini", 1.5, 6.0, 200000},
		{ProviderGoogle, "gemini-2.5-pro", 1.25, 10.0, 1000000},
		{ProviderGoogle, "gemini-2.5-flash", 0.15, 0.60, 1000000},
		{ProviderXAI, "grok-4-1-fast", 3.0, 15.0, 131072},
	}
}

// EstimateCost estimates the cost of a chat request.
func EstimateCost(provider Provider, model string, inputTokens, outputTokens int) float64 {
	for _, m := range KnownModels() {
		if m.Provider == provider && m.Model == model {
			inputCost := float64(inputTokens) / 1_000_000 * m.InputPer1M
			outputCost := float64(outputTokens) / 1_000_000 * m.OutputPer1M
			return inputCost + outputCost
		}
	}
	return 0
}
