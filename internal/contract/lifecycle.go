// Package contract provides contract lifecycle management:
// creation from templates, negotiation tracking, renewal, and enforcement.
package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ContractStatus tracks contract lifecycle.
type ContractStatus string

const (
	ContractDraft     ContractStatus = "draft"
	ContractNegotiating ContractStatus = "negotiating"
	ContractSigned    ContractStatus = "signed"
	ContractActive    ContractStatus = "active"
	ContractRenewed   ContractStatus = "renewed"
	ContractTerminated ContractStatus = "terminated"
	ContractExpired   ContractStatus = "expired"
	ContractBreached  ContractStatus = "breached"
)

// Party represents a contract party.
type Party struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // provider, client, vendor, partner
}

// ContractTemplate is a reusable contract template.
type ContractTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // NDA, SaaS, SLA, vendor, employment, licensing
	Sections    []Section `json:"sections"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
}

// Section is a contract section/clause.
type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Mutable bool   `json:"mutable"` // can be modified during negotiation
}

// Contract is a live contract.
type Contract struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Type        string         `json:"type"`
	TemplateID  string         `json:"template_id,omitempty"`
	Parties     []Party        `json:"parties"`
	Sections    []Section      `json:"sections"`
	Status      ContractStatus `json:"status"`
	Value       float64        `json:"value,omitempty"`
	Currency    string         `json:"currency,omitempty"`
	StartDate   *time.Time     `json:"start_date,omitempty"`
	EndDate     *time.Time     `json:"end_date,omitempty"`
	AutoRenew   bool           `json:"auto_renew"`
	RenewalDays int            `json:"renewal_days,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	SignedAt    *time.Time     `json:"signed_at,omitempty"`
	TerminatedAt *time.Time    `json:"terminated_at,omitempty"`
	TerminationReason string   `json:"termination_reason,omitempty"`
	NegotiationHistory []NegotiationEntry `json:"negotiation_history,omitempty"`
}

