// Package patent provides invention capture, filing pipeline, and prior art monitoring.
// From invention → filing → protection → monitoring.
package patent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// InventionStatus tracks the patent pipeline.
type InventionStatus string

const (
	InventionDisclosed  InventionStatus = "disclosed"
	InventionReviewed   InventionStatus = "reviewed"
	InventionFiled      InventionStatus = "filed"
	InventionGranted    InventionStatus = "granted"
	InventionRejected   InventionStatus = "rejected"
	InventionAbandoned  InventionStatus = "abandoned"
)

// Invention captures a potentially patentable invention.
type Invention struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Inventors    []string        `json:"inventors"` // agent IDs or humans
	DivisionID   string          `json:"division_id"`
	Status       InventionStatus `json:"status"`
	Claims       []string        `json:"claims,omitempty"`
	PriorArt     []string        `json:"prior_art,omitempty"`
	FilingDate   *time.Time      `json:"filing_date,omitempty"`
	GrantDate    *time.Time      `json:"grant_date,omitempty"`
	PatentNumber string          `json:"patent_number,omitempty"`
	Jurisdiction string          `json:"jurisdiction,omitempty"` // US, EU, etc.
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Notes        string          `json:"notes,omitempty"`
}

// PriorArtSearch tracks prior art research results.
type PriorArtSearch struct {
	ID          string    `json:"id"`
	InventionID string    `json:"invention_id"`
	Query       string    `json:"query"`
	Results     []PriorArtResult `json:"results,omitempty"`
	SearchedAt  time.Time `json:"searched_at"`
}

// PriorArtResult is one prior art reference.
type PriorArtResult struct {
	Title     string  `json:"title"`
	Source    string  `json:"source"` // patent_db, paper, product
	Relevance float64 `json:"relevance"` // 0-1
	URL       string  `json:"url,omitempty"`
	Summary   string  `json:"summary,omitempty"`
}

// InfringementAlert flags potential patent infringement.
type InfringementAlert struct {
	ID          string    `json:"id"`
	PatentID    string    `json:"patent_id"`
	SuspectedBy string    `json:"suspected_by"` // org or product name
	Description string    `json:"description"`
	Evidence    string    `json:"evidence,omitempty"`
	Severity    string    `json:"severity"` // low, medium, high
	Status      string    `json:"status"` // open, investigating, resolved
	CreatedAt   time.Time `json:"created_at"`
}

// Store manages patents.
type Store struct {
	mu          sync.RWMutex
	inventions  map[string]*Invention
	searches    map[string]*PriorArtSearch
	alerts      map[string]*InfringementAlert
	path        string
}

// NewStore creates a new patent store.
func NewStore(persistPath string) *Store {
	s := &Store{
		inventions: make(map[string]*Invention),
		searches:   make(map[string]*PriorArtSearch),
		alerts:     make(map[string]*InfringementAlert),
		path:       persistPath,
	}
	s.load()
	return s
}

// Disclose records a new invention disclosure.
func (s *Store) Disclose(title, description string, inventors []string, divisionID string) (*Invention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	inv := &Invention{
		ID:          genID("inv"),
		Title:       title,
		Description: description,
		Inventors:   inventors,
		DivisionID:  divisionID,
		Status:      InventionDisclosed,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.inventions[inv.ID] = inv
	s.persist()
	return inv, nil
}

// Review marks an invention as reviewed by legal/patent team.
func (s *Store) Review(inventionID string, claims []string, priorArt []string, notes string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, ok := s.inventions[inventionID]
	if !ok {
		return fmt.Errorf("invention %s not found", inventionID)
	}
	inv.Status = InventionReviewed
	inv.Claims = claims
	inv.PriorArt = priorArt
	inv.Notes = notes
	inv.UpdatedAt = time.Now().UTC()
	s.persist()
	return nil
}

// File records a patent filing.
func (s *Store) File(inventionID, jurisdiction string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, ok := s.inventions[inventionID]
	if !ok {
		return fmt.Errorf("invention %s not found", inventionID)
	}
	if inv.Status != InventionReviewed {
		return fmt.Errorf("invention must be reviewed before filing, current: %s", inv.Status)
	}
	inv.Status = InventionFiled
	now := time.Now().UTC()
	inv.FilingDate = &now
	inv.Jurisdiction = jurisdiction
	inv.UpdatedAt = now
	s.persist()
	return nil
}

// Grant records a patent grant.
func (s *Store) Grant(inventionID, patentNumber string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, ok := s.inventions[inventionID]
	if !ok {
		return fmt.Errorf("invention %s not found", inventionID)
	}
	inv.Status = InventionGranted
	now := time.Now().UTC()
	inv.GrantDate = &now
	inv.PatentNumber = patentNumber
	inv.UpdatedAt = now
	s.persist()
	return nil
}

// RecordPriorArtSearch records a prior art search.
func (s *Store) RecordPriorArtSearch(inventionID, query string, results []PriorArtResult) (*PriorArtSearch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	search := &PriorArtSearch{
		ID:          genID("pas"),
		InventionID: inventionID,
		Query:       query,
		Results:     results,
		SearchedAt:  time.Now().UTC(),
	}

	s.searches[search.ID] = search
	s.persist()
	return search, nil
}

// FlagInfringement creates an infringement alert.
func (s *Store) FlagInfringement(patentID, suspectedBy, description, evidence, severity string) (*InfringementAlert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert := &InfringementAlert{
		ID:          genID("ialert"),
		PatentID:    patentID,
		SuspectedBy: suspectedBy,
		Description: description,
		Evidence:    evidence,
		Severity:    severity,
		Status:      "open",
		CreatedAt:   time.Now().UTC(),
	}

	s.alerts[alert.ID] = alert
	s.persist()
	return alert, nil
}

// ListInventions returns inventions filtered by status.
func (s *Store) ListInventions(status InventionStatus) []*Invention {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Invention
	for _, inv := range s.inventions {
		if status == "" || inv.Status == status {
			result = append(result, inv)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *Store) persist() {
	if s.path == "" {
		return
	}
	data := struct {
		Inventions map[string]*Invention       `json:"inventions"`
		Searches   map[string]*PriorArtSearch   `json:"searches"`
		Alerts     map[string]*InfringementAlert `json:"alerts"`
	}{s.inventions, s.searches, s.alerts}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(s.path), 0755)
	os.WriteFile(s.path, raw, 0644)
}

func (s *Store) load() {
	if s.path == "" {
		return
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var data struct {
		Inventions map[string]*Invention       `json:"inventions"`
		Searches   map[string]*PriorArtSearch   `json:"searches"`
		Alerts     map[string]*InfringementAlert `json:"alerts"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Inventions != nil { s.inventions = data.Inventions }
		if data.Searches != nil { s.searches = data.Searches }
		if data.Alerts != nil { s.alerts = data.Alerts }
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
