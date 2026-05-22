// Package portfolio provides R&D portfolio management, technology roadmap tracking,
// kill decisions, and exploration vs exploitation balancing for Forge organizations.
// Every project can't live forever. Some need to graduate, some need to die.
//
// Closes gap: organizations need structured portfolio management, not just project lists.
package portfolio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ItemStatus represents the status of a portfolio item.
type ItemStatus string

const (
	ItemExploring  ItemStatus = "exploring"
	ItemActive     ItemStatus = "active"
	ItemGraduated  ItemStatus = "graduated"
	ItemKilled     ItemStatus = "killed"
	ItemPaused     ItemStatus = "paused"
	ItemIncubating ItemStatus = "incubating"
)

// ItemType categorizes a portfolio item.
type ItemType string

const (
	ItemRandD      ItemType = "r_and_d"
	ItemProduct    ItemType = "product"
	ItemInfra      ItemType = "infrastructure"
	ItemProcess    ItemType = "process"
	ItemExperiment ItemType = "experiment"
)

// PortfolioItem represents a single item in the R&D portfolio.
type PortfolioItem struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Type           ItemType   `json:"type"`
	Status         ItemStatus `json:"status"`
	Priority       float64    `json:"priority"`        // 0.0–1.0
	Investment     float64    `json:"investment"`      // cumulative investment
	ExpectedReturn float64    `json:"expected_return"` // projected return
	ActualReturn   float64    `json:"actual_return"`
	RiskLevel      float64    `json:"risk_level"` // 0.0–1.0
	ExplorationScore float64  `json:"exploration_score"` // 0.0–1.0
	ExploitationScore float64 `json:"exploitation_score"` // 0.0–1.0
	StartedAt      time.Time  `json:"started_at"`
	LastReviewed   time.Time  `json:"last_reviewed"`
	Tags           []string   `json:"tags,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	KillReason     string     `json:"kill_reason,omitempty"`
}

// RoadmapEntry represents a point on the technology roadmap.
type RoadmapEntry struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Quarter     string   `json:"quarter"` // e.g. "Q1-2025"
	Category    string   `json:"category"`
	Dependencies []string `json:"dependencies,omitempty"`
	Status      string   `json:"status"` // planned, in_progress, completed, deferred
	Priority    float64  `json:"priority"`
	ItemID      string   `json:"item_id,omitempty"` // linked portfolio item
}

// KillDecision records why and how a portfolio item was killed.
type KillDecision struct {
	ID          string    `json:"id"`
	ItemID      string    `json:"item_id"`
	Reason      string    `json:"reason"`
	Evidence    []string  `json:"evidence,omitempty"`
	DecidedBy   string    `json:"decided_by"`
	DecidedAt   time.Time `json:"decided_at"`
	SunkCost    float64   `json:"sunk_cost"`
	LessonsLearned []string `json:"lessons_learned,omitempty"`
	Irreversible bool     `json:"irreversible"`
}

// ExplorationBalance tracks the exploration vs exploitation ratio.
type ExplorationBalance struct {
	ID               string  `json:"id"`
	ExplorationRatio float64 `json:"exploration_ratio"` // 0.0–1.0
	TargetRatio      float64 `json:"target_ratio"`
	ItemsExploring   int     `json:"items_exploring"`
	ItemsExploiting  int     `json:"items_exploiting"`
	InvestmentExploration float64 `json:"investment_exploration"`
	InvestmentExploitation float64 `json:"investment_exploitation"`
	AssessedAt       time.Time `json:"assessed_at"`
}

// PortfolioScore is an overall portfolio health score.
type PortfolioScore struct {
	OverallScore      float64 `json:"overall_score"`
	DiversificationScore float64 `json:"diversification_score"`
	ReturnScore       float64 `json:"return_score"`
	RiskScore         float64 `json:"risk_score"`
	BalanceScore      float64 `json:"balance_score"`
	ItemsTotal        int     `json:"items_total"`
	ItemsActive       int     `json:"items_active"`
	ItemsKilled       int     `json:"items_killed"`
	AssessedAt        time.Time `json:"assessed_at"`
}

// Store manages portfolio data with JSON persistence.
type Store struct {
	items     []PortfolioItem
	roadmap   []RoadmapEntry
	kills     []KillDecision
	balances  []ExplorationBalance
	filePath  string
	mu        sync.RWMutex
	nextID    int
}

// NewStore creates a new portfolio store.
func NewStore(filePath string) *Store {
	return &Store{
		items:    make([]PortfolioItem, 0),
		roadmap:  make([]RoadmapEntry, 0),
		kills:    make([]KillDecision, 0),
		balances: make([]ExplorationBalance, 0),
		filePath: filePath,
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
		return fmt.Errorf("read portfolio file: %w", err)
	}

	var raw struct {
		Items    []PortfolioItem      `json:"items"`
		Roadmap  []RoadmapEntry       `json:"roadmap"`
		Kills    []KillDecision       `json:"kills"`
		Balances []ExplorationBalance `json:"balances"`
		NextID   int                  `json:"next_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse portfolio file: %w", err)
	}
	s.items = raw.Items
	s.roadmap = raw.Roadmap
	s.kills = raw.Kills
	s.balances = raw.Balances
	s.nextID = raw.NextID
	return nil
}

