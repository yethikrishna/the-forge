// Package reproduction provides a firewall preventing new entity creation
// without human authorization. It closes the gap in uncontrolled
// organizational spawning by enforcing a policy that no new entities (teams,
// projects, subsidiaries, roles) may be created without explicit human
// approval—preventing the organizational equivalent of cancerous replication.
package reproduction

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// SpawnType represents the kind of entity being spawned.
type SpawnType string

const (
	SpawnTeam       SpawnType = "team"
	SpawnProject    SpawnType = "project"
	SpawnSubsidiary SpawnType = "subsidiary"
	SpawnRole       SpawnType = "role"
	SpawnService    SpawnType = "service"
	SpawnProcess    SpawnType = "process"
)

// ApprovalState represents the approval state of a spawn request.
type ApprovalState string

const (
	ApprovalPending   ApprovalState = "pending"
	ApprovalApproved  ApprovalState = "approved"
	ApprovalRejected  ApprovalState = "rejected"
	ApprovalExpired   ApprovalState = "expired"
	ApprovalRevoked   ApprovalState = "revoked"
)

// SpawnRequest represents a request to create a new entity.
type SpawnRequest struct {
	ID            string        `json:"id"`
	Type          SpawnType     `json:"type"`
	Name          string        `json:"name"`
	ParentEntity  string        `json:"parent_entity"`
	Justification string        `json:"justification"`
	RequestedBy   string        `json:"requested_by"`
	ApprovalState ApprovalState `json:"approval_state"`
	ApprovedBy    string        `json:"approved_by"`
	ApprovalNotes string        `json:"approval_notes"`
	EstimatedCost float64       `json:"estimated_cost"`
	EstimatedHeadcount int      `json:"estimated_headcount"`
	CreatedAt     time.Time     `json:"created_at"`
	ReviewedAt    time.Time     `json:"reviewed_at"`
}

// ReproductionPolicy represents the rules governing entity creation.
type ReproductionPolicy struct {
	ID                    string    `json:"id"`
	RequireHumanApproval  bool      `json:"require_human_approval"`
	MaxEntitiesPerMonth   int       `json:"max_entities_per_month"`
	AllowedSpawnTypes     []SpawnType `json:"allowed_spawn_types"`
	AutoApproveBelowCost  float64   `json:"auto_approve_below_cost"`
	FirewallEnabled       bool      `json:"firewall_enabled"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// EntityRecord represents a registered entity in the system.
type EntityRecord struct {
	ID          string    `json:"id"`
	SpawnID     string    `json:"spawn_id"` // link to the spawn request
	Type        SpawnType `json:"type"`
	Name        string    `json:"name"`
	ParentEntity string   `json:"parent_entity"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	DeactivatedAt time.Time `json:"deactivated_at"`
}

// ApprovalGate represents a gate that must be passed for approval.
type ApprovalGate struct {
	ID          string        `json:"id"`
	SpawnID     string        `json:"spawn_id"`
	GateName    string        `json:"gate_name"`
	Required    bool          `json:"required"`
	Passed      bool          `json:"passed"`
	PassedBy    string        `json:"passed_by"`
	PassedAt    time.Time     `json:"passed_at"`
	Notes       string        `json:"notes"`
}

// Store persists reproduction data.
type Store struct {
	mu        sync.Mutex
	filePath  string
	Requests  map[string]SpawnRequest     `json:"requests"`
	Policies  map[string]ReproductionPolicy `json:"policies"`
	Entities  map[string]EntityRecord     `json:"entities"`
	Gates     map[string]ApprovalGate     `json:"gates"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		Requests: make(map[string]SpawnRequest),
		Policies: make(map[string]ReproductionPolicy),
		Entities: make(map[string]EntityRecord),
		Gates:    make(map[string]ApprovalGate),
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

// RequestSpawn creates a new spawn request.
func (s *Store) RequestSpawn(req SpawnRequest) SpawnRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	req.CreatedAt = time.Now().UTC()
	req.ApprovalState = ApprovalPending
	// Check if auto-approve applies
	for _, policy := range s.Policies {
		if policy.FirewallEnabled && policy.AutoApproveBelowCost > 0 {
			if req.EstimatedCost <= policy.AutoApproveBelowCost {
				req.ApprovalState = ApprovalApproved
				req.ApprovedBy = "auto_policy"
				req.ReviewedAt = time.Now().UTC()
			}
		}
	}
	s.Requests[req.ID] = req
	return req
}

// CheckPolicy verifies whether a spawn request complies with policy.
func (s *Store) CheckPolicy(req SpawnRequest) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, policy := range s.Policies {
		if !policy.FirewallEnabled {
			continue
		}
		if policy.RequireHumanApproval && req.ApprovalState != ApprovalApproved {
			return false, "human approval required"
		}
		// Check spawn type is allowed
		if len(policy.AllowedSpawnTypes) > 0 {
			allowed := false
			for _, t := range policy.AllowedSpawnTypes {
				if t == req.Type {
					allowed = true
					break
				}
			}
			if !allowed {
				return false, "spawn type not allowed: " + string(req.Type)
			}
		}
		// Check monthly limit
		if policy.MaxEntitiesPerMonth > 0 {
			count := 0
			now := time.Now().UTC()
			for _, e := range s.Entities {
				if e.CreatedAt.Year() == now.Year() && e.CreatedAt.Month() == now.Month() {
					count++
				}
			}
			if count >= policy.MaxEntitiesPerMonth {
				return false, "monthly entity limit reached"
			}
		}
	}
	return true, ""
}

// RecordEntity registers an approved entity after spawn.
func (s *Store) RecordEntity(entity EntityRecord) (EntityRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Verify the spawn was approved
	if req, ok := s.Requests[entity.SpawnID]; ok {
		if req.ApprovalState != ApprovalApproved {
			return EntityRecord{}, os.ErrPermission
		}
	}
	entity.CreatedAt = time.Now().UTC()
	entity.IsActive = true
	s.Entities[entity.ID] = entity
	return entity, nil
}

// EnforceFirewall checks all entities and flags those without approved spawns.
func (s *Store) EnforceFirewall() []EntityRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	var unauthorized []EntityRecord
	for _, e := range s.Entities {
		if e.SpawnID == "" {
			unauthorized = append(unauthorized, e)
			continue
		}
		if req, ok := s.Requests[e.SpawnID]; ok {
			if req.ApprovalState != ApprovalApproved {
				unauthorized = append(unauthorized, e)
			}
		} else {
			unauthorized = append(unauthorized, e)
		}
	}
	return unauthorized
}

// GenerateReproductionReport produces a summary of reproduction state.
func (s *Store) GenerateReproductionReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	pendingRequests := 0
	approvedRequests := 0
	rejectedRequests := 0
	for _, r := range s.Requests {
		switch r.ApprovalState {
		case ApprovalPending:
			pendingRequests++
		case ApprovalApproved:
			approvedRequests++
		case ApprovalRejected:
			rejectedRequests++
		}
	}
	activeEntities := 0
	for _, e := range s.Entities {
		if e.IsActive {
			activeEntities++
		}
	}
	firewallEnabled := false
	for _, p := range s.Policies {
		if p.FirewallEnabled {
			firewallEnabled = true
			break
		}
	}
	return map[string]interface{}{
		"pending_requests":   pendingRequests,
		"approved_requests":  approvedRequests,
		"rejected_requests":  rejectedRequests,
		"active_entities":    activeEntities,
		"firewall_enabled":   firewallEnabled,
		"total_entities":     len(s.Entities),
	}
}
