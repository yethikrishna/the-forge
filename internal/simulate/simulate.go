// Package simulate tests agents on historical data.
// Replays bug reports, reviews, and cost patterns against agents
// to measure accuracy and catch regressions.
//
// History repeats. Be ready.
package simulate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ScenarioType is the type of simulation scenario.
type ScenarioType string

const (
	ScenarioBugReport ScenarioType = "bug_report"
	ScenarioReview    ScenarioType = "code_review"
	ScenarioCost      ScenarioType = "cost_pattern"
	ScenarioTask      ScenarioType = "task_completion"
)

// Scenario is a test scenario from historical data.
type Scenario struct {
	ID          string            `json:"id"`
	Type        ScenarioType      `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Input       string            `json:"input"`
	Expected    string            `json:"expected"`
	Difficulty  float64           `json:"difficulty"` // 0-1
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Result is the result of running a scenario.
type Result struct {
	ID         string    `json:"id"`
	ScenarioID string    `json:"scenario_id"`
	AgentID    string    `json:"agent_id"`
	Pass       bool      `json:"pass"`
	Score      float64   `json:"score"` // 0-1
	Output     string    `json:"output,omitempty"`
	Duration   string    `json:"duration"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Run is a collection of scenario results.
type Run struct {
	ID          string       `json:"id"`
	AgentID     string       `json:"agent_id"`
	Type        ScenarioType `json:"type"`
	ScenarioIDs []string     `json:"scenario_ids"`
	Results     []Result     `json:"results"`
	PassRate    float64      `json:"pass_rate"`
	AvgScore    float64      `json:"avg_score"`
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at"`
}

// Engine manages simulations.
type Engine struct {
	scenarios map[string]*Scenario
	results   []Result
	runs      map[string]*Run
	storeDir  string
	nextID    int
	runID     int
	mu        sync.RWMutex
}

// NewEngine creates a simulation engine.
func NewEngine(storeDir string) *Engine {
	e := &Engine{
		scenarios: make(map[string]*Scenario),
		results:   make([]Result, 0),
		runs:      make(map[string]*Run),
		storeDir:  storeDir,
	}
	e.registerDefaults()
	e.load()
	return e
}

func (e *Engine) registerDefaults() {
	defaults := []Scenario{
		{ID: "bug-1", Type: ScenarioBugReport, Title: "Null pointer in auth", Description: "Login fails with nil pointer", Input: "stack trace: panic at auth.go:42", Expected: "Add nil check for user object", Difficulty: 0.3, Tags: []string{"auth", "bug"}},
		{ID: "bug-2", Type: ScenarioBugReport, Title: "Race condition in cache", Description: "Concurrent writes corrupt cache", Input: "data corruption after concurrent requests", Expected: "Add mutex/sync around cache writes", Difficulty: 0.6, Tags: []string{"concurrency", "bug"}},
		{ID: "bug-3", Type: ScenarioBugReport, Title: "SQL injection in search", Description: "Search param not sanitized", Input: "SELECT * FROM users WHERE name = '' OR 1=1", Expected: "Use parameterized queries", Difficulty: 0.4, Tags: []string{"security", "sql"}},
		{ID: "review-1", Type: ScenarioReview, Title: "Review error handling", Description: "Check error handling in API handler", Input: "func handler(w http.ResponseWriter, r *http.Request) { result, _ := doWork(); w.Write(result) }", Expected: "Handle error from doWork(), return 500 on failure", Difficulty: 0.3, Tags: []string{"error-handling", "review"}},
		{ID: "review-2", Type: ScenarioReview, Title: "Review goroutine leak", Description: "Check for goroutine leak", Input: "go func() { ch <- val }()", Expected: "Ensure channel is consumed or goroutine has exit condition", Difficulty: 0.5, Tags: []string{"concurrency", "review"}},
		{ID: "cost-1", Type: ScenarioCost, Title: "Token usage spike", Description: "Agent used 10x normal tokens", Input: "Normal: 1K tokens, Current: 10K tokens", Expected: "Detect anomaly, suggest context window reduction", Difficulty: 0.4, Tags: []string{"cost", "anomaly"}},
		{ID: "task-1", Type: ScenarioTask, Title: "Implement CRUD API", Description: "Create a basic CRUD API", Input: "Design REST endpoints for user management", Expected: "Implement GET/POST/PUT/DELETE with proper status codes", Difficulty: 0.5, Tags: []string{"api", "rest"}},
		{ID: "task-2", Type: ScenarioTask, Title: "Write unit tests", Description: "Write tests for a module", Input: "Module has 3 functions: Add, Subtract, Multiply", Expected: "Table-driven tests covering edge cases", Difficulty: 0.3, Tags: []string{"testing"}},
	}

	for i := range defaults {
		e.scenarios[defaults[i].ID] = &defaults[i]
	}
}

// AddScenario adds a custom scenario.
func (e *Engine) AddScenario(s Scenario) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s.ID == "" {
		e.nextID++
		s.ID = fmt.Sprintf("custom-%d", e.nextID)
	}
	e.scenarios[s.ID] = &s
	e.save()
}

