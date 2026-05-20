// Package bench provides performance benchmarking for Forge agents.
// Measure latency, throughput, token usage, and cost across models.
// Track benchmarks over time to catch regressions.
//
// Measure everything. Optimize what matters.
package bench

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Benchmark represents a performance benchmark.
type Benchmark struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Model      string    `json:"model"`
	Agent      string    `json:"agent"`
	Task       string    `json:"task"`
	Iterations int       `json:"iterations"`
	CreatedAt  time.Time `json:"created_at"`
}

// Measurement represents a single benchmark measurement.
type Measurement struct {
	ID          string    `json:"id"`
	BenchmarkID string    `json:"benchmark_id"`
	Iteration   int       `json:"iteration"`
	LatencyMS   int       `json:"latency_ms"`
	TokensIn    int       `json:"tokens_in"`
	TokensOut   int       `json:"tokens_out"`
	CostUSD     float64   `json:"cost_usd"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Summary represents benchmark summary statistics.
type Summary struct {
	BenchmarkID   string  `json:"benchmark_id"`
	TotalRuns     int     `json:"total_runs"`
	SuccessRate   float64 `json:"success_rate"`
	MeanLatencyMS float64 `json:"mean_latency_ms"`
	P50LatencyMS  float64 `json:"p50_latency_ms"`
	P95LatencyMS  float64 `json:"p95_latency_ms"`
	P99LatencyMS  float64 `json:"p99_latency_ms"`
	MinLatencyMS  int     `json:"min_latency_ms"`
	MaxLatencyMS  int     `json:"max_latency_ms"`
	MeanTokensIn  float64 `json:"mean_tokens_in"`
	MeanTokensOut float64 `json:"mean_tokens_out"`
	MeanCostUSD   float64 `json:"mean_cost_usd"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	ThroughputRPS float64 `json:"throughput_rps"` // requests per second
}

// Store manages benchmarks.
type Store struct {
	Dir string
}

// NewStore creates a benchmark store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// CreateBenchmark creates a new benchmark.
func (s *Store) CreateBenchmark(name, model, agent, task string, iterations int) (*Benchmark, error) {
	if err := os.MkdirAll(filepath.Join(s.Dir, "benchmarks"), 0755); err != nil {
		return nil, err
	}
	b := &Benchmark{
		ID:         fmt.Sprintf("bench-%d", time.Now().UnixNano()),
		Name:       name,
		Model:      model,
		Agent:      agent,
		Task:       task,
		Iterations: iterations,
		CreatedAt:  time.Now(),
	}
	if b.Iterations == 0 {
		b.Iterations = 10
	}
	if err := s.writeBenchmark(b); err != nil {
		return nil, err
	}
	return b, nil
}

