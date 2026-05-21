// Package dashboard provides an embedded web dashboard for The Forge.
// It serves a single-page app from Go binary embedded assets and provides
// a REST API for real-time monitoring of agents, tasks, and queues.
package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

//go:embed assets/*
var assetsFS embed.FS

// Stats holds dashboard statistics.
type Stats struct {
	ActiveAgents   int     `json:"active_agents"`
	PendingTasks   int     `json:"pending_tasks"`
	CompletedToday int     `json:"completed_today"`
	SessionCost    float64 `json:"session_cost"`
	QueueDepth     int     `json:"queue_depth"`
	CanaryStatus   string  `json:"canary_status"`
}

// AgentInfo represents agent info for the dashboard.
type AgentInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Tasks        int       `json:"tasks"`
	Cost         float64   `json:"cost"`
	LastActivity time.Time `json:"last_activity"`
}

// TaskInfo represents task info for the dashboard.
type TaskInfo struct {
	ID          string    `json:"id"`
	Queue       string    `json:"queue"`
	Priority    int       `json:"priority"`
	Status      string    `json:"status"`
	AgentID     string    `json:"agent_id"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// LogEntry represents a log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// DataProvider is the interface that provides data to the dashboard.
type DataProvider interface {
	GetStats() *Stats
	GetAgents() []AgentInfo
	GetTasks() []TaskInfo
	GetLog() []LogEntry
}

// MemoryProvider is an in-memory data provider for testing.
type MemoryProvider struct {
	mu     sync.RWMutex
	stats  Stats
	agents []AgentInfo
	tasks  []TaskInfo
	log    []LogEntry
}

// NewMemoryProvider creates a new memory provider with sample data.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		stats: Stats{
			ActiveAgents:   3,
			PendingTasks:   7,
			CompletedToday: 42,
			SessionCost:    2.34,
			QueueDepth:     12,
			CanaryStatus:   "running",
		},
		agents: []AgentInfo{
			{ID: "agent-001", Name: "coder", Type: "coder", Status: "running", Tasks: 15, Cost: 1.20, LastActivity: time.Now()},
			{ID: "agent-002", Name: "reviewer", Type: "reviewer", Status: "running", Tasks: 8, Cost: 0.65, LastActivity: time.Now().Add(-2 * time.Minute)},
			{ID: "agent-003", Name: "planner", Type: "planner", Status: "idle", Tasks: 19, Cost: 0.49, LastActivity: time.Now().Add(-10 * time.Minute)},
		},
		tasks: []TaskInfo{
			{ID: "task-001", Queue: "default", Priority: 3, Status: "running", AgentID: "agent-001", StartedAt: time.Now().Add(-5 * time.Minute)},
			{ID: "task-002", Queue: "default", Priority: 2, Status: "pending"},
			{ID: "task-003", Queue: "default", Priority: 1, Status: "completed", AgentID: "agent-002", StartedAt: time.Now().Add(-15 * time.Minute), CompletedAt: time.Now().Add(-10 * time.Minute)},
			{ID: "task-004", Queue: "build", Priority: 3, Status: "failed", AgentID: "agent-001", StartedAt: time.Now().Add(-20 * time.Minute), CompletedAt: time.Now().Add(-18 * time.Minute)},
		},
		log: []LogEntry{
			{Timestamp: time.Now().Add(-1 * time.Minute), Level: "info", Message: "Agent coder completed task-005"},
			{Timestamp: time.Now().Add(-3 * time.Minute), Level: "warn", Message: "Agent reviewer approaching cost limit"},
			{Timestamp: time.Now().Add(-5 * time.Minute), Level: "info", Message: "New task task-007 enqueued"},
			{Timestamp: time.Now().Add(-10 * time.Minute), Level: "error", Message: "Task task-004 failed: timeout exceeded"},
		},
	}
}

// GetStats returns current stats.
func (m *MemoryProvider) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &m.stats
}

// GetAgents returns agent info.
func (m *MemoryProvider) GetAgents() []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents
}

// GetTasks returns task info.
func (m *MemoryProvider) GetTasks() []TaskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks
}

// GetLog returns log entries.
func (m *MemoryProvider) GetLog() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.log
}

// UpdateStats updates the stats.
func (m *MemoryProvider) UpdateStats(s Stats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats = s
}

// AddLog adds a log entry.
func (m *MemoryProvider) AddLog(level, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = append(m.log, LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	})
	if len(m.log) > 100 {
		m.log = m.log[len(m.log)-100:]
	}
}

// Server is the dashboard HTTP server.
type Server struct {
	provider DataProvider
	addr     string
	server   *http.Server
}

// NewServer creates a new dashboard server.
func NewServer(addr string, provider DataProvider) *Server {
	if provider == nil {
		provider = NewMemoryProvider()
	}
	return &Server{
		provider: provider,
		addr:     addr,
	}
}

// Start starts the dashboard server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/agents", s.handleAgents)
	mux.HandleFunc("/api/v1/tasks", s.handleTasks)
	mux.HandleFunc("/api/v1/log", s.handleLog)

	// Serve embedded assets
	assetsSub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return fmt.Errorf("assets sub: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(assetsSub)))

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Dashboard server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the dashboard server.
func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.provider.GetStats())
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.provider.GetAgents())
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.provider.GetTasks())
}

func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.provider.GetLog())
}
