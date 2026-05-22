// Package temporal provides awareness of market cycles, hype cycles, regulatory
// cycles, generational timing, and work rhythm. It closes the gap in temporal
// intelligence — enabling the Forge to reason about when things happen, not just
// what happens, so decisions account for cyclicality and rhythm.
package temporal

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// MarketCycle represents a phase in a market cycle.
type MarketCycle struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Phase      string    `json:"phase"`       // expansion, peak, contraction, trough
	Sector     string    `json:"sector"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Confidence float64   `json:"confidence"`  // 0-1
	Indicators []string  `json:"indicators"`
}

// HypeCyclePosition represents where a technology sits on the Gartner hype cycle.
type HypeCyclePosition struct {
	ID              string    `json:"id"`
	Technology      string    `json:"technology"`
	Phase           string    `json:"phase"` // trigger, peak, trough, slope, plateau
	EstimatedYear   int       `json:"estimated_year"`
	MaturityPercent float64   `json:"maturity_percent"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RegulatoryCycle tracks regulatory environment changes.
type RegulatoryCycle struct {
	ID          string    `json:"id"`
	Jurisdiction string   `json:"jurisdiction"`
	Category    string    `json:"category"`
	Phase       string    `json:"phase"` // loosening, stable, tightening, upheaval
	ChangeDate  time.Time `json:"change_date"`
	Description string    `json:"description"`
	Impact      string    `json:"impact"` // low, medium, high
}

// GenerationalTrend captures demographic and generational shifts.
type GenerationalTrend struct {
	ID          string    `json:"id"`
	Generation  string    `json:"generation"` // gen_z, millennial, gen_x, boomer
	Trend       string    `json:"trend"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Significance float64  `json:"significance"` // 0-1
}

// WorkRhythm captures periodic work patterns and cadences.
type WorkRhythm struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Period    string    `json:"period"`    // daily, weekly, monthly, quarterly, annual
	PeakTimes []string  `json:"peak_times"`
	TroughTimes []string `json:"trough_times"`
	Notes     string    `json:"notes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemporalReport is a consolidated report of all temporal data.
type TemporalReport struct {
	GeneratedAt       time.Time          `json:"generated_at"`
	MarketCycles      []MarketCycle      `json:"market_cycles"`
	HypePositions     []HypeCyclePosition `json:"hype_positions"`
	RegulatoryCycles  []RegulatoryCycle  `json:"regulatory_cycles"`
	GenerationalTrends []GenerationalTrend `json:"generational_trends"`
	WorkRhythms       []WorkRhythm       `json:"work_rhythms"`
}

// Store persists temporal data to a JSON file with thread safety.
type Store struct {
	mu                sync.Mutex
	filePath          string
	MarketCycles      []MarketCycle       `json:"market_cycles"`
	HypePositions     []HypeCyclePosition `json:"hype_positions"`
	RegulatoryCycles  []RegulatoryCycle   `json:"regulatory_cycles"`
	GenerationalTrends []GenerationalTrend `json:"generational_trends"`
	WorkRhythms       []WorkRhythm        `json:"work_rhythms"`
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

// AddMarketCycle appends a market cycle safely under lock and persists.
func (s *Store) AddMarketCycle(c MarketCycle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MarketCycles = append(s.MarketCycles, c)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// IdentifyCycle classifies a market cycle phase from indicators.
func IdentifyCycle(indicators map[string]float64) MarketCycle {
	phase := "stable"
	confidence := 0.5

	growth, hasGrowth := indicators["gdp_growth"]
	unemployment, hasUnemp := indicators["unemployment"]

	if hasGrowth && hasUnemp {
		if growth > 3.0 && unemployment < 5.0 {
			phase = "expansion"
			confidence = 0.8
		} else if growth > 2.0 && unemployment < 6.0 {
			phase = "peak"
			confidence = 0.6
		} else if growth < 0.5 && unemployment > 9.0 {
			phase = "trough"
			confidence = 0.7
		} else if growth < 1.0 && unemployment > 7.0 {
			phase = "contraction"
			confidence = 0.75
		}
	}

	inds := make([]string, 0, len(indicators))
	for k := range indicators {
		inds = append(inds, k)
	}

	return MarketCycle{
		ID:         generateID("mc"),
		Phase:      phase,
		Confidence: confidence,
		Indicators: inds,
		StartTime:  time.Now(),
	}
}

// AssessHypePosition determines where a technology sits on the hype cycle.
func AssessHypePosition(technology string, mediaMentions, investmentFlow, adoptionRate float64) HypeCyclePosition {
	phase := "trigger"
	maturity := 0.1

	score := mediaMentions*0.3 + investmentFlow*0.3 + adoptionRate*0.4

	switch {
	case mediaMentions > 0.8 && adoptionRate < 0.3:
		phase = "peak"
		maturity = 0.3
	case mediaMentions < 0.3 && investmentFlow < 0.3 && adoptionRate < 0.3:
		phase = "trough"
		maturity = 0.4
	case investmentFlow > 0.5 && adoptionRate > 0.3 && adoptionRate < 0.7:
		phase = "slope"
		maturity = 0.7
	case adoptionRate > 0.7:
		phase = "plateau"
		maturity = 0.95
	case score > 0.2:
		phase = "trigger"
		maturity = 0.1
	}

	return HypeCyclePosition{
		ID:              generateID("hc"),
		Technology:      technology,
		Phase:           phase,
		MaturityPercent: maturity,
		EstimatedYear:   time.Now().Year(),
		UpdatedAt:       time.Now(),
	}
}

// TrackRegulatoryChanges records a regulatory cycle change.
func TrackRegulatoryChanges(jurisdiction, category, description, impact string) RegulatoryCycle {
	phase := "stable"
	switch impact {
	case "high":
		phase = "upheaval"
	case "medium":
		phase = "tightening"
	case "low":
		phase = "loosening"
	}

	return RegulatoryCycle{
		ID:           generateID("rc"),
		Jurisdiction: jurisdiction,
		Category:     category,
		Phase:        phase,
		ChangeDate:   time.Now(),
		Description:  description,
		Impact:       impact,
	}
}

// AnalyzeGenerationalTiming evaluates a generational trend's significance.
func AnalyzeGenerationalTiming(generation, trend string, significance float64) GenerationalTrend {
	return GenerationalTrend{
		ID:           generateID("gt"),
		Generation:   generation,
		Trend:        trend,
		StartDate:    time.Now(),
		Significance: significance,
	}
}

// GenerateRhythm creates a work rhythm from peak and trough time patterns.
func GenerateRhythm(name, period string, peaks, troughs []string) WorkRhythm {
	return WorkRhythm{
		ID:          generateID("wr"),
		Name:        name,
		Period:      period,
		PeakTimes:   peaks,
		TroughTimes: troughs,
		UpdatedAt:   time.Now(),
	}
}

// GenerateTemporalReport produces a consolidated temporal report.
func GenerateTemporalReport(s *Store) TemporalReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return TemporalReport{
		GeneratedAt:        time.Now(),
		MarketCycles:       s.MarketCycles,
		HypePositions:      s.HypePositions,
		RegulatoryCycles:   s.RegulatoryCycles,
		GenerationalTrends: s.GenerationalTrends,
		WorkRhythms:        s.WorkRhythms,
	}
}

func generateID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
