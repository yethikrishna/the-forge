// Package simulate provides historical data simulation for testing
// agents against past bug reports, reviews, and cost patterns.
// It enables replay of historical scenarios to evaluate agent performance.
package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ScenarioType represents the type of simulation scenario.
type ScenarioType string

const (
	ScenarioBugFix     ScenarioType = "bug_fix"
	ScenarioCodeReview ScenarioType = "code_review"
	ScenarioFeature    ScenarioType = "feature"
	ScenarioRefactor   ScenarioType = "refactor"
	ScenarioSecurity   ScenarioType = "security"
	ScenarioPerf       ScenarioType = "performance"
	ScenarioCost       ScenarioType = "cost_estimation"
)

// ScenarioStatus represents the status of a scenario.
type ScenarioStatus string

const (
	StatusDraft     ScenarioStatus = "draft"
	StatusReady     ScenarioStatus = "ready"
	StatusRunning   ScenarioStatus = "running"
	StatusComplete  ScenarioStatus = "complete"
	StatusFailed    ScenarioStatus = "failed"
)

// Metric represents a measurable outcome.
type Metric struct {
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit,omitempty"`
	Best     float64 `json:"best,omitempty"` // best known value
	Worst    float64 `json:"worst,omitempty"`
	Target   float64 `json:"target,omitempty"`
	Pass     bool    `json:"pass"`
}

// Scenario represents a testable scenario from historical data.
type Scenario struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        ScenarioType      `json:"type"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	Source      string            `json:"source,omitempty"` // "git", "jira", "github", "manual"
	SourceRef   string            `json:"source_ref,omitempty"` // issue ID, commit, etc.
	Tags        []string          `json:"tags,omitempty"`
	Difficulty  int               `json:"difficulty"` // 1-5
	Context     string            `json:"context"`    // the problem description / code context
	Input       ScenarioInput     `json:"input"`
	Expected    ScenarioExpected  `json:"expected"`
	Status      ScenarioStatus    `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ScenarioInput represents the input for a scenario.
type ScenarioInput struct {
	Prompt      string            `json:"prompt"`
	Files       map[string]string `json:"files,omitempty"` // filename -> content
	Language    string            `json:"language,omitempty"`
	AgentConfig string            `json:"agent_config,omitempty"`
	Model       string            `json:"model,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
}

// ScenarioExpected represents the expected outcome.
type ScenarioExpected struct {
	OutputContains   []string        `json:"output_contains,omitempty"`
	OutputNotContain []string        `json:"output_not_contains,omitempty"`
	FilesCreated     []string        `json:"files_created,omitempty"`
	FilesModified    []string        `json:"files_modified,omitempty"`
	TestsPass        bool            `json:"tests_pass,omitempty"`
	MaxCost          float64         `json:"max_cost,omitempty"`
	MaxDuration      time.Duration   `json:"max_duration,omitempty"`
	MinQualityScore  float64         `json:"min_quality_score,omitempty"`
	Metrics          []Metric        `json:"metrics,omitempty"`
	Description      string          `json:"description,omitempty"`
}

// Trial represents a single execution of a scenario.
type Trial struct {
	ID          string        `json:"id"`
	ScenarioID  string        `json:"scenario_id"`
	Agent       string        `json:"agent"`
	Model       string        `json:"model"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
	Cost        float64       `json:"cost"`
	Output      string        `json:"output,omitempty"`
	Metrics     []Metric      `json:"metrics,omitempty"`
	Score       float64       `json:"score"` // 0-100
	Pass        bool          `json:"pass"`
	Error       string        `json:"error,omitempty"`
}

// SimulationResult holds the aggregated results of running scenarios.
type SimulationResult struct {
	ScenarioID    string    `json:"scenario_id"`
	ScenarioName  string    `json:"scenario_name"`
	Trials        []Trial   `json:"trials"`
	BestScore     float64   `json:"best_score"`
	AverageScore  float64   `json:"average_score"`
	PassRate      float64   `json:"pass_rate"`
	AverageCost   float64   `json:"average_cost"`
	AverageDuration time.Duration `json:"average_duration"`
	Winner        string    `json:"winner,omitempty"` // best trial agent/model
	CompletedAt   time.Time `json:"completed_at"`
}

