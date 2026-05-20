// Package quality provides multi-dimensional scoring for agent output.
// Evaluates correctness, completeness, style, security, and cost efficiency.
//
// Good enough isn't. Measure everything.
package quality

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Dimension represents a scoring dimension.
type Dimension string

const (
	DimensionCorrectness Dimension = "correctness"
	DimensionCompleteness Dimension = "completeness"
	DimensionStyle       Dimension = "style"
	DimensionSecurity    Dimension = "security"
	DimensionEfficiency  Dimension = "efficiency"
	DimensionReadability Dimension = "readability"
	DimensionTestability Dimension = "testability"
)

// AllDimensions returns all scoring dimensions.
func AllDimensions() []Dimension {
	return []Dimension{
		DimensionCorrectness,
		DimensionCompleteness,
		DimensionStyle,
		DimensionSecurity,
		DimensionEfficiency,
		DimensionReadability,
		DimensionTestability,
	}
}

// Score represents a score on a single dimension (0.0-1.0).
type Score struct {
	Dimension Dimension `json:"dimension"`
	Value     float64   `json:"value"`
	Reason    string    `json:"reason,omitempty"`
	Weight    float64   `json:"weight"`
}

// Report is a complete quality assessment of agent output.
type Report struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	SessionID   string    `json:"session_id"`
	Model       string    `json:"model"`
	Scores      []Score   `json:"scores"`
	Composite   float64   `json:"composite"`
	Cost        float64   `json:"cost"`
	TokensUsed  int       `json:"tokens_used"`
	Duration    float64   `json:"duration_seconds"`
	PromptHash  string    `json:"prompt_hash,omitempty"`
	OutputHash  string    `json:"output_hash,omitempty"`
	Passed      bool      `json:"passed"`
	Threshold   float64   `json:"threshold"`
	CreatedAt   time.Time `json:"created_at"`
}

// Scorer evaluates agent output across multiple dimensions.
type Scorer struct {
	weights   map[Dimension]float64
	threshold float64
	store     *Store
}