// Save writes the store to disk.
// Assumes the caller already holds s.mu (at least RLock).
func (s *Store) Save() error {

	raw := struct {
		Items    []PortfolioItem      `json:"items"`
		Roadmap  []RoadmapEntry       `json:"roadmap"`
		Kills    []KillDecision       `json:"kills"`
		Balances []ExplorationBalance `json:"balances"`
		NextID   int                  `json:"next_id"`
	}{
		Items:    s.items,
		Roadmap:  s.roadmap,
		Kills:    s.kills,
		Balances: s.balances,
		NextID:   s.nextID,
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal portfolio: %w", err)
	}
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create portfolio dir: %w", err)
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write portfolio file: %w", err)
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Store) genID(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%04d", prefix, s.nextID)
}

// ManagePortfolio adds or updates a portfolio item.
func (s *Store) ManagePortfolio(item PortfolioItem) (*PortfolioItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if item.ID == "" {
		item.ID = s.genID("itm")
		item.StartedAt = now
		item.Status = ItemActive
	}
	item.LastReviewed = now

	// Update existing or add new
	for i := range s.items {
		if s.items[i].ID == item.ID {
			s.items[i] = item
			return &s.items[i], s.Save()
		}
	}

	s.items = append(s.items, item)
	return &s.items[len(s.items)-1], s.Save()
}

// BuildRoadmap adds a roadmap entry.
func (s *Store) BuildRoadmap(entry RoadmapEntry) (*RoadmapEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = s.genID("rdm")
	}
	s.roadmap = append(s.roadmap, entry)

	// Sort by quarter
	sort.Slice(s.roadmap, func(i, j int) bool {
		return s.roadmap[i].Quarter < s.roadmap[j].Quarter
	})

	return &entry, s.Save()
}

// MakeKillDecision kills a portfolio item with a recorded decision.
func (s *Store) MakeKillDecision(itemID, reason, decidedBy string, evidence []string) (*KillDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.findItem(itemID)
	if item == nil {
		return nil, fmt.Errorf("item %s not found", itemID)
	}

	if item.Status == ItemKilled {
		return nil, fmt.Errorf("item %s already killed", itemID)
	}

	now := time.Now()
	kd := KillDecision{
		ID:         s.genID("kil"),
		ItemID:     itemID,
		Reason:     reason,
		Evidence:   evidence,
		DecidedBy:  decidedBy,
		DecidedAt:  now,
		SunkCost:   item.Investment,
		Irreversible: true,
	}

	item.Status = ItemKilled
	item.KillReason = reason

	s.kills = append(s.kills, kd)
	return &kd, s.Save()
}

// BalanceExploration assesses and returns the exploration/exploitation balance.
func (s *Store) BalanceExploration(targetRatio float64) (*ExplorationBalance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exploring := 0
	exploiting := 0
	invExplore := 0.0
	invExploit := 0.0

	for _, item := range s.items {
		if item.Status == ItemKilled {
			continue
		}
		if item.ExplorationScore > item.ExploitationScore {
			exploring++
			invExplore += item.Investment
		} else {
			exploiting++
			invExploit += item.Investment
		}
	}

	total := exploring + exploiting
	ratio := 0.5
	if total > 0 {
		ratio = float64(exploring) / float64(total)
	}

	balance := ExplorationBalance{
		ID:                    s.genID("bal"),
		ExplorationRatio:      ratio,
		TargetRatio:           targetRatio,
		ItemsExploring:        exploring,
		ItemsExploiting:       exploiting,
		InvestmentExploration: invExplore,
		InvestmentExploitation: invExploit,
		AssessedAt:            time.Now(),
	}

	s.balances = append(s.balances, balance)
	return &balance, s.Save()
}

