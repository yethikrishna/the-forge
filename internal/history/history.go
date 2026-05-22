// Package history provides industry case study tracking, own-history reasoning,
// cross-era pattern recognition, and collapse awareness. It closes the gap in
// historical intelligence — enabling the Forge to learn from the past, recognize
// recurring patterns across eras, and detect early collapse signals.
package history

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// CaseStudy represents an industry case study with lessons.
type CaseStudy struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Industry    string    `json:"industry"`
	Era         string    `json:"era"` // 1990s, 2000s, 2010s, 2020s
	Summary     string    `json:"summary"`
	Lessons     []string  `json:"lessons"`
	Outcome     string    `json:"outcome"` // success, failure, mixed
	RecordedAt  time.Time `json:"recorded_at"`
}

// HistoricalDecision records a past decision and its consequences.
type HistoricalDecision struct {
	ID           string    `json:"id"`
	Context      string    `json:"context"`
	Decision     string    `json:"decision"`
	Rationale    string    `json:"rationale"`
	Consequence  string    `json:"consequence"`
	Quality      string    `json:"quality"` // good, bad, mixed
	DecidedAt    time.Time `json:"decided_at"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

// Pattern represents a cross-era pattern recognized from historical data.
type Pattern struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Eras        []string  `json:"eras"`
	Frequency   int       `json:"frequency"`
	Reliability  float64  `json:"reliability"` // 0-1
	IdentifiedAt time.Time `json:"identified_at"`
}

// CollapseSignal indicates a potential organizational or market collapse.
type CollapseSignal struct {
	ID           string    `json:"id"`
	SignalType   string    `json:"signal_type"` // financial, cultural, competitive, technical
	Strength     float64   `json:"strength"`    // 0-1
	Description  string    `json:"description"`
	HistoricalPrecedent string `json:"historical_precedent"`
	DetectedAt   time.Time `json:"detected_at"`
	Urgency      string    `json:"urgency"` // low, medium, high, critical
}

// HistoryRecord is a general record in the history log.
type HistoryRecord struct {
	ID        string    `json:"id"`
	EventType string    `json:"event_type"`
	Summary   string    `json:"summary"`
	Details   string    `json:"details"`
	RecordedAt time.Time `json:"recorded_at"`
}

// HistoryReport is a consolidated report of historical data.
type HistoryReport struct {
	GeneratedAt        time.Time           `json:"generated_at"`
	CaseStudies        []CaseStudy         `json:"case_studies"`
	HistoricalDecisions []HistoricalDecision `json:"historical_decisions"`
	Patterns           []Pattern           `json:"patterns"`
	CollapseSignals    []CollapseSignal    `json:"collapse_signals"`
	HistoryRecords     []HistoryRecord     `json:"history_records"`
}

// Store persists history data to a JSON file with thread safety.
type Store struct {
	mu                  sync.Mutex
	filePath            string
	CaseStudies         []CaseStudy         `json:"case_studies"`
	HistoricalDecisions []HistoricalDecision `json:"historical_decisions"`
	Patterns            []Pattern           `json:"patterns"`
	CollapseSignals     []CollapseSignal    `json:"collapse_signals"`
	HistoryRecords      []HistoryRecord     `json:"history_records"`
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

// RecordCaseStudy creates a new case study entry.
func RecordCaseStudy(title, industry, era, summary, outcome string, lessons []string) CaseStudy {
	return CaseStudy{
		ID:         genID("cs"),
		Title:      title,
		Industry:   industry,
		Era:        era,
		Summary:    summary,
		Lessons:    lessons,
		Outcome:    outcome,
		RecordedAt: time.Now(),
	}
}

// SearchHistory finds case studies matching a query in title, summary, or lessons.
func SearchHistory(studies []CaseStudy, query string) []CaseStudy {
	var results []CaseStudy
	for _, cs := range studies {
		if contains(cs.Title, query) || contains(cs.Summary, query) {
			results = append(results, cs)
			continue
		}
		for _, l := range cs.Lessons {
			if contains(l, query) {
				results = append(results, cs)
				break
			}
		}
	}
	return results
}

// RecognizePatterns identifies cross-era patterns from case studies.
func RecognizePatterns(studies []CaseStudy) []Pattern {
	eraMap := make(map[string][]CaseStudy)
	for _, cs := range studies {
		eraMap[cs.Era] = append(eraMap[cs.Era], cs)
	}

	outcomeMap := make(map[string]int)
	for _, cs := range studies {
		outcomeMap[cs.Outcome]++
	}

	var patterns []Pattern

	for outcome, count := range outcomeMap {
		if count >= 2 {
			var eras []string
			for era, eraStudies := range eraMap {
				for _, es := range eraStudies {
					if es.Outcome == outcome {
						eras = append(eras, era)
						break
					}
				}
			}
			patterns = append(patterns, Pattern{
				ID:           genID("pt"),
				Name:         "recurring_" + outcome,
				Description:  outcome + " outcomes appear across multiple eras",
				Eras:         eras,
				Frequency:    count,
				Reliability:  float64(count) / float64(len(studies)),
				IdentifiedAt: time.Now(),
			})
		}
	}

	return patterns
}

// DetectCollapseSignals analyzes metrics for collapse indicators.
func DetectCollapseSignals(metrics map[string]float64) []CollapseSignal {
	var signals []CollapseSignal

	if burn, ok := metrics["burn_rate_multiplier"]; ok && burn > 2.0 {
		urgency := "medium"
		if burn > 3.0 {
			urgency = "critical"
		} else if burn > 2.5 {
			urgency = "high"
		}
		signals = append(signals, CollapseSignal{
			ID:                  genID("col"),
			SignalType:          "financial",
			Strength:            burn / 5.0,
			Description:         "Burn rate significantly elevated",
			HistoricalPrecedent: "dot-com bubble companies 2000-2001",
			DetectedAt:          time.Now(),
			Urgency:             urgency,
		})
	}

	if talent, ok := metrics["key_talent_departure_rate"]; ok && talent > 0.15 {
		signals = append(signals, CollapseSignal{
			ID:                  genID("col"),
			SignalType:          "cultural",
			Strength:            talent,
			Description:         "Key talent departure rate elevated",
			HistoricalPrecedent: "Netscape talent drain 1998",
			DetectedAt:          time.Now(),
			Urgency:             "high",
		})
	}

	if competition, ok := metrics["competitive_pressure"]; ok && competition > 0.7 {
		signals = append(signals, CollapseSignal{
			ID:                  genID("col"),
			SignalType:          "competitive",
			Strength:            competition,
			Description:         "Competitive pressure exceeding thresholds",
			HistoricalPrecedent: "BlackBerry vs iPhone 2007-2012",
			DetectedAt:          time.Now(),
			Urgency:             "medium",
		})
	}

	return signals
}

// GenerateHistoryReport produces a consolidated history report.
func GenerateHistoryReport(s *Store) HistoryReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return HistoryReport{
		GeneratedAt:         time.Now(),
		CaseStudies:         s.CaseStudies,
		HistoricalDecisions: s.HistoricalDecisions,
		Patterns:            s.Patterns,
		CollapseSignals:     s.CollapseSignals,
		HistoryRecords:      s.HistoryRecords,
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
