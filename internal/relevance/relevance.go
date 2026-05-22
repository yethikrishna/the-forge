// Package relevance ensures the human stays CEO, not mascot. Without
// measuring human relevance, automation gradually displaces the human from
// meaningful decisions — turning them into a rubber stamp. This package
// scores relevance, tracks decision participation, defines the human zone,
// and routes meaningful tasks back to human judgment.
package relevance

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// RelevanceScore measures how meaningfully involved a human is.
type RelevanceScore struct {
	ID                      string    `json:"id"`
	PersonID                string    `json:"person_id"`
	Score                   float64   `json:"score"`
	DecisionsMade           int       `json:"decisions_made"`
	DecisionsRubberStampped int       `json:"decisions_rubber_stamped"`
	ZoneCoverage            float64   `json:"zone_coverage"`
	MeasuredAt              time.Time `json:"measured_at"`
}

// DecisionRecord tracks a single decision and who made it.
type DecisionRecord struct {
	ID            string    `json:"id"`
	PersonID      string    `json:"person_id"`
	Decision      string    `json:"decision"`
	Category      string    `json:"category"`
	AutoProposed  bool      `json:"auto_proposed"`
	HumanChoice   bool      `json:"human_choice"`
	Weight        float64   `json:"weight"`
	DecidedAt     time.Time `json:"decided_at"`
}

// HumanZone defines areas where human judgment must be primary.
type HumanZone struct {
	ID          string `json:"id"`
	PersonID    string `json:"person_id"`
	ZoneName    string `json:"zone_name"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	AutoAllowed bool   `json:"auto_allowed"`
}

// MeaningfulTask is a task routed to a human because it requires judgment.
type MeaningfulTask struct {
	ID       string    `json:"id"`
	PersonID string    `json:"person_id"`
	ZoneName string    `json:"zone_name"`
	Task     string    `json:"task"`
	Reason   string    `json:"reason"`
	Priority string    `json:"priority"`
	Status   string    `json:"status"`
	RoutedAt time.Time `json:"routed_at"`
}

// Store provides thread-safe JSON file persistence.
type Store struct {
	mu       sync.Mutex
	filePath string
	data     storeData
}

type storeData struct {
	Scores    map[string]RelevanceScore `json:"scores"`
	Decisions map[string]DecisionRecord `json:"decisions"`
	Zones     map[string]HumanZone      `json:"zones"`
	Tasks     map[string]MeaningfulTask `json:"tasks"`
}

// NewStore creates a Store backed by filePath.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		data: storeData{
			Scores:    make(map[string]RelevanceScore),
			Decisions: make(map[string]DecisionRecord),
			Zones:     make(map[string]HumanZone),
			Tasks:     make(map[string]MeaningfulTask),
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

// ScoreRelevance computes a relevance score for a person.
func (s *Store) ScoreRelevance(personID string) RelevanceScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	decisionsMade := 0
	rubberStamped := 0
	weightedHuman := 0.0
	weightedTotal := 0.0

	for _, d := range s.data.Decisions {
		if d.PersonID == personID {
			decisionsMade++
			if !d.HumanChoice {
				rubberStamped++
			}
			weightedTotal += d.Weight
			if d.HumanChoice {
				weightedHuman += d.Weight
			}
		}
	}

	zonesForPerson := 0
	activeZones := 0
	for _, z := range s.data.Zones {
		if z.PersonID == personID {
			zonesForPerson++
			for _, d := range s.data.Decisions {
				if d.PersonID == personID && d.Category == z.ZoneName && d.HumanChoice {
					activeZones++
					break
				}
			}
		}
	}

	zoneCoverage := 0.0
	if zonesForPerson > 0 {
		zoneCoverage = float64(activeZones) / float64(zonesForPerson)
	}

	decisionScore := 0.0
	if weightedTotal > 0 {
		decisionScore = weightedHuman / weightedTotal
	}

	score := 0.6*decisionScore + 0.4*zoneCoverage

	rs := RelevanceScore{
		ID:                      fmt.Sprintf("rs-%d", time.Now().UTC().UnixNano()),
		PersonID:                personID,
		Score:                   score,
		DecisionsMade:           decisionsMade,
		DecisionsRubberStampped: rubberStamped,
		ZoneCoverage:            zoneCoverage,
		MeasuredAt:              time.Now().UTC(),
	}
	s.data.Scores[rs.ID] = rs
	return rs
}

// TrackDecisions records a decision.
func (s *Store) TrackDecisions(decision DecisionRecord) DecisionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if decision.ID == "" {
		decision.ID = fmt.Sprintf("dec-%d", time.Now().UTC().UnixNano())
	}
	if decision.DecidedAt.IsZero() {
		decision.DecidedAt = time.Now().UTC()
	}
	s.data.Decisions[decision.ID] = decision
	return decision
}

// DefineHumanZone creates a human zone.
func (s *Store) DefineHumanZone(zone HumanZone) HumanZone {
	s.mu.Lock()
	defer s.mu.Unlock()
	if zone.ID == "" {
		zone.ID = fmt.Sprintf("hz-%d", time.Now().UTC().UnixNano())
	}
	s.data.Zones[zone.ID] = zone
	return zone
}

// RouteMeaningfulTask routes a task to a human.
func (s *Store) RouteMeaningfulTask(task MeaningfulTask) MeaningfulTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task.ID == "" {
		task.ID = fmt.Sprintf("mt-%d", time.Now().UTC().UnixNano())
	}
	if task.RoutedAt.IsZero() {
		task.RoutedAt = time.Now().UTC()
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	s.data.Tasks[task.ID] = task
	return task
}

// GenerateRelevanceReport produces a summary of relevance state.
func (s *Store) GenerateRelevanceReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	avgScore := 0.0
	for _, sc := range s.data.Scores {
		avgScore += sc.Score
	}
	if len(s.data.Scores) > 0 {
		avgScore /= float64(len(s.data.Scores))
	}

	pendingTasks := 0
	for _, t := range s.data.Tasks {
		if t.Status == "pending" {
			pendingTasks++
		}
	}

	persons := make(map[string]bool)
	for _, sc := range s.data.Scores {
		persons[sc.PersonID] = true
	}

	return map[string]interface{}{
		"person_count":    len(persons),
		"average_score":   avgScore,
		"total_decisions": len(s.data.Decisions),
		"human_zones":     len(s.data.Zones),
		"pending_tasks":   pendingTasks,
		"total_tasks":     len(s.data.Tasks),
	}
}
