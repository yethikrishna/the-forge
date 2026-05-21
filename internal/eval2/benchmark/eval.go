// Package eval provides agent evaluation and benchmarking capabilities.
// Test every blade before it leaves the forge.
package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Grade represents an evaluation grade.
type Grade string

const (
	GradeAPlus Grade = "A+"
	GradeA     Grade = "A"
	GradeB     Grade = "B"
	GradeC     Grade = "C"
	GradeD     Grade = "D"
	GradeF     Grade = "F"
)

// Benchmark is a named evaluation benchmark.
type Benchmark struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Prompt      string   `json:"prompt"`
	Expected    string   `json:"expected"`
	Timeout     string   `json:"timeout"`
	Difficulty  string   `json:"difficulty"` // easy, medium, hard
	Tags        []string `json:"tags"`
}

// EvalResult is the result of evaluating a single benchmark.
type EvalResult struct {
	BenchmarkID string    `json:"benchmark_id"`
	Agent       string    `json:"agent"`
	Model       string    `json:"model"`
	Output      string    `json:"output"`
	Expected    string    `json:"expected"`
	Score       float64   `json:"score"` // 0.0 - 1.0
	Grade       Grade     `json:"grade"`
	Duration    string    `json:"duration"`
	Cost        float64   `json:"cost"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// RunResult is the result of an evaluation run (multiple benchmarks).
type RunResult struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Agent     string       `json:"agent"`
	Model     string       `json:"model"`
	Results   []EvalResult `json:"results"`
	AvgScore  float64      `json:"avg_score"`
	Grade     Grade        `json:"grade"`
	TotalCost float64      `json:"total_cost"`
	Duration  string       `json:"duration"`
	Timestamp time.Time    `json:"timestamp"`
}

// Scorer evaluates an agent's output against expected output.
type Scorer interface {
	Score(output, expected string) float64
}

// ExactScorer requires exact match.
type ExactScorer struct{}

func (e *ExactScorer) Score(output, expected string) float64 {
	if output == expected {
		return 1.0
	}
	return 0.0
}

// ContainsScorer checks if the output contains the expected string.
type ContainsScorer struct{}

func (c *ContainsScorer) Score(output, expected string) float64 {
	if len(expected) == 0 {
		return 1.0
	}
	if contains(output, expected) {
		return 1.0
	}
	return 0.0
}

// KeywordScorer checks how many expected keywords appear in the output.
type KeywordScorer struct{}

func (k *KeywordScorer) Score(output, expected string) float64 {
	if len(expected) == 0 {
		return 1.0
	}
	keywords := splitKeywords(expected)
	if len(keywords) == 0 {
		return 1.0
	}

	found := 0
	lower := toLower(output)
	for _, kw := range keywords {
		if contains(lower, toLower(kw)) {
			found++
		}
	}

	return float64(found) / float64(len(keywords))
}

// Runner runs evaluations.
type Runner struct {
	scorer  Scorer
	results []RunResult
	store   string
	mu      sync.Mutex
}

// NewRunner creates a new evaluation runner.
func NewRunner(storePath string) *Runner {
	r := &Runner{
		scorer: &KeywordScorer{},
		store:  storePath,
	}
	r.load()
	return r
}

// WithScorer sets the scoring function.
func (r *Runner) WithScorer(s Scorer) *Runner {
	r.scorer = s
	return r
}

// RunBenchmark evaluates a single benchmark.
func (r *Runner) RunBenchmark(benchmark Benchmark, agent, model, output string, duration time.Duration, cost float64) EvalResult {
	score := r.scorer.Score(output, benchmark.Expected)

	result := EvalResult{
		BenchmarkID: benchmark.ID,
		Agent:       agent,
		Model:       model,
		Output:      output,
		Expected:    benchmark.Expected,
		Score:       score,
		Grade:       ScoreToGrade(score),
		Duration:    duration.Round(time.Millisecond).String(),
		Cost:        cost,
		Timestamp:   time.Now().UTC(),
	}

	return result
}

// RunAll evaluates all benchmarks against an agent.
func (r *Runner) RunAll(benchmarks []Benchmark, agent, model string, runFn func(Benchmark) (string, time.Duration, float64, error)) *RunResult {
	run := &RunResult{
		ID:        fmt.Sprintf("run-%d", time.Now().UnixNano()),
		Name:      fmt.Sprintf("eval-%s-%s", agent, time.Now().Format("20060102-150405")),
		Agent:     agent,
		Model:     model,
		Timestamp: time.Now().UTC(),
	}

	start := time.Now()

	for _, bm := range benchmarks {
		output, duration, cost, err := runFn(bm)

		var result EvalResult
		if err != nil {
			result = EvalResult{
				BenchmarkID: bm.ID,
				Agent:       agent,
				Model:       model,
				Score:       0,
				Grade:       GradeF,
				Error:       err.Error(),
				Duration:    duration.Round(time.Millisecond).String(),
				Cost:        cost,
				Timestamp:   time.Now().UTC(),
			}
		} else {
			result = r.RunBenchmark(bm, agent, model, output, duration, cost)
		}

		run.Results = append(run.Results, result)
		run.TotalCost += cost
	}

	// Calculate average score
	var totalScore float64
	for _, r := range run.Results {
		totalScore += r.Score
	}
	if len(run.Results) > 0 {
		run.AvgScore = totalScore / float64(len(run.Results))
	}
	run.Grade = ScoreToGrade(run.AvgScore)
	run.Duration = time.Since(start).Round(time.Millisecond).String()

	r.mu.Lock()
	r.results = append(r.results, *run)
	r.mu.Unlock()
	r.save()

	return run
}

// History returns past evaluation runs.
func (r *Runner) History() []RunResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RunResult, len(r.results))
	copy(out, r.results)
	return out
}

// Compare compares two evaluation runs.
func (r *Runner) Compare(run1, run2 RunResult) string {
	var b string
	b += fmt.Sprintf("Comparison: %s vs %s\n", run1.Name, run2.Name)
	b += fmt.Sprintf("  Run1: Score %.2f (%s) | Cost %s\n", run1.AvgScore, run1.Grade, formatCost(run1.TotalCost))
	b += fmt.Sprintf("  Run2: Score %.2f (%s) | Cost %s\n", run2.AvgScore, run2.Grade, formatCost(run2.TotalCost))

	diff := run2.AvgScore - run1.AvgScore
	if diff > 0 {
		b += fmt.Sprintf("  Result: Run2 is %.2f better\n", diff)
	} else if diff < 0 {
		b += fmt.Sprintf("  Result: Run1 is %.2f better\n", -diff)
	} else {
		b += "  Result: Tied\n"
	}

	return b
}

// BuiltInBenchmarks returns the built-in evaluation benchmarks.
func BuiltInBenchmarks() []Benchmark {
	return []Benchmark{
		{
			ID: "hello-world", Name: "Hello World",
			Description: "Generate a simple hello world program",
			Category:    "code-gen", Difficulty: "easy",
			Prompt:   "Write a hello world program in Go",
			Expected: "package main; fmt.Println; Hello",
			Tags:     []string{"go", "basic"},
		},
		{
			ID: "fix-bug", Name: "Bug Fix",
			Description: "Fix a simple off-by-one bug",
			Category:    "debugging", Difficulty: "easy",
			Prompt:   "Fix the bug: for i := 0; i <= len(arr); i++",
			Expected: "i < len; off-by-one; less than",
			Tags:     []string{"debugging", "go"},
		},
		{
			ID: "api-design", Name: "API Design",
			Description: "Design a REST API for a todo app",
			Category:    "design", Difficulty: "medium",
			Prompt:   "Design a REST API for a todo application with CRUD operations",
			Expected: "GET POST PUT DELETE; /todos; CRUD; status codes",
			Tags:     []string{"api", "rest", "design"},
		},
		{
			ID: "error-handling", Name: "Error Handling",
			Description: "Add proper error handling to Go code",
			Category:    "code-gen", Difficulty: "medium",
			Prompt:   "Add error handling to this function that reads a file",
			Expected: "os.Open; defer Close; if err != nil; return error",
			Tags:     []string{"go", "errors"},
		},
		{
			ID: "concurrency", Name: "Concurrency",
			Description: "Write a concurrent Go program",
			Category:    "code-gen", Difficulty: "hard",
			Prompt:   "Write a Go program that fetches multiple URLs concurrently using goroutines",
			Expected: "go func; sync.WaitGroup; chan; http.Get; concurrent",
			Tags:     []string{"go", "concurrency", "goroutines"},
		},
		{
			ID: "refactor", Name: "Code Refactoring",
			Description: "Refactor code to follow Go best practices",
			Category:    "refactoring", Difficulty: "medium",
			Prompt:   "Refactor this Go code to be more idiomatic",
			Expected: "idiomatic; error wrapping; context; small functions",
			Tags:     []string{"go", "refactoring"},
		},
	}
}

// scoreToGrade converts a 0-1 score to a letter grade.
func ScoreToGrade(score float64) Grade {
	switch {
	case score >= 0.95:
		return GradeAPlus
	case score >= 0.85:
		return GradeA
	case score >= 0.75:
		return GradeB
	case score >= 0.65:
		return GradeC
	case score >= 0.50:
		return GradeD
	default:
		return GradeF
	}
}

// contains checks if s contains substr (case-insensitive not applied here).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func splitKeywords(s string) []string {
	var keywords []string
	current := ""
	for _, c := range s {
		if c == ';' || c == ',' || c == '\n' {
			current = trimSpace(current)
			if current != "" {
				keywords = append(keywords, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	current = trimSpace(current)
	if current != "" {
		keywords = append(keywords, current)
	}
	return keywords
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func formatCost(c float64) string {
	if c < 0.01 {
		return fmt.Sprintf("$%.6f", c)
	}
	return fmt.Sprintf("$%.4f", c)
}

// load reads evaluation results from disk.
func (r *Runner) load() {
	if r.store == "" {
		return
	}
	data, err := os.ReadFile(r.store)
	if err != nil {
		return
	}
	var results []RunResult
	if err := json.Unmarshal(data, &results); err != nil {
		return
	}
	r.results = results
}

// save writes evaluation results to disk.
func (r *Runner) save() {
	if r.store == "" {
		return
	}
	data, err := json.MarshalIndent(r.results, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(r.store)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(r.store, data, 0o644)
}

// FormatRunResult formats a run result for display.
func FormatRunResult(r *RunResult) string {
	var b string
	b += fmt.Sprintf("Run: %s\n", r.Name)
	b += fmt.Sprintf("Agent: %s | Model: %s\n", r.Agent, r.Model)
	b += fmt.Sprintf("Average Score: %.2f (%s)\n", r.AvgScore, r.Grade)
	b += fmt.Sprintf("Total Cost: %s | Duration: %s\n\n", formatCost(r.TotalCost), r.Duration)

	// Sort by score descending
	sorted := make([]EvalResult, len(r.Results))
	copy(sorted, r.Results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	for _, er := range sorted {
		icon := "✓"
		if er.Score < 0.5 {
			icon = "✗"
		} else if er.Score < 0.75 {
			icon = "△"
		}
		b += fmt.Sprintf("  %s %s — %.2f (%s)\n", icon, er.BenchmarkID, er.Score, er.Grade)
		if er.Error != "" {
			b += fmt.Sprintf("     Error: %s\n", er.Error)
		}
	}

	return b
}
