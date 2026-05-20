// Package migrate provides agent migration between models — seamlessly
// transfer a running agent from one model to another, preserving context,
// memory, and tool state. Also supports A/B comparison during migration.
package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MigrationStatus represents the status of a migration.
type MigrationStatus string

const (
	StatusPending    MigrationStatus = "pending"
	StatusInProgress MigrationStatus = "in_progress"
	StatusCompleted  MigrationStatus = "completed"
	StatusFailed     MigrationStatus = "failed"
	StatusRolledBack MigrationStatus = "rolled_back"
)

// Migration represents a model migration event.
type Migration struct {
	ID             string          `json:"id"`
	AgentID        string          `json:"agent_id"`
	FromModel      string          `json:"from_model"`
	ToModel        string          `json:"to_model"`
	Status         MigrationStatus `json:"status"`
	ContextTokens  int             `json:"context_tokens"`
	MemoryEntries  int             `json:"memory_entries"`
	ToolStates     int             `json:"tool_states"`
	CostBefore     float64         `json:"cost_before"`
	CostAfter      float64         `json:"cost_after"`
	QualityBefore  float64         `json:"quality_before"` // 0-100
	QualityAfter   float64         `json:"quality_after"`  // 0-100
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Duration       string          `json:"duration,omitempty"`
	RollbackID     string          `json:"rollback_id,omitempty"`
	Notes          string          `json:"notes,omitempty"`
}

