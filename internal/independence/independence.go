// Package independence measures and improves human autonomy within the
// forge system. Without tracking independence, humans can gradually become
// dependent on automated systems — losing agency, skill, and decision-making
// capability. This package maps dependencies, tracks autonomy metrics, and
// suggests concrete actions to preserve human freedom and self-reliance.
package independence

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// IndependenceScore quantifies how autonomous a human is in a domain.
type IndependenceScore struct {
	ID          string    `json:"id"`
	PersonID    string    `json:"person_id"`
	Domain      string    `json:"domain"` // "financial", "technical", "social", "creative"
	Score       float64   `json:"score"`  // 0.0–1.0
	AssistedPct float64   `json:"assisted_pct"` // % of decisions assisted by AI
	ManualPct   float64   `json:"manual_pct"`   // % of decisions made independently
	MeasuredAt  time.Time `json:"measured_at"`
}

// AutonomyMetric tracks a single autonomy measurement.
type AutonomyMetric struct {
	ID           string    `json:"id"`
	PersonID     string    `json:"person_id"`
	Metric       string    `json:"metric"` // "decision_independence", "skill_retention", "override_frequency"
	Value        float64   `json:"value"`
	Trend        string    `json:"trend"` // "improving", "stable", "declining"
	RecordedAt   time.Time `json:"recorded_at"`
}

// DependencyMap shows what a person depends on and how heavily.
type DependencyMap struct {
	ID           string                  `json:"id"`
	PersonID     string                  `json:"person_id"`
	Dependencies map[string]float64      `json:"dependencies"` // dep_name → weight 0.0–1.0
	TotalWeight  float64                 `json:"total_weight"`
	RiskAreas    []string                `json:"risk_areas"`
	MappedAt     time.Time               `json:"mapped_at"`
}

// FreedomAction is a suggested action to increase autonomy.
type FreedomAction struct {
	ID          string    `json:"id"`
	PersonID    string    `json:"person_id"`
	Domain      string    `json:"domain"`
	Action      string    `json:"action"`
	Priority    string    `json:"priority"` // "low", "medium", "high"
	Rationale   string    `json:"rationale"`
	Status      string    `json:"status"` // "suggested", "accepted", "completed", "skipped"
	SuggestedAt time.Time `json:"suggested_at"`
}

// Store provides thread-safe JSON file persistence.
type Store struct {
	mu       sync.Mutex
	filePath string
	data     storeData
}

type storeData struct {
	Scores    map[string]IndependenceScore `json:"scores"`
	Metrics   map[string]AutonomyMetric   `json:"metrics"`
	DepMaps   map[string]DependencyMap    `json:"dep_maps"`
	Actions   map[string]FreedomAction    `json:"actions"`
}

// NewStore creates a Store backed by filePath.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		data: storeData{
			Scores:  make(map[string]IndependenceScore),
			Metrics: make(map[string]AutonomyMetric),
			DepMaps: make(map[string]DependencyMap),
			Actions: make(map[string]FreedomAction),
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

// MeasureIndependence computes an independence score for a person in a domain.
func (s *Store) MeasureIndependence(personID, domain string, assistedPct, manualPct float64) IndependenceScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	score := manualPct / (assistedPct + manualPct + 0.001) // avoid div-by-zero
	if score > 1 {
		score = 1
	}

	is := IndependenceScore{
		ID:          fmt.Sprintf("is-%d", time.Now().UTC().UnixNano()),
		PersonID:    personID,
		Domain:      domain,
		Score:       score,
		AssistedPct: assistedPct,
		ManualPct:   manualPct,
		MeasuredAt:  time.Now().UTC(),
	}
	s.data.Scores[is.ID] = is
	return is
}

