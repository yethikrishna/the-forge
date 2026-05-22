// Package relationship provides partnership depth tracking, trust building,
// and relationship repair for Forge organizations.
// Healthy partnerships don't just happen — they're built, measured, and repaired.
//
// Closes gap: organizations need structured relationship management, not just deal tracking.
package relationship

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrustLevel represents the depth of trust in a relationship.
type TrustLevel string

const (
	TrustInitial   TrustLevel = "initial"
	TrustBuilding  TrustLevel = "building"
	TrustEstablished TrustLevel = "established"
	TrustDeep      TrustLevel = "deep"
	TrustStrategic TrustLevel = "strategic"
	TrustBroken    TrustLevel = "broken"
)

// PartnershipType categorizes the kind of partnership.
type PartnershipType string

const (
	PartnershipVendor    PartnershipType = "vendor"
	PartnershipClient    PartnershipType = "client"
	PartnershipStrategic PartnershipType = "strategic"
	PartnershipTechnology PartnershipType = "technology"
	PartnershipResearch  PartnershipType = "research"
	PartnershipCommunity PartnershipType = "community"
)

// HealthStatus represents the health of a relationship.
type HealthStatus string

const (
	HealthThriving  HealthStatus = "thriving"
	HealthHealthy   HealthStatus = "healthy"
	HealthStable    HealthStatus = "stable"
	HealthStrained  HealthStatus = "strained"
	HealthAtRisk    HealthStatus = "at_risk"
	HealthBroken    HealthStatus = "broken"
)

// Partnership represents a relationship with an external entity.
type Partnership struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          PartnershipType `json:"type"`
	TrustLevel    TrustLevel      `json:"trust_level"`
	TrustScore    float64         `json:"trust_score"` // 0.0–1.0
	HealthStatus  HealthStatus    `json:"health_status"`
	HealthScore   float64         `json:"health_score"` // 0.0–1.0
	StartDate     time.Time       `json:"start_date"`
	LastContact   time.Time       `json:"last_contact"`
	Interactions  int             `json:"interactions"`
	ValueDelivered float64        `json:"value_delivered"`
	Contacts      []string        `json:"contacts,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
	Notes         string          `json:"notes,omitempty"`
}

// RelationshipHealth is a detailed health assessment.
type RelationshipHealth struct {
	PartnershipID    string  `json:"partnership_id"`
	OverallScore     float64 `json:"overall_score"`
	CommunicationScore float64 `json:"communication_score"`
	DeliveryScore    float64 `json:"delivery_score"`
	TrustScore       float64 `json:"trust_score"`
	AlignmentScore   float64 `json:"alignment_score"`
	LongevityMonths  int     `json:"longevity_months"`
	Status           HealthStatus `json:"status"`
	Warnings         []string `json:"warnings,omitempty"`
	AssessedAt       time.Time `json:"assessed_at"`
}

// RepairAction represents a step to repair a damaged relationship.
type RepairAction struct {
	ID           string    `json:"id"`
	PartnershipID string  `json:"partnership_id"`
	Action       string    `json:"action"`
	Description  string    `json:"description"`
	Priority     string    `json:"priority"` // low, medium, high, critical
	Status       string    `json:"status"`   // pending, in_progress, completed
	AssignedTo   string    `json:"assigned_to,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// RelationshipRecord tracks a specific interaction or event.
type RelationshipRecord struct {
	ID            string    `json:"id"`
	PartnershipID string    `json:"partnership_id"`
	EventType     string    `json:"event_type"` // meeting, delivery, issue, milestone, review
	Description   string    `json:"description"`
	Impact        float64   `json:"impact"` // -1.0 (very negative) to 1.0 (very positive)
	RecordedAt    time.Time `json:"recorded_at"`
}

// Store manages relationship data with JSON persistence.
type Store struct {
	partnerships []Partnership
	records      []RelationshipRecord
	repairs      []RepairAction
	healthCache  map[string]RelationshipHealth
	filePath     string
	mu           sync.RWMutex
	nextID       int
}

// NewStore creates a new relationship store.
func NewStore(filePath string) *Store {
	return &Store{
		partnerships: make([]Partnership, 0),
		records:      make([]RelationshipRecord, 0),
		repairs:      make([]RepairAction, 0),
		healthCache:  make(map[string]RelationshipHealth),
		filePath:     filePath,
	}
}

