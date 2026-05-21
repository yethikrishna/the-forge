// Package migrate provides database schema migration management.
// Track, apply, and rollback migrations with versioned SQL files.
// Supports forward-only and reversible migrations, with checksum verification.
//
// Evolve your schema. Never lose data.
package migrate

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

// Direction indicates migration direction.
type Direction string

const (
	DirUp   Direction = "up"
	DirDown Direction = "down"
)

// Status represents the state of a migration.
type Status string

const (
	StatusPending    Status = "pending"
	StatusApplied    Status = "applied"
	StatusRolledBack Status = "rolled_back"
	StatusFailed     Status = "failed"
)

// Migration represents a single migration.
type Migration struct {
	ID           string    `json:"id"`
	Version      int       `json:"version"`
	Name         string    `json:"name"`
	UpSQL        string    `json:"up_sql"`
	DownSQL      string    `json:"down_sql"`
	Checksum     string    `json:"checksum"`
	Status       Status    `json:"status"`
	AppliedAt    time.Time `json:"applied_at,omitempty"`
	RolledBackAt time.Time `json:"rolled_back_at,omitempty"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Manager manages migrations.
type Manager struct {
	dir        string
	migrations []Migration
	mu         sync.RWMutex
}

// NewManager creates a new migration manager.
func NewManager(dir string) *Manager {
	os.MkdirAll(dir, 0755)
	m := &Manager{
		dir: dir,
	}
	m.load()
	return m
}

// Create creates a new migration.
func (m *Manager) Create(name string) (*Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	version := 1
	if len(m.migrations) > 0 {
		version = m.migrations[len(m.migrations)-1].Version + 1
	}

	// Check for duplicate name
	for _, existing := range m.migrations {
		if existing.Name == name {
			return nil, fmt.Errorf("migration %q already exists", name)
		}
	}

	slug := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	upSQL := fmt.Sprintf("-- Migration %d: %s\n-- Write your UP migration here\n", version, name)
	downSQL := fmt.Sprintf("-- Rollback %d: %s\n-- Write your DOWN migration here\n", version, name)

	h := sha256.Sum256([]byte(upSQL + downSQL))

	migration := Migration{
		ID:        fmt.Sprintf("mig-%d-%s", version, slug),
		Version:   version,
		Name:      name,
		UpSQL:     upSQL,
		DownSQL:   downSQL,
		Checksum:  fmt.Sprintf("%x", h[:8]),
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}

	m.migrations = append(m.migrations, migration)
	m.save()

	return &migration, nil
}

// Apply applies pending migrations up to a target version (0 = all).
func (m *Manager) Apply(targetVersion int) ([]Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	var applied []Migration
	for i := range m.migrations {
		mig := &m.migrations[i]
		if mig.Status != StatusPending {
			continue
		}
		if targetVersion > 0 && mig.Version > targetVersion {
			break
		}

		// Verify checksum
		h := sha256.Sum256([]byte(mig.UpSQL + mig.DownSQL))
		expected := fmt.Sprintf("%x", h[:8])
		if mig.Checksum != expected {
			mig.Status = StatusFailed
			mig.Error = fmt.Sprintf("checksum mismatch: expected %s, got %s", mig.Checksum, expected)
			m.save()
			return applied, fmt.Errorf("migration %s: checksum mismatch", mig.ID)
		}

		mig.Status = StatusApplied
		mig.AppliedAt = time.Now()
		applied = append(applied, *mig)
	}

	m.save()
	return applied, nil
}

// Rollback rolls back the most recent migration or down to a target version.
func (m *Manager) Rollback(targetVersion int) ([]Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version > m.migrations[j].Version // descending
	})

	var rolled []Migration
	for i := range m.migrations {
		mig := &m.migrations[i]
		if mig.Status != StatusApplied {
			continue
		}
		if targetVersion > 0 && mig.Version <= targetVersion {
			break
		}

		mig.Status = StatusRolledBack
		mig.RolledBackAt = time.Now()
		rolled = append(rolled, *mig)
	}

	m.save()
	return rolled, nil
}

// Get returns a migration by ID.
func (m *Manager) Get(id string) (*Migration, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.migrations {
		if m.migrations[i].ID == id {
			copy := m.migrations[i]
			return &copy, true
		}
	}
	return nil, false
}

// List returns all migrations.
func (m *Manager) List() []Migration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Migration, len(m.migrations))
	copy(result, m.migrations)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

// Pending returns pending migrations.
func (m *Manager) Pending() []Migration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Migration
	for _, mig := range m.migrations {
		if mig.Status == StatusPending {
			result = append(result, mig)
		}
	}
	return result
}

// CurrentVersion returns the current applied version.
func (m *Manager) CurrentVersion() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	maxVer := 0
	for _, mig := range m.migrations {
		if mig.Status == StatusApplied && mig.Version > maxVer {
			maxVer = mig.Version
		}
	}
	return maxVer
}

// Status returns migration status summary.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[Status]int)
	for _, mig := range m.migrations {
		counts[mig.Status]++
	}

	return map[string]interface{}{
		"total":           len(m.migrations),
		"current_version": m.CurrentVersion(),
		"by_status":       counts,
	}
}

// RenderMigration renders a migration for display.
func RenderMigration(m *Migration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Migration: %s\n", m.Name)
	fmt.Fprintf(&b, "ID: %s\n", m.ID)
	fmt.Fprintf(&b, "Version: %d\n", m.Version)
	fmt.Fprintf(&b, "Status: %s\n", m.Status)
	fmt.Fprintf(&b, "Checksum: %s\n", m.Checksum)
	if !m.AppliedAt.IsZero() {
		fmt.Fprintf(&b, "Applied: %s\n", m.AppliedAt.Format(time.RFC3339))
	}
	if !m.RolledBackAt.IsZero() {
		fmt.Fprintf(&b, "Rolled Back: %s\n", m.RolledBackAt.Format(time.RFC3339))
	}
	if m.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", m.Error)
	}
	return b.String()
}

func (m *Manager) save() {
	if m.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(m.migrations, "", "  ")
	os.WriteFile(filepath.Join(m.dir, "migrations.json"), data, 0644)
}

func (m *Manager) load() {
	if m.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.dir, "migrations.json"))
	if err == nil {
		json.Unmarshal(data, &m.migrations)
	}
}
