package dashboard

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/forge/sword/internal/costlive"
	"github.com/forge/sword/internal/org"
	"github.com/forge/sword/internal/qualitygate"
	"github.com/forge/sword/internal/trust"
)

// LiveProvider provides real data from live subsystems to the dashboard.
// It replaces MemoryProvider with real org, cost, trust, and quality data.
type LiveProvider struct {
	mu        sync.RWMutex
	orgEngine *org.Org
	costTrack *costlive.LiveTracker
	trustMgr  *trust.Manager
	qg        *qualitygate.QualityGateSystem
	hub       *WebSocketHub
	log       []LogEntry
}

// LiveProviderConfig configures a LiveProvider.
type LiveProviderConfig struct {
	Org        *org.Org
	CostTrack  *costlive.LiveTracker
	TrustMgr   *trust.Manager
	QualityGate *qualitygate.QualityGateSystem
	Hub        *WebSocketHub
}

// NewLiveProvider creates a real data provider wired to live subsystems.
func NewLiveProvider(cfg LiveProviderConfig) *LiveProvider {
	p := &LiveProvider{
		orgEngine: cfg.Org,
		costTrack: cfg.CostTrack,
		trustMgr:  cfg.TrustMgr,
		qg:        cfg.QualityGate,
		hub:       cfg.Hub,
		log:       []LogEntry{},
	}
	return p
}

// GetStats returns real stats from live subsystems.
func (p *LiveProvider) GetStats() *Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := &Stats{
		CanaryStatus: "running",
	}

	// Real agent counts from org
	if p.orgEngine != nil {
		status := p.orgEngine.GetStatus()
		stats.ActiveAgents = status.ActiveAgents
		stats.PendingTasks = status.PendingHandoffs
		stats.QueueDepth = status.OpenEscalations
	}

	// Real cost from costlive tracker
	if p.costTrack != nil {
		live := p.costTrack.Stats()
		stats.SessionCost = live.SessionCost
		stats.CompletedToday = live.TodayCalls
	}

	return stats
}

// GetAgents returns real agent data from the org engine + trust manager.
func (p *LiveProvider) GetAgents() []AgentInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.orgEngine == nil {
		return nil
	}

	agents := p.orgEngine.ListAgents("")
	result := make([]AgentInfo, 0, len(agents))

	for _, a := range agents {
		info := AgentInfo{
			ID:           a.ID,
			Name:         a.Name,
			Type:         a.Role,
			Status:       string(a.Status),
			Tasks:        a.TasksCompleted,
			LastActivity: a.LastActive,
		}

		// Enrich with trust score if available
		if p.trustMgr != nil {
			if score, ok := p.trustMgr.GetScore(a.ID); ok {
				info.Cost = score // repurpose Cost field as trust score for display
			}
		}

		// Enrich with real cost if available
		if p.costTrack != nil {
			live := p.costTrack.Stats()
			if breakdown, ok := live.ByAgent[a.ID]; ok {
				info.Cost = breakdown.Cost
			}
		}

		result = append(result, info)
	}

	return result
}

// GetTasks returns task info derived from org handoffs and escalations.
func (p *LiveProvider) GetTasks() []TaskInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.orgEngine == nil {
		return nil
	}

	var result []TaskInfo

	// Pending handoffs as tasks
	handoffs := p.orgEngine.ListPendingHandoffs("")
	for _, h := range handoffs {
		result = append(result, TaskInfo{
			ID:       h.ID,
			Queue:    "handoff",
			Priority: 2,
			Status:   string(h.Status),
			AgentID:  h.ToAgent,
		})
	}

	// Open escalations as high-priority tasks
	escs := p.orgEngine.ListOpenEscalations("")
	for _, e := range escs {
		result = append(result, TaskInfo{
			ID:       e.ID,
			Queue:    "escalation",
			Priority: int(e.Severity) + 1,
			Status:   string(e.Status),
			AgentID:  e.AgentID,
		})
	}

	return result
}

// GetLog returns recent log entries.
func (p *LiveProvider) GetLog() []LogEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.log
}

// AddLog appends a log entry and broadcasts via WebSocket.
func (p *LiveProvider) AddLog(level, message string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	p.mu.Lock()
	p.log = append(p.log, entry)
	if len(p.log) > 200 {
		p.log = p.log[len(p.log)-200:]
	}
	p.mu.Unlock()

	// Broadcast log event via WebSocket
	if p.hub != nil {
		type wsMsg struct {
			Type  string    `json:"type"`
			Entry LogEntry  `json:"entry"`
		}
		data, _ := json.Marshal(wsMsg{Type: "log", Entry: entry})
		p.hub.Broadcast(data)
	}
}

// PushStateUpdate broadcasts the full dashboard state to all WebSocket clients.
// Call this whenever org, cost, or quality state changes.
func (p *LiveProvider) PushStateUpdate() {
	if p.hub == nil || p.hub.ClientCount() == 0 {
		return
	}

	type StateUpdate struct {
		Type   string      `json:"type"`
		Stats  *Stats      `json:"stats"`
		Agents []AgentInfo `json:"agents"`
		Tasks  []TaskInfo  `json:"tasks"`
	}

	msg := StateUpdate{
		Type:   "state_update",
		Stats:  p.GetStats(),
		Agents: p.GetAgents(),
		Tasks:  p.GetTasks(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	p.hub.Broadcast(data)
}

// QualityStats returns aggregated quality gate stats.
func (p *LiveProvider) QualityStats() map[string]interface{} {
	if p.qg == nil {
		return nil
	}

	pipelines := p.qg.ListPipelines()
	return map[string]interface{}{
		"pipeline_count": len(pipelines),
		"pipelines":      fmt.Sprintf("%d configured", len(pipelines)),
	}
}