// Load reads the store from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read relationship file: %w", err)
	}

	var raw struct {
		Partnerships []Partnership        `json:"partnerships"`
		Records      []RelationshipRecord `json:"records"`
		Repairs      []RepairAction       `json:"repairs"`
		NextID       int                  `json:"next_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse relationship file: %w", err)
	}
	s.partnerships = raw.Partnerships
	s.records = raw.Records
	s.repairs = raw.Repairs
	s.nextID = raw.NextID
	return nil
}

// Save writes the store to disk.
// Save writes the store to disk.
// Assumes the caller already holds s.mu.
func (s *Store) Save() error {

	raw := struct {
		Partnerships []Partnership        `json:"partnerships"`
		Records      []RelationshipRecord `json:"records"`
		Repairs      []RepairAction       `json:"repairs"`
		NextID       int                  `json:"next_id"`
	}{
		Partnerships: s.partnerships,
		Records:      s.records,
		Repairs:      s.repairs,
		NextID:       s.nextID,
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal relationship: %w", err)
	}
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create relationship dir: %w", err)
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write relationship file: %w", err)
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Store) genID(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%04d", prefix, s.nextID)
}

// BuildPartnership creates a new partnership.
func (s *Store) BuildPartnership(name string, ptype PartnershipType) (*Partnership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	p := Partnership{
		ID:           s.genID("ptn"),
		Name:         name,
		Type:         ptype,
		TrustLevel:   TrustInitial,
		TrustScore:   0.1,
		HealthStatus: HealthStable,
		HealthScore:  0.5,
		StartDate:    now,
		LastContact:  now,
	}
	s.partnerships = append(s.partnerships, p)
	return &p, s.Save()
}

// MeasureTrust evaluates and returns the trust level for a partnership.
func (s *Store) MeasureTrust(partnershipID string) (TrustLevel, float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.findPartnership(partnershipID)
	if p == nil {
		return "", 0, fmt.Errorf("partnership %s not found", partnershipID)
	}

	// Recalculate trust based on records
	records := s.recordsFor(partnershipID)
	score := p.TrustScore

	for _, r := range records {
		if r.Impact > 0 {
			score += r.Impact * 0.08 // positive interactions build trust
		} else {
			score += r.Impact * 0.1 // negative interactions erode trust faster
		}
	}

	if score > 1.0 {
		score = 1.0
	}
	if score < 0 {
		score = 0
	}

	p.TrustScore = score
	p.TrustLevel = scoreToTrustLevel(score)
	return p.TrustLevel, p.TrustScore, s.Save()
}

// AssessHealth evaluates the health of a partnership.
func (s *Store) AssessHealth(partnershipID string) (*RelationshipHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.findPartnership(partnershipID)
	if p == nil {
		return nil, fmt.Errorf("partnership %s not found", partnershipID)
	}

	records := s.recordsFor(partnershipID)

	commScore := 0.5
	deliveryScore := 0.5
	alignmentScore := 0.5

	interactions := 0
	deliveries := 0
	issues := 0
	positiveImpact := 0.0
	negativeImpact := 0.0

	for _, r := range records {
		switch r.EventType {
		case "meeting":
			interactions++
		case "delivery":
			deliveries++
		case "issue":
			issues++
		}
		if r.Impact > 0 {
			positiveImpact += r.Impact
		} else {
			negativeImpact += -r.Impact
		}
	}

	// Communication: more regular meetings = better
	if interactions > 0 {
		commScore = 0.3 + float64(min(interactions, 10))/10*0.7
	}

	// Delivery: more deliveries, fewer issues = better
	if deliveries > 0 {
		deliveryScore = 0.3 + float64(min(deliveries, 10))/10*0.5
		if issues > 0 {
			deliveryScore -= float64(min(issues, 5)) / 5 * 0.3
		}
	}

	// Alignment: positive vs negative impact ratio
	totalImpact := positiveImpact + negativeImpact
	if totalImpact > 0 {
		alignmentScore = positiveImpact / totalImpact
	}

	overallScore := (commScore*0.25 + deliveryScore*0.3 + p.TrustScore*0.25 + alignmentScore*0.2)
	longevity := int(time.Since(p.StartDate).Hours() / 24 / 30)

	var warnings []string
	if issues > 3 {
		warnings = append(warnings, "high issue count")
	}
	if p.TrustScore < 0.3 {
		warnings = append(warnings, "trust score critically low")
	}
	if negativeImpact > positiveImpact {
		warnings = append(warnings, "negative impact exceeds positive")
	}

	health := RelationshipHealth{
		PartnershipID:     partnershipID,
		OverallScore:      overallScore,
		CommunicationScore: commScore,
		DeliveryScore:     deliveryScore,
		TrustScore:        p.TrustScore,
		AlignmentScore:    alignmentScore,
		LongevityMonths:   longevity,
		Status:            scoreToHealthStatus(overallScore),
		Warnings:          warnings,
		AssessedAt:        time.Now(),
	}

	p.HealthScore = overallScore
	p.HealthStatus = health.Status
	s.healthCache[partnershipID] = health

	return &health, s.Save()
}

// PlanRepair generates repair actions for an at-risk partnership.
func (s *Store) PlanRepair(partnershipID string) ([]RepairAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.findPartnership(partnershipID)
	if p == nil {
		return nil, fmt.Errorf("partnership %s not found", partnershipID)
	}

	records := s.recordsFor(partnershipID)
	var actions []RepairAction

	issues := 0
	for _, r := range records {
		if r.EventType == "issue" && r.Impact < 0 {
			issues++
		}
	}

	if issues > 0 {
		actions = append(actions, RepairAction{
			ID:            s.genID("rpr"),
			PartnershipID: partnershipID,
			Action:        "Address outstanding issues",
			Description:   fmt.Sprintf("Review and resolve %d outstanding issues", issues),
			Priority:      "high",
			Status:        "pending",
			CreatedAt:     time.Now(),
		})
	}

	if p.TrustScore < 0.3 {
		actions = append(actions, RepairAction{
			ID:            s.genID("rpr"),
			PartnershipID: partnershipID,
			Action:        "Rebuild trust through transparency",
			Description:   "Share roadmap, provide regular updates, offer concessions",
			Priority:      "critical",
			Status:        "pending",
			CreatedAt:     time.Now(),
		})
	}

	if p.HealthScore < 0.4 {
		actions = append(actions, RepairAction{
			ID:            s.genID("rpr"),
			PartnershipID: partnershipID,
			Action:        "Schedule executive review",
			Description:   "Escalate to leadership for strategic intervention",
			Priority:      "high",
			Status:        "pending",
			CreatedAt:     time.Now(),
		})
	}

	if len(actions) == 0 {
		actions = append(actions, RepairAction{
			ID:            s.genID("rpr"),
			PartnershipID: partnershipID,
			Action:        "Regular check-in",
			Description:   "Schedule regular check-in to maintain relationship health",
			Priority:      "low",
			Status:        "pending",
			CreatedAt:     time.Now(),
		})
	}

	s.repairs = append(s.repairs, actions...)
	return actions, s.Save()
}

// TrackRelationship adds a relationship record.
func (s *Store) TrackRelationship(partnershipID, eventType, description string, impact float64) (*RelationshipRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.findPartnership(partnershipID)
	if p == nil {
		return nil, fmt.Errorf("partnership %s not found", partnershipID)
	}

	record := RelationshipRecord{
		ID:            s.genID("rec"),
		PartnershipID: partnershipID,
		EventType:     eventType,
		Description:   description,
		Impact:        impact,
		RecordedAt:    time.Now(),
	}

	s.records = append(s.records, record)
	p.Interactions++
	p.LastContact = record.RecordedAt

	return &record, s.Save()
}

// GenerateRelationshipReport produces a summary of all relationships.
func (s *Store) GenerateRelationshipReport() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := "=== Relationship Report ===\n\n"
	out += fmt.Sprintf("Partnerships: %d\n", len(s.partnerships))
	out += fmt.Sprintf("Records: %d\n", len(s.records))
	out += fmt.Sprintf("Repair Actions: %d\n\n", len(s.repairs))

	if len(s.partnerships) > 0 {
		thriving := 0
		atRisk := 0
		broken := 0
		for _, p := range s.partnerships {
			switch p.HealthStatus {
			case HealthThriving, HealthHealthy:
				thriving++
			case HealthAtRisk, HealthStrained:
				atRisk++
			case HealthBroken:
				broken++
			}
		}
		out += fmt.Sprintf("Thriving/Healthy: %d | At Risk/Strained: %d | Broken: %d\n\n", thriving, atRisk, broken)

		out += "Partnerships:\n"
		for _, p := range s.partnerships {
			out += fmt.Sprintf("  %s [%s] trust=%.2f health=%.2f (%s)\n",
				p.Name, p.Type, p.TrustScore, p.HealthScore, p.HealthStatus)
		}
	}

	pending := 0
	for _, r := range s.repairs {
		if r.Status == "pending" {
			pending++
		}
	}
	if pending > 0 {
		out += fmt.Sprintf("\nPending repairs: %d\n", pending)
	}

	return out
}

// ListPartnerships returns all partnerships.
func (s *Store) ListPartnerships() []Partnership {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Partnership, len(s.partnerships))
	copy(out, s.partnerships)
	return out
}

func (s *Store) findPartnership(id string) *Partnership {
	for i := range s.partnerships {
		if s.partnerships[i].ID == id {
			return &s.partnerships[i]
		}
	}
	return nil
}

func (s *Store) recordsFor(partnershipID string) []RelationshipRecord {
	var result []RelationshipRecord
	for _, r := range s.records {
		if r.PartnershipID == partnershipID {
			result = append(result, r)
		}
	}
	return result
}

func scoreToTrustLevel(score float64) TrustLevel {
	switch {
	case score >= 0.8:
		return TrustStrategic
	case score >= 0.6:
		return TrustDeep
	case score >= 0.4:
		return TrustEstablished
	case score >= 0.2:
		return TrustBuilding
	case score > 0:
		return TrustInitial
	default:
		return TrustBroken
	}
}

func scoreToHealthStatus(score float64) HealthStatus {
	switch {
	case score >= 0.8:
		return HealthThriving
	case score >= 0.6:
		return HealthHealthy
	case score >= 0.4:
		return HealthStable
	case score >= 0.2:
		return HealthStrained
	case score >= 0.1:
		return HealthAtRisk
	default:
		return HealthBroken
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
