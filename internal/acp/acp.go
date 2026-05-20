// Package acp implements the Agent Client Protocol SDK.
// ACP provides a standardized HTTP/WebSocket protocol for communicating
// with AI coding agents. The protocol that binds all swords.
package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Protocol version.
const Version = "0.1.0"

// Message types.
const (
	MessageTypeUser      = "user"
	MessageTypeAssistant = "assistant"
	MessageTypeSystem    = "system"
	MessageTypeError     = "error"
	MessageTypeToolUse   = "tool_use"
	MessageTypeToolResult = "tool_result"
)

// Event types for streaming.
const (
	EventMessageStart  = "message_start"
	EventContentBlock  = "content_block"
	EventContentDelta  = "content_delta"
	EventMessageStop   = "message_stop"
	EventError         = "error"
	EventSessionUpdate = "session_update"
)

// Message represents a chat message.
type Message struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content,omitempty"`
	Role      string                 `json:"role"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]any         `json:"metadata,omitempty"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool invocation within a message.
type ToolCall struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Input    map[string]any `json:"input,omitempty"`
	Output   string         `json:"output,omitempty"`
	Finished bool           `json:"finished"`
}

// ContentBlock represents a block of content in a streaming response.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Event represents a streaming event.
type Event struct {
	Type      string         `json:"type"`
	Message   *Message       `json:"message,omitempty"`
	Block     *ContentBlock  `json:"block,omitempty"`
	Delta     string         `json:"delta,omitempty"`
	Error     string         `json:"error,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Session represents an agent session.
type Session struct {
	ID        string            `json:"id"`
	AgentType string            `json:"agent_type"`
	Model     string            `json:"model"`
	Status    string            `json:"status"` // running, idle, error
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// AgentInfo describes an agent.
type AgentInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Capabilities []string `json:"capabilities"`
}

// Client is an ACP protocol client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// ClientOption configures the ACP client.
type ClientOption func(*Client)

// WithToken sets an authentication token.
func WithToken(token string) ClientOption {
	return func(c *Client) { c.token = token }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient creates a new ACP client.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// SendMessage sends a message to the agent.
func (c *Client) SendMessage(ctx context.Context, content string) (*Message, error) {
	req := struct {
		Content string `json:"content"`
		Type    string `json:"type"`
	}{
		Content: content,
		Type:    MessageTypeUser,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("acp: marshal message: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/message", body)
	if err != nil {
		return nil, fmt.Errorf("acp: send message: %w", err)
	}
	defer resp.Body.Close()

	var msg Message
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, fmt.Errorf("acp: decode response: %w", err)
	}

	return &msg, nil
}

// GetMessages retrieves conversation history.
func (c *Client) GetMessages(ctx context.Context) ([]Message, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/messages", nil)
	if err != nil {
		return nil, fmt.Errorf("acp: get messages: %w", err)
	}
	defer resp.Body.Close()

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("acp: decode messages: %w", err)
	}

	return messages, nil
}

// StreamMessages sends a message and streams the response.
func (c *Client) StreamMessages(ctx context.Context, content string) (<-chan Event, error) {
	req := struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Stream  bool   `json:"stream"`
	}{
		Content: content,
		Type:    MessageTypeUser,
		Stream:  true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("acp: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/message", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("acp: create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("acp: stream request: %w", err)
	}

	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var event Event
			if err := decoder.Decode(&event); err != nil {
				if err == io.EOF {
					return
				}
				ch <- Event{Type: EventError, Error: err.Error()}
				return
			}
			ch <- event
		}
	}()

	return ch, nil
}

// GetSession retrieves the current session info.
func (c *Client) GetSession(ctx context.Context) (*Session, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/session", nil)
	if err != nil {
		return nil, fmt.Errorf("acp: get session: %w", err)
	}
	defer resp.Body.Close()

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("acp: decode session: %w", err)
	}

	return &session, nil
}

// GetAgentInfo retrieves information about the connected agent.
func (c *Client) GetAgentInfo(ctx context.Context) (*AgentInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/agent", nil)
	if err != nil {
		return nil, fmt.Errorf("acp: get agent info: %w", err)
	}
	defer resp.Body.Close()

	var info AgentInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("acp: decode agent info: %w", err)
	}

	return &info, nil
}

// Cancel cancels the current agent operation.
func (c *Client) Cancel(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodPost, "/cancel", nil)
	return err
}

// Health checks if the agent API is healthy.
func (c *Client) Health(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodGet, "/health", nil)
	return err
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	u, _ := url.JoinPath(c.baseURL, path)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("acp: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return resp, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("ACP-Version", Version)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
