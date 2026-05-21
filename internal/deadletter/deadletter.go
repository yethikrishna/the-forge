// Package deadletter provides a dead letter queue for failed agent tasks.
// Failed tasks land in an inspectable queue instead of disappearing,
// allowing manual inspection, retry, or dismissal.
//
// Failed tasks deserve a second chance, not a silent death.
package deadletter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Reason represents why a task entered the dead letter queue.
type Reason string

const (
	ReasonTimeout       Reason = "timeout"
	ReasonProviderError Reason = "provider_error"
	ReasonRateLimit     Reason = "rate_limit"
	ReasonCostCap       Reason = "cost_cap"
	ReasonSandboxEscape Reason = "sandbox_escape"
	ReasonValidation    Reason = "validation_error"
	ReasonCircuitOpen   Reason = "circuit_open"
	ReasonUnknown       Reason = "unknown"
)

// Entry represents a dead-lettered task.
type Entry struct {
	ID         string                 `json:"id"`
	AgentID    string                 `json:"agent_id"`
	Task       string                 `json:"task"`
	Reason     Reason                 `json:"reason"`
	Error      string                 `json:"error"`
	Provider   string                 `json:"provider,omitempty"`
	Model      string                 `json:"model,omitempty"`
	CostUSD    float64                `json:"cost_usd,omitempty"`
	TokensUsed int64                  `json:"tokens_used,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	RetryCount int                    `json:"retry_count"`
	MaxRetries int                    `json:"max_retries"`
	Status     string                 `json:"status"` // pending, retried, dismissed, expired
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	ExpiresAt  *time.Time             `json:"expires_at,omitempty"`
}

// Store manages dead letter entries.
type Store struct {
	Dir string
}

// NewStore creates a dead letter store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Add places a task in the dead letter queue.
func (s *Store) Add(entry Entry) (*Entry, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dead letter dir: %w", err)
	}

	if entry.ID == "" {
		entry.ID = fmt.Sprintf("dl-%d", time.Now().UnixNano())
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now()
	}
	if entry.Status == "" {
		entry.Status = "pending"
	}
	if entry.MaxRetries == 0 {
		entry.MaxRetries = 3
	}

	if err := s.writeEntry(&entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// Get retrieves a dead letter entry by ID.
func (s *Store) Get(id string) (*Entry, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("dead letter entry %q not found", id)
		}
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to parse entry: %w", err)
	}

	return &entry, nil
}

// List returns all dead letter entries, newest first.
func (s *Store) List() ([]*Entry, error) {
	entries, err := s.readAll()
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	return entries, nil
}

// ListByReason returns entries for a specific reason.
func (s *Store) ListByReason(reason Reason) ([]*Entry, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	var filtered []*Entry
	for _, e := range all {
		if e.Reason == reason {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// ListByStatus returns entries with a specific status.
func (s *Store) ListByStatus(status string) ([]*Entry, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	var filtered []*Entry
	for _, e := range all {
		if e.Status == status {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// Retry marks an entry for retry and increments the retry count.
func (s *Store) Retry(id string) (*Entry, error) {
	entry, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	if entry.RetryCount >= entry.MaxRetries {
		return nil, fmt.Errorf("entry %q has exceeded max retries (%d/%d)", id, entry.RetryCount, entry.MaxRetries)
	}

	entry.RetryCount++
	entry.Status = "retried"
	entry.UpdatedAt = time.Now()

	if err := s.writeEntry(entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// Dismiss marks an entry as dismissed (will not be retried).
func (s *Store) Dismiss(id string) (*Entry, error) {
	entry, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	entry.Status = "dismissed"
	entry.UpdatedAt = time.Now()

	if err := s.writeEntry(entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// Purge removes expired entries.
func (s *Store) Purge() (int, error) {
	entries, err := s.List()
	if err != nil {
		return 0, err
	}

	now := time.Now()
	purged := 0
	for _, e := range entries {
		if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
			if err := s.Delete(e.ID); err == nil {
				purged++
			}
		}
	}

	return purged, nil
}

// Delete removes a dead letter entry.
func (s *Store) Delete(id string) error {
	return os.Remove(filepath.Join(s.Dir, id+".json"))
}

// Stats returns statistics about the dead letter queue.
type Stats struct {
	Total    int            `json:"total"`
	ByReason map[Reason]int `json:"by_reason"`
	ByStatus map[string]int `json:"by_status"`
	ByAgent  map[string]int `json:"by_agent"`
}

// Stats returns dead letter queue statistics.
func (s *Store) Stats() (*Stats, error) {
	entries, err := s.List()
	if err != nil {
		return nil, err
	}

	stats := &Stats{
		Total:    len(entries),
		ByReason: make(map[Reason]int),
		ByStatus: make(map[string]int),
		ByAgent:  make(map[string]int),
	}

	for _, e := range entries {
		stats.ByReason[e.Reason]++
		stats.ByStatus[e.Status]++
		stats.ByAgent[e.AgentID]++
	}

	return stats, nil
}

// FormatEntry renders an entry for display.
func FormatEntry(entry *Entry) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Entry: %s\n", entry.ID))
	sb.WriteString(fmt.Sprintf("  Agent:   %s\n", entry.AgentID))
	sb.WriteString(fmt.Sprintf("  Task:    %s\n", entry.Task))
	sb.WriteString(fmt.Sprintf("  Reason:  %s\n", entry.Reason))
	sb.WriteString(fmt.Sprintf("  Error:   %s\n", entry.Error))
	sb.WriteString(fmt.Sprintf("  Status:  %s (retry %d/%d)\n", entry.Status, entry.RetryCount, entry.MaxRetries))
	if entry.Provider != "" {
		sb.WriteString(fmt.Sprintf("  Provider: %s/%s\n", entry.Provider, entry.Model))
	}
	if entry.CostUSD > 0 {
		sb.WriteString(fmt.Sprintf("  Cost:    $%.4f\n", entry.CostUSD))
	}
	sb.WriteString(fmt.Sprintf("  Created: %s\n", entry.CreatedAt.Format(time.RFC3339)))

	return sb.String()
}

// FormatStats renders queue statistics for display.
func FormatStats(stats *Stats) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Dead Letter Queue: %d entries\n\n", stats.Total))

	sb.WriteString("By Reason:\n")
	for r, n := range stats.ByReason {
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", r, n))
	}

	sb.WriteString("\nBy Status:\n")
	for s, n := range stats.ByStatus {
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", s, n))
	}

	if len(stats.ByAgent) > 0 {
		sb.WriteString("\nBy Agent:\n")
		for a, n := range stats.ByAgent {
			sb.WriteString(fmt.Sprintf("  %-20s %d\n", a, n))
		}
	}

	return sb.String()
}

func (s *Store) readAll() ([]*Entry, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []*Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		entry, err := s.Get(id)
		if err != nil {
			continue
		}
		result = append(result, entry)
	}

	return result, nil
}

func (s *Store) writeEntry(entry *Entry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, entry.ID+".json"), data, 0o644)
}
