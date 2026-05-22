// Package evolution provides org development stage assessment, pivot detection,
// spinoff/speciation proposals, culture development tracking, and maturity modeling.
// It closes the gap in organizational evolution intelligence — enabling the Forge
// to reason about where an org is in its lifecycle and where it's heading.
package evolution

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// DevStage represents a stage in organizational development.
type DevStage struct {
	ID          string    `json:"id"`
	Stage       string    `json:"stage"` // seed, startup, growth, scale, mature, decline
	OrgID       string    `json:"org_id"`
	AssessedAt  time.Time `json:"assessed_at"`
	Revenue     float64   `json:"revenue"`
	Headcount   int       `json:"headcount"`
	Confidence  float64   `json:"confidence"`
	Notes       string    `json:"notes"`
}

// PivotSignal indicates a potential need for strategic redirection.
type PivotSignal struct {
	ID           string    `json:"id"`
	SignalType   string    `json:"signal_type"` // market_shift, tech_disruption, customer_churn, internal_crisis
	Strength     float64   `json:"strength"`    // 0-1
	Description  string    `json:"description"`
	DetectedAt   time.Time `json:"detected_at"`
	ActionNeeded bool      `json:"action_needed"`
}

// Spinoff represents a proposed or actual organizational spinoff.
type Spinoff struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	FromOrgID   string    `json:"from_org_id"`
	Rationale   string    `json:"rationale"`
	Status      string    `json:"status"` // proposed, approved, active, dissolved
	ProposedAt  time.Time `json:"proposed_at"`
	TeamSize    int       `json:"team_size"`
}

// MaturityLevel captures organizational maturity across dimensions.
type MaturityLevel struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	ProcessMaturity float64   `json:"process_maturity"` // 0-1
	TechMaturity    float64   `json:"tech_maturity"`
	CultureMaturity float64   `json:"culture_maturity"`
	OverallMaturity float64   `json:"overall_maturity"`
	AssessedAt      time.Time `json:"assessed_at"`
}

// EvolutionRecord tracks an org's evolution over time.
type EvolutionRecord struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	FromStage  string    `json:"from_stage"`
	ToStage    string    `json:"to_stage"`
	ChangedAt  time.Time `json:"changed_at"`
	Trigger    string    `json:"trigger"`
	Notes      string    `json:"notes"`
}

// EvolutionReport is a consolidated report of evolution data.
type EvolutionReport struct {
	GeneratedAt    time.Time        `json:"generated_at"`
	DevStages      []DevStage       `json:"dev_stages"`
	PivotSignals   []PivotSignal    `json:"pivot_signals"`
	Spinoffs       []Spinoff        `json:"spinoffs"`
	MaturityLevels []MaturityLevel  `json:"maturity_levels"`
	EvolutionRecords []EvolutionRecord `json:"evolution_records"`
}

// Store persists evolution data to a JSON file with thread safety.
type Store struct {
	mu               sync.Mutex
	filePath         string
	DevStages        []DevStage        `json:"dev_stages"`
	PivotSignals     []PivotSignal     `json:"pivot_signals"`
	Spinoffs         []Spinoff         `json:"spinoffs"`
	MaturityLevels   []MaturityLevel   `json:"maturity_levels"`
	EvolutionRecords []EvolutionRecord `json:"evolution_records"`
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

// AssessStage determines the current development stage from metrics.
func AssessStage(orgID string, revenue float64, headcount int) DevStage {
	stage := "seed"
	confidence := 0.5

	switch {
	case revenue == 0 && headcount <= 3:
		stage = "seed"
		confidence = 0.9
	case revenue < 1_000_000 && headcount <= 15:
		stage = "startup"
		confidence = 0.8
	case revenue < 10_000_000 && headcount <= 100:
		stage = "growth"
		confidence = 0.7
	case revenue < 100_000_000 && headcount <= 500:
		stage = "scale"
		confidence = 0.7
	case revenue >= 100_000_000:
		stage = "mature"
		confidence = 0.6
	case headcount > 500 && revenue < 10_000_000:
		stage = "decline"
		confidence = 0.5
	}

	return DevStage{
		ID:         genID("ds"),
		OrgID:      orgID,
		Stage:      stage,
		Revenue:    revenue,
		Headcount:  headcount,
		Confidence: confidence,
		AssessedAt: time.Now(),
	}
}

// DetectPivotSignals scans for indicators that a pivot may be needed.
func DetectPivotSignals(metrics map[string]float64) []PivotSignal {
	var signals []PivotSignal

	if churn, ok := metrics["customer_churn_rate"]; ok && churn > 0.2 {
		signals = append(signals, PivotSignal{
			ID:           genID("ps"),
			SignalType:   "customer_churn",
			Strength:     churn,
			Description:  "Customer churn exceeds 20%",
			DetectedAt:   time.Now(),
			ActionNeeded: churn > 0.3,
		})
	}

	if disruption, ok := metrics["tech_disruption_score"]; ok && disruption > 0.6 {
		signals = append(signals, PivotSignal{
			ID:           genID("ps"),
			SignalType:   "tech_disruption",
			Strength:     disruption,
			Description:  "Significant technology disruption detected",
			DetectedAt:   time.Now(),
			ActionNeeded: disruption > 0.8,
		})
	}

	if marketShift, ok := metrics["market_shift_score"]; ok && marketShift > 0.5 {
		signals = append(signals, PivotSignal{
			ID:           genID("ps"),
			SignalType:   "market_shift",
			Strength:     marketShift,
			Description:  "Market shift detected",
			DetectedAt:   time.Now(),
			ActionNeeded: marketShift > 0.7,
		})
	}

	return signals
}

// ProposeSpinoff creates a spinoff proposal from rationale.
func ProposeSpinoff(name, fromOrgID, rationale string, teamSize int) Spinoff {
	return Spinoff{
		ID:         genID("sp"),
		Name:       name,
		FromOrgID:  fromOrgID,
		Rationale:  rationale,
		Status:     "proposed",
		ProposedAt: time.Now(),
		TeamSize:   teamSize,
	}
}

// MeasureMaturity computes maturity across process, tech, and culture dimensions.
func MeasureMaturity(orgID string, process, tech, culture float64) MaturityLevel {
	overall := (process*0.35 + tech*0.35 + culture*0.30)
	return MaturityLevel{
		ID:              genID("ml"),
		OrgID:           orgID,
		ProcessMaturity: process,
		TechMaturity:    tech,
		CultureMaturity: culture,
		OverallMaturity: overall,
		AssessedAt:      time.Now(),
	}
}

// TrackEvolution records a stage transition.
func TrackEvolution(orgID, fromStage, toStage, trigger string) EvolutionRecord {
	return EvolutionRecord{
		ID:        genID("er"),
		OrgID:     orgID,
		FromStage: fromStage,
		ToStage:   toStage,
		ChangedAt: time.Now(),
		Trigger:   trigger,
	}
}

// GenerateEvolutionReport produces a consolidated evolution report.
func GenerateEvolutionReport(s *Store) EvolutionReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return EvolutionReport{
		GeneratedAt:      time.Now(),
		DevStages:        s.DevStages,
		PivotSignals:     s.PivotSignals,
		Spinoffs:         s.Spinoffs,
		MaturityLevels:   s.MaturityLevels,
		EvolutionRecords: s.EvolutionRecords,
	}
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
