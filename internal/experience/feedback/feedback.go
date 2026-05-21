// Package feedback collects and analyzes agent feedback loops.
// Track user corrections, agent self-assessments, and quality signals
// to continuously improve agent behavior.
//
// Agents that don't learn from feedback are just fancy scripts.
package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SignalType represents the type of feedback signal.
type SignalType string

const (
	SignalThumbsUp   SignalType = "thumbs_up"
	SignalThumbsDown SignalType = "thumbs_down"
	SignalCorrection SignalType = "correction"
	SignalRating     SignalType = "rating"
	SignalBug        SignalType = "bug"
	SignalPraise     SignalType = "praise"
	SignalSuggestion SignalType = "suggestion"
	SignalSelfAssess SignalType = "self_assessment"
	SignalCostAlert  SignalType = "cost_alert"
	SignalTimeout    SignalType = "timeout"
)

// Signal represents a single feedback signal.
type Signal struct {
	ID         string            `json:"id"`
	Type       SignalType        `json:"type"`
	Agent      string            `json:"agent"`
	SessionID  string            `json:"session_id,omitempty"`
	Model      string            `json:"model,omitempty"`
	Prompt     string            `json:"prompt,omitempty"`
	Response   string            `json:"response,omitempty"`
	Correction string            `json:"correction,omitempty"` // user's correction
	Rating     int               `json:"rating,omitempty"`     // 1-5
	Tags       []string          `json:"tags,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
}

// Analysis represents aggregated feedback analysis.
type Analysis struct {
	Agent            string           `json:"agent"`
	TotalSignals     int              `json:"total_signals"`
	PositiveCount    int              `json:"positive_count"`
	NegativeCount    int              `json:"negative_count"`
	SatisfactionRate float64          `json:"satisfaction_rate"` // 0-1
	AvgRating        float64          `json:"avg_rating"`
	CommonIssues     []IssueFrequency `json:"common_issues,omitempty"`
	TrendDirection   string           `json:"trend_direction"` // improving, declining, stable
	TrendSlope       float64          `json:"trend_slope"`
	Period           string           `json:"period"`
	Since            time.Time        `json:"since"`
	Until            time.Time        `json:"until"`
}

// IssueFrequency represents how often an issue occurs.
type IssueFrequency struct {
	Issue   string  `json:"issue"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// Loop represents a feedback loop — a cycle of signals about an agent.
type Loop struct {
	ID         string    `json:"id"`
	Agent      string    `json:"agent"`
	SignalIDs  []string  `json:"signal_ids"`
	Status     string    `json:"status"` // open, improving, resolved, ignored
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Resolution string    `json:"resolution,omitempty"`
}

// Store manages feedback signals.
type Store struct {
	Dir string
}

// NewStore creates a feedback store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Record saves a feedback signal.
func (s *Store) Record(signal Signal) (*Signal, error) {
	os.MkdirAll(s.Dir, 0o755)

	if signal.ID == "" {
		signal.ID = fmt.Sprintf("sig-%d", time.Now().UnixNano())
	}
	if signal.Timestamp.IsZero() {
		signal.Timestamp = time.Now()
	}

	data, err := json.MarshalIndent(signal, "", "  ")
	if err != nil {
		return nil, err
	}

	path := filepath.Join(s.Dir, signal.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}

	return &signal, nil
}

// Get retrieves a signal by ID.
func (s *Store) Get(id string) (*Signal, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("signal %q not found", id)
	}
	var signal Signal
	if err := json.Unmarshal(data, &signal); err != nil {
		return nil, err
	}
	return &signal, nil
}

// List returns signals, optionally filtered by agent.
func (s *Store) List(agent string, limit int) ([]*Signal, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var signals []*Signal
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var sig Signal
		if err := json.Unmarshal(data, &sig); err != nil {
			continue
		}

		if agent != "" && sig.Agent != agent {
			continue
		}

		signals = append(signals, &sig)
	}

	sort.Slice(signals, func(i, k int) bool {
		return signals[i].Timestamp.After(signals[k].Timestamp)
	})

	if limit > 0 && len(signals) > limit {
		signals = signals[:limit]
	}

	return signals, nil
}