// ScorerConfig configures the quality scorer.
type ScorerConfig struct {
	Weights          map[Dimension]float64 // dimension weights (sum should be 1.0)
	Threshold        float64               // minimum composite score to pass (0.0-1.0)
	StoreDir         string                // directory for persisting reports
	StrictSecurity   bool                  // fail entire report if security score < 0.5
	MinCompleteness  float64               // fail if completeness below this
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() ScorerConfig {
	return ScorerConfig{
		Weights: map[Dimension]float64{
			DimensionCorrectness: 0.25,
			DimensionCompleteness: 0.20,
			DimensionStyle:       0.10,
			DimensionSecurity:    0.20,
			DimensionEfficiency:  0.10,
			DimensionReadability: 0.08,
			DimensionTestability: 0.07,
		},
		Threshold:       0.6,
		StrictSecurity:  true,
		MinCompleteness: 0.4,
	}
}

// NewScorer creates a quality scorer with the given config.
func NewScorer(cfg ScorerConfig) (*Scorer, error) {
	// Normalize weights
	total := 0.0
	for _, w := range cfg.Weights {
		total += w
	}
	if total > 0 && math.Abs(total-1.0) > 0.01 {
		for d, w := range cfg.Weights {
			cfg.Weights[d] = w / total
		}
	}

	if cfg.StoreDir == "" {
		home, _ := os.UserHomeDir()
		cfg.StoreDir = filepath.Join(home, ".forge", "quality")
	}

	store, err := NewStore(cfg.StoreDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create quality store: %w", err)
	}

	return &Scorer{
		weights:   cfg.Weights,
		threshold: cfg.Threshold,
		store:     store,
	}, nil
}

// EvaluateInput is the input to a quality evaluation.
type EvaluateInput struct {
	AgentID    string
	SessionID  string
	Model      string
	Prompt     string
	Output     string
	FilesChanged []FileChange
	Cost       float64
	TokensUsed int
	Duration   float64
	Scores     map[Dimension]Score // pre-computed dimension scores (optional)
}

// FileChange represents a file modified by the agent.
type FileChange struct {
	Path        string `json:"path"`
	Operation   string `json:"operation"` // create, modify, delete
	LinesAdded  int    `json:"lines_added"`
	LinesRemoved int   `json:"lines_removed"`
}

// Evaluate computes quality scores for agent output.
func (s *Scorer) Evaluate(input EvaluateInput) (*Report, error) {
	scores := make([]Score, 0, len(s.weights))

	if len(input.Scores) > 0 {
		// Use provided scores
		for _, sc := range input.Scores {
			weight, ok := s.weights[sc.Dimension]
			if !ok {
				weight = 0.1
			}
			sc.Weight = weight
			sc.Value = clamp01(sc.Value)
			scores = append(scores, sc)
		}
	} else {
		// Compute heuristic scores from output analysis
		scores = s.computeHeuristicScores(input)
	}

	// Compute composite score
	composite := 0.0
	for _, sc := range scores {
		composite += sc.Value * sc.Weight
	}

	report := &Report{
		ID:         generateID("qr"),
		AgentID:    input.AgentID,
		SessionID:  input.SessionID,
		Model:      input.Model,
		Scores:     scores,
		Composite:  composite,
		Cost:       input.Cost,
		TokensUsed: input.TokensUsed,
		Duration:   input.Duration,
		Threshold:  s.threshold,
		Passed:     composite >= s.threshold,
		CreatedAt:  time.Now(),
	}

	// Check strict rules
	for _, sc := range scores {
		if sc.Dimension == DimensionSecurity && sc.Value < 0.5 {
			report.Passed = false
			report.Composite = math.Min(report.Composite, sc.Value)
		}
		if sc.Dimension == DimensionCompleteness && sc.Value < 0.4 {
			report.Passed = false
		}
	}

	// Persist
	if err := s.store.Save(report); err != nil {
		return report, fmt.Errorf("failed to save report: %w", err)
	}

	return report, nil
}

// computeHeuristicScores computes scores from static analysis of the output.
func (s *Scorer) computeHeuristicScores(input EvaluateInput) []Score {
	scores := make([]Score, 0, len(s.weights))

	output := input.Output
	lines := strings.Split(output, "\n")
	nonEmptyLines := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmptyLines++
		}
	}

	for dim, weight := range s.weights {
		score := Score{
			Dimension: dim,
			Weight:    weight,
		}

		switch dim {
		case DimensionCorrectness:
			// Heuristic: check for common error indicators
			score.Value = s.scoreCorrectness(output, input.FilesChanged)
		case DimensionCompleteness:
			score.Value = s.scoreCompleteness(output, input.Prompt, nonEmptyLines)
		case DimensionStyle:
			score.Value = s.scoreStyle(output)
		case DimensionSecurity:
			score.Value = s.scoreSecurity(output)
		case DimensionEfficiency:
			score.Value = s.scoreEfficiency(input.Cost, input.TokensUsed, input.Duration, nonEmptyLines)
		case DimensionReadability:
			score.Value = s.scoreReadability(output, nonEmptyLines)
		case DimensionTestability:
			score.Value = s.scoreTestability(output, input.FilesChanged)
		}

		score.Value = clamp01(score.Value)
		scores = append(scores, score)
	}

	return scores
}

func (s *Scorer) scoreCorrectness(output string, files []FileChange) float64 {
	score := 0.8 // base score

	// Penalize error patterns
	errorPatterns := []string{"error:", "panic:", "fatal:", "undefined", "nil pointer", "TODO:", "FIXME:", "HACK:"}
	for _, p := range errorPatterns {
		if strings.Contains(strings.ToLower(output), p) {
			score -= 0.05
		}
	}

	// Bonus for successful patterns
	successPatterns := []string{"success", "passed", "ok", "done", "complete"}
	successCount := 0
	lower := strings.ToLower(output)
	for _, p := range successPatterns {
		if strings.Contains(lower, p) {
			successCount++
		}
	}
	score += float64(successCount) * 0.02

	// Bonus for files changed (means the agent actually did something)
	if len(files) > 0 {
		score += 0.05
	}
	if len(files) > 3 {
		score += 0.03
	}

	return score
}

