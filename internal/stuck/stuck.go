// Package stuck provides silent failure detection for AI agents.
// Agents that stop producing output, enter error loops, or exhibit circular
// behavior are detected and escalated with automated recovery attempts.
package stuck

import (
	"fmt"
	"sync"
	"time"
)

// EscalationLevel determines urgency of response.
type EscalationLevel int

const (
	EscalateNone EscalationLevel = iota
	EscalateLog
	EscalateNotify
	EscalateEscalate
	EscalateTerminate
)

func (e EscalationLevel) String() string {
	return [...]string{"none", "log", "notify", "escalate", "terminate"}[e]
}

// Heartbeat is a periodic pulse from an agent.
type Heartbeat struct {
	AgentID    string
	Status     string // "working", "idle", "waiting"
	TaskID     string
	Progress   float64 // 0-1
	LastAction string
	Timestamp  time.Time
}

// StuckReport describes why an agent is believed to be stuck.
type StuckReport struct {
	AgentID     string
	Level       EscalationLevel
	Reasons     []string
	LastAction  time.Time
	Duration    time.Duration
	RecoveryTried int
	Actions     []RecoveryAction
}

// RecoveryAction is an automated recovery attempt.
type RecoveryAction struct {
	Type       string // "context_inject", "approach_switch", "peer_assist", "task_decompose"
	Success    bool
	Timestamp  time.Time
	Detail     string
}

// RecoveryResult captures the outcome of a recovery attempt.
type RecoveryResult struct {
	AgentID   string
	Recovered bool
	Action    RecoveryAction
}

// StuckMetrics tracks stuck detection statistics.
type StuckMetrics struct {
	TotalDetected   int
	TotalRecovered  int
	AvgRecoveryTime time.Duration
	PerAgent        map[string]int // agentID → times detected stuck
}

// MonitorConfig configures monitoring for an agent.
type MonitorConfig struct {
	HeartbeatInterval time.Duration // expected heartbeat frequency
	NoOutputTimeout   time.Duration // how long without output = stuck
	MaxConsecErrors   int           // consecutive errors before stuck
	MaxSameAction     int           // same action repeated = circular
	AutoRecover       bool          // try automated recovery
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() MonitorConfig {
	return MonitorConfig{
		HeartbeatInterval: 30 * time.Second,
		NoOutputTimeout:   15 * time.Minute,
		MaxConsecErrors:   5,
		MaxSameAction:     3,
		AutoRecover:       true,
	}
}

// StuckDetector is the main stuck detection engine.
type StuckDetector struct {
	configs     map[string]MonitorConfig
	heartbeats  map[string]Heartbeat
	errors      map[string]int  // agentID → consecutive error count
	actions     map[string]map[string]int // agentID → action → count
	recoveries  map[string]int  // agentID → recovery attempts
	mu          sync.RWMutex
}

// NewStuckDetector creates a new stuck detection engine.
func NewStuckDetector() *StuckDetector {
	return &StuckDetector{
		configs:    make(map[string]MonitorConfig),
		heartbeats: make(map[string]Heartbeat),
		errors:     make(map[string]int),
		actions:    make(map[string]map[string]int),
		recoveries: make(map[string]int),
	}
}

// Register adds an agent for monitoring.
func (sd *StuckDetector) Register(agentID string, config MonitorConfig) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.configs[agentID] = config
	sd.actions[agentID] = make(map[string]int)
}

// Heartbeat processes a heartbeat pulse.
func (sd *StuckDetector) Heartbeat(pulse Heartbeat) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	pulse.Timestamp = time.Now()
	sd.heartbeats[pulse.AgentID] = pulse
	// Reset consecutive errors on heartbeat
	sd.errors[pulse.AgentID] = 0
}

// RecordError records an error from an agent.
func (sd *StuckDetector) RecordError(agentID string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.errors[agentID]++
}

// RecordAction records an action from an agent (for circular detection).
func (sd *StuckDetector) RecordAction(agentID, action string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if sd.actions[agentID] == nil {
		sd.actions[agentID] = make(map[string]int)
	}
	sd.actions[agentID][action]++
}