// Report represents a full simulation report.
type Report struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	CreatedAt     time.Time          `json:"created_at"`
	ScenarioCount int                `json:"scenario_count"`
	TrialCount    int                `json:"trial_count"`
	Results       []SimulationResult `json:"results"`
	Summary       ReportSummary      `json:"summary"`
}

// ReportSummary holds aggregate statistics.
type ReportSummary struct {
	OverallPassRate    float64          `json:"overall_pass_rate"`
	AverageScore       float64          `json:"average_score"`
	TotalCost          float64          `json:"total_cost"`
	TotalDuration      time.Duration    `json:"total_duration"`
	BestPerformer      string           `json:"best_performer,omitempty"`
	ByType             map[ScenarioType]TypeSummary `json:"by_type"`
}

// TypeSummary holds per-type statistics.
type TypeSummary struct {
	Count     int     `json:"count"`
	PassRate  float64 `json:"pass_rate"`
	AvgScore  float64 `json:"avg_score"`
	AvgCost   float64 `json:"avg_cost"`
}

// Store manages scenarios and trial data.
type Store struct {
	mu        sync.RWMutex
	dir       string
	scenarios map[string]*Scenario
	trials    map[string][]*Trial // scenarioID -> trials
}

// NewStore creates a new simulation store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create sim dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "trials"), 0755); err != nil {
		return nil, fmt.Errorf("create trials dir: %w", err)
	}

	s := &Store{
		dir:       dir,
		scenarios: make(map[string]*Scenario),
		trials:    make(map[string][]*Trial),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	// Load scenarios
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var sc Scenario
		if err := json.Unmarshal(data, &sc); err != nil {
			continue
		}
		s.scenarios[sc.ID] = &sc
	}

	// Load trials
	trialsDir := filepath.Join(s.dir, "trials")
	entries, err = os.ReadDir(trialsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(trialsDir, e.Name()))
		if err != nil {
			continue
		}
		var trial Trial
		if err := json.Unmarshal(data, &trial); err != nil {
			continue
		}
		s.trials[trial.ScenarioID] = append(s.trials[trial.ScenarioID], &trial)
	}
}

// CreateScenario creates a new scenario.
func (s *Store) CreateScenario(sc *Scenario) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sc.ID == "" {
		sc.ID = fmt.Sprintf("sc-%d", time.Now().UnixNano())
	}
	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = time.Now()
	}
	if sc.Status == "" {
		sc.Status = StatusDraft
	}
	s.scenarios[sc.ID] = sc
	return s.saveScenario(sc)
}

// GetScenario retrieves a scenario.
func (s *Store) GetScenario(id string) (*Scenario, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.scenarios[id]
	return sc, ok
}

