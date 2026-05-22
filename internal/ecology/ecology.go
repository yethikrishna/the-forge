// Package ecology provides carbon footprint tracking, resource usage measurement,
// sustainability scoring, and sustainable growth modeling. It closes the gap in
// ecological intelligence — enabling the Forge to reason about environmental
// impact and sustainable practices alongside business metrics.
package ecology

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// CarbonFootprint tracks carbon emissions for an entity.
type CarbonFootprint struct {
	ID            string    `json:"id"`
	EntityID      string    `json:"entity_id"`
	TonnesCO2     float64   `json:"tonnes_co2"`
	Scope         string    `json:"scope"` // scope1, scope2, scope3
	Category      string    `json:"category"`
	Period        string    `json:"period"` // monthly, quarterly, annual
	RecordedAt    time.Time `json:"recorded_at"`
	OffsetPercent float64   `json:"offset_percent"`
}

// ResourceUsage tracks consumption of a specific resource.
type ResourceUsage struct {
	ID          string    `json:"id"`
	EntityID    string    `json:"entity_id"`
	Resource    string    `json:"resource"` // compute, storage, bandwidth, energy, water
	Amount      float64   `json:"amount"`
	Unit        string    `json:"unit"` // kwh, gb, tb, gallons
	Period      string    `json:"period"`
	Efficiency  float64   `json:"efficiency"` // 0-1 utilization efficiency
	RecordedAt  time.Time `json:"recorded_at"`
}

// SustainabilityScore represents an overall sustainability assessment.
type SustainabilityScore struct {
	ID               string    `json:"id"`
	EntityID         string    `json:"entity_id"`
	CarbonScore      float64   `json:"carbon_score"`      // 0-1
	ResourceScore    float64   `json:"resource_score"`    // 0-1
	WasteScore       float64   `json:"waste_score"`       // 0-1
	OverallScore     float64   `json:"overall_score"`     // 0-1
	Grade            string    `json:"grade"`             // A, B, C, D, F
	AssessedAt       time.Time `json:"assessed_at"`
}

// GrowthModel represents a sustainable growth projection.
type GrowthModel struct {
	ID                string    `json:"id"`
	EntityID          string    `json:"entity_id"`
	CurrentRevenue    float64   `json:"current_revenue"`
	TargetRevenue     float64   `json:"target_revenue"`
	GrowthRateAnnual  float64   `json:"growth_rate_annual"`
	CarbonBudgetTonnes float64  `json:"carbon_budget_tonnes"`
	ResourceBudget    map[string]float64 `json:"resource_budget"`
	IsSustainable     bool      `json:"is_sustainable"`
	ProjectedAt       time.Time `json:"projected_at"`
}

// EcologyReport is a consolidated report of ecological data.
type EcologyReport struct {
	GeneratedAt        time.Time          `json:"generated_at"`
	CarbonFootprints   []CarbonFootprint  `json:"carbon_footprints"`
	ResourceUsages     []ResourceUsage    `json:"resource_usages"`
	SustainabilityScores []SustainabilityScore `json:"sustainability_scores"`
	GrowthModels       []GrowthModel      `json:"growth_models"`
}

// Store persists ecology data to a JSON file with thread safety.
type Store struct {
	mu                  sync.Mutex
	filePath            string
	CarbonFootprints    []CarbonFootprint     `json:"carbon_footprints"`
	ResourceUsages      []ResourceUsage       `json:"resource_usages"`
	SustainabilityScores []SustainabilityScore `json:"sustainability_scores"`
	GrowthModels        []GrowthModel         `json:"growth_models"`
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

// TrackCarbon records a carbon footprint entry.
func TrackCarbon(entityID, scope, category, period string, tonnesCO2, offsetPercent float64) CarbonFootprint {
	return CarbonFootprint{
		ID:            genID("cf"),
		EntityID:      entityID,
		TonnesCO2:     tonnesCO2,
		Scope:         scope,
		Category:      category,
		Period:        period,
		RecordedAt:    time.Now(),
		OffsetPercent: offsetPercent,
	}
}

// MeasureResourceUsage records resource consumption data.
func MeasureResourceUsage(entityID, resource, unit, period string, amount, efficiency float64) ResourceUsage {
	return ResourceUsage{
		ID:         genID("ru"),
		EntityID:   entityID,
		Resource:   resource,
		Amount:     amount,
		Unit:       unit,
		Period:     period,
		Efficiency: efficiency,
		RecordedAt: time.Now(),
	}
}

// AssessSustainability computes a sustainability score from component scores.
func AssessSustainability(entityID string, carbon, resource, waste float64) SustainabilityScore {
	overall := carbon*0.4 + resource*0.35 + waste*0.25

	grade := "F"
	switch {
	case overall >= 0.9:
		grade = "A"
	case overall >= 0.75:
		grade = "B"
	case overall >= 0.6:
		grade = "C"
	case overall >= 0.4:
		grade = "D"
	}

	return SustainabilityScore{
		ID:            genID("ss"),
		EntityID:      entityID,
		CarbonScore:   carbon,
		ResourceScore: resource,
		WasteScore:    waste,
		OverallScore:  overall,
		Grade:         grade,
		AssessedAt:    time.Now(),
	}
}

// ModelSustainableGrowth projects whether a growth plan is ecologically sustainable.
func ModelSustainableGrowth(entityID string, currentRev, targetRev, growthRate, carbonBudget float64, resourceBudget map[string]float64) GrowthModel {
	yearsToTarget := 0.0
	if growthRate > 0 && currentRev > 0 {
		yearsToTarget = (targetRev - currentRev) / (currentRev * growthRate)
	}

	// Assume carbon scales linearly with revenue unless offset
	projectedCarbon := carbonBudget * (1 + growthRate*0.5) // conservative estimate
	isSustainable := projectedCarbon <= carbonBudget && yearsToTarget > 0 && yearsToTarget <= 10

	return GrowthModel{
		ID:                 genID("gm"),
		EntityID:           entityID,
		CurrentRevenue:     currentRev,
		TargetRevenue:      targetRev,
		GrowthRateAnnual:   growthRate,
		CarbonBudgetTonnes: carbonBudget,
		ResourceBudget:     resourceBudget,
		IsSustainable:      isSustainable,
		ProjectedAt:        time.Now(),
	}
}

// GenerateEcologyReport produces a consolidated ecology report.
func GenerateEcologyReport(s *Store) EcologyReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return EcologyReport{
		GeneratedAt:         time.Now(),
		CarbonFootprints:    s.CarbonFootprints,
		ResourceUsages:      s.ResourceUsages,
		SustainabilityScores: s.SustainabilityScores,
		GrowthModels:        s.GrowthModels,
	}
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