// GetBenchmark retrieves a benchmark.
func (s *Store) GetBenchmark(id string) (*Benchmark, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, "benchmarks", id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("benchmark %q not found", id)
		}
		return nil, err
	}
	var b Benchmark
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// ListBenchmarks returns all benchmarks.
func (s *Store) ListBenchmarks() ([]*Benchmark, error) {
	dir := filepath.Join(s.Dir, "benchmarks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Benchmark
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		b, err := s.GetBenchmark(id)
		if err != nil {
			continue
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// DeleteBenchmark removes a benchmark and its measurements.
func (s *Store) DeleteBenchmark(id string) error {
	// Delete measurements
	measDir := filepath.Join(s.Dir, "measurements", id)
	os.RemoveAll(measDir)
	return os.Remove(filepath.Join(s.Dir, "benchmarks", id+".json"))
}

// RecordMeasurement records a benchmark measurement.
func (s *Store) RecordMeasurement(m Measurement) (*Measurement, error) {
	if err := os.MkdirAll(filepath.Join(s.Dir, "measurements", m.BenchmarkID), 0755); err != nil {
		return nil, err
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("meas-%d", time.Now().UnixNano())
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return &m, os.WriteFile(filepath.Join(s.Dir, "measurements", m.BenchmarkID, m.ID+".json"), data, 0644)
}

// GetMeasurements returns all measurements for a benchmark.
func (s *Store) GetMeasurements(benchmarkID string) ([]*Measurement, error) {
	dir := filepath.Join(s.Dir, "measurements", benchmarkID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Measurement
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m Measurement
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		out = append(out, &m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Iteration < out[j].Iteration })
	return out, nil
}

// Summarize calculates summary statistics for a benchmark.
func (s *Store) Summarize(benchmarkID string) (*Summary, error) {
	measurements, err := s.GetMeasurements(benchmarkID)
	if err != nil {
		return nil, err
	}
	if len(measurements) == 0 {
		return nil, fmt.Errorf("no measurements for benchmark %s", benchmarkID)
	}

	summary := &Summary{BenchmarkID: benchmarkID}

	var latencies []float64
	var totalCost float64
	var successes int
	var totalTokensIn, totalTokensOut int
	var totalLatency float64

	for _, m := range measurements {
		summary.TotalRuns++
		if m.Success {
			successes++
		}
		latencies = append(latencies, float64(m.LatencyMS))
		totalLatency += float64(m.LatencyMS)
		totalCost += m.CostUSD
		totalTokensIn += m.TokensIn
		totalTokensOut += m.TokensOut
	}

	summary.SuccessRate = float64(successes) / float64(summary.TotalRuns)
	summary.TotalCostUSD = totalCost
	summary.MeanCostUSD = totalCost / float64(summary.TotalRuns)
	summary.MeanTokensIn = float64(totalTokensIn) / float64(summary.TotalRuns)
	summary.MeanTokensOut = float64(totalTokensOut) / float64(summary.TotalRuns)

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		summary.MinLatencyMS = int(latencies[0])
		summary.MaxLatencyMS = int(latencies[len(latencies)-1])
		summary.MeanLatencyMS = totalLatency / float64(len(latencies))
		summary.P50LatencyMS = percentile(latencies, 50)
		summary.P95LatencyMS = percentile(latencies, 95)
		summary.P99LatencyMS = percentile(latencies, 99)

		if totalLatency > 0 {
			summary.ThroughputRPS = float64(summary.TotalRuns) / (totalLatency / 1000)
		}
	}

	return summary, nil
}

// Compare compares two benchmark summaries.
type Comparison struct {
	SummaryA *Summary
	SummaryB *Summary
	Delta    map[string]float64
	Better   string // "a", "b", or "tie"
}

// CompareSummaries compares two benchmark summaries.
func CompareSummaries(a, b *Summary) *Comparison {
	c := &Comparison{SummaryA: a, SummaryB: b}
	c.Delta = map[string]float64{
		"latency_mean":  b.MeanLatencyMS - a.MeanLatencyMS,
		"latency_p95":   b.P95LatencyMS - a.P95LatencyMS,
		"cost_mean":     b.MeanCostUSD - a.MeanCostUSD,
		"success_rate":  b.SuccessRate - a.SuccessRate,
		"throughput":    b.ThroughputRPS - a.ThroughputRPS,
	}

	// Simple scoring: lower latency/cost is better, higher success/throughput is better
	scoreA := a.SuccessRate + a.ThroughputRPS/100 - a.MeanLatencyMS/10000 - a.MeanCostUSD*10
	scoreB := b.SuccessRate + b.ThroughputRPS/100 - b.MeanLatencyMS/10000 - b.MeanCostUSD*10

	if scoreA > scoreB*1.05 {
		c.Better = "a"
	} else if scoreB > scoreA*1.05 {
		c.Better = "b"
	} else {
		c.Better = "tie"
	}

	return c
}

// FormatSummary renders a summary for display.
func FormatSummary(s *Summary) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Benchmark: %s\n", s.BenchmarkID))
	sb.WriteString(fmt.Sprintf("  Runs:          %d (success: %.1f%%)\n", s.TotalRuns, s.SuccessRate*100))
	sb.WriteString(fmt.Sprintf("  Latency (mean): %.1fms\n", s.MeanLatencyMS))
	sb.WriteString(fmt.Sprintf("  Latency (p50):  %.1fms\n", s.P50LatencyMS))
	sb.WriteString(fmt.Sprintf("  Latency (p95):  %.1fms\n", s.P95LatencyMS))
	sb.WriteString(fmt.Sprintf("  Latency (p99):  %.1fms\n", s.P99LatencyMS))
	sb.WriteString(fmt.Sprintf("  Latency range:  %d-%dms\n", s.MinLatencyMS, s.MaxLatencyMS))
	sb.WriteString(fmt.Sprintf("  Tokens in/out:  %.0f/%.0f\n", s.MeanTokensIn, s.MeanTokensOut))
	sb.WriteString(fmt.Sprintf("  Cost (mean):    $%.4f\n", s.MeanCostUSD))
	sb.WriteString(fmt.Sprintf("  Cost (total):   $%.4f\n", s.TotalCostUSD))
	sb.WriteString(fmt.Sprintf("  Throughput:     %.2f req/s\n", s.ThroughputRPS))
	return sb.String()
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower] + frac*(sorted[upper]-sorted[lower])
}

func (s *Store) writeBenchmark(b *Benchmark) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, "benchmarks", b.ID+".json"), data, 0644)
}