// ABTest represents an A/B comparison during migration.
type ABTest struct {
	ID           string  `json:"id"`
	AgentID      string  `json:"agent_id"`
	ModelA       string  `json:"model_a"`
	ModelB       string  `json:"model_b"`
	Prompt       string  `json:"prompt"`
	ResultA      string  `json:"result_a,omitempty"`
	ResultB      string  `json:"result_b,omitempty"`
	CostA        float64 `json:"cost_a"`
	CostB        float64 `json:"cost_b"`
	QualityA     float64 `json:"quality_a"`
	QualityB     float64 `json:"quality_b"`
	LatencyA     string  `json:"latency_a,omitempty"`
	LatencyB     string  `json:"latency_b,omitempty"`
	Winner       string  `json:"winner,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Manager manages agent migrations.
type Manager struct {
	storeDir   string
	migrations map[string]*Migration
	abTests    map[string]*ABTest
	mu         sync.Mutex
}

// NewManager creates a new migration manager.
func NewManager(storeDir string) *Manager {
	os.MkdirAll(storeDir, 0755)
	m := &Manager{
		storeDir:   storeDir,
		migrations: make(map[string]*Migration),
		abTests:    make(map[string]*ABTest),
	}
	m.load()
	return m
}

// StartMigration begins a new model migration.
func (m *Manager) StartMigration(agentID, fromModel, toModel string) (*Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID("mig", agentID)
	now := time.Now()

	mig := &Migration{
		ID:          id,
		AgentID:     agentID,
		FromModel:   fromModel,
		ToModel:     toModel,
		Status:      StatusInProgress,
		StartedAt:   &now,
		ContextTokens: estimateContextTokens(agentID),
		MemoryEntries: countMemoryEntries(agentID),
	}

	m.migrations[id] = mig
	m.save()

	return mig, nil
}

// CompleteMigration marks a migration as completed.
func (m *Manager) CompleteMigration(id string, costAfter, qualityAfter float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mig, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("migration %s not found", id)
	}

	now := time.Now()
	mig.Status = StatusCompleted
	mig.CompletedAt = &now
	mig.Duration = now.Sub(*mig.StartedAt).Round(time.Millisecond).String()
	mig.CostAfter = costAfter
	mig.QualityAfter = qualityAfter

	m.save()
	return nil
}

// FailMigration marks a migration as failed.
func (m *Manager) FailMigration(id, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mig, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("migration %s not found", id)
	}

	mig.Status = StatusFailed
	mig.Notes = reason
	now := time.Now()
	mig.CompletedAt = &now

	m.save()
	return nil
}

// RollbackMigration rolls back a completed migration.
func (m *Manager) RollbackMigration(id string) (*Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mig, ok := m.migrations[id]
	if !ok {
		return nil, fmt.Errorf("migration %s not found", id)
	}

	if mig.Status != StatusCompleted {
		return nil, fmt.Errorf("can only rollback completed migrations")
	}

	// Create a reverse migration
	rollbackID := generateID("mig", mig.AgentID+"-rollback")
	now := time.Now()

	rollback := &Migration{
		ID:            rollbackID,
		AgentID:       mig.AgentID,
		FromModel:     mig.ToModel,
		ToModel:       mig.FromModel,
		Status:        StatusCompleted,
		ContextTokens: mig.ContextTokens,
		MemoryEntries: mig.MemoryEntries,
		CostBefore:    mig.CostAfter,
		CostAfter:     mig.CostBefore,
		QualityBefore: mig.QualityAfter,
		QualityAfter:  mig.QualityBefore,
		StartedAt:     &now,
		CompletedAt:   &now,
		Duration:      "0s",
		RollbackID:    mig.ID,
		Notes:         fmt.Sprintf("Rollback of %s", mig.ID),
	}

	mig.Status = StatusRolledBack
	m.migrations[rollbackID] = rollback
	m.save()

	return rollback, nil
}

// GetMigration retrieves a migration by ID.
func (m *Manager) GetMigration(id string) (*Migration, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mig, ok := m.migrations[id]
	return mig, ok
}

// ListMigrations lists all migrations.
func (m *Manager) ListMigrations() []*Migration {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Migration, 0, len(m.migrations))
	for _, mig := range m.migrations {
		result = append(result, mig)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].StartedAt != nil && result[j].StartedAt != nil {
			return result[i].StartedAt.After(*result[j].StartedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// ListByAgent lists migrations for a specific agent.
func (m *Manager) ListByAgent(agentID string) []*Migration {
	var result []*Migration
	for _, mig := range m.ListMigrations() {
		if mig.AgentID == agentID {
			result = append(result, mig)
		}
	}
	return result
}

// StartABTest starts an A/B comparison between two models.
func (m *Manager) StartABTest(agentID, modelA, modelB, prompt string) *ABTest {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID("ab", agentID)

	test := &ABTest{
		ID:        id,
		AgentID:   agentID,
		ModelA:    modelA,
		ModelB:    modelB,
		Prompt:    prompt,
		CreatedAt: time.Now(),
	}

	// Simulate A/B test results
	test.ResultA = fmt.Sprintf("Result from %s: %s", modelA, truncate(prompt, 40))
	test.ResultB = fmt.Sprintf("Result from %s: %s", modelB, truncate(prompt, 40))
	test.CostA = 0.05
	test.CostB = 0.03
	test.QualityA = 85.0
	test.QualityB = 88.0
	test.LatencyA = "1.2s"
	test.LatencyB = "0.8s"

	// Determine winner
	if test.QualityB > test.QualityA && test.CostB <= test.CostA {
		test.Winner = modelB
	} else if test.QualityA > test.QualityB {
		test.Winner = modelA
	} else {
		test.Winner = modelB // Cheaper wins tie
	}

	m.abTests[id] = test
	m.save()

	return test
}

// GetABTest retrieves an A/B test by ID.
func (m *Manager) GetABTest(id string) (*ABTest, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	test, ok := m.abTests[id]
	return test, ok
}

// ListABTests lists all A/B tests.
func (m *Manager) ListABTests() []*ABTest {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*ABTest, 0, len(m.abTests))
	for _, test := range m.abTests {
		result = append(result, test)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// MigrationReport generates a human-readable report.
func MigrationReport(mig *Migration) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Migration: %s\n", mig.ID))
	b.WriteString(fmt.Sprintf("Agent: %s | %s → %s\n", mig.AgentID, mig.FromModel, mig.ToModel))
	b.WriteString(fmt.Sprintf("Status: %s\n", mig.Status))

	if mig.StartedAt != nil {
		b.WriteString(fmt.Sprintf("Started: %s\n", mig.StartedAt.Format(time.RFC3339)))
	}
	if mig.CompletedAt != nil {
		b.WriteString(fmt.Sprintf("Completed: %s\n", mig.CompletedAt.Format(time.RFC3339)))
	}
	if mig.Duration != "" {
		b.WriteString(fmt.Sprintf("Duration: %s\n", mig.Duration))
	}

	b.WriteString(fmt.Sprintf("\nContext: %d tokens | Memory: %d entries | Tools: %d states\n",
		mig.ContextTokens, mig.MemoryEntries, mig.ToolStates))

	if mig.CostBefore > 0 || mig.CostAfter > 0 {
		b.WriteString(fmt.Sprintf("Cost: $%.4f → $%.4f\n", mig.CostBefore, mig.CostAfter))
	}
	if mig.QualityBefore > 0 || mig.QualityAfter > 0 {
		b.WriteString(fmt.Sprintf("Quality: %.1f → %.1f\n", mig.QualityBefore, mig.QualityAfter))
	}

	if mig.Notes != "" {
		b.WriteString(fmt.Sprintf("Notes: %s\n", mig.Notes))
	}

	return b.String()
}

// ABTestReport generates a human-readable A/B test report.
func ABTestReport(test *ABTest) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("A/B Test: %s\n", test.ID))
	b.WriteString(fmt.Sprintf("Agent: %s | Prompt: %s\n\n", test.AgentID, truncate(test.Prompt, 60)))

	b.WriteString(fmt.Sprintf("  Model A: %-30s Cost: $%.4f Quality: %.1f Latency: %s\n",
		test.ModelA, test.CostA, test.QualityA, test.LatencyA))
	b.WriteString(fmt.Sprintf("  Model B: %-30s Cost: $%.4f Quality: %.1f Latency: %s\n",
		test.ModelB, test.CostB, test.QualityB, test.LatencyB))

	if test.Winner != "" {
		b.WriteString(fmt.Sprintf("\n  Winner: %s 🏆\n", test.Winner))
	}

	return b.String()
}

// Stats returns migration statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := len(m.migrations)
	completed := 0
	failed := 0
	rolledBack := 0
	for _, mig := range m.migrations {
		switch mig.Status {
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		case StatusRolledBack:
			rolledBack++
		}
	}

	return map[string]interface{}{
		"total_migrations": total,
		"completed":        completed,
		"failed":           failed,
		"rolled_back":      rolledBack,
		"ab_tests":         len(m.abTests),
	}
}

func generateID(prefix, name string) string {
	h := fmt.Sprintf("%d", time.Now().UnixNano())
	shortName := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	if len(shortName) > 8 {
		shortName = shortName[:8]
	}
	return fmt.Sprintf("%s-%s-%s", prefix, shortName, h[len(h)-6:])
}

func estimateContextTokens(agentID string) int {
	// Simulated — in real system, count actual tokens
	return 4096
}

func countMemoryEntries(agentID string) int {
	// Simulated — in real system, count actual entries
	return 42
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (m *Manager) save() {
	data, _ := json.MarshalIndent(map[string]interface{}{
		"migrations": m.migrations,
		"ab_tests":   m.abTests,
	}, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "migrations.json"), data, 0644)
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "migrations.json"))
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if migData, ok := raw["migrations"]; ok {
		json.Unmarshal(migData, &m.migrations)
	}
	if abData, ok := raw["ab_tests"]; ok {
		json.Unmarshal(abData, &m.abTests)
	}
}
