// Package qualitycorpus provides an agent quality evaluation corpus
// with curated test cases, scoring rubrics, and comparative benchmarking.
// It enables systematic evaluation of agent capabilities against known
// challenges.
//
// "You can't improve what you don't measure."
package qualitycorpus

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Difficulty represents challenge difficulty.
type Difficulty string

const (
	DifficultyTrivial    Difficulty = "trivial"
	DifficultyEasy       Difficulty = "easy"
	DifficultyMedium     Difficulty = "medium"
	DifficultyHard       Difficulty = "hard"
	DifficultyExpert     Difficulty = "expert"
	DifficultyImpossible Difficulty = "impossible"
)

// Category represents the challenge domain.
type Category string

const (
	CatCodeGeneration Category = "code-generation"
	CatCodeReview     Category = "code-review"
	CatDebugging      Category = "debugging"
	CatRefactoring    Category = "refactoring"
	CatTesting        Category = "testing"
	CatDocumentation  Category = "documentation"
	CatSecurity       Category = "security"
	CatPerformance    Category = "performance"
	CatArchitecture   Category = "architecture"
	CatDataAnalysis   Category = "data-analysis"
)

// Challenge represents a test case for agent evaluation.
type Challenge struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Category    Category          `json:"category"`
	Difficulty  Difficulty        `json:"difficulty"`
	Tags        []string          `json:"tags,omitempty"`
	Input       string            `json:"input"`
	Expected    string            `json:"expected"`
	Hints       []string          `json:"hints,omitempty"`
	TimeLimit   time.Duration     `json:"time_limit,omitempty"`
	CostLimit   float64           `json:"cost_limit,omitempty"`
	Scoring     *ScoringRubric    `json:"scoring,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ScoringRubric defines how a challenge is scored.
type ScoringRubric struct {
	MaxScore        float64 `json:"max_score"`
	Correctness     float64 `json:"correctness_weight"`   // 0-1
	Efficiency      float64 `json:"efficiency_weight"`    // 0-1
	Style           float64 `json:"style_weight"`         // 0-1
	Security        float64 `json:"security_weight"`      // 0-1
	Completeness    float64 `json:"completeness_weight"`  // 0-1
	BonusThreshold  float64 `json:"bonus_threshold"`      // Score for bonus
	BonusPoints     float64 `json:"bonus_points"`
	PenaltyPerHint  float64 `json:"penalty_per_hint"`
	PenaltyPerRetry float64 `json:"penalty_per_retry"`
}

// Submission represents an agent's attempt at a challenge.
type Submission struct {
	ID          string        `json:"id"`
	ChallengeID string        `json:"challenge_id"`
	AgentID     string        `json:"agent_id"`
	AgentModel  string        `json:"agent_model"`
	Output      string        `json:"output"`
	Score       float64       `json:"score"`
	MaxScore    float64       `json:"max_score"`
	Duration    time.Duration `json:"duration"`
	CostUSD     float64       `json:"cost_usd"`
	HintsUsed   int           `json:"hints_used"`
	Retries     int           `json:"retries"`
	Passed      bool          `json:"passed"`
	GradedAt    time.Time     `json:"graded_at"`
	Grades      []*Grade      `json:"grades,omitempty"`
}

// Grade represents a score on a specific dimension.
type Grade struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
	MaxScore  float64 `json:"max_score"`
	Feedback  string  `json:"feedback,omitempty"`
}

// LeaderboardEntry represents a ranked entry.
type LeaderboardEntry struct {
	Rank         int     `json:"rank"`
	AgentID      string  `json:"agent_id"`
	AgentModel   string  `json:"agent_model"`
	TotalScore   float64 `json:"total_score"`
	Challenges   int     `json:"challenges"`
	PassRate     float64 `json:"pass_rate"`
	AvgScore     float64 `json:"avg_score"`
	AvgDuration  string  `json:"avg_duration"`
	AvgCost      float64 `json:"avg_cost"`
}

// Corpus manages a collection of challenges and submissions.
type Corpus struct {
	mu          sync.RWMutex
	dir         string
	challenges  map[string]*Challenge
	submissions map[string][]*Submission
}

// NewCorpus creates a new quality corpus.
func NewCorpus(dir string) *Corpus {
	return &Corpus{
		dir:         dir,
		challenges:  make(map[string]*Challenge),
		submissions: make(map[string][]*Submission),
	}
}

// Load loads the corpus from disk.
func (c *Corpus) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	os.MkdirAll(c.dir, 0o755)

	// Load challenges
	challengesDir := filepath.Join(c.dir, "challenges")
	entries, err := os.ReadDir(challengesDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(challengesDir, e.Name()))
			if err != nil {
				continue
			}
			var ch Challenge
			if err := json.Unmarshal(data, &ch); err != nil {
				continue
			}
			c.challenges[ch.ID] = &ch
		}
	}

	// Load submissions
	submissionsDir := filepath.Join(c.dir, "submissions")
	entries, err = os.ReadDir(submissionsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(submissionsDir, e.Name()))
			if err != nil {
				continue
			}
			var sub Submission
			if err := json.Unmarshal(data, &sub); err != nil {
				continue
			}
			c.submissions[sub.ChallengeID] = append(c.submissions[sub.ChallengeID], &sub)
		}
	}

	return nil
}

// AddChallenge adds a challenge to the corpus.
func (c *Corpus) AddChallenge(ch *Challenge) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ch.ID == "" {
		ch.ID = generateChallengeID(ch)
	}

	c.challenges[ch.ID] = ch

	// Persist
	dir := filepath.Join(c.dir, "challenges")
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, ch.ID+".json")
	data, err := json.MarshalIndent(ch, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// GetChallenge retrieves a challenge by ID.
func (c *Corpus) GetChallenge(id string) (*Challenge, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ch, ok := c.challenges[id]
	return ch, ok
}

// ListChallenges returns all challenges, optionally filtered.
func (c *Corpus) ListChallenges(filter func(*Challenge) bool) []*Challenge {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Challenge
	for _, ch := range c.challenges {
		if filter == nil || filter(ch) {
			result = append(result, ch)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// Submit adds a submission and grades it.
func (c *Corpus) Submit(_ context.Context, sub *Submission) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if sub.ID == "" {
		sub.ID = fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}

	// Grade the submission
	ch, ok := c.challenges[sub.ChallengeID]
	if !ok {
		return fmt.Errorf("challenge %s not found", sub.ChallengeID)
	}

	c.gradeSubmission(sub, ch)

	c.submissions[sub.ChallengeID] = append(c.submissions[sub.ChallengeID], sub)

	// Persist
	dir := filepath.Join(c.dir, "submissions")
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, sub.ID+".json")
	data, err := json.MarshalIndent(sub, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Leaderboard generates a leaderboard from submissions.
func (c *Corpus) Leaderboard() []*LeaderboardEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Aggregate by agent
	agentStats := make(map[string]*struct {
		totalScore float64
		challenges int
		passed     int
		duration   time.Duration
		cost       float64
		model      string
	})

	for _, subs := range c.submissions {
		for _, sub := range subs {
			key := sub.AgentID
			if _, ok := agentStats[key]; !ok {
				agentStats[key] = &struct {
					totalScore float64
					challenges int
					passed     int
					duration   time.Duration
					cost       float64
					model      string
				}{model: sub.AgentModel}
			}
			stats := agentStats[key]
			stats.totalScore += sub.Score
			stats.challenges++
			if sub.Passed {
				stats.passed++
			}
			stats.duration += sub.Duration
			stats.cost += sub.CostUSD
		}
	}

	var entries []*LeaderboardEntry
	for agentID, stats := range agentStats {
		entries = append(entries, &LeaderboardEntry{
			AgentID:    agentID,
			AgentModel: stats.model,
			TotalScore: stats.totalScore,
			Challenges: stats.challenges,
			PassRate:   float64(stats.passed) / float64(stats.challenges),
			AvgScore:   stats.totalScore / float64(stats.challenges),
			AvgDuration: func() string {
				if stats.challenges > 0 {
					return (stats.duration / time.Duration(stats.challenges)).Round(time.Millisecond).String()
				}
				return "0s"
			}(),
			AvgCost: stats.cost / float64(stats.challenges),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TotalScore > entries[j].TotalScore
	})

	for i, e := range entries {
		e.Rank = i + 1
	}

	return entries
}

// Stats returns corpus statistics.
func (c *Corpus) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalSubs := 0
	passedSubs := 0
	for _, subs := range c.submissions {
		totalSubs += len(subs)
		for _, s := range subs {
			if s.Passed {
				passedSubs++
			}
		}
	}

	passRate := 0.0
	if totalSubs > 0 {
		passRate = float64(passedSubs) / float64(totalSubs) * 100
	}

	return map[string]interface{}{
		"challenges":     len(c.challenges),
		"submissions":    totalSubs,
		"pass_rate":      fmt.Sprintf("%.1f%%", passRate),
		"agents":         len(c.submissions),
	}
}

// gradeSubmission evaluates a submission against a challenge's rubric.
func (c *Corpus) gradeSubmission(sub *Submission, ch *Challenge) {
	if ch.Scoring == nil {
		// Simple pass/fail based on output matching
		sub.Passed = strings.Contains(strings.ToLower(sub.Output), strings.ToLower(ch.Expected))
		if sub.Passed {
			sub.Score = 100
			sub.MaxScore = 100
		} else {
			sub.Score = 0
			sub.MaxScore = 100
		}
		return
	}

	rubric := ch.Scoring
	sub.MaxScore = rubric.MaxScore

	// Grade each dimension
	var grades []*Grade
	totalWeight := rubric.Correctness + rubric.Efficiency + rubric.Style + rubric.Security + rubric.Completeness

	if totalWeight == 0 {
		totalWeight = 1
	}

	// Correctness: does output match expected?
	correctnessScore := 0.0
	if strings.Contains(strings.ToLower(sub.Output), strings.ToLower(ch.Expected)) {
		correctnessScore = rubric.MaxScore * (rubric.Correctness / totalWeight)
	}
	grades = append(grades, &Grade{
		Dimension: "correctness",
		Score:     correctnessScore,
		MaxScore:  rubric.MaxScore * (rubric.Correctness / totalWeight),
	})

	// Efficiency: shorter duration = higher score
	efficiencyScore := 0.0
	if ch.TimeLimit > 0 {
		ratio := float64(sub.Duration) / float64(ch.TimeLimit)
		if ratio < 1 {
			efficiencyScore = rubric.MaxScore * (rubric.Efficiency / totalWeight) * (1 - ratio)
		}
	}
	grades = append(grades, &Grade{
		Dimension: "efficiency",
		Score:     efficiencyScore,
		MaxScore:  rubric.MaxScore * (rubric.Efficiency / totalWeight),
	})

	// Style, Security, Completeness: heuristic scoring
	styleScore := rubric.MaxScore * (rubric.Style / totalWeight) * 0.8
	grades = append(grades, &Grade{Dimension: "style", Score: styleScore, MaxScore: rubric.MaxScore * (rubric.Style / totalWeight)})

	securityScore := rubric.MaxScore * (rubric.Security / totalWeight) * 0.9
	grades = append(grades, &Grade{Dimension: "security", Score: securityScore, MaxScore: rubric.MaxScore * (rubric.Security / totalWeight)})

	completenessScore := rubric.MaxScore * (rubric.Completeness / totalWeight) * 0.85
	grades = append(grades, &Grade{Dimension: "completeness", Score: completenessScore, MaxScore: rubric.MaxScore * (rubric.Completeness / totalWeight)})

	// Sum scores
	totalScore := correctnessScore + efficiencyScore + styleScore + securityScore + completenessScore

	// Apply penalties
	totalScore -= float64(sub.HintsUsed) * rubric.PenaltyPerHint
	totalScore -= float64(sub.Retries) * rubric.PenaltyPerRetry

	// Apply bonus
	if totalScore >= rubric.BonusThreshold {
		totalScore += rubric.BonusPoints
	}

	if totalScore < 0 {
		totalScore = 0
	}
	if totalScore > rubric.MaxScore+rubric.BonusPoints {
		totalScore = rubric.MaxScore + rubric.BonusPoints
	}

	sub.Score = totalScore
	sub.Grades = grades
	sub.Passed = totalScore >= rubric.MaxScore*0.6 // 60% threshold
	sub.GradedAt = time.Now()
}

func generateChallengeID(ch *Challenge) string {
	h := sha256.Sum256([]byte(ch.Title + ch.Description))
	return fmt.Sprintf("ch-%s-%x", ch.Category, h[:4])
}
