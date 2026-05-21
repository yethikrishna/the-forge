// Package ledger provides an immutable transaction ledger for tracking
// all agent costs, actions, and resource usage. Every entry is append-only
// and content-addressed, creating a tamper-evident audit trail that can
// be verified at any time.
//
// Every token, every dollar, every action — accounted for.
package ledger

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EntryType represents the type of ledger entry.
type EntryType string

const (
	EntryTokenUsage  EntryType = "token_usage"
	EntryCost        EntryType = "cost"
	EntryAction      EntryType = "action"
	EntryRefund      EntryType = "refund"
	EntryBudgetSet   EntryType = "budget_set"
	EntryBudgetAlert EntryType = "budget_alert"
	EntryTransfer    EntryType = "transfer"
)

// Entry represents a single ledger entry.
type Entry struct {
	Index       int64             `json:"index"`
	Hash        string            `json:"hash"`
	PrevHash    string            `json:"prev_hash"`
	Type        EntryType         `json:"type"`
	AgentID     string            `json:"agent_id"`
	SessionID   string            `json:"session_id"`
	Model       string            `json:"model,omitempty"`
	TokensIn    int64             `json:"tokens_in,omitempty"`
	TokensOut   int64             `json:"tokens_out,omitempty"`
	CostUSD     float64           `json:"cost_usd"`
	TotalCost   float64           `json:"total_cost"` // running total
	Action      string            `json:"action,omitempty"`
	Resource    string            `json:"resource,omitempty"`
	Duration    time.Duration     `json:"duration,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Ledger is the append-only transaction ledger.
type Ledger struct {
	mu        sync.RWMutex
	dir       string
	entries   []Entry
	agentCost map[string]float64 // agent -> cumulative cost
	modelCost map[string]float64 // model -> cumulative cost
	sessionCost map[string]float64 // session -> cumulative cost
	totalCost float64
	budget    float64
	nextIndex int64
}

// NewLedger creates a new ledger.
func NewLedger(dir string) *Ledger {
	return &Ledger{
		dir:         dir,
		entries:     make([]Entry, 0),
		agentCost:   make(map[string]float64),
		modelCost:   make(map[string]float64),
		sessionCost: make(map[string]float64),
		budget:      -1, // -1 means no budget
	}
}

// SetBudget sets a cost budget. If total cost exceeds this, budget alerts fire.
func (l *Ledger) SetBudget(budget float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.budget = budget

	l.append(Entry{
		Type:      EntryBudgetSet,
		CostUSD:   budget,
		Timestamp: time.Now(),
		Metadata:  map[string]string{"budget": fmt.Sprintf("%.2f", budget)},
	})
}

// RecordUsage records token usage and cost.
func (l *Ledger) RecordUsage(agentID, sessionID, model string, tokensIn, tokensOut int64, costUSD float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check budget
	if l.budget > 0 && l.totalCost+costUSD > l.budget {
		l.append(Entry{
			Type:      EntryBudgetAlert,
			AgentID:   agentID,
			SessionID: sessionID,
			CostUSD:   costUSD,
			TotalCost: l.totalCost + costUSD,
			Timestamp: time.Now(),
			Metadata:  map[string]string{"budget": fmt.Sprintf("%.2f", l.budget), "message": "budget exceeded"},
		})
		return fmt.Errorf("budget exceeded: $%.2f > $%.2f", l.totalCost+costUSD, l.budget)
	}

	l.agentCost[agentID] += costUSD
	l.modelCost[model] += costUSD
	l.sessionCost[sessionID] += costUSD
	l.totalCost += costUSD

	l.append(Entry{
		Type:      EntryTokenUsage,
		AgentID:   agentID,
		SessionID: sessionID,
		Model:     model,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		CostUSD:   costUSD,
		TotalCost: l.totalCost,
		Timestamp: time.Now(),
	})

	return nil
}

// RecordAction records an agent action.
func (l *Ledger) RecordAction(agentID, sessionID, action, resource string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.append(Entry{
		Type:      EntryAction,
		AgentID:   agentID,
		SessionID: sessionID,
		Action:    action,
		Resource:  resource,
		Duration:  duration,
		TotalCost: l.totalCost,
		Timestamp: time.Now(),
	})
}

// RecordRefund records a cost refund.
func (l *Ledger) RecordRefund(agentID, sessionID string, amount float64, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.agentCost[agentID] -= amount
	l.totalCost -= amount

	l.append(Entry{
		Type:      EntryRefund,
		AgentID:   agentID,
		SessionID: sessionID,
		CostUSD:   -amount,
		TotalCost: l.totalCost,
		Timestamp: time.Now(),
		Metadata:  map[string]string{"reason": reason},
	})
}

// Entries returns all entries.
func (l *Ledger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// EntriesByAgent returns entries for a specific agent.
func (l *Ledger) EntriesByAgent(agentID string) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []Entry
	for _, e := range l.entries {
		if e.AgentID == agentID {
			result = append(result, e)
		}
	}
	return result
}

// EntriesByType returns entries of a specific type.
func (l *Ledger) EntriesByType(entryType EntryType) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []Entry
	for _, e := range l.entries {
		if e.Type == entryType {
			result = append(result, e)
		}
	}
	return result
}

// AgentCost returns the total cost for an agent.
func (l *Ledger) AgentCost(agentID string) float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.agentCost[agentID]
}

// ModelCost returns the total cost for a model.
func (l *Ledger) ModelCost(model string) float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.modelCost[model]
}

// TotalCost returns the overall total cost.
func (l *Ledger) TotalCost() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.totalCost
}

// BudgetRemaining returns how much budget is left.
func (l *Ledger) BudgetRemaining() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.budget < 0 {
		return -1 // no budget set
	}
	return l.budget - l.totalCost
}

// Stats returns ledger statistics.
func (l *Ledger) Stats() LedgerStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := LedgerStats{
		TotalEntries: len(l.entries),
		TotalCost:    l.totalCost,
		Budget:       l.budget,
		AgentCount:   len(l.agentCost),
		ModelCount:   len(l.modelCost),
		SessionCount: len(l.sessionCost),
	}

	stats.AgentBreakdown = make(map[string]float64)
	for k, v := range l.agentCost {
		stats.AgentBreakdown[k] = v
	}
	stats.ModelBreakdown = make(map[string]float64)
	for k, v := range l.modelCost {
		stats.ModelBreakdown[k] = v
	}

	for _, e := range l.entries {
		stats.TotalTokensIn += e.TokensIn
		stats.TotalTokensOut += e.TokensOut
	}

	return stats
}

// LedgerStats holds ledger statistics.
type LedgerStats struct {
	TotalEntries    int                `json:"total_entries"`
	TotalCost       float64            `json:"total_cost"`
	TotalTokensIn   int64              `json:"total_tokens_in"`
	TotalTokensOut  int64              `json:"total_tokens_out"`
	Budget          float64            `json:"budget"`
	AgentCount      int                `json:"agent_count"`
	ModelCount      int                `json:"model_count"`
	SessionCount    int                `json:"session_count"`
	AgentBreakdown  map[string]float64 `json:"agent_breakdown"`
	ModelBreakdown  map[string]float64 `json:"model_breakdown"`
}

// Verify checks the integrity of the ledger by verifying hashes.
func (l *Ledger) Verify() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for i, entry := range l.entries {
		// Verify index
		if entry.Index != int64(i) {
			return fmt.Errorf("entry %d has wrong index %d", i, entry.Index)
		}

		// Verify hash
		expected := l.computeHash(entry)
		if entry.Hash != expected {
			return fmt.Errorf("entry %d has wrong hash: expected %s, got %s", i, expected, entry.Hash)
		}

		// Verify prev hash chain
		if i > 0 && entry.PrevHash != l.entries[i-1].Hash {
			return fmt.Errorf("entry %d has broken prev_hash chain", i)
		}
	}

	return nil
}

// Save persists the ledger to disk.
func (l *Ledger) Save() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if err := os.MkdirAll(l.dir, 0755); err != nil {
		return fmt.Errorf("create ledger dir: %w", err)
	}

	data, err := json.MarshalIndent(l.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal entries: %w", err)
	}

	return os.WriteFile(filepath.Join(l.dir, "ledger.json"), data, 0644)
}

// Load reads the ledger from disk.
func (l *Ledger) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(l.dir, "ledger.json"))
	if err != nil {
		return fmt.Errorf("read ledger: %w", err)
	}

	if err := json.Unmarshal(data, &l.entries); err != nil {
		return fmt.Errorf("unmarshal entries: %w", err)
	}

	// Rebuild running state
	l.totalCost = 0
	l.agentCost = make(map[string]float64)
	l.modelCost = make(map[string]float64)
	l.sessionCost = make(map[string]float64)

	for _, e := range l.entries {
		l.totalCost = e.TotalCost
		l.agentCost[e.AgentID] += e.CostUSD
		l.modelCost[e.Model] += e.CostUSD
		l.sessionCost[e.SessionID] += e.CostUSD
	}

	if len(l.entries) > 0 {
		l.nextIndex = l.entries[len(l.entries)-1].Index + 1
	}

	return nil
}

// ExportMarkdown exports the ledger as markdown.
func (l *Ledger) ExportMarkdown() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := l.Stats()

	var b strings.Builder
	fmt.Fprintf(&b, "# Agent Ledger\n\n")
	fmt.Fprintf(&b, "**Total Cost:** $%.4f | **Entries:** %d | **Agents:** %d | **Models:** %d\n\n",
		stats.TotalCost, stats.TotalEntries, stats.AgentCount, stats.ModelCount)

	if len(stats.AgentBreakdown) > 0 {
		b.WriteString("## Cost by Agent\n\n")
		agents := make([]string, 0, len(stats.AgentBreakdown))
		for a := range stats.AgentBreakdown {
			agents = append(agents, a)
		}
		sort.Strings(agents)
		for _, a := range agents {
			fmt.Fprintf(&b, "- **%s:** $%.4f\n", a, stats.AgentBreakdown[a])
		}
		b.WriteString("\n")
	}

	if len(stats.ModelBreakdown) > 0 {
		b.WriteString("## Cost by Model\n\n")
		models := make([]string, 0, len(stats.ModelBreakdown))
		for m := range stats.ModelBreakdown {
			models = append(models, m)
		}
		sort.Strings(models)
		for _, m := range models {
			fmt.Fprintf(&b, "- **%s:** $%.4f\n", m, stats.ModelBreakdown[m])
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Internal methods

func (l *Ledger) append(entry Entry) {
	var prevHash string
	if len(l.entries) > 0 {
		prevHash = l.entries[len(l.entries)-1].Hash
	}

	entry.Index = l.nextIndex
	entry.PrevHash = prevHash
	entry.Hash = l.computeHash(entry)
	l.nextIndex++

	l.entries = append(l.entries, entry)
}

func (l *Ledger) computeHash(entry Entry) string {
	// Hash everything except the hash field itself
	copy := entry
	copy.Hash = ""
	data, _ := json.Marshal(copy)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:16])
}
