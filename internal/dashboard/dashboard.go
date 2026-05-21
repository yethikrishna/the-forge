// Package dashboard provides an embedded web dashboard for Forge.
// The smith's view — watch every hammer strike in real time.
package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"
)

//go:embed static/*
var staticFS embed.FS

// DashboardServer serves the real-time web dashboard.
type DashboardServer struct {
	addr      string
	hub       *WebSocketHub
	store     Store
	templates *template.Template
	mu        sync.RWMutex
	running   bool
	server    *http.Server
	handler   http.Handler
}

// Store provides data for the dashboard.
type Store interface {
	GetAgentStatuses() ([]AgentStatus, error)
	GetRecentSessions(limit int) ([]SessionInfo, error)
	GetCostSummary(period string) (*CostSummary, error)
	GetTraceSummary() (*TraceSummary, error)
	GetMetrics() (*DashboardMetrics, error)
}

// AgentStatus represents a running agent's current state.
type AgentStatus struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Role        string            `json:"role"`
	Model       string            `json:"model"`
	Status      string            `json:"status"` // running, idle, error, completed
	StartedAt   time.Time         `json:"started_at"`
	TokensUsed  int64             `json:"tokens_used"`
	Cost        float64           `json:"cost"`
	Progress    float64           `json:"progress"` // 0-1
	CurrentTask string            `json:"current_task"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SessionInfo represents a completed or running session.
type SessionInfo struct {
	ID            string     `json:"id"`
	AgentID       string     `json:"agent_id"`
	AgentName     string     `json:"agent_name"`
	Model         string     `json:"model"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Duration      string     `json:"duration"`
	TokensIn      int64      `json:"tokens_in"`
	TokensOut     int64      `json:"tokens_out"`
	Cost          float64    `json:"cost"`
	OutputPreview string    `json:"output_preview,omitempty"`
}

// CostSummary represents cost data for a period.
type CostSummary struct {
	Period      string      `json:"period"`
	TotalCost   float64     `json:"total_cost"`
	TotalTokens int64       `json:"total_tokens"`
	ByModel     []ModelCost `json:"by_model"`
	ByAgent     []AgentCost `json:"by_agent"`
	DailyCosts  []DailyCost `json:"daily_costs"`
	BudgetUsed  float64     `json:"budget_used"`
	BudgetTotal float64     `json:"budget_total"`
}

// ModelCost is cost breakdown by model.
type ModelCost struct {
	Model    string  `json:"model"`
	Cost     float64 `json:"cost"`
	Tokens   int64   `json:"tokens"`
	Requests int     `json:"requests"`
}

// AgentCost is cost breakdown by agent.
type AgentCost struct {
	AgentID   string  `json:"agent_id"`
	AgentName string  `json:"agent_name"`
	Cost      float64 `json:"cost"`
	Tokens    int64   `json:"tokens"`
	Sessions  int     `json:"sessions"`
}

// DailyCost is cost per day.
type DailyCost struct {
	Date     string  `json:"date"`
	Cost     float64 `json:"cost"`
	Tokens   int64   `json:"tokens"`
	Sessions int     `json:"sessions"`
}

// TraceSummary represents trace data.
type TraceSummary struct {
	TotalTraces  int          `json:"total_traces"`
	AvgDuration  float64      `json:"avg_duration_ms"`
	ErrorRate    float64      `json:"error_rate"`
	ByOperation  []OpTrace    `json:"by_operation"`
	RecentTraces []TraceEntry `json:"recent_traces"`
}

// OpTrace is trace data grouped by operation.
type OpTrace struct {
	Operation string  `json:"operation"`
	Count     int     `json:"count"`
	AvgMs     float64 `json:"avg_ms"`
	ErrorRate float64 `json:"error_rate"`
	P50Ms     float64 `json:"p50_ms"`
	P99Ms     float64 `json:"p99_ms"`
}

