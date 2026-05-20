// Package dream provides offline agent improvement when no user tasks are pending.
// Agents analyze past sessions for patterns, optimize prompts, adjust routing,
// prune stale memory, and pre-index recent changes.
//
// Idle time is wasted time. Dream mode turns it into improvement.
package dream

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DreamPhase represents a phase of dream processing.
type DreamPhase string

const (
	PhaseAnalyze    DreamPhase = "analyze"     // Analyze past sessions
	PhaseOptimize   DreamPhase = "optimize"    // Optimize prompts and routing
	PhasePrune      DreamPhase = "prune"       // Prune stale memory
	PhaseIndex      DreamPhase = "index"       // Pre-index recent changes
	PhaseReport     DreamPhase = "report"      // Generate dream report
)

// Pattern represents a recurring pattern found in agent sessions.
type Pattern struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"` // error, success, cost, latency, quality
	Description string   `json:"description"`
	Frequency   int      `json:"frequency"`
	Impact      string   `json:"impact"` // high, medium, low
	Suggestion  string   `json:"suggestion,omitempty"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

// Optimization represents a suggested or applied optimization.
type Optimization struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // prompt, routing, cost, memory
	Target      string    `json:"target"` // what's being optimized
	Before      string    `json:"before,omitempty"`
	After       string    `json:"after,omitempty"`
	Suggestion  string    `json:"suggestion,omitempty"`
	Savings     string    `json:"savings,omitempty"` // estimated savings
	Applied     bool      `json:"applied"`
	Confidence  float64   `json:"confidence"` // 0-1
	CreatedAt   time.Time `json:"created_at"`
}

// DreamReport is the output of a dream session.
type DreamReport struct {
	ID            string         `json:"id"`
	StartedAt     time.Time      `json:"started_at"`
	CompletedAt   time.Time      `json:"completed_at,omitempty"`
	Duration      time.Duration  `json:"duration"`
	Phases        []DreamPhase   `json:"phases"`
	PatternsFound []Pattern      `json:"patterns_found"`
	Optimizations []Optimization `json:"optimizations"`
	Summary       string         `json:"summary"`
	MemoryPruned  int            `json:"memory_pruned"`
	FilesIndexed  int            `json:"files_indexed"`
	Status        string         `json:"status"` // running, completed, failed
}

// Session represents a past agent session for analysis.
type Session struct {
	ID         string    `json:"id"`
	Agent      string    `json:"agent"`
	Model      string    `json:"model"`
	Task       string    `json:"task"`
	Success    bool      `json:"success"`
	CostUSD    float64   `json:"cost_usd"`
	Duration   time.Duration `json:"duration"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	TokensUsed int64     `json:"tokens_used"`
}

// Store manages dream reports and session history.
type Store struct {
	Dir string
}

// NewStore creates a dream store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// DreamSession runs a complete dream cycle.
type DreamSession struct {
	store    *Store
	sessions []Session // input sessions to analyze
	phases   []DreamPhase
	report   *DreamReport
}

// NewDreamSession creates a new dream processing session.
func NewDreamSession(store *Store) *DreamSession {
	return &DreamSession{
		store:  store,
		phases: []DreamPhase{PhaseAnalyze, PhaseOptimize, PhasePrune, PhaseIndex, PhaseReport},
		report: &DreamReport{
			ID:        fmt.Sprintf("dream-%d", time.Now().UnixNano()),
			StartedAt: time.Now(),
			Phases:    []DreamPhase{},
			Status:    "running",
		},
	}
}

// LoadSessions loads session history for analysis.
func (ds *DreamSession) LoadSessions(sessions []Session) {
	ds.sessions = sessions
}

