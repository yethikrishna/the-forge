// Package legalstatus provides defined legal status and clear boundaries
// for entities within the forge. Without explicit legal status, entities
// operate in ambiguity — unclear liability, undefined authority, and no
// compliance framework. This package defines legal entities, sets boundaries,
// and tracks compliance to ensure every actor knows their legal standing.
package legalstatus

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// LegalEntity represents an entity with defined legal status.
type LegalEntity struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"` // "corporation", "llc", "nonprofit", "individual", "government"
	Jurisdiction string            `json:"jurisdiction"`
	Status       string            `json:"status"` // "active", "suspended", "dissolved"
	RegisteredAt time.Time         `json:"registered_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// StatusFramework defines the legal framework governing an entity.
type StatusFramework struct {
	ID          string    `json:"id"`
	EntityID    string    `json:"entity_id"`
	Framework   string    `json:"framework"` // "GDPR", "SOX", "HIPAA", "CCPA", "custom"
	Version     string    `json:"version"`
	ActiveSince time.Time `json:"active_since"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

// Boundary defines a legal boundary or constraint.
type Boundary struct {
	ID          string    `json:"id"`
	EntityID    string    `json:"entity_id"`
	Category    string    `json:"category"` // "data_access", "authority", "liability", "jurisdiction"
	Description string    `json:"description"`
	Limit       string    `json:"limit"` // description of the limit
	Enforced    bool      `json:"enforced"`
	SetAt       time.Time `json:"set_at"`
}

// ComplianceRecord tracks compliance status against a framework.
type ComplianceRecord struct {
	ID           string    `json:"id"`
	EntityID     string    `json:"entity_id"`
	FrameworkID  string    `json:"framework_id"`
	Status       string    `json:"status"` // "compliant", "non-compliant", "partial", "unknown"
	LastAudited  time.Time `json:"last_audited"`
	NextAudit    time.Time `json:"next_audit,omitempty"`
	Issues       int       `json:"issues"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// Store provides thread-safe JSON file persistence.
type Store struct {
	mu       sync.Mutex
	filePath string
	data     storeData
}

type storeData struct {
	Entities   map[string]LegalEntity     `json:"entities"`
	Frameworks map[string]StatusFramework `json:"frameworks"`
	Boundaries map[string]Boundary        `json:"boundaries"`
	Compliance map[string]ComplianceRecord `json:"compliance"`
}

// NewStore creates a Store backed by filePath.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		data: storeData{
			Entities:   make(map[string]LegalEntity),
			Frameworks: make(map[string]StatusFramework),
			Boundaries: make(map[string]Boundary),
			Compliance: make(map[string]ComplianceRecord),
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

// DefineStatus creates a legal entity with defined status.
func (s *Store) DefineStatus(entity LegalEntity) LegalEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entity.ID == "" {
		entity.ID = fmt.Sprintf("entity-%d", time.Now().UTC().UnixNano())
	}
	if entity.RegisteredAt.IsZero() {
		entity.RegisteredAt = time.Now().UTC()
	}
	if entity.Status == "" {
		entity.Status = "active"
	}
	s.data.Entities[entity.ID] = entity
	return entity
}

// SetBoundary defines a legal boundary for an entity.
func (s *Store) SetBoundary(boundary Boundary) Boundary {
	s.mu.Lock()
	defer s.mu.Unlock()
	if boundary.ID == "" {
		boundary.ID = fmt.Sprintf("bnd-%d", time.Now().UTC().UnixNano())
	}
	boundary.SetAt = time.Now().UTC()
	s.data.Boundaries[boundary.ID] = boundary
	return boundary
}

// CheckCompliance evaluates compliance of an entity against its frameworks.
func (s *Store) CheckCompliance(entityID string) []ComplianceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find all frameworks for this entity
	var frameworks []StatusFramework
	for _, fw := range s.data.Frameworks {
		if fw.EntityID == entityID {
			frameworks = append(frameworks, fw)
		}
	}

	var records []ComplianceRecord
	for _, fw := range frameworks {
		// Check boundaries for this entity
		enforcedCount := 0
		violatedCount := 0
		for _, b := range s.data.Boundaries {
			if b.EntityID == entityID {
				if b.Enforced {
					enforcedCount++
				} else {
					violatedCount++
				}
			}
		}

		status := "compliant"
		issues := 0
		if violatedCount > 0 {
			status = "non-compliant"
			issues = violatedCount
		} else if enforcedCount == 0 {
			status = "unknown"
		}

		// Check if framework is expired
		if !fw.ExpiresAt.IsZero() && time.Now().UTC().After(fw.ExpiresAt) {
			status = "non-compliant"
			issues++
		}

		cr := ComplianceRecord{
			ID:          fmt.Sprintf("cr-%d", time.Now().UTC().UnixNano()),
			EntityID:    entityID,
			FrameworkID: fw.ID,
			Status:      status,
			LastAudited: time.Now().UTC(),
			Issues:      issues,
			RecordedAt:  time.Now().UTC(),
		}
		s.data.Compliance[cr.ID] = cr
		records = append(records, cr)
	}

	return records
}

// AddFramework adds a legal framework for an entity.
func (s *Store) AddFramework(fw StatusFramework) StatusFramework {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fw.ID == "" {
		fw.ID = fmt.Sprintf("fw-%d", time.Now().UTC().UnixNano())
	}
	if fw.ActiveSince.IsZero() {
		fw.ActiveSince = time.Now().UTC()
	}
	s.data.Frameworks[fw.ID] = fw
	return fw
}

// GenerateLegalStatusReport produces a summary of legal status.
func (s *Store) GenerateLegalStatusReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	statusCounts := map[string]int{}
	for _, e := range s.data.Entities {
		statusCounts[e.Status]++
	}

	complianceCounts := map[string]int{}
	for _, cr := range s.data.Compliance {
		complianceCounts[cr.Status]++
	}

	boundaryEnforced := 0
	boundaryUnenforced := 0
	for _, b := range s.data.Boundaries {
		if b.Enforced {
			boundaryEnforced++
		} else {
			boundaryUnenforced++
		}
	}

	return map[string]interface{}{
		"entity_count":          len(s.data.Entities),
		"entity_status":         statusCounts,
		"framework_count":       len(s.data.Frameworks),
		"boundary_count":        len(s.data.Boundaries),
		"boundaries_enforced":   boundaryEnforced,
		"boundaries_unenforced": boundaryUnenforced,
		"compliance_status":     complianceCounts,
	}
}

// GetEntity retrieves an entity by ID.
func (s *Store) GetEntity(id string) (LegalEntity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data.Entities[id]
	return e, ok
}
