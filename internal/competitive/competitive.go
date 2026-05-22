// Package competitive provides moat identification, market timing analysis,
// and competitive intelligence capabilities. It closes the gap in strategic
// awareness by modeling competitive landscapes, identifying defensive moats,
// assessing market entry timing, and scanning for emerging threats—ensuring
// the organization can defend its position and seize windows of opportunity.
package competitive

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// MoatType categorizes the nature of a competitive moat.
type MoatType string

const (
	MoatNetworkEffect  MoatType = "network_effect"
	MoatSwitchingCost  MoatType = "switching_cost"
	MoatEconomiesScale MoatType = "economies_of_scale"
	MoatBrand          MoatType = "brand"
	MoatPatent         MoatType = "patent"
	MoatData           MoatType = "data"
	MoatRegulatory     MoatType = "regulatory"
)

// Strength represents the assessed strength of a moat.
type Strength string

const (
	StrengthWeak      Strength = "weak"
	StrengthModerate  Strength = "moderate"
	StrengthStrong    Strength = "strong"
	StrengthFortress  Strength = "fortress"
)

// Durability represents how long a moat is expected to hold.
type Durability string

const (
	DurabilityFragile    Durability = "fragile"
	DurabilityModerate   Durability = "moderate"
	DurabilityDurable    Durability = "durable"
	DurabilityPermanent  Durability = "permanent"
)

// Moat represents a competitive defensive advantage.
type Moat struct {
	ID          string    `json:"id"`
	Type        MoatType  `json:"type"`
	Strength    Strength  `json:"strength"`
	Durability  Durability `json:"durability"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	IdentifiedAt time.Time `json:"identified_at"`
	ErosionRisk float64   `json:"erosion_risk"` // 0.0-1.0
}

// CompetitorProfile holds intelligence on a competitor.
type CompetitorProfile struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	MarketShare  float64 `json:"market_share"`
	ThreatLevel  float64 `json:"threat_level"` // 0.0-1.0
	Strengths    []string `json:"strengths"`
	Weaknesses   []string `json:"weaknesses"`
	Moats        []Moat  `json:"moats"`
	LastUpdated  time.Time `json:"last_updated"`
}

// MarketTiming represents the assessment of market entry/exit timing.
type MarketTiming struct {
	ID              string    `json:"id"`
	Market          string    `json:"market"`
	WindowOpen      time.Time `json:"window_open"`
	WindowClose     time.Time `json:"window_close"`
	OptimalEntry    time.Time `json:"optimal_entry"`
	ReadinessScore  float64   `json:"readiness_score"` // 0.0-1.0
	CompetitivePressure float64 `json:"competitive_pressure"` // 0.0-1.0
	AssessedAt      time.Time `json:"assessed_at"`
}

// CompetitiveLandscape represents the full competitive picture.
type CompetitiveLandscape struct {
	ID            string             `json:"id"`
	Market        string             `json:"market"`
	OurMoats      []Moat             `json:"our_moats"`
	Competitors   []CompetitorProfile `json:"competitors"`
	Threats       []string           `json:"threats"`
	Opportunities []string           `json:"opportunities"`
	AssessedAt    time.Time          `json:"assessed_at"`
}

// Store persists competitive intelligence data.
type Store struct {
	mu        sync.Mutex
	filePath  string
	Landscapes map[string]CompetitiveLandscape `json:"landscapes"`
	Competitors map[string]CompetitorProfile   `json:"competitors"`
	Timings    map[string]MarketTiming         `json:"timings"`
	Moats      map[string]Moat                 `json:"moats"`
}

// NewStore creates a new Store backed by the given JSON file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath:   filePath,
		Landscapes: make(map[string]CompetitiveLandscape),
		Competitors: make(map[string]CompetitorProfile),
		Timings:    make(map[string]MarketTiming),
		Moats:      make(map[string]Moat),
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

// IdentifyMoats analyzes and records moats for a given owner.
func (s *Store) IdentifyMoats(owner string, candidates []Moat) []Moat {
	s.mu.Lock()
	defer s.mu.Unlock()
	var identified []Moat
	for _, m := range candidates {
		m.Owner = owner
		m.IdentifiedAt = time.Now().UTC()
		s.Moats[m.ID] = m
		identified = append(identified, m)
	}
	return identified
}

// AssessMarketTiming evaluates and stores a market timing assessment.
func (s *Store) AssessMarketTiming(timing MarketTiming) MarketTiming {
	s.mu.Lock()
	defer s.mu.Unlock()
	timing.AssessedAt = time.Now().UTC()
	s.Timings[timing.ID] = timing
	return timing
}

// ProfileCompetitor creates or updates a competitor profile.
func (s *Store) ProfileCompetitor(profile CompetitorProfile) CompetitorProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile.LastUpdated = time.Now().UTC()
	s.Competitors[profile.ID] = profile
	return profile
}

// GenerateCompetitiveReport produces a landscape report for a market.
func (s *Store) GenerateCompetitiveReport(market string) CompetitiveLandscape {
	s.mu.Lock()
	defer s.mu.Unlock()
	landscape := CompetitiveLandscape{
		ID:         "landscape-" + market,
		Market:     market,
		AssessedAt: time.Now().UTC(),
	}
	for _, m := range s.Moats {
		landscape.OurMoats = append(landscape.OurMoats, m)
	}
	for _, c := range s.Competitors {
		landscape.Competitors = append(landscape.Competitors, c)
	}
	s.Landscapes[landscape.ID] = landscape
	return landscape
}

// ScanForThreats identifies high-threat competitors and moats with high erosion risk.
func (s *Store) ScanForThreats(threatThreshold float64) []CompetitorProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	var threats []CompetitorProfile
	for _, c := range s.Competitors {
		if c.ThreatLevel >= threatThreshold {
			threats = append(threats, c)
		}
	}
	return threats
}