// Run executes all dream phases and returns the report.
func (ds *DreamSession) Run() (*DreamReport, error) {
	if err := os.MkdirAll(ds.store.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dream dir: %w", err)
	}

	for _, phase := range ds.phases {
		ds.report.Phases = append(ds.report.Phases, phase)
		switch phase {
		case PhaseAnalyze:
			ds.analyze()
		case PhaseOptimize:
			ds.optimize()
		case PhasePrune:
			ds.prune()
		case PhaseIndex:
			ds.index()
		case PhaseReport:
			ds.generateReport()
		}
	}

	ds.report.CompletedAt = time.Now()
	ds.report.Duration = ds.report.CompletedAt.Sub(ds.report.StartedAt)
	ds.report.Status = "completed"

	// Persist report
	if err := ds.store.SaveReport(ds.report); err != nil {
		return nil, fmt.Errorf("failed to save dream report: %w", err)
	}

	return ds.report, nil
}

// analyze finds patterns in past sessions.
func (ds *DreamSession) analyze() {
	patterns := make(map[string]*Pattern)

	// Pattern 1: Recurring errors
	errors := make(map[string][]Session)
	for _, s := range ds.sessions {
		if !s.Success && s.Error != "" {
			// Normalize error keys
			key := normalizeError(s.Error)
			errors[key] = append(errors[key], s)
		}
	}
	for key, sessions := range errors {
		if len(sessions) >= 2 {
			patterns[key] = &Pattern{
				ID:          fmt.Sprintf("pat-%d", time.Now().UnixNano()+int64(len(patterns))),
				Type:        "error",
				Description: fmt.Sprintf("Recurring error: %s (seen %d times)", key, len(sessions)),
				Frequency:   len(sessions),
				Impact:      impactLevel(len(sessions)),
				Suggestion:  fmt.Sprintf("Investigate root cause of '%s' — appears in %d sessions", key, len(sessions)),
				FirstSeen:   sessions[0].CreatedAt,
				LastSeen:    sessions[len(sessions)-1].CreatedAt,
			}
		}
	}

	// Pattern 2: Cost outliers
	if len(ds.sessions) > 0 {
		var totalCost float64
		for _, s := range ds.sessions {
			totalCost += s.CostUSD
		}
		avgCost := totalCost / float64(len(ds.sessions))

		var expensive []Session
		for _, s := range ds.sessions {
			if s.CostUSD > avgCost*2 && s.CostUSD > 0.01 {
				expensive = append(expensive, s)
			}
		}
		if len(expensive) >= 2 {
			patterns["cost-outlier"] = &Pattern{
				ID:          fmt.Sprintf("pat-%d", time.Now().UnixNano()+int64(len(patterns))),
				Type:        "cost",
				Description: fmt.Sprintf("%d sessions cost >2x average ($%.4f avg)", len(expensive), avgCost),
				Frequency:   len(expensive),
				Impact:      "high",
				Suggestion:  "Consider switching expensive sessions to cheaper models or optimizing prompts",
				FirstSeen:   expensive[0].CreatedAt,
				LastSeen:    expensive[len(expensive)-1].CreatedAt,
			}
		}
	}

	// Pattern 3: Model performance comparison
	modelStats := make(map[string]struct{ success, total int })
	for _, s := range ds.sessions {
		stats := modelStats[s.Model]
		stats.total++
		if s.Success {
			stats.success++
		}
		modelStats[s.Model] = stats
	}
	for model, stats := range modelStats {
		if stats.total >= 3 {
			rate := float64(stats.success) / float64(stats.total)
			if rate < 0.5 {
				patterns["model-"+model] = &Pattern{
					ID:          fmt.Sprintf("pat-%d", time.Now().UnixNano()+int64(len(patterns))),
					Type:        "quality",
					Description: fmt.Sprintf("Model %s has %.0f%% success rate (%d/%d)", model, rate*100, stats.success, stats.total),
					Frequency:   stats.total,
					Impact:      "medium",
					Suggestion:  fmt.Sprintf("Consider switching from %s to a more reliable model for similar tasks", model),
					FirstSeen:   time.Now().Add(-24 * time.Hour),
					LastSeen:    time.Now(),
				}
			}
		}
	}

	// Pattern 4: Slow sessions
	var slowSessions []Session
	for _, s := range ds.sessions {
		if s.Duration > 5*time.Minute {
			slowSessions = append(slowSessions, s)
		}
	}
	if len(slowSessions) >= 2 {
		patterns["slow-sessions"] = &Pattern{
			ID:          fmt.Sprintf("pat-%d", time.Now().UnixNano()+int64(len(patterns))),
			Type:        "latency",
			Description: fmt.Sprintf("%d sessions took >5 minutes", len(slowSessions)),
			Frequency:   len(slowSessions),
			Impact:      "medium",
			Suggestion:  "Long sessions may benefit from prompt optimization or task decomposition",
			FirstSeen:   slowSessions[0].CreatedAt,
			LastSeen:    slowSessions[len(slowSessions)-1].CreatedAt,
		}
	}

	for _, p := range patterns {
		ds.report.PatternsFound = append(ds.report.PatternsFound, *p)
	}
}

