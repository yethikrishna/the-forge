// Package realitycheck provides external validation, organized dissent, and
// devil's advocate capabilities. It closes the gap in groupthink prevention
// by systematically challenging assumptions, scoring organizational reality
// alignment, and running structured adversarial reviews—ensuring decisions
// are grounded in evidence rather than echo chambers.
package realitycheck

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// AssumptionStatus represents the verification state of an assumption.
type AssumptionStatus string

const (
	AssumptionUnverified AssumptionStatus = "unverified"
	AssumptionChallenged AssumptionStatus = "challenged"
	AssumptionValidated  AssumptionStatus = "validated"
	AssumptionInvalidated AssumptionStatus = "invalidated"
)

// ChallengeSeverity represents how serious a challenge is.
type ChallengeSeverity string

const (
	SeverityLow      ChallengeSeverity = "low"
	SeverityMedium   ChallengeSeverity = "medium"
	SeverityHigh     ChallengeSeverity = "high"
	SeverityCritical ChallengeSeverity = "critical"
)

// Assumption represents a belief or premise the organization holds.
type Assumption struct {
	ID          string            `json:"id"`
	Statement   string            `json:"statement"`
	Category    string            `json:"category"`
	Status      AssumptionStatus  `json:"status"`
	Confidence  float64           `json:"confidence"` // 0.0-1.0
	Owner       string            `json:"owner"`
	Evidence    []string          `json:"evidence"`
	Challenges  []string          `json:"challenge_ids"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Challenge represents a structured challenge to an assumption.
type Challenge struct {
	ID           string           `json:"id"`
	AssumptionID string           `json:"assumption_id"`
	Challenger   string           `json:"challenger"`
	Argument     string           `json:"argument"`
	Severity     ChallengeSeverity `json:"severity"`
	CounterEvidence []string     `json:"counter_evidence"`
	Resolved     bool             `json:"resolved"`
	Resolution   string           `json:"resolution"`
	CreatedAt    time.Time        `json:"created_at"`
}

// ExternalValidation represents an external check on an assumption.
type ExternalValidation struct {
	ID           string    `json:"id"`
	AssumptionID string    `json:"assumption_id"`
	Source       string    `json:"source"`
	Verdict      string    `json:"verdict"` // supports, contradicts, inconclusive
	Evidence     string    `json:"evidence"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
}

// RealityScore represents the overall reality alignment of the organization.
type RealityScore struct {
	ID               string    `json:"id"`
	Score            float64   `json:"score"` // 0.0-1.0
	TotalAssumptions int       `json:"total_assumptions"`
	Validated        int       `json:"validated"`
	Challenged       int       `json:"challenged"`
	Invalidated      int       `json:"invalidated"`
	Unverified       int       `json:"unverified"`
	ComputedAt       time.Time `json:"computed_at"`
}

// Store persists reality check data.
type Store struct {
	mu          sync.Mutex
	filePath    string
	Assumptions map[string]Assumption         `json:"assumptions"`
	Challenges  map[string]Challenge          `json:"challenges"`
	Validations map[string]ExternalValidation `json:"validations"`
	Scores      map[string]RealityScore       `json:"scores"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath:    filePath,
		Assumptions: make(map[string]Assumption),
		Challenges:  make(map[string]Challenge),
		Validations: make(map[string]ExternalValidation),
		Scores:      make(map[string]RealityScore),
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

// RegisterAssumption adds a new assumption to track.
func (s *Store) RegisterAssumption(a Assumption) Assumption {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = AssumptionUnverified
	}
	s.Assumptions[a.ID] = a
	return a
}

// ChallengeConsensus creates a challenge against an assumption.
func (s *Store) ChallengeConsensus(c Challenge) (Challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Assumptions[c.AssumptionID]; !ok {
		return Challenge{}, os.ErrNotExist
	}
	c.CreatedAt = time.Now().UTC()
	s.Challenges[c.ID] = c
	// Update assumption status
	a := s.Assumptions[c.AssumptionID]
	a.Status = AssumptionChallenged
	a.UpdatedAt = time.Now().UTC()
	a.Challenges = append(a.Challenges, c.ID)
	s.Assumptions[c.AssumptionID] = a
	return c, nil
}

// ValidateExternally records an external validation.
func (s *Store) ValidateExternally(ev ExternalValidation) ExternalValidation {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev.CreatedAt = time.Now().UTC()
	s.Validations[ev.ID] = ev
	// Update assumption based on verdict
	if a, ok := s.Assumptions[ev.AssumptionID]; ok {
		if ev.Verdict == "supports" {
			a.Status = AssumptionValidated
		} else if ev.Verdict == "contradicts" {
			a.Status = AssumptionInvalidated
		}
		a.UpdatedAt = time.Now().UTC()
		s.Assumptions[ev.AssumptionID] = a
	}
	return ev
}

// RunDevilsAdvocate systematically challenges all unverified assumptions.
func (s *Store) RunDevilsAdvocate() []Challenge {
	s.mu.Lock()
	defer s.mu.Unlock()
	var challenges []Challenge
	i := 0
	for _, a := range s.Assumptions {
		if a.Status == AssumptionUnverified {
			c := Challenge{
				ID:           "da-" + a.ID,
				AssumptionID: a.ID,
				Challenger:   "devils_advocate",
				Argument:     "This assumption has not been validated: " + a.Statement,
				Severity:     SeverityHigh,
				Resolved:     false,
				CreatedAt:    time.Now().UTC(),
			}
			s.Challenges[c.ID] = c
			a.Status = AssumptionChallenged
			a.UpdatedAt = time.Now().UTC()
			a.Challenges = append(a.Challenges, c.ID)
			s.Assumptions[a.ID] = a
			challenges = append(challenges, c)
			i++
		}
	}
	return challenges
}

// GenerateRealityCheck computes the overall reality score.
func (s *Store) GenerateRealityCheck() RealityScore {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := len(s.Assumptions)
	validated := 0
	challenged := 0
	invalidated := 0
	unverified := 0
	for _, a := range s.Assumptions {
		switch a.Status {
		case AssumptionValidated:
			validated++
		case AssumptionChallenged:
			challenged++
		case AssumptionInvalidated:
			invalidated++
		case AssumptionUnverified:
			unverified++
		}
	}
	score := 0.0
	if total > 0 {
		score = float64(validated) / float64(total)
	}
	rs := RealityScore{
		ID:               "rs-" + time.Now().Format("20060102-150405"),
		Score:            score,
		TotalAssumptions: total,
		Validated:        validated,
		Challenged:       challenged,
		Invalidated:      invalidated,
		Unverified:       unverified,
		ComputedAt:       time.Now().UTC(),
	}
	s.Scores[rs.ID] = rs
	return rs
}