// TraceEntry is a single trace.
type TraceEntry struct {
	ID         string            `json:"id"`
	Operation  string            `json:"operation"`
	DurationMs float64           `json:"duration_ms"`
	Status     string            `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// DashboardMetrics are real-time dashboard metrics.
type DashboardMetrics struct {
	ActiveAgents    int              `json:"active_agents"`
	RunningSessions int              `json:"running_sessions"`
	TodayCost       float64          `json:"today_cost"`
	TodayTokens     int64            `json:"today_tokens"`
	TotalSessions   int              `json:"total_sessions"`
	RecentEvents    []DashboardEvent `json:"recent_events"`
	Uptime          string           `json:"uptime"`
}

// DashboardEvent is a real-time event pushed to the dashboard.
type DashboardEvent struct {
	Type      string      `json:"type"` // agent_start, agent_stop, cost_update, error, trace
	AgentID   string      `json:"agent_id,omitempty"`
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// MemoryStore is an in-memory implementation of Store.
type MemoryStore struct {
	agents   []AgentStatus
	sessions []SessionInfo
	cost     CostSummary
	traces   TraceSummary
	metrics  DashboardMetrics
	mu       sync.RWMutex
}

// NewMemoryStore creates an in-memory store with sample data.
func NewMemoryStore() *MemoryStore {
	now := time.Now().UTC()
	return &MemoryStore{
		agents: []AgentStatus{
			{ID: "agent-1", Name: "coder", Role: "coder", Model: "gpt-4.1", Status: "running", StartedAt: now.Add(-15 * time.Minute), TokensUsed: 12500, Cost: 0.45, Progress: 0.7, CurrentTask: "Implementing auth module"},
			{ID: "agent-2", Name: "reviewer", Role: "reviewer", Model: "claude-sonnet-4", Status: "idle", StartedAt: now.Add(-30 * time.Minute), TokensUsed: 8200, Cost: 0.32, Progress: 1.0, CurrentTask: ""},
			{ID: "agent-3", Name: "tester", Role: "tester", Model: "gpt-4.1-mini", Status: "running", StartedAt: now.Add(-5 * time.Minute), TokensUsed: 3100, Cost: 0.08, Progress: 0.3, CurrentTask: "Running integration tests"},
		},
		sessions: []SessionInfo{
			{ID: "s-1", AgentID: "agent-1", AgentName: "coder", Model: "gpt-4.1", Status: "running", StartedAt: now.Add(-15 * time.Minute), Duration: "15m", TokensIn: 5000, TokensOut: 7500, Cost: 0.45, OutputPreview: "package auth\n\nimport ("},
			{ID: "s-2", AgentID: "agent-2", AgentName: "reviewer", Model: "claude-sonnet-4", Status: "completed", StartedAt: now.Add(-30 * time.Minute), Duration: "12m", TokensIn: 3200, TokensOut: 5000, Cost: 0.32, OutputPreview: "Review: 3 issues found..."},
		},
		cost: CostSummary{
			Period:      "today",
			TotalCost:   2.45,
			TotalTokens: 87500,
			ByModel: []ModelCost{
				{Model: "gpt-4.1", Cost: 1.20, Tokens: 45000, Requests: 12},
				{Model: "claude-sonnet-4", Cost: 0.95, Tokens: 32000, Requests: 8},
				{Model: "gpt-4.1-mini", Cost: 0.30, Tokens: 10500, Requests: 5},
			},
			ByAgent: []AgentCost{
				{AgentID: "agent-1", AgentName: "coder", Cost: 1.20, Tokens: 45000, Sessions: 5},
				{AgentID: "agent-2", AgentName: "reviewer", Cost: 0.95, Tokens: 32000, Sessions: 3},
			},
			DailyCosts: []DailyCost{
				{Date: now.Add(-6 * 24 * time.Hour).Format("2006-01-02"), Cost: 3.10, Tokens: 112000, Sessions: 15},
				{Date: now.Add(-5 * 24 * time.Hour).Format("2006-01-02"), Cost: 2.80, Tokens: 98000, Sessions: 12},
				{Date: now.Add(-4 * 24 * time.Hour).Format("2006-01-02"), Cost: 4.20, Tokens: 145000, Sessions: 18},
				{Date: now.Add(-3 * 24 * time.Hour).Format("2006-01-02"), Cost: 1.90, Tokens: 67000, Sessions: 9},
				{Date: now.Add(-2 * 24 * time.Hour).Format("2006-01-02"), Cost: 3.50, Tokens: 120000, Sessions: 14},
				{Date: now.Add(-1 * 24 * time.Hour).Format("2006-01-02"), Cost: 2.10, Tokens: 75000, Sessions: 11},
				{Date: now.Format("2006-01-02"), Cost: 2.45, Tokens: 87500, Sessions: 7},
			},
			BudgetUsed:  20.05,
			BudgetTotal: 50.00,
		},
		traces: TraceSummary{
			TotalTraces: 156,
			AvgDuration: 320,
			ErrorRate:    0.05,
			ByOperation: []OpTrace{
				{Operation: "agent.run", Count: 45, AvgMs: 2500, ErrorRate: 0.04, P50Ms: 2200, P99Ms: 8500},
				{Operation: "model.complete", Count: 89, AvgMs: 1200, ErrorRate: 0.03, P50Ms: 950, P99Ms: 4200},
				{Operation: "tool.execute", Count: 22, AvgMs: 450, ErrorRate: 0.09, P50Ms: 320, P99Ms: 1800},
			},
			RecentTraces: []TraceEntry{
				{ID: "t-1", Operation: "agent.run", DurationMs: 3200, Status: "ok", Timestamp: now.Add(-2 * time.Minute), Attributes: map[string]string{"agent": "coder"}},
				{ID: "t-2", Operation: "model.complete", DurationMs: 1500, Status: "ok", Timestamp: now.Add(-1 * time.Minute), Attributes: map[string]string{"model": "gpt-4.1"}},
				{ID: "t-3", Operation: "tool.execute", DurationMs: 800, Status: "error", Timestamp: now.Add(-30 * time.Second), Attributes: map[string]string{"tool": "search"}},
			},
		},
		metrics: DashboardMetrics{
			ActiveAgents:    2,
			RunningSessions: 2,
			TodayCost:       2.45,
			TodayTokens:     87500,
			TotalSessions:   86,
			Uptime:          "3d 14h 22m",
			RecentEvents: []DashboardEvent{
				{Type: "agent_start", AgentID: "agent-1", Message: "coder started", Timestamp: now.Add(-15 * time.Minute)},
				{Type: "cost_update", Message: "Daily cost: $2.45", Timestamp: now.Add(-5 * time.Minute)},
				{Type: "error", AgentID: "agent-3", Message: "search tool timeout", Timestamp: now.Add(-1 * time.Minute)},
			},
		},
	}
}

func (m *MemoryStore) GetAgentStatuses() ([]AgentStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents, nil
}

func (m *MemoryStore) GetRecentSessions(limit int) ([]SessionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit > len(m.sessions) {
		return m.sessions, nil
	}
	return m.sessions[:limit], nil
}

func (m *MemoryStore) GetCostSummary(period string) (*CostSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &m.cost, nil
}

func (m *MemoryStore) GetTraceSummary() (*TraceSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &m.traces, nil
}

func (m *MemoryStore) GetMetrics() (*DashboardMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &m.metrics, nil
}

// UpdateAgent updates an agent status in the memory store.
func (m *MemoryStore) UpdateAgent(status AgentStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, a := range m.agents {
		if a.ID == status.ID {
			m.agents[i] = status
			return
		}
	}
	m.agents = append(m.agents, status)
}

// PushEvent pushes a real-time event.
func (m *MemoryStore) PushEvent(event DashboardEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.RecentEvents = append([]DashboardEvent{event}, m.metrics.RecentEvents...)
	if len(m.metrics.RecentEvents) > 50 {
		m.metrics.RecentEvents = m.metrics.RecentEvents[:50]
	}
}

// NewDashboardServer creates a new dashboard server.
func NewDashboardServer(addr string, store Store) *DashboardServer {
	templates := template.Must(template.New("").Funcs(template.FuncMap{
		"sub": func(a, b float64) float64 { return a - b },
		"mul": func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 { return a / b },
	}).ParseFS(staticFS, "static/*.html"))

	ds := &DashboardServer{
		addr:      addr,
		store:     store,
		hub:       NewWebSocketHub(),
		templates: templates,
	}
	ds.handler = ds.buildHandler()
	return ds
}

// buildHandler creates the HTTP handler.
func (ds *DashboardServer) buildHandler() http.Handler {
	mux := http.NewServeMux()

	// Static files
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	// Pages
	mux.HandleFunc("/", ds.handleIndex)
	mux.HandleFunc("/agents", ds.handleAgents)
	mux.HandleFunc("/costs", ds.handleCosts)
	mux.HandleFunc("/traces", ds.handleTraces)

	// API
	mux.HandleFunc("/api/agents", ds.handleAPIAgents)
	mux.HandleFunc("/api/sessions", ds.handleAPISessions)
	mux.HandleFunc("/api/costs", ds.handleAPICosts)
	mux.HandleFunc("/api/traces", ds.handleAPITraces)
	mux.HandleFunc("/api/metrics", ds.handleAPIMetrics)

	// WebSocket
	mux.HandleFunc("/ws", ds.handleWebSocket)

	return mux
}

// Start starts the dashboard server.
func (ds *DashboardServer) Start() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.running {
		return fmt.Errorf("dashboard already running")
	}

	ds.server = &http.Server{
		Addr:    ds.addr,
		Handler: ds.handler,
	}

	go ds.hub.Run()
	ds.running = true

	go func() {
		log.Printf("Forge Dashboard: http://%s", ds.addr)
		if err := ds.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Dashboard error: %v", err)
		}
	}()

	return nil
}

// Stop stops the dashboard server.
func (ds *DashboardServer) Stop() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if !ds.running {
		return nil
	}

	ds.running = false
	return ds.server.Close()
}

// BroadcastEvent sends a real-time event to all connected clients.
func (ds *DashboardServer) BroadcastEvent(event DashboardEvent) {
	data, _ := json.Marshal(event)
	ds.hub.Broadcast(data)
}

// IsRunning returns whether the dashboard is running.
func (ds *DashboardServer) IsRunning() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.running
}

// Handler returns the HTTP handler (for testing).
func (ds *DashboardServer) Handler() http.Handler {
	return ds.handler
}

// ---- Page Handlers ----

func (ds *DashboardServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	metrics, _ := ds.store.GetMetrics()
	agents, _ := ds.store.GetAgentStatuses()

	data := map[string]interface{}{
		"Metrics": metrics,
		"Agents":  agents,
		"Title":   "Forge Dashboard",
	}

	ds.templates.ExecuteTemplate(w, "index.html", data)
}

func (ds *DashboardServer) handleAgents(w http.ResponseWriter, r *http.Request) {
	agents, _ := ds.store.GetAgentStatuses()
	sessions, _ := ds.store.GetRecentSessions(20)

	data := map[string]interface{}{
		"Agents":   agents,
		"Sessions": sessions,
		"Title":    "Agents — Forge Dashboard",
	}

	ds.templates.ExecuteTemplate(w, "agents.html", data)
}

func (ds *DashboardServer) handleCosts(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}
	cost, _ := ds.store.GetCostSummary(period)

	data := map[string]interface{}{
		"Cost":  cost,
		"Title": "Costs — Forge Dashboard",
	}

	ds.templates.ExecuteTemplate(w, "costs.html", data)
}

func (ds *DashboardServer) handleTraces(w http.ResponseWriter, r *http.Request) {
	traces, _ := ds.store.GetTraceSummary()

	data := map[string]interface{}{
		"Traces": traces,
		"Title":  "Traces — Forge Dashboard",
	}

	ds.templates.ExecuteTemplate(w, "traces.html", data)
}

// ---- API Handlers ----

func (ds *DashboardServer) handleAPIAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := ds.store.GetAgentStatuses()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (ds *DashboardServer) handleAPISessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := ds.store.GetRecentSessions(50)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (ds *DashboardServer) handleAPICosts(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}
	cost, err := ds.store.GetCostSummary(period)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cost)
}

func (ds *DashboardServer) handleAPITraces(w http.ResponseWriter, r *http.Request) {
	traces, err := ds.store.GetTraceSummary()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(traces)
}

func (ds *DashboardServer) handleAPIMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := ds.store.GetMetrics()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// ---- WebSocket ----

func (ds *DashboardServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ServeWebSocket(ds.hub, w, r)
}