// Check evaluates whether an agent is stuck.
func (sd *StuckDetector) Check(agentID string) *StuckReport {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	config, ok := sd.configs[agentID]
	if !ok {
		return &StuckReport{AgentID: agentID, Level: EscalateNone}
	}

	report := &StuckReport{AgentID: agentID}

	// Heuristic 1: No heartbeat for too long
	if hb, exists := sd.heartbeats[agentID]; exists {
		since := time.Since(hb.Timestamp)
		if since > config.NoOutputTimeout {
			report.Level = EscalateNotify
			report.Reasons = append(report.Reasons, fmt.Sprintf("no heartbeat for %v", since.Round(time.Minute)))
			report.LastAction = hb.Timestamp
			report.Duration = since
		}
	} else {
		// Never sent a heartbeat
		report.Level = EscalateLog
		report.Reasons = append(report.Reasons, "no heartbeat ever received")
	}

	// Heuristic 2: Too many consecutive errors
	if sd.errors[agentID] >= config.MaxConsecErrors {
		if report.Level < EscalateEscalate {
			report.Level = EscalateEscalate
		}
		report.Reasons = append(report.Reasons, fmt.Sprintf("%d consecutive errors", sd.errors[agentID]))
	}

	// Heuristic 3: Circular behavior (same action repeated)
	for action, count := range sd.actions[agentID] {
		if count >= config.MaxSameAction {
			if report.Level < EscalateNotify {
				report.Level = EscalateNotify
			}
			report.Reasons = append(report.Reasons, fmt.Sprintf("action '%s' repeated %d times", action, count))
		}
	}

	// Heuristic 4: No progress despite heartbeats
	if hb, exists := sd.heartbeats[agentID]; exists {
		if hb.Progress == 0 && time.Since(hb.Timestamp) < config.NoOutputTimeout {
			// Agent is pulsing but not making progress
			report.Reasons = append(report.Reasons, "heartbeats received but no progress")
			if report.Level < EscalateLog {
				report.Level = EscalateLog
			}
		}
	}

	report.RecoveryTried = sd.recoveries[agentID]
	return report
}

// Recover attempts automated recovery for a stuck agent.
func (sd *StuckDetector) Recover(agentID string) (*RecoveryResult, error) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	config, ok := sd.configs[agentID]
	if !ok {
		return nil, fmt.Errorf("agent %s not registered", agentID)
	}
	if !config.AutoRecover {
		return nil, fmt.Errorf("auto-recovery disabled for agent %s", agentID)
	}

	sd.recoveries[agentID]++
	attempt := sd.recoveries[agentID]

	action := RecoveryAction{
		Timestamp: time.Now(),
	}

	// Recovery strategy depends on attempt number
	switch {
	case attempt == 1:
		action.Type = "context_inject"
		action.Detail = "reminding agent of task context and constraints"
	case attempt == 2:
		action.Type = "approach_switch"
		action.Detail = "suggesting alternative approach to current task"
	case attempt == 3:
		action.Type = "peer_assist"
		action.Detail = "routing stuck point to peer agent for input"
	case attempt <= 5:
		action.Type = "task_decompose"
		action.Detail = fmt.Sprintf("breaking task into smaller pieces (attempt %d)", attempt)
	default:
		action.Type = "escalate_human"
		action.Detail = "all recovery attempts exhausted, human intervention needed"
	}

	// Reset tracking state for fresh detection
	sd.errors[agentID] = 0
	sd.actions[agentID] = make(map[string]int)

	return &RecoveryResult{
		AgentID:   agentID,
		Recovered: attempt <= 5, // optimistic
		Action:    action,
	}, nil
}

// Metrics returns stuck detection statistics.
func (sd *StuckDetector) Metrics() *StuckMetrics {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	metrics := &StuckMetrics{
		PerAgent: make(map[string]int),
	}
	for agentID := range sd.configs {
		stuckReport := sd.Check(agentID)
		if stuckReport.Level >= EscalateNotify {
			metrics.TotalDetected++
			metrics.PerAgent[agentID]++
		}
		if sd.recoveries[agentID] > 0 {
			metrics.TotalRecovered++
		}
	}
	return metrics
}

// Escalate triggers manual escalation.
func (sd *StuckDetector) Escalate(agentID string, level EscalationLevel) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if _, ok := sd.configs[agentID]; !ok {
		return fmt.Errorf("agent %s not registered", agentID)
	}
	// In production, this would notify the division head / human
	// For now, it's a stub that records the escalation
	return nil
}
