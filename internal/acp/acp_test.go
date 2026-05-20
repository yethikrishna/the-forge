package acp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forge/sword/internal/acp"
)

func TestClientSendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/message" {
			t.Errorf("expected /message, got %s", r.URL.Path)
		}

		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)

		resp := acp.Message{
			ID:        "msg-1",
			Type:      acp.MessageTypeAssistant,
			Content:   "Hello from agent",
			Role:      "assistant",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := acp.NewClient(server.URL)
	msg, err := client.SendMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("send message error: %v", err)
	}
	if msg.Content != "Hello from agent" {
		t.Errorf("expected 'Hello from agent', got %q", msg.Content)
	}
}

func TestClientGetMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		messages := []acp.Message{
			{ID: "1", Type: acp.MessageTypeUser, Content: "hi", Role: "user"},
			{ID: "2", Type: acp.MessageTypeAssistant, Content: "hello", Role: "assistant"},
		}
		json.NewEncoder(w).Encode(messages)
	}))
	defer server.Close()

	client := acp.NewClient(server.URL)
	msgs, err := client.GetMessages(context.Background())
	if err != nil {
		t.Fatalf("get messages error: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestClientHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := acp.NewClient(server.URL)
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("health check error: %v", err)
	}
}

func TestClientWithToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := acp.NewClient(server.URL, acp.WithToken("test-token"))
	client.GetSession(context.Background())
}

func TestClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := acp.NewClient(server.URL)
	_, err := client.GetSession(context.Background())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestProtocolVersion(t *testing.T) {
	if acp.Version == "" {
		t.Error("version should not be empty")
	}
}