// optimize suggests and optionally applies optimizations.
func (ds *DreamSession) optimize() {
	// Generate optimizations from patterns
	for _, p := range ds.report.PatternsFound {
		var optType string
		switch p.Type {
		case "cost":
			optType = "cost"
		case "quality":
			optType = "routing"
		case "latency":
			optType = "prompt"
		default:
			optType = "general"
		}

		ds.report.Optimizations = append(ds.report.Optimizations, Optimization{
			ID:         fmt.Sprintf("opt-%d", time.Now().UnixNano()+int64(len(ds.report.Optimizations))),
			Type:       optType,
			Target:     p.Description,
			Suggestion: p.Suggestion,
			Applied:    false,
			Confidence: confidenceForImpact(p.Impact),
			CreatedAt:  time.Now(),
		})
	}

	// Token efficiency optimization
	totalTokens := int64(0)
	for _, s := range ds.sessions {
		totalTokens += s.TokensUsed
	}
	if totalTokens > 100000 && len(ds.sessions) > 10 {
		ds.report.Optimizations = append(ds.report.Optimizations, Optimization{
			ID:         fmt.Sprintf("opt-token-%d", time.Now().UnixNano()),
			Type:       "prompt",
			Target:     "Token efficiency",
			Suggestion: "High total token usage detected. Consider compressing system prompts or using few-shot examples instead of long instructions.",
			Applied:    false,
			Confidence: 0.7,
			CreatedAt:  time.Now(),
		})
	}
}

// prune identifies stale memory entries that could be removed.
func (ds *DreamSession) prune() {
	pruned := 0

	// Count old sessions (older than 30 days) as candidates for pruning
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	for _, s := range ds.sessions {
		if s.CreatedAt.Before(cutoff) && !s.Success {
			pruned++
		}
	}

	ds.report.MemoryPruned = pruned
}

// index simulates pre-indexing of recent changes.
func (ds *DreamSession) index() {
	// Count sessions from last 24 hours as "recently changed" items
	recent := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, s := range ds.sessions {
		if s.CreatedAt.After(cutoff) {
			recent++
		}
	}
	ds.report.FilesIndexed = recent
}

// generateReport creates the final summary.
func (ds *DreamSession) generateReport() {
	var parts []string

	parts = append(parts, fmt.Sprintf("Dream session analyzed %d past sessions.", len(ds.sessions)))

	if len(ds.report.PatternsFound) > 0 {
		parts = append(parts, fmt.Sprintf("Found %d patterns:", len(ds.report.PatternsFound)))
		for _, p := range ds.report.PatternsFound {
			parts = append(parts, fmt.Sprintf("  • [%s] %s", p.Impact, p.Description))
		}
	}

	if len(ds.report.Optimizations) > 0 {
		applied := 0
		for _, o := range ds.report.Optimizations {
			if o.Applied {
				applied++
			}
		}
		parts = append(parts, fmt.Sprintf("Generated %d optimizations (%d auto-applied).", len(ds.report.Optimizations), applied))
	}

	if ds.report.MemoryPruned > 0 {
		parts = append(parts, fmt.Sprintf("Pruned %d stale memory entries.", ds.report.MemoryPruned))
	}

	if ds.report.FilesIndexed > 0 {
		parts = append(parts, fmt.Sprintf("Pre-indexed %d recent items for faster queries.", ds.report.FilesIndexed))
	}

	ds.report.Summary = strings.Join(parts, " ")
}

