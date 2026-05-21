package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewA2AServer(t *testing.T) {
	card := AgentCard{
		ID:          "forge-agent-1",
		Name:        "Forge Agent",
		Description: "Test agent",
		Endpoint:    "http://localhost:8080/a2a",
		Capabilities: []Capability{
			{ID: "code-gen", Name: "Code Generation", Tags: []string{"code", "generation"}},
			{ID: "review", Name: "Code Review", Tags: []string{"code", "review"}},
		},
	}

	srv := NewA2AServer(card)
	if srv.GetCard().ID != "forge-agent-1" {
		t.Fatal("wrong agent ID")
	}
}

func TestRegisterAgent(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	remote := AgentCard{
		ID:          "remote-1",
		Name:        "Remote Agent",
		Description: "A remote agent",
		Endpoint:    "http://remote:8080/a2a",
		Capabilities: []Capability{
			{ID: "test", Name: "Testing", Tags: []string{"test", "qa"}},
		},
	}

	srv.RegisterAgent(remote)
	agents := srv.ListAgents()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].ID != "remote-1" {
		t.Fatal("wrong agent ID")
	}
}

func TestUnregisterAgent(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	srv.RegisterAgent(AgentCard{ID: "remote-1", Name: "Remote 1"})
	srv.RegisterAgent(AgentCard{ID: "remote-2", Name: "Remote 2"})

	srv.UnregisterAgent("remote-1")
	agents := srv.ListAgents()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent after unregister, got %d", len(agents))
	}
}

func TestFindByCapability(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	srv.RegisterAgent(AgentCard{
		ID: "coder",
		Capabilities: []Capability{
			{ID: "code-gen", Name: "Code Gen", Tags: []string{"code"}},
		},
	})
	srv.RegisterAgent(AgentCard{
		ID: "reviewer",
		Capabilities: []Capability{
			{ID: "review", Name: "Review", Tags: []string{"review"}},
		},
	})

	found := srv.FindByCapability("code-gen")
	if len(found) != 1 {
		t.Fatalf("expected 1 agent with code-gen, got %d", len(found))
	}
	if found[0].ID != "coder" {
		t.Fatal("wrong agent found")
	}
}

func TestFindByTag(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	srv.RegisterAgent(AgentCard{
		ID: "coder",
		Capabilities: []Capability{
			{ID: "gen", Name: "Gen", Tags: []string{"code", "generation"}},
		},
	})
	srv.RegisterAgent(AgentCard{
		ID: "reviewer",
		Capabilities: []Capability{
			{ID: "rev", Name: "Rev", Tags: []string{"code", "review"}},
		},
	})

	found := srv.FindByTag("code")
	if len(found) != 2 {
		t.Fatalf("expected 2 agents with code tag, got %d", len(found))
	}
}

func TestHandleTaskRequest(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	reqPayload, _ := json.Marshal(TaskRequest{
		TaskID:     "task-1",
		Capability: "code-gen",
		Priority:   5,
	})

	msg := Message{
		ID:        "msg-1",
		Type:      MessageTaskRequest,
		From:      "remote-1",
		To:        "local",
		Protocol:  "a2a/" + ProtocolVersion,
		Timestamp: time.Now().UTC(),
		Payload:   reqPayload,
	}

	resp, err := srv.HandleMessage(msg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if resp.Type != MessageTaskResponse {
		t.Fatalf("expected task_response, got %s", resp.Type)
	}

	var taskResp TaskResponse
	json.Unmarshal(resp.Payload, &taskResp)
	if taskResp.TaskID != "task-1" {
		t.Fatal("wrong task ID in response")
	}
}

func TestHandleCapabilityQuery(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	srv.RegisterAgent(AgentCard{
		ID:           "coder",
		Capabilities: []Capability{{ID: "gen", Name: "Gen"}},
	})

	queryPayload, _ := json.Marshal(CapabilityQuery{Query: "code"})

	msg := Message{
		ID:        "msg-2",
		Type:      MessageCapabilityQuery,
		From:      "remote-1",
		To:        "local",
		Payload:   queryPayload,
		Timestamp: time.Now().UTC(),
	}

	resp, err := srv.HandleMessage(msg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if resp.Type != MessageCapabilityResponse {
		t.Fatalf("expected capability_response, got %s", resp.Type)
	}
}

func TestHandleHeartbeat(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	msg := Message{
		ID:        "msg-3",
		Type:      MessageHeartbeat,
		From:      "remote-1",
		To:        "local",
		Payload:   json.RawMessage(`{}`),
		Timestamp: time.Now().UTC(),
	}

	resp, err := srv.HandleMessage(msg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if resp.Type != MessageHeartbeat {
		t.Fatalf("expected heartbeat, got %s", resp.Type)
	}
}

func TestHandleUnknownType(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	msg := Message{
		ID:        "msg-4",
		Type:      "unknown_type",
		From:      "remote-1",
		To:        "local",
		Payload:   json.RawMessage(`{}`),
		Timestamp: time.Now().UTC(),
	}

	resp, err := srv.HandleMessage(msg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if resp.Type != MessageError {
		t.Fatalf("expected error response, got %s", resp.Type)
	}
}

func TestServeHTTP(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	msg := Message{
		ID:        "msg-5",
		Type:      MessageHeartbeat,
		From:      "remote-1",
		To:        "local",
		Payload:   json.RawMessage(`{}`),
		Timestamp: time.Now().UTC(),
	}

	data, _ := json.Marshal(msg)
	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestServeHTTPMethodNotAllowed(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	req := httptest.NewRequest(http.MethodGet, "/a2a", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestNewMessage(t *testing.T) {
	srv := NewA2AServer(AgentCard{ID: "local"})

	msg, err := srv.NewMessage(MessageTaskRequest, "remote-1", TaskRequest{
		TaskID:     "task-1",
		Capability: "code-gen",
	})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.From != "local" {
		t.Fatal("wrong from")
	}
	if msg.To != "remote-1" {
		t.Fatal("wrong to")
	}
	if msg.Type != MessageTaskRequest {
		t.Fatal("wrong type")
	}
}

func TestStats(t *testing.T) {
	srv := NewA2AServer(AgentCard{
		ID:           "local",
		Capabilities: []Capability{{ID: "c1"}, {ID: "c2"}},
	})

	srv.RegisterAgent(AgentCard{ID: "remote-1"})

	stats := srv.Stats()
	if stats.AgentID != "local" {
		t.Fatal("wrong agent ID")
	}
	if stats.RegisteredAgents != 1 {
		t.Fatalf("expected 1 registered agent, got %d", stats.RegisteredAgents)
	}
	if stats.Capabilities != 2 {
		t.Fatalf("expected 2 capabilities, got %d", stats.Capabilities)
	}
}

func TestFormatAgentCard(t *testing.T) {
	card := &AgentCard{
		ID:          "test-agent",
		Name:        "Test Agent",
		Description: "Test",
		Endpoint:    "http://localhost:8080",
		Capabilities: []Capability{
			{ID: "gen", Name: "Generate", Tags: []string{"code"}},
		},
	}
	output := FormatAgentCard(card)
	if len(output) == 0 {
		t.Fatal("empty card format")
	}
}

func TestFormatStats(t *testing.T) {
	stats := A2AStats{AgentID: "test", RegisteredAgents: 5, Capabilities: 3}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Fatal("empty stats format")
	}
}

// strings used above