func (s *Scorer) scoreCompleteness(output, prompt string, lines int) float64 {
	score := 0.5

	// More output = more complete (up to a point)
	if lines > 5 {
		score += 0.1
	}
	if lines > 20 {
		score += 0.1
	}
	if lines > 50 {
		score += 0.05
	}

	// Check if output addresses prompt keywords
	if prompt != "" {
		promptWords := strings.Fields(strings.ToLower(prompt))
		outputLower := strings.ToLower(output)
		matched := 0
		for _, w := range promptWords {
			if len(w) > 3 && strings.Contains(outputLower, w) {
				matched++
			}
		}
		if len(promptWords) > 0 {
			ratio := float64(matched) / float64(len(promptWords))
			score += ratio * 0.25
		}
	}

	// Check for structured output
	if strings.Contains(output, "```") {
		score += 0.05 // has code blocks
	}

	return score
}

func (s *Scorer) scoreStyle(output string) float64 {
	score := 0.7

	// Check for consistent formatting
	lines := strings.Split(output, "\n")
	indented := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "\t") {
			indented++
		}
	}

	if len(lines) > 0 && indented > 0 {
		ratio := float64(indented) / float64(len(lines))
		if ratio > 0.1 && ratio < 0.8 {
			score += 0.1 // reasonable indentation
		}
	}

	// Bonus for headers/structure
	if strings.Contains(output, "##") || strings.Contains(output, "**") {
		score += 0.05
	}

	// Penalize very long lines
	longLines := 0
	for _, l := range lines {
		if len(l) > 120 {
			longLines++
		}
	}
	if len(lines) > 0 {
		longRatio := float64(longLines) / float64(len(lines))
		score -= longRatio * 0.1
	}

	return score
}

func (s *Scorer) scoreSecurity(output string) float64 {
	score := 0.9 // start high, penalize for issues

	lower := strings.ToLower(output)

	// Check for dangerous patterns
	dangerous := []string{
		"rm -rf", "sudo ", "chmod 777", "password =",
		"api_key =", "secret =", "token =",
		"eval(", "exec(", "os.system(",
		"subprocess.call(", "shell=true",
		"<script>", "onclick=", "onerror=",
		"drop table", "delete from",
	}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			score -= 0.1
		}
	}

	// Check for redaction
	if strings.Contains(output, "••••") || strings.Contains(output, "[REDACTED]") || strings.Contains(output, "***") {
		score += 0.05 // secrets are being redacted
	}

	return score
}

func (s *Scorer) scoreEfficiency(cost, tokensUsed, duration float64, outputLines int) float64 {
	score := 0.7

	// Cost efficiency
	if cost > 0 {
		if cost < 0.01 {
			score += 0.2
		} else if cost < 0.10 {
			score += 0.1
		} else if cost > 1.0 {
			score -= 0.1
		}
	}

	// Token efficiency (output per token)
	if tokensUsed > 0 && outputLines > 0 {
		ratio := float64(outputLines) / float64(tokensUsed)
		if ratio > 0.1 {
			score += 0.05
		}
		if ratio < 0.01 {
			score -= 0.05
		}
	}

	// Time efficiency
	if duration > 0 {
		if duration < 5 {
			score += 0.1
		} else if duration > 60 {
			score -= 0.1
		}
	}

	return score
}

func (s *Scorer) scoreReadability(output string, lines int) float64 {
	score := 0.7

	// Average line length
	avgLen := 0.0
	if lines > 0 {
		totalLen := 0
		for _, l := range strings.Split(output, "\n") {
			totalLen += len(l)
		}
		avgLen = float64(totalLen) / float64(lines)
	}

	if avgLen > 20 && avgLen < 80 {
		score += 0.15
	} else if avgLen > 100 {
		score -= 0.1
	}

	// Paragraph breaks
	blankLines := 0
	for _, l := range strings.Split(output, "\n") {
		if strings.TrimSpace(l) == "" {
			blankLines++
		}
	}
	if lines > 0 {
		blankRatio := float64(blankLines) / float64(lines)
		if blankRatio > 0.05 && blankRatio < 0.4 {
			score += 0.1
		}
	}

	return score
}

