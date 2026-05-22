// Package science provides hypothesis-driven research, experiment management,
// reproducibility checking, peer review simulation, and basic science tracking.
// It closes the gap in scientific intelligence — enabling the Forge to reason
// rigorously, test assumptions, and pursue curiosity-driven inquiry.
package science

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Hypothesis represents a testable scientific hypothesis.
type Hypothesis struct {
	ID          string    `json:"id"`
	Statement   string    `json:"statement"`
	Domain      string    `json:"domain"`
	Predictions []string  `json:"predictions"`
	Status      string    `json:"status"` // proposed, testing, confirmed, refuted, inconclusive
	Confidence  float64   `json:"confidence"` // 0-1
	CreatedAt   time.Time `json:"created_at"`
	TestedAt    time.Time `json:"tested_at"`
}

// Experiment represents a controlled experiment.
type Experiment struct {
	ID           string    `json:"id"`
	HypothesisID string    `json:"hypothesis_id"`
	Name         string    `json:"name"`
	Method       string    `json:"method"`
	Variables    map[string]string `json:"variables"` // variable -> control/experiment
	Results      string    `json:"results"`
	Conclusion   string    `json:"conclusion"`
	Status       string    `json:"status"` // planned, running, completed, failed
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}

// ReviewResult captures a peer review outcome.
type ReviewResult struct {
	ID            string    `json:"id"`
	TargetID      string    `json:"target_id"`
	Reviewer      string    `json:"reviewer"`
	Verdict       string    `json:"verdict"` // accept, revise, reject
	Comments      string    `json:"comments"`
	RigorScore    float64   `json:"rigor_score"`    // 0-1
	ClarityScore  float64   `json:"clarity_score"`  // 0-1
	NoveltyScore  float64   `json:"novelty_score"`  // 0-1
	ReviewedAt    time.Time `json:"reviewed_at"`
}

// ReproducibilityScore measures how reproducible a finding is.
type ReproducibilityScore struct {
	ID              string    `json:"id"`
	FindingID       string    `json:"finding_id"`
	AttemptCount    int       `json:"attempt_count"`
	SuccessCount    int       `json:"success_count"`
	Score           float64   `json:"score"` // 0-1
	Notes           string    `json:"notes"`
	AssessedAt      time.Time `json:"assessed_at"`
}

// ResearchProject tracks a long-running research effort.
type ResearchProject struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	Hypotheses  []string  `json:"hypotheses"`
	Status      string    `json:"status"` // exploratory, active, concluded, shelved
	Priority    string    `json:"priority"` // curiosity, applied, critical
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ScienceReport is a consolidated science report.
type ScienceReport struct {
	GeneratedAt        time.Time          `json:"generated_at"`
	Hypotheses         []Hypothesis       `json:"hypotheses"`
	Experiments        []Experiment       `json:"experiments"`
	ReviewResults      []ReviewResult     `json:"review_results"`
	ReproducibilityScores []ReproducibilityScore `json:"reproducibility_scores"`
	ResearchProjects   []ResearchProject  `json:"research_projects"`
}

// Store persists science data to a JSON file with thread safety.
type Store struct {
	mu                  sync.Mutex
	filePath            string
	Hypotheses          []Hypothesis           `json:"hypotheses"`
	Experiments         []Experiment           `json:"experiments"`
	ReviewResults       []ReviewResult         `json:"review_results"`
	ReproducibilityScores []ReproducibilityScore `json:"reproducibility_scores"`
	ResearchProjects    []ResearchProject      `json:"research_projects"`
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

// FormHypothesis creates a new hypothesis from a statement and predictions.
func FormHypothesis(statement, domain string, predictions []string) Hypothesis {
	return Hypothesis{
		ID:          genID("hy"),
		Statement:   statement,
		Domain:      domain,
		Predictions: predictions,
		Status:      "proposed",
		Confidence:  0.5,
		CreatedAt:   time.Now(),
	}
}

// RunExperiment creates and runs an experiment against a hypothesis.
func RunExperiment(hypothesisID, name, method string, variables map[string]string) Experiment {
	return Experiment{
		ID:           genID("ex"),
		HypothesisID: hypothesisID,
		Name:         name,
		Method:       method,
		Variables:    variables,
		Status:       "running",
		StartedAt:    time.Now(),
	}
}

// PeerReview performs a simulated peer review on a finding.
func PeerReview(targetID, reviewer, verdict, comments string, rigor, clarity, novelty float64) ReviewResult {
	return ReviewResult{
		ID:           genID("rv"),
		TargetID:     targetID,
		Reviewer:     reviewer,
		Verdict:      verdict,
		Comments:     comments,
		RigorScore:   rigor,
		ClarityScore: clarity,
		NoveltyScore: novelty,
		ReviewedAt:   time.Now(),
	}
}

// CheckReproducibility computes a reproducibility score from attempt/success counts.
func CheckReproducibility(findingID string, attempts, successes int, notes string) ReproducibilityScore {
	score := 0.0
	if attempts > 0 {
		score = float64(successes) / float64(attempts)
	}
	return ReproducibilityScore{
		ID:           genID("rp"),
		FindingID:    findingID,
		AttemptCount: attempts,
		SuccessCount: successes,
		Score:        score,
		Notes:        notes,
		AssessedAt:   time.Now(),
	}
}

// GenerateScienceReport produces a consolidated science report.
func GenerateScienceReport(s *Store) ScienceReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return ScienceReport{
		GeneratedAt:          time.Now(),
		Hypotheses:           s.Hypotheses,
		Experiments:          s.Experiments,
		ReviewResults:        s.ReviewResults,
		ReproducibilityScores: s.ReproducibilityScores,
		ResearchProjects:     s.ResearchProjects,
	}
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
