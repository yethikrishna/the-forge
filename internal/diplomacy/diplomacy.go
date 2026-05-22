// Package diplomacy provides treaty management, standards body participation
// tracking, and inter-organization agreement handling. It closes the gap in
// external relations governance by modeling diplomatic relations, negotiations,
// and standards contributions—ensuring the organization engages strategically
// with external entities and standards bodies.
package diplomacy

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// TreatyStatus represents the current state of a treaty or agreement.
type TreatyStatus string

const (
	TreatyProposed  TreatyStatus = "proposed"
	TreatyNegotiating TreatyStatus = "negotiating"
	TreatySigned    TreatyStatus = "signed"
	TreatyActive    TreatyStatus = "active"
	TreatySuspended TreatyStatus = "suspended"
	TreatyExpired   TreatyStatus = "expired"
	TreatyTerminated TreatyStatus = "terminated"
)

// RelationType represents the nature of a diplomatic relationship.
type RelationType string

const (
	RelationAlly      RelationType = "ally"
	RelationPartner   RelationType = "partner"
	RelationNeutral   RelationType = "neutral"
	RelationCompetitor RelationType = "competitor"
	RelationAdversary RelationType = "adversary"
)

// NegotiationStatus represents the state of a negotiation.
type NegotiationStatus string

const (
	NegotiationPending    NegotiationStatus = "pending"
	NegotiationInProgress NegotiationStatus = "in_progress"
	NegotiationCompleted  NegotiationStatus = "completed"
	NegotiationFailed     NegotiationStatus = "failed"
)

// Treaty represents a formal agreement between organizations.
type Treaty struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Counterparty string      `json:"counterparty"`
	Status       TreatyStatus `json:"status"`
	Terms        string       `json:"terms"`
	StartDate    time.Time    `json:"start_date"`
	EndDate      time.Time    `json:"end_date"`
	Signatories  []string     `json:"signatories"`
	Notes        string       `json:"notes"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// StandardContribution represents participation in a standards body.
type StandardContribution struct {
	ID           string    `json:"id"`
	StandardBody string    `json:"standard_body"`
	StandardName string    `json:"standard_name"`
	Role         string    `json:"role"` // author, reviewer, implementer, observer
	Status       string    `json:"status"`
	Contribution string    `json:"contribution"`
	SubmittedAt  time.Time `json:"submitted_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// DiplomaticRelation represents the relationship with another organization.
type DiplomaticRelation struct {
	ID             string       `json:"id"`
	Organization   string       `json:"organization"`
	Type           RelationType `json:"type"`
	TrustLevel     float64      `json:"trust_level"` // 0.0-1.0
	History        string       `json:"history"`
	KeyContacts    []string     `json:"key_contacts"`
	ActiveTreaties []string     `json:"active_treaties"`
	LastContactAt  time.Time    `json:"last_contact_at"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// Negotiation represents an ongoing or completed negotiation.
type Negotiation struct {
	ID           string            `json:"id"`
	Counterparty string            `json:"counterparty"`
	Subject      string            `json:"subject"`
	Status       NegotiationStatus `json:"status"`
	OurPosition  string            `json:"our_position"`
	TheirPosition string           `json:"their_position"`
	Outcome      string            `json:"outcome"`
	StartedAt    time.Time         `json:"started_at"`
	CompletedAt  time.Time         `json:"completed_at"`
	CreatedAt    time.Time         `json:"created_at"`
}

// Store persists diplomacy data.
type Store struct {
	mu           sync.Mutex
	filePath     string
	Treaties     map[string]Treaty               `json:"treaties"`
	Standards    map[string]StandardContribution  `json:"standards"`
	Relations    map[string]DiplomaticRelation    `json:"relations"`
	Negotiations map[string]Negotiation           `json:"negotiations"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath:     filePath,
		Treaties:     make(map[string]Treaty),
		Standards:    make(map[string]StandardContribution),
		Relations:    make(map[string]DiplomaticRelation),
		Negotiations: make(map[string]Negotiation),
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
		return err
	}
	return json.Unmarshal(data, s)
}

// Save writes the store to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// ProposeTreaty creates a new proposed treaty.
func (s *Store) ProposeTreaty(t Treaty) Treaty {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if t.Status == "" {
		t.Status = TreatyProposed
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	s.Treaties[t.ID] = t
	return t
}

// TrackStandards records a standards body contribution.
func (s *Store) TrackStandards(sc StandardContribution) StandardContribution {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc.CreatedAt = time.Now().UTC()
	if sc.SubmittedAt.IsZero() {
		sc.SubmittedAt = sc.CreatedAt
	}
	s.Standards[sc.ID] = sc
	return sc
}

// ManageRelations creates or updates a diplomatic relation.
func (s *Store) ManageRelations(dr DiplomaticRelation) DiplomaticRelation {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	dr.UpdatedAt = now
	if dr.CreatedAt.IsZero() {
		dr.CreatedAt = now
	}
	s.Relations[dr.ID] = dr
	return dr
}

// RecordNegotiation creates or updates a negotiation.
func (s *Store) RecordNegotiation(n Negotiation) Negotiation {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	if n.Status == NegotiationCompleted || n.Status == NegotiationFailed {
		n.CompletedAt = now
	}
	s.Negotiations[n.ID] = n
	return n
}

// GenerateDiplomacyReport produces a summary of diplomatic state.
func (s *Store) GenerateDiplomacyReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	activeTreaties := 0
	for _, t := range s.Treaties {
		if t.Status == TreatyActive || t.Status == TreatySigned {
			activeTreaties++
		}
	}
	allyCount := 0
	for _, r := range s.Relations {
		if r.Type == RelationAlly || r.Type == RelationPartner {
			allyCount++
		}
	}
	activeNegotiations := 0
	for _, n := range s.Negotiations {
		if n.Status == NegotiationInProgress {
			activeNegotiations++
		}
	}
	return map[string]interface{}{
		"active_treaties":      activeTreaties,
		"total_treaties":       len(s.Treaties),
		"ally_partner_count":   allyCount,
		"standards_contributions": len(s.Standards),
		"active_negotiations":  activeNegotiations,
		"relation_count":       len(s.Relations),
	}
}
