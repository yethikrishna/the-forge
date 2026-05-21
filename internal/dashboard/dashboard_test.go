package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMemoryProviderStats(t *testing.T) {
	p := NewMemoryProvider()
	stats := p.GetStats()

	if stats.ActiveAgents == 0 {
		t.Error("Expected non-zero active agents in default data")
	}
	if stats.PendingTasks == 0 {
		t.Error("Expected non-zero pending tasks in default data")
	}
}

func TestMemoryProviderAgents(t *testing.T) {
	p := NewMemoryProvider()
	agents := p.GetAgents()

	if len(agents) == 0 {
		t.Error("Expected agents in default data")
	}
}

func TestMemoryProviderTasks(t *testing.T) {
	p := NewMemoryProvider()
	tasks := p.GetTasks()

	if len(tasks) == 0 {
		t.Error("Expected tasks in default data")
	}
}

func TestMemoryProviderLog(t *testing.T) {
	p := NewMemoryProvider()
	log := p.GetLog()

	if len(log) == 0 {
		t.Error("Expected log entries in default data")
	}
}

func TestUpdateStats(t *testing.T) {
	p := NewMemoryProvider()
	p.UpdateStats(Stats{
		ActiveAgents:  5,
		PendingTasks:  10,
		CompletedToday: 100,
		SessionCost:   15.50,
		QueueDepth:    20,
		CanaryStatus:  "promoted",
	})

	stats := p.GetStats()
	if stats.ActiveAgents != 5 {
		t.Errorf("Expected 5 active agents, got %d", stats.ActiveAgents)
	}
	if stats.CanaryStatus != "promoted" {
		t.Errorf("Expected 'promoted', got %s", stats.CanaryStatus)
	}
}

func TestAddLog(t *testing.T) {
	p := NewMemoryProvider()
	initialLen := len(p.GetLog())

	p.AddLog("info", "Test log message")

	log := p.GetLog()
	if len(log) != initialLen+1 {
		t.Errorf("Expected %d log entries, got %d", initialLen+1, len(log))
	}

	lastEntry := log[len(log)-1]
	if lastEntry.Message != "Test log message" {
		t.Errorf("Expected 'Test log message', got %q", lastEntry.Message)
	}
	if lastEntry.Level != "info" {
		t.Errorf("Expected 'info' level, got %q", lastEntry.Level)
	}
}

func TestAddLogRotation(t *testing.T) {
	p := NewMemoryProvider()

	// Add more than 100 entries
	for i := 0; i < 110; i++ {
		p.AddLog("info", "entry")
	}

	log := p.GetLog()
	if len(log) > 100 {
		t.Errorf("Expected log rotation at 100, got %d entries", len(log))
	}
}

func TestAPIStatsEndpoint(t *testing.T) {
	p := NewMemoryProvider()
	s := NewServer(":0", p)

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	s.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var stats Stats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if stats.ActiveAgents == 0 {
		t.Error("Expected non-zero active agents")
	}
}

func TestAPIAgentsEndpoint(t *testing.T) {
	p := NewMemoryProvider()
	s := NewServer(":0", p)

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()

	s.handleAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var agents []AgentInfo
	if err := json.NewDecoder(w.Body).Decode(&agents); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(agents) == 0 {
		t.Error("Expected agents")
	}
}

func TestAPITasksEndpoint(t *testing.T) {
	p := NewMemoryProvider()
	s := NewServer(":0", p)

	req := httptest.NewRequest("GET", "/api/v1/tasks", nil)
	w := httptest.NewRecorder()

	s.handleTasks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestAPILogEndpoint(t *testing.T) {
	p := NewMemoryProvider()
	s := NewServer(":0", p)

	req := httptest.NewRequest("GET", "/api/v1/log", nil)
	w := httptest.NewRecorder()

	s.handleLog(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestNewServerDefaultProvider(t *testing.T) {
	s := NewServer(":0", nil)
	if s.provider == nil {
		t.Error("Expected default provider")
	}
}

func TestCORSHeaders(t *testing.T) {
	p := NewMemoryProvider()
	s := NewServer(":0", p)

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	s.handleStats(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header")
	}
}

var _ = time.Now // ensure time import used
