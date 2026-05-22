// Package distributed provides cross-organization resilience through
// federation backup. Without distributed backup, a single org failure means
// total data loss. This package ensures every org has federation nodes that
// can restore its state, and measures the resilience of the overall network.
package distributed

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// FederationNode represents a node in the federation network.
type FederationNode struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	Endpoint   string    `json:"endpoint"`
	Region     string    `json:"region"`
	Status     string    `json:"status"` // "active", "standby", "offline"
	LastPing   time.Time `json:"last_ping"`
	Registered time.Time `json:"registered"`
}

// BackupRecord tracks a backup operation.
type BackupRecord struct {
	ID         string            `json:"id"`
	OrgID      string            `json:"org_id"`
	NodeID     string            `json:"node_id"`
	SizeBytes  int64             `json:"size_bytes"`
	Checksum   string            `json:"checksum"`
	Status     string            `json:"status"` // "completed", "failed", "in_progress"
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	CompletedAt time.Time        `json:"completed_at,omitempty"`
}

// ResilienceScore measures how resilient an org is to node failures.
type ResilienceScore struct {
	OrgID          string    `json:"org_id"`
	Score          float64   `json:"score"` // 0.0–1.0
	ActiveNodes    int       `json:"active_nodes"`
	TotalNodes     int       `json:"total_nodes"`
	LastBackupAge  string    `json:"last_backup_age"`
	CanRestore     bool      `json:"can_restore"`
	MeasuredAt     time.Time `json:"measured_at"`
}

// FederationConfig holds configuration for the federation.
type FederationConfig struct {
	MinNodesPerOrg  int           `json:"min_nodes_per_org"`
	BackupInterval  time.Duration `json:"backup_interval"`
	MaxBackupAge    time.Duration `json:"max_backup_age"`
	ReplicationFactor int         `json:"replication_factor"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() FederationConfig {
	return FederationConfig{
		MinNodesPerOrg:    3,
		BackupInterval:    time.Hour,
		MaxBackupAge:      24 * time.Hour,
		ReplicationFactor: 2,
	}
}

// Store provides thread-safe JSON file persistence.
type Store struct {
	mu       sync.Mutex
	filePath string
	data     storeData
	config   FederationConfig
}

type storeData struct {
	Nodes    map[string]FederationNode  `json:"nodes"`
	Backups  map[string]BackupRecord    `json:"backups"`
	Scores   map[string]ResilienceScore `json:"scores"`
}

// NewStore creates a Store backed by filePath.
func NewStore(filePath string, cfg FederationConfig) *Store {
	return &Store{
		filePath: filePath,
		config:   cfg,
		data: storeData{
			Nodes:   make(map[string]FederationNode),
			Backups: make(map[string]BackupRecord),
			Scores:  make(map[string]ResilienceScore),
		},
	}
}

// Load reads persisted data from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(raw, &s.data)
}

// Save writes current data to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0644)
}

// RegisterNode adds or updates a federation node.
func (s *Store) RegisterNode(node FederationNode) FederationNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	if node.ID == "" {
		node.ID = fmt.Sprintf("node-%d", time.Now().UTC().UnixNano())
	}
	if node.Registered.IsZero() {
		node.Registered = time.Now().UTC()
	}
	if node.Status == "" {
		node.Status = "active"
	}
	s.data.Nodes[node.ID] = node
	return node
}

// PerformBackup creates a backup record for an org on a specific node.
func (s *Store) PerformBackup(orgID, nodeID string, sizeBytes int64, checksum string) BackupRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	backup := BackupRecord{
		ID:        fmt.Sprintf("backup-%d", time.Now().UTC().UnixNano()),
		OrgID:     orgID,
		NodeID:    nodeID,
		SizeBytes: sizeBytes,
		Checksum:  checksum,
		Status:    "completed",
		CreatedAt: time.Now().UTC(),
		CompletedAt: time.Now().UTC(),
	}
	s.data.Backups[backup.ID] = backup
	return backup
}

// CheckResilience computes a ResilienceScore for the given org.
func (s *Store) CheckResilience(orgID string) ResilienceScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeNodes := 0
	totalNodes := 0
	for _, n := range s.data.Nodes {
		if n.OrgID == orgID {
			totalNodes++
			if n.Status == "active" {
				activeNodes++
			}
		}
	}

	hasBackup := false
	var lastBackupTime time.Time
	for _, b := range s.data.Backups {
		if b.OrgID == orgID && b.Status == "completed" {
			hasBackup = true
			if b.CompletedAt.After(lastBackupTime) {
				lastBackupTime = b.CompletedAt
			}
		}
	}

	// Score based on: node coverage (50%) + backup freshness (50%)
	nodeScore := 0.0
	if totalNodes > 0 {
		nodeScore = float64(activeNodes) / float64(totalNodes)
		if activeNodes >= s.config.MinNodesPerOrg {
			nodeScore = 1.0
		}
	}

	backupScore := 0.0
	if hasBackup && !lastBackupTime.IsZero() {
		age := time.Since(lastBackupTime)
		if age <= s.config.MaxBackupAge {
			backupScore = 1.0
		} else {
			backupScore = max(0, 1.0-age.Hours()/s.config.MaxBackupAge.Hours())
		}
	}

	score := 0.5*nodeScore + 0.5*backupScore

	lastAge := "never"
	if !lastBackupTime.IsZero() {
		lastAge = time.Since(lastBackupTime).Round(time.Minute).String()
	}

	rs := ResilienceScore{
		OrgID:         orgID,
		Score:         score,
		ActiveNodes:   activeNodes,
		TotalNodes:    totalNodes,
		LastBackupAge: lastAge,
		CanRestore:    hasBackup && activeNodes >= s.config.ReplicationFactor,
		MeasuredAt:    time.Now().UTC(),
	}
	s.data.Scores[orgID] = rs
	return rs
}

// RestoreFromBackup simulates restoring an org from its latest backup.
func (s *Store) RestoreFromBackup(orgID string) (BackupRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var latest BackupRecord
	found := false
	for _, b := range s.data.Backups {
		if b.OrgID == orgID && b.Status == "completed" {
			if !found || b.CompletedAt.After(latest.CompletedAt) {
				latest = b
				found = true
			}
		}
	}
	return latest, found
}

// GenerateFederationReport produces a summary of the federation's state.
func (s *Store) GenerateFederationReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	orgNodes := map[string]int{}
	orgActive := map[string]int{}
	for _, n := range s.data.Nodes {
		orgNodes[n.OrgID]++
		if n.Status == "active" {
			orgActive[n.OrgID]++
		}
	}

	backupCounts := map[string]int{}
	for _, b := range s.data.Backups {
		if b.Status == "completed" {
			backupCounts[b.OrgID]++
		}
	}

	avgScore := 0.0
	for _, sc := range s.data.Scores {
		avgScore += sc.Score
	}
	if len(s.data.Scores) > 0 {
		avgScore /= float64(len(s.data.Scores))
	}

	return map[string]interface{}{
		"total_nodes":       len(s.data.Nodes),
		"total_backups":     len(s.data.Backups),
		"orgs_with_nodes":   orgNodes,
		"orgs_active_nodes": orgActive,
		"backup_counts":     backupCounts,
		"average_resilience": avgScore,
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
