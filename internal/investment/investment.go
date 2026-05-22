// Package investment provides pitch generation, equity management, and
// fundraising tracking. It closes the gap in capital strategy by modeling
// investment rounds, cap tables, valuations, and investor relations—ensuring
// the organization can raise and manage capital with precision and transparency.
package investment

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// RoundStage represents the stage of an equity round.
type RoundStage string

const (
	StagePreSeed RoundStage = "pre_seed"
	StageSeed    RoundStage = "seed"
	StageSeriesA RoundStage = "series_a"
	StageSeriesB RoundStage = "series_b"
	StageSeriesC RoundStage = "series_c"
	StageGrowth  RoundStage = "growth"
)

// RoundStatus represents the state of a funding round.
type RoundStatus string

const (
	RoundPlanning RoundStatus = "planning"
	RoundOpen     RoundStatus = "open"
	RoundClosed   RoundStatus = "closed"
	RoundCancelled RoundStatus = "cancelled"
)

// Pitch represents an investment pitch document.
type Pitch struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Problem        string    `json:"problem"`
	Solution       string    `json:"solution"`
	MarketSize     float64   `json:"market_size"`
	BusinessModel  string    `json:"business_model"`
	Traction       string    `json:"traction"`
	Ask            float64   `json:"ask"`
	Valuation      float64   `json:"valuation"`
	Stage          RoundStage `json:"stage"`
	CreatedAt      time.Time `json:"created_at"`
}

// EquityRound represents a fundraising round.
type EquityRound struct {
	ID             string      `json:"id"`
	Stage          RoundStage  `json:"stage"`
	Status         RoundStatus `json:"status"`
	TargetAmount   float64     `json:"target_amount"`
	RaisedAmount   float64     `json:"raised_amount"`
	PreMoneyValuation float64  `json:"pre_money_valuation"`
	SharePrice     float64     `json:"share_price"`
	OpenedAt       time.Time   `json:"opened_at"`
	ClosedAt       time.Time   `json:"closed_at"`
	InvestorIDs    []string    `json:"investor_ids"`
	CreatedAt      time.Time   `json:"created_at"`
}

// Investor represents an investor in the cap table.
type Investor struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"` // individual, vc, angel, strategic
	Shares       float64   `json:"shares"`
	InvestedAmount float64 `json:"invested_amount"`
	BoardSeat    bool      `json:"board_seat"`
	FirstInvested time.Time `json:"first_invested"`
	CreatedAt    time.Time `json:"created_at"`
}

// Valuation represents a company valuation event.
type Valuation struct {
	ID          string    `json:"id"`
	Amount      float64   `json:"amount"`
	Method      string    `json:"method"` // dcf, comparable, berkus, risk_factor
	Date        time.Time `json:"date"`
	RoundID     string    `json:"round_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// CapTable represents the capitalization table.
type CapTable struct {
	ID          string     `json:"id"`
	TotalShares float64    `json:"total_shares"`
	Entries     []CapEntry `json:"entries"`
	AsOf        time.Time  `json:"as_of"`
}

// CapEntry is a single entry in the cap table.
type CapEntry struct {
	InvestorID   string  `json:"investor_id"`
	Shares       float64 `json:"shares"`
	Percentage   float64 `json:"percentage"`
	InvestedAmount float64 `json:"invested_amount"`
}

// Store persists investment data.
type Store struct {
	mu         sync.Mutex
	filePath   string
	Pitches    map[string]Pitch       `json:"pitches"`
	Rounds     map[string]EquityRound `json:"rounds"`
	Investors  map[string]Investor    `json:"investors"`
	Valuations map[string]Valuation   `json:"valuations"`
	CapTables  map[string]CapTable    `json:"cap_tables"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath:   filePath,
		Pitches:    make(map[string]Pitch),
		Rounds:     make(map[string]EquityRound),
		Investors:  make(map[string]Investor),
		Valuations: make(map[string]Valuation),
		CapTables:  make(map[string]CapTable),
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

// GeneratePitch creates a new pitch from provided data.
func (s *Store) GeneratePitch(p Pitch) Pitch {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.CreatedAt = time.Now().UTC()
	s.Pitches[p.ID] = p
	return p
}

// TrackRound creates or updates a funding round.
func (s *Store) TrackRound(r EquityRound) EquityRound {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.CreatedAt = time.Now().UTC()
	s.Rounds[r.ID] = r
	return r
}

// ManageCapTable builds the cap table from current investor data.
func (s *Store) ManageCapTable(id string) CapTable {
	s.mu.Lock()
	defer s.mu.Unlock()
	var entries []CapEntry
	totalShares := 0.0
	for _, inv := range s.Investors {
		totalShares += inv.Shares
	}
	for _, inv := range s.Investors {
		pct := 0.0
		if totalShares > 0 {
			pct = inv.Shares / totalShares * 100
		}
		entries = append(entries, CapEntry{
			InvestorID:    inv.ID,
			Shares:        inv.Shares,
			Percentage:    pct,
			InvestedAmount: inv.InvestedAmount,
		})
	}
	ct := CapTable{
		ID:          id,
		TotalShares: totalShares,
		Entries:     entries,
		AsOf:        time.Now().UTC(),
	}
	s.CapTables[id] = ct
	return ct
}

// CalculateValuation computes a simple valuation based on method.
func (s *Store) CalculateValuation(v Valuation) Valuation {
	s.mu.Lock()
	defer s.mu.Unlock()
	v.CreatedAt = time.Now().UTC()
	v.Date = time.Now().UTC()
	s.Valuations[v.ID] = v
	return v
}

// GenerateInvestmentReport produces a summary of the investment state.
func (s *Store) GenerateInvestmentReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	totalRaised := 0.0
	openRounds := 0
	for _, r := range s.Rounds {
		totalRaised += r.RaisedAmount
		if r.Status == RoundOpen {
			openRounds++
		}
	}
	totalInvested := 0.0
	for _, inv := range s.Investors {
		totalInvested += inv.InvestedAmount
	}
	latestValuation := 0.0
	for _, v := range s.Valuations {
		if v.Amount > latestValuation {
			latestValuation = v.Amount
		}
	}
	return map[string]interface{}{
		"total_raised":      totalRaised,
		"total_invested":    totalInvested,
		"open_rounds":       openRounds,
		"investor_count":    len(s.Investors),
		"pitch_count":       len(s.Pitches),
		"latest_valuation":  latestValuation,
	}
}
