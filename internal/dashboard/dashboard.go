// Package dashboard provides a web dashboard for The Forge.
// Every forge needs a window to see the flames.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// AgentStatus represents the status of an agent.
type AgentStatus struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	Status    string    `json:"status"` // "running", "idle", "error", "stopped"
	Requests  int64     `json:"requests"`
	LastActive time.Time `json:"last_active"`
	Uptime    string    `json:"uptime"`
	Model     string    `json:"model"`
}

// SystemStats holds system-level statistics.
type SystemStats struct {
	Version      string  `json:"version"`
	Uptime       string  `json:"uptime"`
	AgentsActive int     `json:"agents_active"`
	AgentsTotal  int     `json:"agents_total"`
	Requests     int64   `json:"requests_total"`
	TokensUsed   int64   `json:"tokens_used"`
	CostUSD      float64 `json:"cost_usd"`
	GoRoutines   int     `json:"goroutines"`
	MemoryMB     float64 `json:"memory_mb"`
}

// Dashboard is the web dashboard server.
type Dashboard struct {
	port      int
	stats     SystemStats
	agents    []AgentStatus
	startTime time.Time
	mu        sync.RWMutex
	requests  int64
	server    *http.Server
}

// New creates a new dashboard server.
func New(port int) *Dashboard {
	return &Dashboard{
		port:      port,
		startTime: time.Now(),
		stats: SystemStats{
			Version: "0.4.0",
		},
	}
}

// Start starts the dashboard server.
func (d *Dashboard) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/stats", d.handleStats)
	mux.HandleFunc("/api/agents", d.handleAgents)
	mux.HandleFunc("/api/health", d.handleHealth)
	mux.HandleFunc("/api/events", d.handleEvents)

	// Dashboard UI
	mux.HandleFunc("/", d.handleDashboard)

	d.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", d.port),
		Handler: mux,
	}

	go func() {
		select {
		case <-ctx.Done():
			d.server.Shutdown(context.Background())
		}
	}()

	fmt.Printf("Forge: Dashboard on http://localhost:%d\n", d.port)
	return d.server.ListenAndServe()
}

// StartWithSignal starts the dashboard with signal handling.
func (d *Dashboard) StartWithSignal() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		d.server.Shutdown(context.Background())
		cancel()
	}()

	return d.Start(ctx)
}

// UpdateAgent updates an agent's status.
func (d *Dashboard) UpdateAgent(name string, status AgentStatus) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, a := range d.agents {
		if a.Name == name {
			d.agents[i] = status
			return
		}
	}
	d.agents = append(d.agents, status)
}

// IncrementRequests increments the request counter.
func (d *Dashboard) IncrementRequests() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests++
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	d.stats.Uptime = time.Since(d.startTime).Round(time.Second).String()
	d.stats.Requests = d.requests
	d.stats.AgentsTotal = len(d.agents)
	d.stats.AgentsActive = 0
	for _, a := range d.agents {
		if a.Status == "running" {
			d.stats.AgentsActive++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.stats)
}

func (d *Dashboard) handleAgents(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.agents)
}

func (d *Dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","uptime":"%s"}`, time.Since(d.startTime).Round(time.Second))
}

func (d *Dashboard) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	for i := 0; i < 5; i++ {
		select {
		case <-r.Context().Done():
			return
		default:
			fmt.Fprintf(w, "data: {\"type\":\"heartbeat\",\"ts\":%d}\n\n", time.Now().Unix())
			flusher.Flush()
			time.Sleep(2 * time.Second)
		}
	}
}

func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>The Forge — Dashboard</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: system-ui, -apple-system, sans-serif; background: #0a0a0a; color: #e0e0e0; min-height: 100vh; }
header { padding: 20px 32px; background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%); border-bottom: 2px solid #e94560; display: flex; align-items: center; justify-content: space-between; }
h1 { font-size: 1.5em; font-weight: 600; }
.sword { color: #e94560; font-size: 1.3em; }
.version { color: #888; font-size: 0.85em; }
.stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; padding: 24px 32px; }
.stat-card { background: #16213e; border-radius: 12px; padding: 20px; border: 1px solid #0f3460; transition: transform 0.2s; }
.stat-card:hover { transform: translateY(-2px); }
.stat-value { font-size: 2em; font-weight: 700; color: #e94560; }
.stat-label { font-size: 0.85em; color: #888; margin-top: 4px; }
.agents { padding: 0 32px 32px; }
.agents h2 { margin-bottom: 16px; color: #00d2ff; font-size: 1.2em; }
.agent-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; }
.agent-card { background: #16213e; border-radius: 8px; padding: 16px; border: 1px solid #0f3460; }
.agent-name { font-weight: 600; font-size: 1.1em; margin-bottom: 4px; }
.agent-status { font-size: 0.85em; }
.status-running { color: #4ade80; }
.status-idle { color: #facc15; }
.status-error { color: #f87171; }
.status-stopped { color: #888; }
.agent-meta { font-size: 0.8em; color: #666; margin-top: 8px; }
.footer { padding: 24px 32px; text-align: center; color: #444; font-size: 0.85em; border-top: 1px solid #1a1a2e; }
</style>
</head>
<body>
<header>
  <h1><span class="sword">⚔️</span> The Forge</h1>
  <span class="version" id="version">v0.4.0</span>
</header>
<div class="stats" id="stats">
  <div class="stat-card"><div class="stat-value" id="agents-active">0</div><div class="stat-label">Active Agents</div></div>
  <div class="stat-card"><div class="stat-value" id="requests">0</div><div class="stat-label">Total Requests</div></div>
  <div class="stat-card"><div class="stat-value" id="tokens">0</div><div class="stat-label">Tokens Used</div></div>
  <div class="stat-card"><div class="stat-value" id="cost">$0.00</div><div class="stat-label">Total Cost</div></div>
  <div class="stat-card"><div class="stat-value" id="uptime">0s</div><div class="stat-label">Uptime</div></div>
</div>
<div class="agents">
  <h2>Agents</h2>
  <div class="agent-grid" id="agent-grid">Loading...</div>
</div>
<div class="footer">The wielder and the sword are one.</div>
<script>
async function refresh() {
  try {
    const r = await fetch('/api/stats');
    const d = await r.json();
    document.getElementById('agents-active').textContent = d.agents_active;
    document.getElementById('requests').textContent = d.requests_total;
    document.getElementById('tokens').textContent = d.tokens_used.toLocaleString();
    document.getElementById('cost').textContent = '$' + d.cost_usd.toFixed(2);
    document.getElementById('uptime').textContent = d.uptime;
    document.getElementById('version').textContent = 'v' + d.version;
  } catch(e) {}

  try {
    const r = await fetch('/api/agents');
    const agents = await r.json();
    const grid = document.getElementById('agent-grid');
    if (agents.length === 0) {
      grid.innerHTML = '<div style="color:#666">No agents running. Start one with <code>forge serve</code></div>';
    } else {
      grid.innerHTML = agents.map(a =>
        '<div class="agent-card">' +
        '<div class="agent-name">' + a.name + '</div>' +
        '<div class="agent-status status-' + a.status + '">' + a.status + '</div>' +
        '<div class="agent-meta">' + a.type + ' · ' + a.model + '</div>' +
        '</div>'
      ).join('');
    }
  } catch(e) {}
}
refresh();
setInterval(refresh, 3000);
</script>
</body>
</html>`
