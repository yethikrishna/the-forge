// Package trust provides composite trust scoring for agents.
// Aggregates feedback, undo rate, test results, and security findings
// into a single 0-100 trust score.
//
// Trust is earned, not given.
package trust

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrustLevel represents trustworthiness.
type TrustLevel string

const (
	LevelUntrusted TrustLevel = "untrusted" // 0-25
	LevelRisky     TrustLevel = "risky"     // 26-50
	LevelCautious  TrustLevel = "cautious"  // 51-75
	LevelTrusted   TrustLevel = "trusted"   // 76-90
	LevelVerified  TrustLevel = "verified"  // 91-100
)

// AgentRecord tracks trust metrics for one agent.
type AgentRecord struct {
	AgentID          string        `json:"agent_id"`
	TrustScore       float64       `json:"trust_score"`
	TotalActions     int           `json:"total_actions"`
	SuccessActions   int           `json:"success_actions"`
	UndoneActions    int           `json:"undone_actions"`
	FeedbackPositive int           `json:"feedback_positive"`
	FeedbackNegative int           `json:"feedback_negative"`
	TestsPassed      int           `json:"tests_passed"`
	TestsFailed      int           `json:"tests_failed"`
	SecurityIssues   int           `json:"security_issues"`
	LastActionAt     time.Time     `json:"last_action_at"`
	CreatedAt        time.Time     `json:"created_at"`
	History          []ScoreChange `json:"history,omitempty"`
}

// ScoreChange records a trust score change.
type ScoreChange struct {
	Timestamp time.Time `json:"timestamp"`
	OldScore  float64   `json:"old_score"`
	NewScore  float64   `json:"new_score"`
	Reason    string    `json:"reason"`
}

// TrustLevelFor converts a score to a trust level.
func TrustLevelFor(score float64) TrustLevel {
	switch {
	case score >= 91:
		return LevelVerified
	case score >= 76:
		return LevelTrusted
	case score >= 51:
		return LevelCautious
	case score >= 26:
		return LevelRisky
	default:
		return LevelUntrusted
	}
}

// Manager manages trust scores for all agents.
type Manager struct {
	agents map[string]*AgentRecord
	dir    string
	mu     sync.RWMutex
}

// NewManager creates a trust manager.
func NewManager(dir string) *Manager {
	m := &Manager{
		agents: make(map[string]*AgentRecord),
		dir:    dir,
	}
	m.load()
	return m
}

// RecordAction records an agent action result.
func (m *Manager) RecordAction(agentID string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.getOrCreate(agentID)
	r.TotalActions++
	r.LastActionAt = time.Now()

	if success {
		r.SuccessActions++
		m.adjustScore(r, 0.5, "successful action")
	} else {
		m.adjustScore(r, -2.0, "failed action")
	}

	m.save()
}

// RecordUndo records that an action was undone.
func (m *Manager) RecordUndo(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.getOrCreate(agentID)
	r.UndoneActions++
	m.adjustScore(r, -5.0, "action undone")
	m.save()
}

// RecordFeedback records user feedback.
func (m *Manager) RecordFeedback(agentID string, positive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.getOrCreate(agentID)
	if positive {
		r.FeedbackPositive++
		m.adjustScore(r, 3.0, "positive feedback")
	} else {
		r.FeedbackNegative++
		m.adjustScore(r, -5.0, "negative feedback")
	}
	m.save()
}

// RecordTestResult records a test result.
func (m *Manager) RecordTestResult(agentID string, passed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.getOrCreate(agentID)
	if passed {
		r.TestsPassed++
		m.adjustScore(r, 2.0, "test passed")
	} else {
		r.TestsFailed++
		m.adjustScore(r, -3.0, "test failed")
	}
	m.save()
}

// RecordSecurityIssue records a security finding.
func (m *Manager) RecordSecurityIssue(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.getOrCreate(agentID)
	r.SecurityIssues++
	m.adjustScore(r, -10.0, "security issue")
	m.save()
}

// GetScore returns the trust score for an agent.
func (m *Manager) GetScore(agentID string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.agents[agentID]
	if !ok {
		return 0, false
	}
	return r.TrustScore, true
}