// GetScenario returns a scenario.
func (e *Engine) GetScenario(id string) (*Scenario, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s, ok := e.scenarios[id]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

// ListScenarios returns all scenarios, optionally filtered by type.
func (e *Engine) ListScenarios(scenarioType ScenarioType) []Scenario {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []Scenario
	for _, s := range e.scenarios {
		if scenarioType == "" || s.Type == scenarioType {
			result = append(result, *s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// SubmitResult submits a simulation result.
func (e *Engine) SubmitResult(scenarioID, agentID, output string, pass bool, score float64) (*Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.scenarios[scenarioID]; !ok {
		return nil, fmt.Errorf("scenario %q not found", scenarioID)
	}

	e.nextID++
	r := &Result{
		ID:         fmt.Sprintf("result-%d", e.nextID),
		ScenarioID: scenarioID,
		AgentID:    agentID,
		Pass:       pass,
		Score:      score,
		Output:     output,
		Timestamp:  time.Now(),
	}
	e.results = append(e.results, *r)
	e.save()
	return r, nil
}

// RunSimulation runs multiple scenarios and returns a Run.
func (e *Engine) RunSimulation(agentID string, scenarioIDs []string, fn func(Scenario) (string, bool, float64)) (*Run, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.runID++
	run := &Run{
		ID:          fmt.Sprintf("run-%d", e.runID),
		AgentID:     agentID,
		ScenarioIDs: scenarioIDs,
		StartedAt:   time.Now(),
	}

	for _, sid := range scenarioIDs {
		scenario, ok := e.scenarios[sid]
		if !ok {
			continue
		}

		start := time.Now()
		output, pass, score := fn(*scenario)
		duration := time.Since(start)

		e.nextID++
		result := Result{
			ID:         fmt.Sprintf("result-%d", e.nextID),
			ScenarioID: sid,
			AgentID:    agentID,
			Pass:       pass,
			Score:      score,
			Output:     output,
			Duration:   duration.String(),
			Timestamp:  time.Now(),
		}
		run.Results = append(run.Results, result)
		e.results = append(e.results, result)
	}

	run.FinishedAt = time.Now()

	// Calculate stats
	if len(run.Results) > 0 {
		passCount := 0
		var totalScore float64
		for _, r := range run.Results {
			if r.Pass {
				passCount++
			}
			totalScore += r.Score
		}
		run.PassRate = float64(passCount) / float64(len(run.Results))
		run.AvgScore = totalScore / float64(len(run.Results))
	}

	e.runs[run.ID] = run
	e.save()
	return run, nil
}

// GetRun returns a run by ID.
func (e *Engine) GetRun(id string) (*Run, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.runs[id]
	if !ok {
		return nil, false
	}
	copy := *r
	return &copy, true
}

// ListRuns returns all runs.
func (e *Engine) ListRuns() []Run {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]Run, 0, len(e.runs))
	for _, r := range e.runs {
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	return result
}

// AgentStats returns simulation stats for an agent.
func (e *Engine) AgentStats(agentID string) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var agentResults []Result
	for _, r := range e.results {
		if r.AgentID == agentID {
			agentResults = append(agentResults, r)
		}
	}

	if len(agentResults) == 0 {
		return map[string]interface{}{
			"agent":   agentID,
			"results": 0,
		}
	}

	passCount := 0
	var totalScore float64
	for _, r := range agentResults {
		if r.Pass {
			passCount++
		}
		totalScore += r.Score
	}

	return map[string]interface{}{
		"agent":     agentID,
		"results":   len(agentResults),
		"pass_rate": float64(passCount) / float64(len(agentResults)),
		"avg_score": totalScore / float64(len(agentResults)),
	}
}

func (e *Engine) save() {
	if e.storeDir == "" {
		return
	}
	os.MkdirAll(e.storeDir, 0755)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"scenarios": e.scenarios,
		"results":   e.results,
		"runs":      e.runs,
	}, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "simulation.json"), data, 0644)
}

func (e *Engine) load() {
	if e.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(e.storeDir, "simulation.json"))
	if err != nil {
		return
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if raw, ok := stored["scenarios"]; ok {
		json.Unmarshal(raw, &e.scenarios)
	}
	if raw, ok := stored["results"]; ok {
		json.Unmarshal(raw, &e.results)
	}
	if raw, ok := stored["runs"]; ok {
		json.Unmarshal(raw, &e.runs)
	}
	e.nextID = len(e.results)
	e.runID = len(e.runs)
}

// FormatRun formats a run for display.
func FormatRun(r *Run) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Run:        %s\n", r.ID))
	b.WriteString(fmt.Sprintf("Agent:      %s\n", r.AgentID))
	b.WriteString(fmt.Sprintf("Scenarios:  %d\n", len(r.Results)))
	b.WriteString(fmt.Sprintf("Pass Rate:  %.0f%%\n", r.PassRate*100))
	b.WriteString(fmt.Sprintf("Avg Score:  %.2f\n", r.AvgScore))
	b.WriteString(fmt.Sprintf("Duration:   %s\n", r.FinishedAt.Sub(r.StartedAt)))

	for _, res := range r.Results {
		status := "PASS"
		if !res.Pass {
			status = "FAIL"
		}
		b.WriteString(fmt.Sprintf("  [%s] %s (score: %.2f)\n", status, res.ScenarioID, res.Score))
	}
	return b.String()
}