// ScorePortfolio calculates the overall portfolio health score.
func (s *Store) ScorePortfolio() (*PortfolioScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.items) == 0 {
		return &PortfolioScore{AssessedAt: time.Now()}, nil
	}

	active := 0
	killed := 0
	typeCounts := make(map[ItemType]int)
	totalInvestment := 0.0
	totalReturn := 0.0
	totalRisk := 0.0
	explorationTotal := 0.0
	exploitationTotal := 0.0

	for _, item := range s.items {
		if item.Status == ItemActive || item.Status == ItemExploring || item.Status == ItemIncubating {
			active++
		}
		if item.Status == ItemKilled {
			killed++
		}
		typeCounts[item.Type]++
		totalInvestment += item.Investment
		totalReturn += item.ActualReturn
		totalRisk += item.RiskLevel
		explorationTotal += item.ExplorationScore
		exploitationTotal += item.ExploitationScore
	}

	// Diversification: more type spread = better
	diversification := float64(len(typeCounts)) / float64(len(s.items)) * 2
	if diversification > 1.0 {
		diversification = 1.0
	}

	// Return score
	returnScore := 0.0
	if totalInvestment > 0 {
		returnScore = totalReturn / totalInvestment
		if returnScore > 1.0 {
			returnScore = 1.0
		}
	}

	// Risk score (inverse — lower risk = higher score)
	avgRisk := totalRisk / float64(len(s.items))
	riskScore := 1.0 - avgRisk

	// Balance score
	balanceScore := 0.5
	if explorationTotal+exploitationTotal > 0 {
		ratio := explorationTotal / (explorationTotal + exploitationTotal)
		// Optimal around 0.3–0.5
		if ratio >= 0.3 && ratio <= 0.5 {
			balanceScore = 1.0
		} else if ratio >= 0.2 && ratio <= 0.6 {
			balanceScore = 0.7
		} else {
			balanceScore = 0.4
		}
	}

	overall := (diversification*0.2 + returnScore*0.3 + riskScore*0.25 + balanceScore*0.25)
	if overall > 1.0 {
		overall = 1.0
	}

	return &PortfolioScore{
		OverallScore:        overall,
		DiversificationScore: diversification,
		ReturnScore:         returnScore,
		RiskScore:           riskScore,
		BalanceScore:        balanceScore,
		ItemsTotal:          len(s.items),
		ItemsActive:         active,
		ItemsKilled:         killed,
		AssessedAt:          time.Now(),
	}, nil
}

// GeneratePortfolioReport produces a human-readable portfolio report.
func (s *Store) GeneratePortfolioReport() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	score, _ := s.ScorePortfolio()

	out := "=== Portfolio Report ===\n\n"
	out += fmt.Sprintf("Overall Score: %.2f\n", score.OverallScore)
	out += fmt.Sprintf("Items: %d total | %d active | %d killed\n", score.ItemsTotal, score.ItemsActive, score.ItemsKilled)
	out += fmt.Sprintf("Diversification: %.2f | Return: %.2f | Risk: %.2f | Balance: %.2f\n\n",
		score.DiversificationScore, score.ReturnScore, score.RiskScore, score.BalanceScore)

	if len(s.items) > 0 {
		out += "Portfolio Items:\n"
		for _, item := range s.items {
			out += fmt.Sprintf("  %s [%s/%s] invest=%.0f return=%.0f risk=%.2f\n",
				item.Name, item.Type, item.Status, item.Investment, item.ActualReturn, item.RiskLevel)
		}
	}

	if len(s.roadmap) > 0 {
		out += "\nRoadmap:\n"
		for _, r := range s.roadmap {
			out += fmt.Sprintf("  %s (%s) [%s] - %s\n", r.Title, r.Quarter, r.Category, r.Status)
		}
	}

	if len(s.kills) > 0 {
		out += fmt.Sprintf("\nKill Decisions: %d\n", len(s.kills))
	}

	return out
}

// ListItems returns all portfolio items.
func (s *Store) ListItems() []PortfolioItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PortfolioItem, len(s.items))
	copy(out, s.items)
	return out
}

// ListRoadmap returns all roadmap entries.
func (s *Store) ListRoadmap() []RoadmapEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RoadmapEntry, len(s.roadmap))
	copy(out, s.roadmap)
	return out
}

// ListKillDecisions returns all kill decisions.
func (s *Store) ListKillDecisions() []KillDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]KillDecision, len(s.kills))
	copy(out, s.kills)
	return out
}

func (s *Store) findItem(id string) *PortfolioItem {
	for i := range s.items {
		if s.items[i].ID == id {
			return &s.items[i]
		}
	}
	return nil
}