// TrackAutonomy records an autonomy metric for a person.
func (s *Store) TrackAutonomy(personID, metric string, value float64) AutonomyMetric {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine trend by comparing with last metric of same type
	trend := "stable"
	for _, m := range s.data.Metrics {
		if m.PersonID == personID && m.Metric == metric {
			if value > m.Value+0.05 {
				trend = "improving"
			} else if value < m.Value-0.05 {
				trend = "declining"
			}
			break
		}
	}

	am := AutonomyMetric{
		ID:         fmt.Sprintf("am-%d", time.Now().UTC().UnixNano()),
		PersonID:   personID,
		Metric:     metric,
		Value:      value,
		Trend:      trend,
		RecordedAt: time.Now().UTC(),
	}
	s.data.Metrics[am.ID] = am
	return am
}

// MapDependencies creates a dependency map for a person.
func (s *Store) MapDependencies(personID string, deps map[string]float64) DependencyMap {
	s.mu.Lock()
	defer s.mu.Unlock()

	total := 0.0
	var riskAreas []string
	for dep, weight := range deps {
		total += weight
		if weight > 0.7 {
			riskAreas = append(riskAreas, dep)
		}
	}

	dm := DependencyMap{
		ID:           fmt.Sprintf("dm-%d", time.Now().UTC().UnixNano()),
		PersonID:     personID,
		Dependencies: deps,
		TotalWeight:  total,
		RiskAreas:    riskAreas,
		MappedAt:     time.Now().UTC(),
	}
	s.data.DepMaps[dm.ID] = dm
	return dm
}

// SuggestFreedomActions generates actions to reduce dependency.
func (s *Store) SuggestFreedomActions(personID string) []FreedomAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the person's latest dependency map
	var latestMap DependencyMap
	for _, dm := range s.data.DepMaps {
		if dm.PersonID == personID {
			if dm.MappedAt.After(latestMap.MappedAt) {
				latestMap = dm
			}
		}
	}

	var actions []FreedomAction
	for dep, weight := range latestMap.Dependencies {
		if weight >= 0.5 {
			priority := "medium"
			if weight >= 0.8 {
				priority = "high"
			}
			fa := FreedomAction{
				ID:          fmt.Sprintf("fa-%d", time.Now().UTC().UnixNano()),
				PersonID:    personID,
				Domain:      dep,
				Action:      fmt.Sprintf("Reduce dependency on %s through skill building", dep),
				Priority:    priority,
				Rationale:   fmt.Sprintf("Current dependency weight: %.2f", weight),
				Status:      "suggested",
				SuggestedAt: time.Now().UTC(),
			}
			s.data.Actions[fa.ID] = fa
			actions = append(actions, fa)
		}
	}

	// If no dep map, suggest general autonomy actions
	if len(actions) == 0 {
		fa := FreedomAction{
			ID:          fmt.Sprintf("fa-%d", time.Now().UTC().UnixNano()),
			PersonID:    personID,
			Domain:      "general",
			Action:      "Conduct regular self-review of AI-assisted decisions",
			Priority:    "low",
			Rationale:   "Proactive monitoring prevents dependency creep",
			Status:      "suggested",
			SuggestedAt: time.Now().UTC(),
		}
		s.data.Actions[fa.ID] = fa
		actions = append(actions, fa)
	}

	return actions
}

// GenerateIndependenceReport produces a summary of independence state.
func (s *Store) GenerateIndependenceReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	avgScore := 0.0
	for _, sc := range s.data.Scores {
		avgScore += sc.Score
	}
	if len(s.data.Scores) > 0 {
		avgScore /= float64(len(s.data.Scores))
	}

	declining := 0
	improving := 0
	for _, m := range s.data.Metrics {
		if m.Trend == "declining" {
			declining++
		} else if m.Trend == "improving" {
			improving++
		}
	}

	return map[string]interface{}{
		"score_count":       len(s.data.Scores),
		"average_score":     avgScore,
		"metric_count":      len(s.data.Metrics),
		"declining_metrics": declining,
		"improving_metrics": improving,
		"dependency_maps":   len(s.data.DepMaps),
		"suggested_actions": len(s.data.Actions),
	}
}