func (s *Scorer) scoreTestability(output string, files []FileChange) float64 {
	score := 0.5

	// Check for test-related content
	lower := strings.ToLower(output)
	testPatterns := []string{"test", "spec", "assert", "expect", "mock", "stub"}
	for _, p := range testPatterns {
		if strings.Contains(lower, p) {
			score += 0.05
		}
	}

	// Check for test files in changes
	for _, f := range files {
		if strings.Contains(f.Path, "_test.") || strings.Contains(f.Path, "_spec.") || strings.Contains(f.Path, ".test.") {
			score += 0.1
		}
	}

	// Check for function/method definitions (testable units)
	funcCount := strings.Count(output, "func ") + strings.Count(output, "function ") + strings.Count(output, "def ")
	if funcCount > 0 {
		score += math.Min(float64(funcCount)*0.05, 0.2)
	}

	return score
}

// GetReport retrieves a saved report.
func (s *Scorer) GetReport(id string) (*Report, error) {
	return s.store.Get(id)
}

// ListReports returns recent quality reports.
func (s *Scorer) ListReports(agentID string, limit int) ([]*Report, error) {
	return s.store.List(agentID, limit)
}

// Trend computes quality trends for an agent.
func (s *Scorer) Trend(agentID string, lastN int) (*TrendReport, error) {
	reports, err := s.store.List(agentID, lastN)
	if err != nil {
		return nil, err
	}

	if len(reports) == 0 {
		return nil, fmt.Errorf("no reports found for agent %s", agentID)
	}

	tr := &TrendReport{
		AgentID:   agentID,
		SampleSize: len(reports),
	}

	// Compute averages per dimension
	dimSums := make(map[Dimension]float64)
	dimCounts := make(map[Dimension]int)
	compositeSum := 0.0
	passCount := 0
	costSum := 0.0

	for _, r := range reports {
		compositeSum += r.Composite
		if r.Passed {
			passCount++
		}
		costSum += r.Cost
		for _, sc := range r.Scores {
			dimSums[sc.Dimension] += sc.Value
			dimCounts[sc.Dimension]++
		}
	}

	n := float64(len(reports))
	tr.AvgComposite = compositeSum / n
	tr.PassRate = float64(passCount) / n
	tr.AvgCost = costSum / n

	tr.DimensionAverages = make(map[Dimension]float64)
	for d, sum := range dimSums {
		if dimCounts[d] > 0 {
			tr.DimensionAverages[d] = sum / float64(dimCounts[d])
		}
	}

	// Compute trend direction
	if len(reports) >= 3 {
		firstThird := reports[:len(reports)/3]
		lastThird := reports[len(reports)-len(reports)/3:]
		firstAvg := avgComposite(firstThird)
		lastAvg := avgComposite(lastThird)
		diff := lastAvg - firstAvg
		if diff > 0.05 {
			tr.Direction = "improving"
		} else if diff < -0.05 {
			tr.Direction = "declining"
		} else {
			tr.Direction = "stable"
		}
	}

	return tr, nil
}

// TrendReport shows quality trends.
type TrendReport struct {
	AgentID           string             `json:"agent_id"`
	SampleSize        int                `json:"sample_size"`
	AvgComposite      float64            `json:"avg_composite"`
	PassRate          float64            `json:"pass_rate"`
	AvgCost           float64            `json:"avg_cost"`
	Direction         string             `json:"direction"` // improving, declining, stable
	DimensionAverages map[Dimension]float64 `json:"dimension_averages"`
}

