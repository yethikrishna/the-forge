package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	agents, err := store.GetAgentStatuses()
	if err != nil {
		t.Fatalf("GetAgentStatuses: %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("no agents in memory store")
	}

	sessions, err := store.GetRecentSessions(10)
	if err != nil {
		t.Fatalf("GetRecentSessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("no sessions in memory store")
	}

	cost, err := store.GetCostSummary("today")
	if err != nil {
		t.Fatalf("GetCostSummary: %v", err)
	}
	if cost.TotalCost <= 0 {
		t.Fatal("expected positive total cost")
	}

	traces, err := store.GetTraceSummary()
	if err != nil {
		t.Fatalf("GetTraceSummary: %v", err)
	}
	if traces.TotalTraces == 0 {
		t.Fatal("no traces in memory store")
	}

	metrics, err := store.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if metrics.ActiveAgents == 0 {
		t.Fatal("no active agents in metrics")
	}
}

func TestMemoryStoreUpdateAgent(t *testing.T) {
	store := NewMemoryStore()

	newAgent := AgentStatus{
		ID:          "agent-new",
		Name:        "deployer",
		Role:        "devops",
		Model:       "gpt-4.1",
		Status:      "running",
		StartedAt:   time.Now(),
		TokensUsed:  1000,
		Cost:        0.05,
		Progress:    0.5,
		CurrentTask: "Deploying to staging",
	}
	store.UpdateAgent(newAgent)

	agents, _ := store.GetAgentStatuses()
	found := false
	for _, a := range agents {
		if a.ID == "agent-new" && a.Name == "deployer" {
			found = true
		}
	}
	if !found {
		t.Fatal("new agent not found after update")
	}
}

func TestMemoryStorePushEvent(t *testing.T) {
	store := NewMemoryStore()

	event := DashboardEvent{
		Type:      "agent_start",
		AgentID:   "agent-1",
		Message:   "Agent started",
		Timestamp: time.Now(),
	}
	store.PushEvent(event)

	metrics, _ := store.GetMetrics()
	if len(metrics.RecentEvents) == 0 {
		t.Fatal("no events after push")
	}
	if metrics.RecentEvents[0].Message != "Agent started" {
		t.Fatal("event not at front of list")
	}
}

func TestDashboardServerAPIs(t *testing.T) {
	store := NewMemoryStore()
	srv := NewDashboardServer(":0", store)

	tests := []struct {
		path   string
		target interface{}
	}{
		{"/api/agents", &[]AgentStatus{}},
		{"/api/sessions", &[]SessionInfo{}},
		{"/api/costs", &CostSummary{}},
		{"/api/traces", &TraceSummary{}},
		{"/api/metrics", &DashboardMetrics{}},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", tt.path, rec.Code)
		}

		if err := json.NewDecoder(rec.Body).Decode(tt.target); err != nil {
			t.Errorf("%s: decode error: %v", tt.path, err)
		}
	}
}

func TestDashboardServerPages(t *testing.T) {
	store := NewMemoryStore()
	srv := NewDashboardServer(":0", store)

	tests := []string{"/", "/agents", "/costs", "/traces"}

	for _, path := range tests {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, rec.Code)
		}
	}
}

func TestWebSocketHub(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	if hub.ClientCount() != 0 {
		t.Fatal("expected 0 clients initially")
	}

	// Test broadcast with no clients
	hub.Broadcast([]byte("test"))
}

func TestDashboardBroadcast(t *testing.T) {
	store := NewMemoryStore()
	srv := NewDashboardServer(":0", store)

	event := DashboardEvent{
		Type:      "test",
		Message:   "Test event",
		Timestamp: time.Now(),
	}
	srv.BroadcastEvent(event)
	// Should not panic
}

func TestCostSummaryByModel(t *testing.T) {
	store := NewMemoryStore()
	cost, _ := store.GetCostSummary("today")

	if len(cost.ByModel) == 0 {
		t.Fatal("no model cost breakdown")
	}

	totalModelCost := 0.0
	for _, mc := range cost.ByModel {
		totalModelCost += mc.Cost
	}
	if totalModelCost <= 0 {
		t.Fatal("expected positive model cost total")
	}
}

func TestTraceSummaryOperations(t *testing.T) {
	store := NewMemoryStore()
	traces, _ := store.GetTraceSummary()

	if len(traces.ByOperation) == 0 {
		t.Fatal("no operation breakdown")
	}

	for _, op := range traces.ByOperation {
		if op.Count <= 0 {
			t.Fatalf("operation %s has zero count", op.Operation)
		}
	}
}