// GetRecord returns the full record for an agent.
func (m *Manager) GetRecord(agentID string) (*AgentRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.agents[agentID]
	if !ok {
		return nil, false
	}
	copy := *r
	return &copy, true
}

// ListAgents returns all agent IDs.
func (m *Manager) ListAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.agents))
	for id := range m.agents {
		ids = append(ids, id)
	}
	return ids
}

// Recalculate recomputes the trust score from raw metrics.
func (m *Manager) Recalculate(agentID string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.agents[agentID]
	if !ok {
		return 0, fmt.Errorf("agent %q not found", agentID)
	}

	score := m.computeScore(r)
	r.TrustScore = score
	m.save()
	return score, nil
}

// computeScore calculates trust from metrics.
func (m *Manager) computeScore(r *AgentRecord) float64 {
	score := 50.0 // start neutral

	// Success rate (max ±25)
	if r.TotalActions > 0 {
		successRate := float64(r.SuccessActions) / float64(r.TotalActions)
		score += successRate * 25
		score -= (1 - successRate) * 10
	}

	// Undo penalty (max -15)
	if r.TotalActions > 0 {
		undoRate := float64(r.UndoneActions) / float64(r.TotalActions)
		score -= undoRate * 15
	}

	// Feedback (max ±15)
	totalFeedback := r.FeedbackPositive + r.FeedbackNegative
	if totalFeedback > 0 {
		positiveRate := float64(r.FeedbackPositive) / float64(totalFeedback)
		score += positiveRate * 15
		score -= (1 - positiveRate) * 10
	}

	// Tests (max ±15)
	totalTests := r.TestsPassed + r.TestsFailed
	if totalTests > 0 {
		passRate := float64(r.TestsPassed) / float64(totalTests)
		score += passRate * 15
	}

	// Security issues (max -20)
	score -= float64(r.SecurityIssues) * 5

	// Clamp to 0-100
	return math.Max(0, math.Min(100, math.Round(score*100)/100))
}

func (m *Manager) adjustScore(r *AgentRecord, delta float64, reason string) {
	old := r.TrustScore
	r.TrustScore = math.Max(0, math.Min(100, r.TrustScore+delta))

	r.History = append(r.History, ScoreChange{
		Timestamp: time.Now(),
		OldScore:  old,
		NewScore:  r.TrustScore,
		Reason:    reason,
	})

	// Keep last 50 history entries
	if len(r.History) > 50 {
		r.History = r.History[len(r.History)-50:]
	}
}

func (m *Manager) getOrCreate(agentID string) *AgentRecord {
	r, ok := m.agents[agentID]
	if !ok {
		r = &AgentRecord{
			AgentID:    agentID,
			TrustScore: 50, // start neutral
			CreatedAt:  time.Now(),
		}
		m.agents[agentID] = r
	}
	return r
}

func (m *Manager) save() {
	if m.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(m.agents, "", "  ")
	os.MkdirAll(m.dir, 0755)
	os.WriteFile(filepath.Join(m.dir, "trust.json"), data, 0644)
}

func (m *Manager) load() {
	if m.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.dir, "trust.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.agents)
}

// FormatRecord formats an agent record for display.
func FormatRecord(r *AgentRecord) string {
	level := TrustLevelFor(r.TrustScore)
	s := fmt.Sprintf("Agent:     %s\n", r.AgentID)
	s += fmt.Sprintf("Score:     %.1f/100 [%s]\n", r.TrustScore, level)
	s += fmt.Sprintf("Actions:   %d total, %d success\n", r.TotalActions, r.SuccessActions)
	s += fmt.Sprintf("Undone:    %d\n", r.UndoneActions)
	s += fmt.Sprintf("Feedback:  %d positive, %d negative\n", r.FeedbackPositive, r.FeedbackNegative)
	s += fmt.Sprintf("Tests:     %d passed, %d failed\n", r.TestsPassed, r.TestsFailed)
	s += fmt.Sprintf("Security:  %d issues\n", r.SecurityIssues)
	return s
}