// FormatReport renders a report for display.
func FormatReport(r *Report) string {
	var sb strings.Builder
	status := "PASS"
	if !r.Passed {
		status = "FAIL"
	}
	sb.WriteString(fmt.Sprintf("Quality Report: %s [%s]\n", r.ID, status))
	sb.WriteString(fmt.Sprintf("  Agent:    %s\n", r.AgentID))
	sb.WriteString(fmt.Sprintf("  Model:    %s\n", r.Model))
	sb.WriteString(fmt.Sprintf("  Session:  %s\n", r.SessionID))
	sb.WriteString(fmt.Sprintf("  Score:    %.2f / %.2f\n", r.Composite, r.Threshold))
	sb.WriteString(fmt.Sprintf("  Cost:     $%.4f\n", r.Cost))
	sb.WriteString(fmt.Sprintf("  Tokens:   %d\n", r.TokensUsed))
	sb.WriteString(fmt.Sprintf("  Duration: %.1fs\n", r.Duration))
	sb.WriteString("\n  Dimensions:\n")
	for _, sc := range r.Scores {
		bar := scoreBar(sc.Value)
		sb.WriteString(fmt.Sprintf("    %-14s %s %.2f  %s\n", sc.Dimension, bar, sc.Value, sc.Reason))
	}
	sb.WriteString(fmt.Sprintf("\n  Created: %s\n", r.CreatedAt.Format(time.RFC3339)))
	return sb.String()
}

// FormatTrend renders a trend report.
func FormatTrend(tr *TrendReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Quality Trend: %s\n", tr.AgentID))
	sb.WriteString(fmt.Sprintf("  Samples:     %d\n", tr.SampleSize))
	sb.WriteString(fmt.Sprintf("  Avg Score:   %.2f\n", tr.AvgComposite))
	sb.WriteString(fmt.Sprintf("  Pass Rate:   %.0f%%\n", tr.PassRate*100))
	sb.WriteString(fmt.Sprintf("  Avg Cost:    $%.4f\n", tr.AvgCost))
	sb.WriteString(fmt.Sprintf("  Direction:   %s\n", tr.Direction))
	sb.WriteString("\n  Dimension Averages:\n")
	for _, dim := range AllDimensions() {
		if avg, ok := tr.DimensionAverages[dim]; ok {
			bar := scoreBar(avg)
			sb.WriteString(fmt.Sprintf("    %-14s %s %.2f\n", dim, bar, avg))
		}
	}
	return sb.String()
}

func scoreBar(v float64) string {
	filled := int(v * 10)
	empty := 10 - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func avgComposite(reports []*Report) float64 {
	sum := 0.0
	for _, r := range reports {
		sum += r.Composite
	}
	if len(reports) == 0 {
		return 0
	}
	return sum / float64(len(reports))
}

// Store persists quality reports.
type Store struct {
	mu  sync.RWMutex
	dir string
}

// NewStore creates a quality report store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Save persists a report.
func (st *Store) Save(r *Report) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	// Save by ID
	path := filepath.Join(st.dir, r.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	// Also append to agent log
	agentDir := filepath.Join(st.dir, r.AgentID)
	os.MkdirAll(agentDir, 0o755)
	indexPath := filepath.Join(agentDir, "index.jsonl")
	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	entry, _ := json.Marshal(map[string]interface{}{
		"id":        r.ID,
		"composite": r.Composite,
		"passed":    r.Passed,
		"cost":      r.Cost,
		"created":   r.CreatedAt,
	})
	f.Write(entry)
	f.Write([]byte("\n"))

	return nil
}

// Get retrieves a report by ID.
func (st *Store) Get(id string) (*Report, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	path := filepath.Join(st.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("report %q not found", id)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// List returns recent reports, optionally filtered by agent.
func (st *Store) List(agentID string, limit int) ([]*Report, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var reports []*Report

	searchDir := st.dir
	if agentID != "" {
		searchDir = filepath.Join(st.dir, agentID)
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, nil
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(searchDir, e.Name()))
		if err != nil {
			continue
		}
		var r Report
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		reports = append(reports, &r)
		if len(reports) >= limit {
			break
		}
	}

	return reports, nil
}

func generateID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("%s-%x", prefix, b)
}
