// Package dreamreview provides scheduled memory review for Forge agents.
// While the forge sleeps, the hammer remembers — patterns surface from the deep.
package dreamreview

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

// ReviewSession is a single memory review session.
type ReviewSession struct {
	ID           string          `json:"id"`
	StartedAt    time.Time       `json:"started_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
	InputsScanned int            `json:"inputs_scanned"`
	PatternsFound []Pattern      `json:"patterns_found"`
	Suggestions   []Suggestion    `json:"suggestions"`
	PrunedCount   int            `json:"pruned_count"`
	NewMemory     []MemoryEntry  `json:"new_memory"`
	Status        string         `json:"status"`
}

// Pattern represents a discovered pattern in agent interactions.
type Pattern struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Frequency   int      `json:"frequency"`
	Confidence  float64  `json:"confidence"`
	Examples    []string `json:"examples,omitempty"`
	Category    string   `json:"category"`
	Actionable  bool     `json:"actionable"`
}

// Suggestion is an actionable recommendation from a pattern.
type Suggestion struct {
	ID          string  `json:"id"`
	PatternID   string  `json:"pattern_id"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Priority    int     `json:"priority"`
	Impact      string  `json:"impact"`
	AutoApply   bool    `json:"auto_apply"`
	Applied     bool    `json:"applied"`
}

// MemoryEntry is a distilled memory extracted from patterns.
type MemoryEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	Confidence float64  `json:"confidence"`
	CreatedAt time.Time `json:"created_at"`
	TTL       string    `json:"ttl,omitempty"`
}

// ReviewConfig configures the dream review process.
type ReviewConfig struct {
	MaxSessionsPerRun  int     `json:"max_sessions_per_run"`
	MinPatternFreq     int     `json:"min_pattern_freq"`
	MinConfidence      float64 `json:"min_confidence"`
	AutoPrune          bool    `json:"auto_prune"`
	PruneOlderThan     string  `json:"prune_older_than"`
	AutoApplyThreshold float64 `json:"auto_apply_threshold"`
	StoreDir           string  `json:"store_dir"`
}

// DefaultReviewConfig returns sensible defaults.
func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		MaxSessionsPerRun:  100,
		MinPatternFreq:     3,
		MinConfidence:      0.6,
		AutoPrune:          true,
		PruneOlderThan:     "30d",
		AutoApplyThreshold: 0.85,
	}
}

// DreamReviewer performs scheduled memory reviews.
type DreamReviewer struct {
	config   ReviewConfig
	sessions []ReviewSession
	mu       sync.RWMutex
}

// NewDreamReviewer creates a new dream reviewer.
func NewDreamReviewer(config ReviewConfig) *DreamReviewer {
	os.MkdirAll(config.StoreDir, 0o755)
	dr := &DreamReviewer{
		config: config,
	}
	dr.load()
	return dr
}

// Run executes a memory review cycle.
func (dr *DreamReviewer) Run(inputs []ReviewInput) (*ReviewSession, error) {
	session := &ReviewSession{
		ID:          fmt.Sprintf("dream-%d", time.Now().UnixNano()),
		StartedAt:   time.Now().UTC(),
		Status:      "running",
		InputsScanned: len(inputs),
	}

	// Phase 1: Pattern Detection
	patterns := dr.detectPatterns(inputs)
	session.PatternsFound = patterns

	// Phase 2: Suggestion Generation
	suggestions := dr.generateSuggestions(patterns)
	session.Suggestions = suggestions

	// Phase 3: Memory Extraction
	newMemories := dr.extractMemories(patterns, inputs)
	session.NewMemory = newMemories

	// Phase 4: Auto-pruning
	if dr.config.AutoPrune {
		pruned := dr.autoPrune(inputs)
		session.PrunedCount = pruned
	}

	// Phase 5: Auto-apply high-confidence suggestions
	for i := range session.Suggestions {
		if session.Suggestions[i].AutoApply && session.Suggestions[i].Priority >= 8 {
			session.Suggestions[i].Applied = true
		}
	}

	now := time.Now().UTC()
	session.FinishedAt = &now
	session.Status = "completed"

	dr.mu.Lock()
	dr.sessions = append(dr.sessions, *session)
	dr.mu.Unlock()

	dr.save()
	return session, nil
}

