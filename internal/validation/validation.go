// Package validation closes the gap between simulation and reality.
// Organizations run internal simulations to predict outcomes, but without
// systematic reality-checking, simulations drift from truth. This package
// registers simulations, runs reality checks, and flags divergences before
// they compound into catastrophic decisions.
package validation

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

// Simulation represents a predicted state or outcome.
type Simulation struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Domain      string                 `json:"domain"`
	Predicted   map[string]float64     `json:"predicted"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// RealityCheck compares a simulation against actual observed values.
type RealityCheck struct {
	ID            string             `json:"id"`
	SimulationID  string             `json:"simulation_id"`
	Actual        map[string]float64 `json:"actual"`
	AccuracyScore float64            `json:"accuracy_score"`
	Passed        bool               `json:"passed"`
	CheckedAt     time.Time          `json:"checked_at"`
}

// ValidationResult aggregates the outcome of validating one or more simulations.
type ValidationResult struct {
	ID           string         `json:"id"`
	SimulationID string         `json:"simulation_id"`
	Checks       []RealityCheck `json:"checks"`
	OverallScore float64        `json:"overall_score"`
	Status       string         `json:"status"` // "pass", "warn", "fail"
	CreatedAt    time.Time      `json:"created_at"`
}

// DivergencePoint flags a specific metric where simulation diverges from reality.
type DivergencePoint struct {
	ID            string    `json:"id"`
	SimulationID  string    `json:"simulation_id"`
	Metric        string    `json:"metric"`
	Predicted     float64   `json:"predicted"`
	Actual        float64   `json:"actual"`
	Divergence    float64   `json:"divergence"` // absolute difference
	Severity      string    `json:"severity"`   // "low", "medium", "high", "critical"
	FlaggedAt     time.Time `json:"flagged_at"`
}

// Store provides thread-safe JSON file persistence.
type Store struct {
	mu       sync.Mutex
	filePath string
	data     storeData
}

type storeData struct {
	Simulations map[string]Simulation      `json:"simulations"`
	Checks      map[string]RealityCheck    `json:"checks"`
	Results     map[string]ValidationResult `json:"results"`
	Divergences map[string]DivergencePoint  `json:"divergences"`
}

// NewStore creates a Store backed by filePath.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		data: storeData{
			Simulations: make(map[string]Simulation),
			Checks:      make(map[string]RealityCheck),
			Results:     make(map[string]ValidationResult),
			Divergences: make(map[string]DivergencePoint),
		},
	}
}

// Load reads persisted data from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(raw, &s.data)
}

// Save writes current data to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0644)
}

// RegisterSimulation stores a new simulation.
func (s *Store) RegisterSimulation(sim Simulation) Simulation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sim.ID == "" {
		sim.ID = fmt.Sprintf("sim-%d", time.Now().UTC().UnixNano())
	}
	if sim.CreatedAt.IsZero() {
		sim.CreatedAt = time.Now().UTC()
	}
	s.data.Simulations[sim.ID] = sim
	return sim
}

// RunRealityCheck compares a simulation's predictions against actual values.
func (s *Store) RunRealityCheck(simID string, actual map[string]float64) (RealityCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sim, ok := s.data.Simulations[simID]
	if !ok {
		return RealityCheck{}, fmt.Errorf("simulation %s not found", simID)
	}

	check := RealityCheck{
		ID:           fmt.Sprintf("check-%d", time.Now().UTC().UnixNano()),
		SimulationID: simID,
		Actual:       actual,
		CheckedAt:    time.Now().UTC(),
	}

	// Calculate accuracy: 1 - average relative error
	totalError := 0.0
	count := 0
	for key, predicted := range sim.Predicted {
		if actualVal, exists := actual[key]; exists {
			if predicted != 0 {
				relError := math.Abs(actualVal-predicted) / math.Abs(predicted)
				totalError += math.Min(relError, 1.0) // cap at 100%
			} else if actualVal != 0 {
				totalError += 1.0
			}
			count++
		}
	}

	if count > 0 {
		check.AccuracyScore = 1.0 - totalError/float64(count)
	} else {
		check.AccuracyScore = 0
	}
	check.Passed = check.AccuracyScore >= 0.8

	s.data.Checks[check.ID] = check
	return check, nil
}

// CompareSimulationReality returns divergence points between a simulation and actuals.
func (s *Store) CompareSimulationReality(simID string, actual map[string]float64) []DivergencePoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	sim, ok := s.data.Simulations[simID]
	if !ok {
		return nil
	}

	var divergences []DivergencePoint
	for key, predicted := range sim.Predicted {
		if actualVal, exists := actual[key]; exists {
			div := math.Abs(actualVal - predicted)
			if div > 0.01 { // threshold
				dp := DivergencePoint{
					ID:           fmt.Sprintf("div-%d-%s", time.Now().UTC().UnixNano(), key),
					SimulationID: simID,
					Metric:       key,
					Predicted:    predicted,
					Actual:       actualVal,
					Divergence:   div,
					Severity:     classifyDivergence(div, predicted),
					FlaggedAt:    time.Now().UTC(),
				}
				s.data.Divergences[dp.ID] = dp
				divergences = append(divergences, dp)
			}
		}
	}
	return divergences
}

// FlagDivergences returns all divergences above a severity threshold.
func (s *Store) FlagDivergences(minSeverity string) []DivergencePoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	threshold := severityRank(minSeverity)
	var result []DivergencePoint
	for _, dp := range s.data.Divergences {
		if severityRank(dp.Severity) >= threshold {
			result = append(result, dp)
		}
	}
	return result
}

// GenerateValidationReport produces a summary of validation state.
func (s *Store) GenerateValidationReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	passCount := 0
	failCount := 0
	for _, c := range s.data.Checks {
		if c.Passed {
			passCount++
		} else {
			failCount++
		}
	}

	severityCounts := map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0}
	for _, dp := range s.data.Divergences {
		severityCounts[dp.Severity]++
	}

	return map[string]interface{}{
		"simulation_count":   len(s.data.Simulations),
		"checks_passed":      passCount,
		"checks_failed":      failCount,
		"divergence_count":   len(s.data.Divergences),
		"severity_breakdown": severityCounts,
	}
}

// GetSimulation retrieves a simulation by ID.
func (s *Store) GetSimulation(id string) (Simulation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sim, ok := s.data.Simulations[id]
	return sim, ok
}

func classifyDivergence(div, predicted float64) string {
	if predicted == 0 {
		return "critical"
	}
	relative := div / math.Abs(predicted)
	switch {
	case relative > 0.5:
		return "critical"
	case relative > 0.2:
		return "high"
	case relative > 0.1:
		return "medium"
	default:
		return "low"
	}
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