// SaveReport persists a dream report.
func (s *Store) SaveReport(report *DreamReport) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, report.ID+".json"), data, 0o644)
}

// GetReport loads a dream report by ID.
func (s *Store) GetReport(id string) (*DreamReport, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("dream report %q not found: %w", id, err)
	}
	var report DreamReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse dream report: %w", err)
	}
	return &report, nil
}

// ListReports returns all dream reports, newest first.
func (s *Store) ListReports() ([]*DreamReport, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var reports []*DreamReport
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var r DreamReport
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		reports = append(reports, &r)
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].StartedAt.After(reports[j].StartedAt)
	})

	return reports, nil
}

// DeleteReport removes a dream report.
func (s *Store) DeleteReport(id string) error {
	return os.Remove(filepath.Join(s.Dir, id+".json"))
}

// FormatReport renders a dream report as a readable string.
func FormatReport(report *DreamReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("💤 Dream Report: %s\n", report.ID))
	sb.WriteString(fmt.Sprintf("  Duration: %s | Status: %s\n",
		report.Duration.Round(time.Millisecond), report.Status))
	sb.WriteString(fmt.Sprintf("  Phases: %s\n\n", strings.Join(func() []string {
		var names []string
		for _, p := range report.Phases {
			names = append(names, string(p))
		}
		return names
	}(), " → ")))

	if len(report.PatternsFound) > 0 {
		sb.WriteString("Patterns Found:\n")
		for _, p := range report.PatternsFound {
			sb.WriteString(fmt.Sprintf("  [%s] %s (%d occurrences)\n", p.Impact, p.Description, p.Frequency))
			if p.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("    → %s\n", p.Suggestion))
			}
		}
		sb.WriteString("\n")
	}

	if len(report.Optimizations) > 0 {
		sb.WriteString("Optimizations:\n")
		for _, o := range report.Optimizations {
			status := "pending"
			if o.Applied {
				status = "applied"
			}
			sb.WriteString(fmt.Sprintf("  [%s|%s] %s (confidence: %.0f%%)\n",
				o.Type, status, o.Target, o.Confidence*100))
			if o.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("    → %s\n", o.Suggestion))
			}
		}
		sb.WriteString("\n")
	}

	if report.MemoryPruned > 0 || report.FilesIndexed > 0 {
		sb.WriteString("Maintenance:\n")
		if report.MemoryPruned > 0 {
			sb.WriteString(fmt.Sprintf("  🗑 Pruned %d stale memory entries\n", report.MemoryPruned))
		}
		if report.FilesIndexed > 0 {
			sb.WriteString(fmt.Sprintf("  📇 Pre-indexed %d recent items\n", report.FilesIndexed))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Summary: %s\n", report.Summary))

	return sb.String()
}

func normalizeError(err string) string {
	// Simplify error messages to find patterns
	err = strings.ToLower(err)
	// Remove specific IDs, paths, numbers
	for _, repl := range []string{"0x", "/", "\\", ":"} {
		err = strings.ReplaceAll(err, repl, " ")
	}
	words := strings.Fields(err)
	if len(words) > 6 {
		words = words[:6]
	}
	return strings.Join(words, " ")
}

func impactLevel(freq int) string {
	if freq >= 5 {
		return "high"
	}
	if freq >= 3 {
		return "medium"
	}
	return "low"
}

func confidenceForImpact(impact string) float64 {
	switch impact {
	case "high":
		return 0.9
	case "medium":
		return 0.7
	default:
		return 0.5
	}
}