// ReviewInput is data to be reviewed (e.g., session logs, agent outputs).
type ReviewInput struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // session, error, cost, feedback
	Agent     string            `json:"agent"`
	Model     string            `json:"model"`
	Timestamp time.Time         `json:"timestamp"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// detectPatterns finds recurring patterns in review inputs.
func (dr *DreamReviewer) detectPatterns(inputs []ReviewInput) []Pattern {
	var patterns []Pattern

	// Pattern: Repeated errors with same agent
	errorFreq := make(map[string][]string) // agent -> error messages
	for _, input := range inputs {
		if input.Type == "error" {
			key := input.Agent + ":" + input.Content
			errorFreq[key] = append(errorFreq[key], input.ID)
		}
	}
	for key, ids := range errorFreq {
		if len(ids) >= dr.config.MinPatternFreq {
			parts := strings.SplitN(key, ":", 2)
			pattern := Pattern{
				ID:          fmt.Sprintf("pat-err-%d", len(patterns)),
				Type:        "repeated_error",
				Description: fmt.Sprintf("Agent %s repeatedly encounters: %s", parts[0], truncate(parts[1], 80)),
				Frequency:   len(ids),
				Confidence:  float64(len(ids)) / float64(len(inputs)),
				Examples:    ids[:min(3, len(ids))],
				Category:    "reliability",
				Actionable:  true,
			}
			if pattern.Confidence < dr.config.MinConfidence {
				pattern.Confidence = dr.config.MinConfidence
			}
			patterns = append(patterns, pattern)
		}
	}

	// Pattern: Model preference patterns
	modelFreq := make(map[string]int) // model -> count
	for _, input := range inputs {
		if input.Model != "" {
			modelFreq[input.Model]++
		}
	}
	for model, count := range modelFreq {
		if count >= dr.config.MinPatternFreq {
			pattern := Pattern{
				ID:          fmt.Sprintf("pat-model-%d", len(patterns)),
				Type:        "model_usage",
				Description: fmt.Sprintf("Model %s used %d times (%.0f%% of inputs)", model, count, float64(count)/float64(len(inputs))*100),
				Frequency:   count,
				Confidence:  float64(count) / float64(len(inputs)),
				Category:    "cost",
				Actionable:  true,
			}
			patterns = append(patterns, pattern)
		}
	}

	// Pattern: Agent success/failure ratio
	agentOutcomes := make(map[string]struct{ success, fail int })
	for _, input := range inputs {
		if input.Type == "session" || input.Type == "error" {
			outcomes := agentOutcomes[input.Agent]
			if input.Type == "error" {
				outcomes.fail++
			} else {
				outcomes.success++
			}
			agentOutcomes[input.Agent] = outcomes
		}
	}
	for agent, outcomes := range agentOutcomes {
		total := outcomes.success + outcomes.fail
		if total < dr.config.MinPatternFreq {
			continue
		}
		failRate := float64(outcomes.fail) / float64(total)
		if failRate > 0.3 {
			pattern := Pattern{
				ID:          fmt.Sprintf("pat-fail-%d", len(patterns)),
				Type:        "high_failure_rate",
				Description: fmt.Sprintf("Agent %s has %.0f%% failure rate (%d/%d)", agent, failRate*100, outcomes.fail, total),
				Frequency:   outcomes.fail,
				Confidence:  failRate,
				Category:    "reliability",
				Actionable:  true,
			}
			patterns = append(patterns, pattern)
		}
	}

	// Pattern: Time-based patterns (e.g., more errors at certain times)
	hourBuckets := make(map[int]struct{ total, errors int })
	for _, input := range inputs {
		hour := input.Timestamp.Hour()
		b := hourBuckets[hour]
		b.total++
		if input.Type == "error" {
			b.errors++
		}
		hourBuckets[hour] = b
	}
	for hour, b := range hourBuckets {
		if b.total >= dr.config.MinPatternFreq {
			errRate := float64(b.errors) / float64(b.total)
			if errRate > 0.4 {
				pattern := Pattern{
					ID:          fmt.Sprintf("pat-time-%d", len(patterns)),
					Type:        "temporal_error_spike",
					Description: fmt.Sprintf("Higher error rate at hour %02d:00 (%.0f%%, %d/%d)", hour, errRate*100, b.errors, b.total),
					Frequency:   b.errors,
					Confidence:  errRate,
					Category:    "temporal",
					Actionable:  false,
				}
				patterns = append(patterns, pattern)
			}
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})

	return patterns
}

// generateSuggestions creates actionable suggestions from patterns.
func (dr *DreamReviewer) generateSuggestions(patterns []Pattern) []Suggestion {
	var suggestions []Suggestion

	for _, p := range patterns {
		if !p.Actionable {
			continue
		}

		switch p.Type {
		case "repeated_error":
			suggestions = append(suggestions, Suggestion{
				ID:          fmt.Sprintf("sug-%d", len(suggestions)),
				PatternID:   p.ID,
				Type:        "fix_prompt",
				Description: fmt.Sprintf("Update agent prompt to handle: %s", truncate(p.Description, 60)),
				Priority:    7,
				Impact:      "Reduces repeated errors by addressing root cause",
				AutoApply:   p.Confidence >= dr.config.AutoApplyThreshold,
			})

		case "model_usage":
			suggestions = append(suggestions, Suggestion{
				ID:          fmt.Sprintf("sug-%d", len(suggestions)),
				PatternID:   p.ID,
				Type:        "cost_optimize",
				Description: fmt.Sprintf("Review if %s is the optimal model for these tasks", truncate(p.Description, 60)),
				Priority:    5,
				Impact:      "Potential cost savings with model optimization",
				AutoApply:   false,
			})

		case "high_failure_rate":
			suggestions = append(suggestions, Suggestion{
				ID:          fmt.Sprintf("sug-%d", len(suggestions)),
				PatternID:   p.ID,
				Type:        "agent_reconfigure",
				Description: fmt.Sprintf("Investigate and fix high failure rate: %s", truncate(p.Description, 60)),
				Priority:    9,
				Impact:      "Critical: agent reliability issue",
				AutoApply:   false,
			})
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority > suggestions[j].Priority
	})

	return suggestions
}

// extractMemories creates distilled memory entries from patterns.
func (dr *DreamReviewer) extractMemories(patterns []Pattern, inputs []ReviewInput) []MemoryEntry {
	var memories []MemoryEntry

	for _, p := range patterns {
		if p.Confidence < dr.config.MinConfidence {
			continue
		}

		entry := MemoryEntry{
			Key:        fmt.Sprintf("pattern:%s:%s", p.Type, p.ID),
			Value:      p.Description,
			Source:      "dream_review",
			Confidence: p.Confidence,
			CreatedAt:  time.Now().UTC(),
			TTL:        "7d",
		}

		switch p.Category {
		case "reliability":
			entry.TTL = "30d"
		case "cost":
			entry.TTL = "14d"
		case "temporal":
			entry.TTL = "3d"
		}

		memories = append(memories, entry)
	}

	// Extract common success patterns
	successContent := make(map[string]int)
	for _, input := range inputs {
		if input.Type == "session" && input.Content != "" {
			successContent[input.Content]++
		}
	}
	for content, count := range successContent {
		if count >= dr.config.MinPatternFreq {
			memories = append(memories, MemoryEntry{
				Key:        fmt.Sprintf("success:%d", len(memories)),
				Value:      fmt.Sprintf("Successful pattern (x%d): %s", count, truncate(content, 80)),
				Source:      "dream_review",
				Confidence: float64(count) / float64(len(inputs)),
				CreatedAt:  time.Now().UTC(),
				TTL:        "14d",
			})
		}
	}

	return memories
}

// autoPrune removes old/stale entries.
func (dr *DreamReviewer) autoPrune(inputs []ReviewInput) int {
	if dr.config.PruneOlderThan == "" {
		return 0
	}

	duration, err := time.ParseDuration(dr.config.PruneOlderThan)
	if err != nil {
		// Try days
		days := 30
		fmt.Sscanf(dr.config.PruneOlderThan, "%dd", &days)
		duration = time.Duration(days) * 24 * time.Hour
	}

	cutoff := time.Now().UTC().Add(-duration)
	pruned := 0
	for _, input := range inputs {
		if input.Timestamp.Before(cutoff) {
			pruned++
		}
	}
	return pruned
}

// History returns past review sessions.
func (dr *DreamReviewer) History(limit int) []ReviewSession {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	sessions := make([]ReviewSession, len(dr.sessions))
	copy(sessions, dr.sessions)

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})

	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions
}

// Stats returns dream reviewer statistics.
func (dr *DreamReviewer) Stats() DreamStats {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	stats := DreamStats{
		TotalReviews: len(dr.sessions),
	}

	for _, s := range dr.sessions {
		stats.TotalPatterns += len(s.PatternsFound)
		stats.TotalSuggestions += len(s.Suggestions)
		stats.TotalPruned += s.PrunedCount
		stats.TotalInputsScanned += s.InputsScanned
		if s.Status == "completed" {
			stats.CompletedReviews++
		}
	}

	return stats
}

// DreamStats holds dream reviewer statistics.
type DreamStats struct {
	TotalReviews       int `json:"total_reviews"`
	CompletedReviews   int `json:"completed_reviews"`
	TotalPatterns      int `json:"total_patterns"`
	TotalSuggestions   int `json:"total_suggestions"`
	TotalPruned        int `json:"total_pruned"`
	TotalInputsScanned int `json:"total_inputs_scanned"`
}

// FormatSession renders a review session.
func FormatSession(s *ReviewSession) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Dream Review: %s\n", s.ID))
	sb.WriteString(fmt.Sprintf("  Status:     %s\n", s.Status))
	sb.WriteString(fmt.Sprintf("  Started:    %s\n", s.StartedAt.Format(time.RFC3339)))
	if s.FinishedAt != nil {
		sb.WriteString(fmt.Sprintf("  Finished:   %s\n", s.FinishedAt.Format(time.RFC3339)))
	}
	sb.WriteString(fmt.Sprintf("  Inputs:     %d\n", s.InputsScanned))
	sb.WriteString(fmt.Sprintf("  Patterns:   %d\n", len(s.PatternsFound)))
	sb.WriteString(fmt.Sprintf("  Suggestions: %d\n", len(s.Suggestions)))
	sb.WriteString(fmt.Sprintf("  Pruned:     %d\n", s.PrunedCount))

	if len(s.PatternsFound) > 0 {
		sb.WriteString("\n  Patterns:\n")
		for _, p := range s.PatternsFound {
			sb.WriteString(fmt.Sprintf("    [%s] %s (freq: %d, conf: %.2f)\n", p.Category, p.Description, p.Frequency, p.Confidence))
		}
	}

	if len(s.Suggestions) > 0 {
		sb.WriteString("\n  Suggestions:\n")
		for _, s := range s.Suggestions {
			applied := ""
			if s.Applied {
				applied = " [APPLIED]"
			}
			sb.WriteString(fmt.Sprintf("    P%d: %s%s\n", s.Priority, s.Description, applied))
		}
	}

	return sb.String()
}

// FormatStats renders dream stats.
func FormatStats(stats DreamStats) string {
	return fmt.Sprintf("Dream Review Stats:\n  Reviews:    %d (%d completed)\n  Patterns:   %d\n  Suggestions: %d\n  Pruned:     %d\n  Inputs:     %d\n",
		stats.TotalReviews, stats.CompletedReviews, stats.TotalPatterns, stats.TotalSuggestions, stats.TotalPruned, stats.TotalInputsScanned)
}

func (dr *DreamReviewer) save() {
	data, _ := json.MarshalIndent(dr.sessions, "", "  ")
	os.WriteFile(filepath.Join(dr.config.StoreDir, "dream-sessions.json"), data, 0o644)
}

func (dr *DreamReviewer) load() {
	data, err := os.ReadFile(filepath.Join(dr.config.StoreDir, "dream-sessions.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &dr.sessions)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
