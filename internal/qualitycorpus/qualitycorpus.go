// Package qualitycorpus provides opt-in data collection for agent improvement.
// Collects anonymized task outcomes for forge tune/breed.
//
// Better data, better agents.
package qualitycorpus

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

// Outcome is a recorded task outcome.
type Outcome struct {
	ID           string            `json:"id"`
	AgentID      string            `json:"agent_id"`
	TaskType     string            `json:"task_type"`
	Model        string            `json:"model"`
	Success      bool              `json:"success"`
	QualityScore float64           `json:"quality_score"` // 0-1
	Duration     time.Duration     `json:"duration"`
	TokensUsed   int64             `json:"tokens_used"`
	Cost         float64           `json:"cost"`
	Tags         []string          `json:"tags"`
	Feedback     string            `json:"feedback,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	OptIn        bool              `json:"opt_in"`
	Anonymized   bool              `json:"anonymized"`
	Timestamp    time.Time         `json:"timestamp"`
}

// Metric is an aggregated metric.
type Metric struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Count  int     `json:"count"`
	Period string  `json:"period"` // daily, weekly, all
}

// Corpus manages the quality corpus.
type Corpus struct {
	outcomes []Outcome
	storeDir string
	nextID   int
	optIn    bool
	mu       sync.RWMutex
}

// NewCorpus creates a quality corpus.
func NewCorpus(storeDir string, optIn bool) *Corpus {
	c := &Corpus{
		outcomes: make([]Outcome, 0),
		storeDir: storeDir,
		optIn:    optIn,
	}
	c.load()
	return c
}

// Record records a task outcome (only if opted in).
func (c *Corpus) Record(outcome Outcome) (*Outcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.optIn {
		return nil, fmt.Errorf("data collection not opted in")
	}

	c.nextID++
	outcome.ID = fmt.Sprintf("out-%d", c.nextID)
	if outcome.Timestamp.IsZero() {
		outcome.Timestamp = time.Now()
	}
	outcome.OptIn = true
	outcome.Anonymized = true // Always anonymize

	// Strip identifying info
	outcome.AgentID = anonymize(outcome.AgentID)
	outcome.Feedback = "" // Don't store free-text feedback

	c.outcomes = append(c.outcomes, outcome)
	if len(c.outcomes) > 10000 {
		c.outcomes = c.outcomes[len(c.outcomes)-10000:]
	}
	c.save()
	return &outcome, nil
}

// IsOptedIn returns whether data collection is enabled.
func (c *Corpus) IsOptedIn() bool {
	return c.optIn
}

// SetOptIn enables or disables data collection.
func (c *Corpus) SetOptIn(enabled bool) {
	c.mu.Lock()
	c.optIn = enabled
	c.mu.Unlock()
}

// Metrics returns aggregated metrics.
func (c *Corpus) Metrics() []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.outcomes) == 0 {
		return nil
	}

	var metrics []Metric

	// Success rate
	successCount := 0
	for _, o := range c.outcomes {
		if o.Success {
			successCount++
		}
	}
	metrics = append(metrics, Metric{
		Name:   "success_rate",
		Value:  float64(successCount) / float64(len(c.outcomes)),
		Count:  len(c.outcomes),
		Period: "all",
	})

	// Average quality
	var totalQuality float64
	for _, o := range c.outcomes {
		totalQuality += o.QualityScore
	}
	metrics = append(metrics, Metric{
		Name:   "avg_quality",
		Value:  totalQuality / float64(len(c.outcomes)),
		Count:  len(c.outcomes),
		Period: "all",
	})

	// Average duration
	var totalDuration float64
	for _, o := range c.outcomes {
		totalDuration += o.Duration.Seconds()
	}
	metrics = append(metrics, Metric{
		Name:   "avg_duration_sec",
		Value:  totalDuration / float64(len(c.outcomes)),
		Count:  len(c.outcomes),
		Period: "all",
	})

	// Total tokens
	var totalTokens int64
	for _, o := range c.outcomes {
		totalTokens += o.TokensUsed
	}
	metrics = append(metrics, Metric{
		Name:   "total_tokens",
		Value:  float64(totalTokens),
		Count:  len(c.outcomes),
		Period: "all",
	})

	// Total cost
	var totalCost float64
	for _, o := range c.outcomes {
		totalCost += o.Cost
	}
	metrics = append(metrics, Metric{
		Name:   "total_cost",
		Value:  totalCost,
		Count:  len(c.outcomes),
		Period: "all",
	})

	return metrics
}

// MetricsByModel returns metrics grouped by model.
func (c *Corpus) MetricsByModel() map[string]Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	grouped := make(map[string][]Outcome)
	for _, o := range c.outcomes {
		grouped[o.Model] = append(grouped[o.Model], o)
	}

	result := make(map[string]Metric)
	for model, outcomes := range grouped {
		success := 0
		var quality float64
		for _, o := range outcomes {
			if o.Success {
				success++
			}
			quality += o.QualityScore
		}
		result[model] = Metric{
			Name:   model,
			Value:  float64(success) / float64(len(outcomes)),
			Count:  len(outcomes),
			Period: "all",
		}
	}
	return result
}

// MetricsByTaskType returns metrics grouped by task type.
func (c *Corpus) MetricsByTaskType() map[string]Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	grouped := make(map[string][]Outcome)
	for _, o := range c.outcomes {
		grouped[o.TaskType] = append(grouped[o.TaskType], o)
	}

	result := make(map[string]Metric)
	for taskType, outcomes := range grouped {
		success := 0
		for _, o := range outcomes {
			if o.Success {
				success++
			}
		}
		result[taskType] = Metric{
			Name:   taskType,
			Value:  float64(success) / float64(len(outcomes)),
			Count:  len(outcomes),
			Period: "all",
		}
	}
	return result
}

// Recent returns the N most recent outcomes.
func (c *Corpus) Recent(limit int) []Outcome {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit > len(c.outcomes) {
		limit = len(c.outcomes)
	}
	result := make([]Outcome, limit)
	copy(result, c.outcomes[len(c.outcomes)-limit:])
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	return result
}

// Export exports the corpus as JSON (anonymized).
func (c *Corpus) Export() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return json.MarshalIndent(c.outcomes, "", "  ")
}

// Count returns total recorded outcomes.
func (c *Corpus) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.outcomes)
}

// Clear removes all collected data.
func (c *Corpus) Clear() {
	c.mu.Lock()
	c.outcomes = c.outcomes[:0]
	c.mu.Unlock()
	c.save()
}

func anonymize(id string) string {
	if len(id) < 4 {
		return "****"
	}
	return id[:2] + strings.Repeat("*", len(id)-2)
}

func (c *Corpus) save() {
	if c.storeDir == "" {
		return
	}
	os.MkdirAll(c.storeDir, 0755)
	data, _ := json.MarshalIndent(c.outcomes, "", "  ")
	os.WriteFile(filepath.Join(c.storeDir, "corpus.json"), data, 0644)
}

func (c *Corpus) load() {
	if c.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(c.storeDir, "corpus.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &c.outcomes)
	c.nextID = len(c.outcomes)
}

// FormatMetric formats a metric for display.
func FormatMetric(m Metric) string {
	return fmt.Sprintf("%-25s %.2f (%d samples, %s)", m.Name, m.Value, m.Count, m.Period)
}
