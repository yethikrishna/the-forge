// Package death provides shutdown planning, succession planning, legacy definition,
// archival, and death directive management. It closes the gap in end-of-life
// intelligence — enabling the Forge to plan graceful shutdowns, preserve institutional
// knowledge, and ensure continuity even in termination scenarios.
package death

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// ShutdownPlan represents a plan for organizational or project shutdown.
type ShutdownPlan struct {
	ID            string    `json:"id"`
	EntityID      string    `json:"entity_id"`
	Reason        string    `json:"reason"` // financial, strategic, regulatory, failure
	Timeline      string    `json:"timeline"`
	Phases        []string  `json:"phases"`
	Status        string    `json:"status"` // draft, approved, executing, completed
	DataMigration bool      `json:"data_migration"`
	StakeholderNotify []string `json:"stakeholder_notify"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SuccessionPlan defines how leadership and responsibility transfer.
type SuccessionPlan struct {
	ID              string    `json:"id"`
	EntityID        string    `json:"entity_id"`
	Role            string    `json:"role"`
	CurrentHolder   string    `json:"current_holder"`
	Successor       string    `json:"successor"`
	Readiness       float64   `json:"readiness"` // 0-1
	TransitionPlan  string    `json:"transition_plan"`
	Status          string    `json:"status"` // planned, in_progress, completed
	CreatedAt       time.Time `json:"created_at"`
}

// LegacyItem represents something to be preserved after shutdown.
type LegacyItem struct {
	ID          string    `json:"id"`
	EntityID    string    `json:"entity_id"`
	Type        string    `json:"type"` // knowledge, code, relationship, brand, process
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Priority    string    `json:"priority"` // critical, high, medium, low
	PreservedAt time.Time `json:"preserved_at"`
}

// ArchiveRecord tracks what has been archived.
type ArchiveRecord struct {
	ID          string    `json:"id"`
	EntityID    string    `json:"entity_id"`
	ContentType string    `json:"content_type"` // documents, code, data, communications
	Location    string    `json:"location"`
	SizeBytes   int64     `json:"size_bytes"`
	Checksum    string    `json:"checksum"`
	ArchivedAt  time.Time `json:"archived_at"`
	ExpiryDate  time.Time `json:"expiry_date"`
}

// DeathDirective represents a pre-planned directive for termination scenarios.
type DeathDirective struct {
	ID          string    `json:"id"`
	EntityID    string    `json:"entity_id"`
	Trigger     string    `json:"trigger"` // financial_threshold, legal_order, founder_death, consensus
	Action      string    `json:"action"`
	Conditions  []string  `json:"conditions"`
	Priority    int       `json:"priority"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// DeathReport is a consolidated death/end-of-life report.
type DeathReport struct {
	GeneratedAt    time.Time        `json:"generated_at"`
	ShutdownPlans  []ShutdownPlan   `json:"shutdown_plans"`
	SuccessionPlans []SuccessionPlan `json:"succession_plans"`
	LegacyItems    []LegacyItem     `json:"legacy_items"`
	ArchiveRecords []ArchiveRecord  `json:"archive_records"`
	DeathDirectives []DeathDirective `json:"death_directives"`
}

// Store persists death/end-of-life data to a JSON file with thread safety.
type Store struct {
	mu              sync.Mutex
	filePath        string
	ShutdownPlans   []ShutdownPlan   `json:"shutdown_plans"`
	SuccessionPlans []SuccessionPlan `json:"succession_plans"`
	LegacyItems     []LegacyItem     `json:"legacy_items"`
	ArchiveRecords  []ArchiveRecord  `json:"archive_records"`
	DeathDirectives []DeathDirective `json:"death_directives"`
}

// NewStore creates a new Store backed by the given file path.
func NewStore(filePath string) *Store {
	return &Store{filePath: filePath}
}

// Load reads data from the backing file.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, s)
}

// Save writes data to the backing file.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// PlanShutdown creates a shutdown plan.
func PlanShutdown(entityID, reason, timeline string, phases []string, dataMigration bool) ShutdownPlan {
	return ShutdownPlan{
		ID:             genID("sp"),
		EntityID:       entityID,
		Reason:         reason,
		Timeline:       timeline,
		Phases:         phases,
		Status:         "draft",
		DataMigration:  dataMigration,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// PlanSuccession creates a succession plan.
func PlanSuccession(entityID, role, currentHolder, successor, transitionPlan string, readiness float64) SuccessionPlan {
	return SuccessionPlan{
		ID:             genID("scp"),
		EntityID:       entityID,
		Role:           role,
		CurrentHolder:  currentHolder,
		Successor:      successor,
		Readiness:      readiness,
		TransitionPlan: transitionPlan,
		Status:         "planned",
		CreatedAt:      time.Now(),
	}
}

// DefineLegacy creates a legacy preservation item.
func DefineLegacy(entityID, legacyType, name, description, location, priority string) LegacyItem {
	return LegacyItem{
		ID:          genID("li"),
		EntityID:    entityID,
		Type:        legacyType,
		Name:        name,
		Description: description,
		Location:    location,
		Priority:    priority,
		PreservedAt: time.Now(),
	}
}

// CreateArchive records an archive action.
func CreateArchive(entityID, contentType, location, checksum string, sizeBytes int64, expiryDate time.Time) ArchiveRecord {
	return ArchiveRecord{
		ID:          genID("ar"),
		EntityID:    entityID,
		ContentType: contentType,
		Location:    location,
		SizeBytes:   sizeBytes,
		Checksum:    checksum,
		ArchivedAt:  time.Now(),
		ExpiryDate:  expiryDate,
	}
}

// ExecuteDirective creates a death directive.
func ExecuteDirective(entityID, trigger, action string, conditions []string, priority int) DeathDirective {
	return DeathDirective{
		ID:         genID("dd"),
		EntityID:   entityID,
		Trigger:    trigger,
		Action:     action,
		Conditions: conditions,
		Priority:   priority,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}
}

// GenerateDeathReport produces a consolidated death report.
func GenerateDeathReport(s *Store) DeathReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return DeathReport{
		GeneratedAt:     time.Now(),
		ShutdownPlans:   s.ShutdownPlans,
		SuccessionPlans: s.SuccessionPlans,
		LegacyItems:     s.LegacyItems,
		ArchiveRecords:  s.ArchiveRecords,
		DeathDirectives: s.DeathDirectives,
	}
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