// ListScenarios lists scenarios, optionally filtered by type.
func (s *Store) ListScenarios(scenarioType ScenarioType) []*Scenario {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Scenario
	for _, sc := range s.scenarios {
		if scenarioType != "" && sc.Type != scenarioType {
			continue
		}
		result = append(result, sc)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// RecordTrial records a trial result.
func (s *Store) RecordTrial(trial *Trial) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if trial.ID == "" {
		trial.ID = fmt.Sprintf("trial-%d", time.Now().UnixNano())
	}
	s.trials[trial.ScenarioID] = append(s.trials[trial.ScenarioID], trial)

	data, err := json.MarshalIndent(trial, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trial: %w", err)
	}
	return os.WriteFile(filepath.Join(s.dir, "trials", trial.ID+".json"), data, 0644)
}

// GetTrials returns trials for a scenario.
func (s *Store) GetTrials(scenarioID string) []*Trial {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trials[scenarioID]
}

// RunScenario simulates running a scenario with the given agent/model configuration.
func (s *Store) RunScenario(ctx context.Context, scenarioID, agent, model string) (*Trial, error) {
	sc, ok := s.GetScenario(scenarioID)
	if !ok {
		return nil, fmt.Errorf("scenario %s not found", scenarioID)
	}

	trial := &Trial{
		ID:         fmt.Sprintf("trial-%d", time.Now().UnixNano()),
		ScenarioID: scenarioID,
		Agent:      agent,
		Model:      model,
		StartedAt:  time.Now(),
	}

	// Simulate execution (in reality, this would invoke the agent)
	// For now, generate a synthetic result based on scenario difficulty
	baseScore := 100.0 - float64(sc.Difficulty)*10
	// Add some variance based on model quality
	modelBonus := modelScoreBonus(model)
	score := baseScore + modelBonus
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	trial.Score = score
	trial.Pass = score >= sc.Expected.MinQualityScore || (sc.Expected.MinQualityScore == 0 && score >= 50)
	trial.Cost = estimateCost(sc, model)
	trial.Duration = estimateDuration(sc, model)
	trial.Output = fmt.Sprintf("[Simulated] Completed %s with score %.1f", sc.Name, score)
	trial.Metrics = generateMetrics(sc, score)
	trial.CompletedAt = time.Now()

	// Check specific expectations
	if len(sc.Expected.OutputContains) > 0 {
		allFound := true
		for _, expected := range sc.Expected.OutputContains {
			if !strings.Contains(trial.Output, expected) {
				allFound = false
				break
			}
		}
		if !allFound {
			trial.Score -= 10
			trial.Pass = false
		}
	}

	if sc.Expected.MaxCost > 0 && trial.Cost > sc.Expected.MaxCost {
		trial.Score -= 15
		trial.Pass = false
	}

	if trial.Score < 0 {
		trial.Score = 0
	}

	if err := s.RecordTrial(trial); err != nil {
		return nil, fmt.Errorf("record trial: %w", err)
	}

	return trial, nil
}

// RunSimulation runs multiple scenarios and produces a report.
func (s *Store) RunSimulation(ctx context.Context, scenarioTypes []ScenarioType, agents []string, models []string) (*Report, error) {
	report := &Report{
		ID:        fmt.Sprintf("sim-%d", time.Now().UnixNano()),
		Name:      fmt.Sprintf("Simulation %s", time.Now().Format("2006-01-02")),
		CreatedAt: time.Now(),
	}

	// Collect scenarios
	var scenarios []*Scenario
	for _, sc := range s.ListScenarios("") {
		if len(scenarioTypes) > 0 {
			found := false
			for _, t := range scenarioTypes {
				if sc.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		scenarios = append(scenarios, sc)
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("no scenarios found")
	}

	report.ScenarioCount = len(scenarios)

	// Run each scenario with each agent/model combo
	for _, sc := range scenarios {
		result := SimulationResult{
			ScenarioID:   sc.ID,
			ScenarioName: sc.Name,
		}

		for _, agent := range agents {
			for _, model := range models {
				if ctx.Err() != nil {
					break
				}
				trial, err := s.RunScenario(ctx, sc.ID, agent, model)
				if err != nil {
					continue
				}
				result.Trials = append(result.Trials, *trial)
				report.TrialCount++
			}
		}

		// Calculate result stats
		calculateResultStats(&result)
		report.Results = append(report.Results, result)
	}

	// Calculate overall summary
	calculateReportSummary(report)

	return report, nil
}

// GenerateFromGit creates scenarios from git history.
func GenerateFromGit(projectPath string, limit int) ([]Scenario, error) {
	var scenarios []Scenario

	// Read git log for bug fixes
	// This would use git commands in a real implementation
	// For now, generate template scenarios
	templates := []Scenario{
		{
			Name: "Fix nil pointer dereference in handler",
			Type: ScenarioBugFix,
			Description: "A nil pointer dereference occurs when the handler receives an empty request body",
			Difficulty: 2,
			Context: "The getUserHandler function does not check if the request body is nil before accessing fields",
			Source: "git",
			Expected: ScenarioExpected{
				OutputContains:   []string{"nil", "check", "error"},
				MinQualityScore: 60,
			},
		},
		{
			Name: "Review PR with security implications",
			Type: ScenarioCodeReview,
			Description: "Review a pull request that introduces SQL query construction from user input",
			Difficulty: 3,
			Context: "PR adds a new search endpoint that constructs SQL queries from URL parameters",
			Source: "github",
			Expected: ScenarioExpected{
				OutputContains:   []string{"SQL injection", "parameterized", "sanitize"},
				MinQualityScore: 70,
			},
		},
		{
			Name: "Implement rate limiting middleware",
			Type: ScenarioFeature,
			Description: "Add rate limiting to the API server with configurable limits per endpoint",
			Difficulty: 3,
			Context: "Current API has no rate limiting and is vulnerable to abuse",
			Source: "jira",
			Expected: ScenarioExpected{
				OutputContains:   []string{"rate", "limit", "middleware"},
				FilesCreated:     []string{"middleware/ratelimit.go"},
				MinQualityScore: 65,
			},
		},
		{
			Name: "Refactor monolithic handler into service layer",
			Type: ScenarioRefactor,
			Description: "Extract business logic from HTTP handlers into a service layer",
			Difficulty: 4,
			Context: "All business logic is currently in HTTP handlers, making it hard to test",
			Source: "manual",
			Expected: ScenarioExpected{
				MinQualityScore: 55,
			},
		},
		{
			Name: "Fix authentication bypass vulnerability",
			Type: ScenarioSecurity,
			Description: "Authentication can be bypassed by manipulating the JWT token header",
			Difficulty: 4,
			Context: "The JWT validation does not verify the algorithm field",
			Source: "git",
			Expected: ScenarioExpected{
				OutputContains:   []string{"algorithm", "verify", "validate"},
				MinQualityScore: 80,
			},
		},
	}

	for i := range templates {
		t := templates[i]
		t.ID = fmt.Sprintf("sc-git-%d", i+1)
		t.CreatedAt = time.Now()
		t.Status = StatusReady
		scenarios = append(scenarios, t)
		if limit > 0 && len(scenarios) >= limit {
			break
		}
	}

	return scenarios, nil
}

// helper functions

func (s *Store) saveScenario(sc *Scenario) error {
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scenario: %w", err)
	}
	return os.WriteFile(filepath.Join(s.dir, sc.ID+".json"), data, 0644)
}

func modelScoreBonus(model string) float64 {
	// Simulated model quality differences
	bonuses := map[string]float64{
		"claude-sonnet-4":  15,
		"claude-opus-4":    20,
		"gpt-4.1":          12,
		"gpt-4.1-mini":     5,
		"deepseek-v3":      10,
		"llama-3":          3,
		"ollama/llama3":    0,
	}
	if bonus, ok := bonuses[model]; ok {
		return bonus
	}
	return 5 // default for unknown models
}

func estimateCost(sc *Scenario, model string) float64 {
	// Rough cost estimation based on difficulty and model
	baseCost := float64(sc.Difficulty) * 0.02
	modelMultiplier := 1.0
	expensive := map[string]float64{
		"claude-opus-4": 3.0,
		"claude-sonnet-4": 1.5,
		"gpt-4.1": 1.2,
		"gpt-4.1-mini": 0.3,
		"deepseek-v3": 0.2,
	}
	if m, ok := expensive[model]; ok {
		modelMultiplier = m
	}
	return baseCost * modelMultiplier
}

func estimateDuration(sc *Scenario, model string) time.Duration {
	baseSeconds := float64(sc.Difficulty) * 15
	return time.Duration(baseSeconds) * time.Second
}

func generateMetrics(sc *Scenario, score float64) []Metric {
	return []Metric{
		{Name: "quality", Value: score, Unit: "score", Best: 100, Worst: 0, Target: 70, Pass: score >= 70},
		{Name: "cost_efficiency", Value: math.Max(0, 100-score*0.5), Unit: "score", Pass: true},
		{Name: "completeness", Value: math.Min(100, score+5), Unit: "%", Pass: score >= 60},
	}
}

func calculateResultStats(result *SimulationResult) {
	if len(result.Trials) == 0 {
		return
	}

	var totalScore, totalCost float64
	var totalDuration time.Duration
	var passes int
	var bestTrial *Trial

	for i := range result.Trials {
		t := &result.Trials[i]
		totalScore += t.Score
		totalCost += t.Cost
		totalDuration += t.Duration
		if t.Pass {
			passes++
		}
		if bestTrial == nil || t.Score > bestTrial.Score {
			bestTrial = t
		}
	}

	n := float64(len(result.Trials))
	result.AverageScore = totalScore / n
	result.BestScore = bestTrial.Score
	result.PassRate = float64(passes) / n * 100
	result.AverageCost = totalCost / n
	result.AverageDuration = time.Duration(float64(totalDuration) / n)
	result.Winner = fmt.Sprintf("%s/%s", bestTrial.Agent, bestTrial.Model)
	result.CompletedAt = time.Now()
}

func calculateReportSummary(report *Report) {
	var totalPassRate, totalScore, totalCost float64
	var totalDuration time.Duration
	bestScore := 0.0
	bestPerformer := ""
	byType := make(map[ScenarioType]TypeSummary)

	for _, result := range report.Results {
		totalPassRate += result.PassRate
		totalScore += result.AverageScore
		totalCost += result.AverageCost
		totalDuration += result.AverageDuration

		if result.BestScore > bestScore {
			bestScore = result.BestScore
			bestPerformer = result.Winner
		}
	}

	n := float64(len(report.Results))
	if n > 0 {
		report.Summary.OverallPassRate = totalPassRate / n
		report.Summary.AverageScore = totalScore / n
		report.Summary.BestPerformer = bestPerformer
	}
	report.Summary.TotalCost = totalCost
	report.Summary.TotalDuration = totalDuration

	// Per-type stats
	typeData := make(map[ScenarioType][]float64)
	for _, result := range report.Results {
		if sc, ok := report.getScenarioByID(result.ScenarioID); ok {
			data := typeData[sc.Type]
			data = append(data, result.AverageScore, result.PassRate, result.AverageCost)
			typeData[sc.Type] = data
		}
	}

	for t, data := range typeData {
		n := float64(len(data) / 3)
		if n > 0 {
			var scoreSum, passSum, costSum float64
			for i := 0; i < len(data); i += 3 {
				scoreSum += data[i]
				passSum += data[i+1]
				costSum += data[i+2]
			}
			byType[t] = TypeSummary{
				Count:    int(n),
				AvgScore: scoreSum / n,
				PassRate: passSum / n,
				AvgCost:  costSum / n,
			}
		}
	}
	report.Summary.ByType = byType
}

func (r *Report) getScenarioByID(id string) (*Scenario, bool) {
	// This is a helper - in practice, we'd need the store reference
	_ = id
	return nil, false
}

// FormatReport formats a simulation report as markdown.
func FormatReport(report *Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Simulation Report: %s\n\n", report.Name)
	fmt.Fprintf(&b, "**Created:** %s\n", report.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "**Scenarios:** %d | **Trials:** %d\n\n", report.ScenarioCount, report.TrialCount)

	// Summary
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Overall Pass Rate | %.1f%% |\n", report.Summary.OverallPassRate)
	fmt.Fprintf(&b, "| Average Score | %.1f |\n", report.Summary.AverageScore)
	fmt.Fprintf(&b, "| Total Cost | $%.4f |\n", report.Summary.TotalCost)
	fmt.Fprintf(&b, "| Total Duration | %s |\n", report.Summary.TotalDuration)
	if report.Summary.BestPerformer != "" {
		fmt.Fprintf(&b, "| Best Performer | %s |\n", report.Summary.BestPerformer)
	}
	b.WriteString("\n")

	// Per-scenario results
	fmt.Fprintf(&b, "## Scenario Results\n\n")
	for _, result := range report.Results {
		fmt.Fprintf(&b, "### %s\n\n", result.ScenarioName)
		fmt.Fprintf(&b, "- Pass Rate: %.1f%%\n", result.PassRate)
		fmt.Fprintf(&b, "- Best Score: %.1f\n", result.BestScore)
		fmt.Fprintf(&b, "- Average Score: %.1f\n", result.AverageScore)
		fmt.Fprintf(&b, "- Average Cost: $%.4f\n", result.AverageCost)
		if result.Winner != "" {
			fmt.Fprintf(&b, "- Winner: %s\n", result.Winner)
		}

		if len(result.Trials) > 0 {
			fmt.Fprintf(&b, "\n| Agent/Model | Score | Cost | Duration | Pass |\n|-------------|-------|------|----------|------|\n")
			for _, t := range result.Trials {
				pass := "❌"
				if t.Pass {
					pass = "✅"
				}
				fmt.Fprintf(&b, "| %s/%s | %.1f | $%.4f | %s | %s |\n",
					t.Agent, t.Model, t.Score, t.Cost, t.Duration.Round(time.Second), pass)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