// Analyze aggregates feedback signals for an agent.
func (s *Store) Analyze(agent string, since, until time.Time) (*Analysis, error) {
	signals, err := s.List(agent, 0)
	if err != nil {
		return nil, err
	}

	analysis := &Analysis{
		Agent:  agent,
		Since:  since,
		Until:  until,
		Period: fmt.Sprintf("%s to %s", since.Format("2006-01-02"), until.Format("2006-01-02")),
	}

	issueCounts := make(map[string]int)

	for _, sig := range signals {
		if sig.Timestamp.Before(since) || sig.Timestamp.After(until) {
			continue
		}

		analysis.TotalSignals++

		switch sig.Type {
		case SignalThumbsUp, SignalPraise:
			analysis.PositiveCount++
		case SignalThumbsDown, SignalBug, SignalTimeout:
			analysis.NegativeCount++
			issueCounts[string(sig.Type)]++
		case SignalCorrection:
			analysis.NegativeCount++
			issueCounts["correction"]++
		case SignalRating:
			if sig.Rating >= 4 {
				analysis.PositiveCount++
			} else if sig.Rating <= 2 {
				analysis.NegativeCount++
				issueCounts["low_rating"]++
			}
		}
	}

	if analysis.TotalSignals > 0 {
		analysis.SatisfactionRate = float64(analysis.PositiveCount) / float64(analysis.TotalSignals)
	}

	// Calculate average rating
	var totalRating int
	var ratingCount int
	for _, sig := range signals {
		if sig.Timestamp.Before(since) || sig.Timestamp.After(until) {
			continue
		}
		if sig.Type == SignalRating && sig.Rating > 0 {
			totalRating += sig.Rating
			ratingCount++
		}
	}
	if ratingCount > 0 {
		analysis.AvgRating = float64(totalRating) / float64(ratingCount)
	}

	// Common issues
	for issue, count := range issueCounts {
		analysis.CommonIssues = append(analysis.CommonIssues, IssueFrequency{
			Issue:   issue,
			Count:   count,
			Percent: float64(count) / float64(analysis.NegativeCount) * 100,
		})
	}
	sort.Slice(analysis.CommonIssues, func(i, k int) bool {
		return analysis.CommonIssues[i].Count > analysis.CommonIssues[k].Count
	})

	// Calculate trend (simple: compare first half vs second half satisfaction)
	analysis.TrendDirection = "stable"
	if analysis.TotalSignals >= 4 {
		midpoint := since.Add(until.Sub(since) / 2)
		var firstPositive, firstTotal, secondPositive, secondTotal int

		for _, sig := range signals {
			if sig.Timestamp.Before(since) || sig.Timestamp.After(until) {
				continue
			}
			if sig.Timestamp.Before(midpoint) {
				firstTotal++
				if sig.Type == SignalThumbsUp || sig.Type == SignalPraise || (sig.Type == SignalRating && sig.Rating >= 4) {
					firstPositive++
				}
			} else {
				secondTotal++
				if sig.Type == SignalThumbsUp || sig.Type == SignalPraise || (sig.Type == SignalRating && sig.Rating >= 4) {
					secondPositive++
				}
			}
		}

		var firstRate, secondRate float64
		if firstTotal > 0 {
			firstRate = float64(firstPositive) / float64(firstTotal)
		}
		if secondTotal > 0 {
			secondRate = float64(secondPositive) / float64(secondTotal)
		}

		analysis.TrendSlope = secondRate - firstRate
		if analysis.TrendSlope > 0.1 {
			analysis.TrendDirection = "improving"
		} else if analysis.TrendSlope < -0.1 {
			analysis.TrendDirection = "declining"
		}
	}

	return analysis, nil
}

// Delete removes a signal.
func (s *Store) Delete(id string) error {
	return os.Remove(filepath.Join(s.Dir, id+".json"))
}

// FormatAnalysis renders an analysis for display.
func FormatAnalysis(a *Analysis) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Feedback Analysis: %s\n", a.Agent))
	sb.WriteString(fmt.Sprintf("  Period:      %s\n", a.Period))
	sb.WriteString(fmt.Sprintf("  Signals:     %d (positive: %d, negative: %d)\n",
		a.TotalSignals, a.PositiveCount, a.NegativeCount))
	sb.WriteString(fmt.Sprintf("  Satisfaction: %.1f%%\n", a.SatisfactionRate*100))
	if a.AvgRating > 0 {
		sb.WriteString(fmt.Sprintf("  Avg Rating:  %.1f/5\n", a.AvgRating))
	}
	sb.WriteString(fmt.Sprintf("  Trend:       %s (slope: %.2f)\n", a.TrendDirection, a.TrendSlope))

	if len(a.CommonIssues) > 0 {
		sb.WriteString("\n  Top Issues:\n")
		for _, issue := range a.CommonIssues {
			sb.WriteString(fmt.Sprintf("    - %s: %d (%.0f%%)\n", issue.Issue, issue.Count, issue.Percent))
		}
	}

	return sb.String()
}

// FormatSignal renders a signal for display.
func FormatSignal(sig *Signal) string {
	icon := "●"
	switch sig.Type {
	case SignalThumbsUp, SignalPraise:
		icon = "👍"
	case SignalThumbsDown, SignalBug:
		icon = "👎"
	case SignalCorrection:
		icon = "✏️"
	case SignalRating:
		icon = fmt.Sprintf("⭐%d", sig.Rating)
	case SignalSuggestion:
		icon = "💡"
	case SignalSelfAssess:
		icon = "🪞"
	}

	ts := sig.Timestamp.Format("Jan 02 15:04")
	return fmt.Sprintf("%s [%s] %s — %s %s", icon, sig.Type, sig.Agent, ts, truncate(sig.Prompt, 50))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