// NegotiationEntry tracks a negotiation event.
type NegotiationEntry struct {
	ID        string    `json:"id"`
	ByParty   string    `json:"by_party"`
	Action    string    `json:"action"` // proposed_change, accepted, rejected, counter_offer
	Section   string    `json:"section,omitempty"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// Store manages contracts.
type Store struct {
	mu         sync.RWMutex
	templates  map[string]*ContractTemplate
	contracts  map[string]*Contract
	path       string
}

// NewStore creates a new contract store.
func NewStore(persistPath string) *Store {
	s := &Store{
		templates: make(map[string]*ContractTemplate),
		contracts: make(map[string]*Contract),
		path:      persistPath,
	}
	s.load()
	return s
}

// --- Templates ---

// CreateTemplate creates a reusable contract template.
func (s *Store) CreateTemplate(name, type_ string, sections []Section) (*ContractTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmpl := &ContractTemplate{
		ID:        genID("tmpl"),
		Name:      name,
		Type:      type_,
		Sections:  sections,
		CreatedAt: time.Now().UTC(),
	}

	s.templates[tmpl.ID] = tmpl
	s.persist()
	return tmpl, nil
}

// GetTemplate returns a template by ID.
func (s *Store) GetTemplate(id string) (*ContractTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.templates[id]
	if !ok {
		return nil, fmt.Errorf("template %s not found", id)
	}
	return t, nil
}

// ListTemplates returns all templates.
func (s *Store) ListTemplates() []*ContractTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*ContractTemplate
	for _, t := range s.templates {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

// --- Contracts ---

// CreateFromTemplate creates a contract from a template.
func (s *Store) CreateFromTemplate(title, type_ string, parties []Party, templateID string, value float64, currency string, start, end *time.Time, autoRenew bool) (*Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sections := []Section{}
	if templateID != "" {
		if tmpl, ok := s.templates[templateID]; ok {
			sections = make([]Section, len(tmpl.Sections))
			copy(sections, tmpl.Sections)
		}
	}

	now := time.Now().UTC()
	c := &Contract{
		ID:        genID("contract"),
		Title:     title,
		Type:      type_,
		TemplateID: templateID,
		Parties:   parties,
		Sections:  sections,
		Status:    ContractDraft,
		Value:     value,
		Currency:  currency,
		StartDate: start,
		EndDate:   end,
		AutoRenew: autoRenew,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.contracts[c.ID] = c
	s.persist()
	return c, nil
}

// Create creates a contract without a template.
func (s *Store) Create(title, type_ string, parties []Party, sections []Section, value float64, currency string, start, end *time.Time) (*Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	c := &Contract{
		ID:        genID("contract"),
		Title:     title,
		Type:      type_,
		Parties:   parties,
		Sections:  sections,
		Status:    ContractDraft,
		Value:     value,
		Currency:  currency,
		StartDate: start,
		EndDate:   end,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.contracts[c.ID] = c
	s.persist()
	return c, nil
}

// StartNegotiation moves contract to negotiation.
func (s *Store) StartNegotiation(contractID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.contracts[contractID]
	if !ok {
		return fmt.Errorf("contract %s not found", contractID)
	}
	c.Status = ContractNegotiating
	c.UpdatedAt = time.Now().UTC()
	s.persist()
	return nil
}

// ProposeChange adds a negotiation entry.
func (s *Store) ProposeChange(contractID, byParty, section, details string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.contracts[contractID]
	if !ok {
		return fmt.Errorf("contract %s not found", contractID)
	}

	c.NegotiationHistory = append(c.NegotiationHistory, NegotiationEntry{
		ID:        genID("nego"),
		ByParty:   byParty,
		Action:    "proposed_change",
		Section:   section,
		Details:   details,
		Timestamp: time.Now().UTC(),
	})
	c.UpdatedAt = time.Now().UTC()
	s.persist()
	return nil
}

// SignContract marks a contract as signed.
func (s *Store) SignContract(contractID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.contracts[contractID]
	if !ok {
		return fmt.Errorf("contract %s not found", contractID)
	}
	if c.Status != ContractNegotiating && c.Status != ContractDraft {
		return fmt.Errorf("contract must be in draft or negotiation to sign, current: %s", c.Status)
	}
	c.Status = ContractSigned
	now := time.Now().UTC()
	c.SignedAt = &now
	c.UpdatedAt = now

	// If start date is today or past, activate
	if c.StartDate == nil || !c.StartDate.After(now) {
		c.Status = ContractActive
	}

	s.persist()
	return nil
}

// RenewContract renews an active contract.
func (s *Store) RenewContract(contractID string, newEnd *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.contracts[contractID]
	if !ok {
		return fmt.Errorf("contract %s not found", contractID)
	}
	c.Status = ContractRenewed
	c.EndDate = newEnd
	c.UpdatedAt = time.Now().UTC()
	s.persist()
	return nil
}

// TerminateContract terminates a contract.
func (s *Store) TerminateContract(contractID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.contracts[contractID]
	if !ok {
		return fmt.Errorf("contract %s not found", contractID)
	}
	c.Status = ContractTerminated
	now := time.Now().UTC()
	c.TerminatedAt = &now
	c.TerminationReason = reason
	c.UpdatedAt = now
	s.persist()
	return nil
}

// ListContracts returns contracts filtered by status.
func (s *Store) ListContracts(status ContractStatus) []*Contract {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Contract
	for _, c := range s.contracts {
		if status == "" || c.Status == status {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// GetExpiring returns contracts expiring within the given duration.
func (s *Store) GetExpiring(within time.Duration) []*Contract {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(within)
	var result []*Contract
	for _, c := range s.contracts {
		if c.Status == ContractActive && c.EndDate != nil && !c.EndDate.After(cutoff) {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].EndDate.Before(*result[j].EndDate) })
	return result
}

func (s *Store) persist() {
	if s.path == "" { return }
	data := struct {
		Templates map[string]*ContractTemplate `json:"templates"`
		Contracts map[string]*Contract         `json:"contracts"`
	}{s.templates, s.contracts}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(s.path), 0755)
	os.WriteFile(s.path, raw, 0644)
}

func (s *Store) load() {
	if s.path == "" { return }
	raw, err := os.ReadFile(s.path)
	if err != nil { return }
	var data struct {
		Templates map[string]*ContractTemplate `json:"templates"`
		Contracts map[string]*Contract         `json:"contracts"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Templates != nil { s.templates = data.Templates }
		if data.Contracts != nil { s.contracts = data.Contracts }
	}
}

func genID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }
