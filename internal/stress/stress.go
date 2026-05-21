// Package stress provides agent load and stress testing.
// Simulate concurrent agent sessions, measure throughput, latency,
// error rates, and resource consumption under load.
//
// Know your limits before your users do.
package stress

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TestType classifies the type of stress test.
type TestType string

const (
	TypeRampUp     TestType = "ramp-up"     // gradually increase load
	TypeSustained  TestType = "sustained"   // constant load
	TypeSpike      TestType = "spike"       // sudden load spike
	TypeWave       TestType = "wave"        // sine wave pattern
	TypeCustom     TestType = "custom"      // user-defined pattern
)

// SessionResult records the result of a single simulated session.
type SessionResult struct {
	ID        string        `json:"id"`
	AgentID   string        `json:"agent_id"`
	Duration  time.Duration `json:"duration"`
	TokensIn  int           `json:"tokens_in"`
	TokensOut int           `json:"tokens_out"`
	Cost      float64       `json:"cost"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
}

// TestConfig configures a stress test.
type TestConfig struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        TestType      `json:"type"`
	AgentIDs    []string      `json:"agent_ids"`
	Prompt      string        `json:"prompt"`
	Concurrency int           `json:"concurrency"`
	Duration    time.Duration `json:"duration"`
	RampUpTime  time.Duration `json:"ramp_up_time"`
	MaxRPS      float64       `json:"max_rps"`    // requests per second
	TokensInAvg int           `json:"tokens_in_avg"`
	TokensOutAvg int          `json:"tokens_out_avg"`
	CostPerToken float64      `json:"cost_per_token"`
	ErrorRate   float64       `json:"error_rate"` // simulated error rate 0-1
}

// TestReport is the result of a completed stress test.
type TestReport struct {
	ConfigID      string          `json:"config_id"`
	ConfigName    string          `json:"config_name"`
	Type          TestType        `json:"type"`
	TotalSessions int             `json:"total_sessions"`
	SuccessCount  int             `json:"success_count"`
	FailureCount  int             `json:"failure_count"`
	ErrorRate     float64         `json:"error_rate"`
	TotalTokensIn  int64          `json:"total_tokens_in"`
	TotalTokensOut int64          `json:"total_tokens_out"`
	TotalCost     float64         `json:"total_cost"`
	AvgLatency    time.Duration   `json:"avg_latency"`
	P50Latency    time.Duration   `json:"p50_latency"`
	P90Latency    time.Duration   `json:"p90_latency"`
	P99Latency    time.Duration   `json:"p99_latency"`
	MaxLatency    time.Duration   `json:"max_latency"`
	MinLatency    time.Duration   `json:"min_latency"`
	ThroughputRPS float64         `json:"throughput_rps"`
	PeakRPS       float64         `json:"peak_rps"`
	StartedAt     time.Time       `json:"started_at"`
	EndedAt       time.Time       `json:"ended_at"`
	WallDuration  time.Duration   `json:"wall_duration"`
	Sessions      []SessionResult `json:"sessions,omitempty"`
}

// Runner executes stress tests.
type Runner struct {
	dir     string
	configs map[string]*TestConfig
	reports map[string]*TestReport
	mu      sync.RWMutex
}

// NewRunner creates a new stress test runner.
func NewRunner(dir string) *Runner {
	os.MkdirAll(dir, 0755)
	r := &Runner{
		dir:     dir,
		configs: make(map[string]*TestConfig),
		reports: make(map[string]*TestReport),
	}
	r.load()
	return r
}

// CreateConfig creates a new stress test configuration.
func (r *Runner) CreateConfig(name string, testType TestType) *TestConfig {
	r.mu.Lock()
	defer r.mu.Unlock()

	c := &TestConfig{
		ID:       fmt.Sprintf("stress-cfg-%d", time.Now().UnixNano()),
		Name:     name,
		Type:     testType,
		AgentIDs: []string{"default"},
	}
	r.configs[c.ID] = c
	r.save()
	return c
}

// GetConfig returns a config by ID.
func (r *Runner) GetConfig(id string) (*TestConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.configs[id]
	if !ok {
		return nil, false
	}
	copy := *c
	return &copy, true
}

// ListConfigs returns all test configurations.
func (r *Runner) ListConfigs() []TestConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]TestConfig, 0, len(r.configs))
	for _, c := range r.configs {
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// UpdateConfig updates a test configuration.
func (r *Runner) UpdateConfig(id string, fn func(*TestConfig)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.configs[id]
	if !ok {
		return fmt.Errorf("config %q not found", id)
	}
	fn(c)
	r.save()
	return nil
}

// DeleteConfig removes a configuration.
func (r *Runner) DeleteConfig(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.configs[id]; !ok {
		return fmt.Errorf("config %q not found", id)
	}
	delete(r.configs, id)
	r.save()
	return nil
}

// Run executes a stress test based on the given configuration.
// This is a simulation — it doesn't call real agents but models
// realistic timing and resource usage.
func (r *Runner) Run(configID string) (*TestReport, error) {
	r.mu.RLock()
	cfg, ok := r.configs[configID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("config %q not found", configID)
	}

	report := &TestReport{
		ConfigID:   cfg.ID,
		ConfigName: cfg.Name,
		Type:       cfg.Type,
		StartedAt:  time.Now(),
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	duration := cfg.Duration
	if duration <= 0 {
		duration = 30 * time.Second
	}

	tokensInAvg := cfg.TokensInAvg
	if tokensInAvg <= 0 {
		tokensInAvg = 500
	}

	tokensOutAvg := cfg.TokensOutAvg
	if tokensOutAvg <= 0 {
		tokensOutAvg = 200
	}

	costPerToken := cfg.CostPerToken
	if costPerToken <= 0 {
		costPerToken = 0.00003 // ~$0.03 per 1K tokens
	}

	simErrorRate := cfg.ErrorRate
	if simErrorRate <= 0 {
		simErrorRate = 0.02 // 2% error rate
	}

	var sessions []SessionResult
	var mu sync.Mutex
	var successCount, failureCount int64
	var totalTokensIn, totalTokensOut int64
	var totalCost float64

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	sessionCounter := int64(0)

	// Generate load pattern
	startTime := time.Now()
	deadline := startTime.Add(duration)

	for time.Now().Before(deadline) {
		sem <- struct{}{}
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			sid := fmt.Sprintf("session-%d", atomic.AddInt64(&sessionCounter, 1))

			// Simulate latency based on type
			latency := simulateLatency(tokensInAvg, tokensOutAvg, cfg.Type, startTime)

			// Simulate success/failure
			isSuccess := simulateSuccess(simErrorRate)

			tIn := tokensInAvg + randomJitter(tokensInAvg/5)
			tOut := tokensOutAvg + randomJitter(tokensOutAvg/5)
			cost := float64(tIn+tOut) * costPerToken

			sess := SessionResult{
				ID:        sid,
				Duration:  latency,
				TokensIn:  tIn,
				TokensOut: tOut,
				Cost:      cost,
				Success:   isSuccess,
				StartedAt: time.Now(),
				EndedAt:   time.Now().Add(latency),
			}

			if !isSuccess {
				sess.Error = "simulated error"
			}

			mu.Lock()
			sessions = append(sessions, sess)
			if isSuccess {
				successCount++
			} else {
				failureCount++
			}
			totalTokensIn += int64(tIn)
			totalTokensOut += int64(tOut)
			totalCost += cost
			mu.Unlock()

			// Simulate processing time
			time.Sleep(latency)
		}()

		// Rate control
		if cfg.MaxRPS > 0 {
			time.Sleep(time.Duration(float64(time.Second) / cfg.MaxRPS))
		} else {
			time.Sleep(time.Duration(1000/concurrency) * time.Millisecond)
		}
	}

	wg.Wait()
	report.EndedAt = time.Now()
	report.WallDuration = report.EndedAt.Sub(report.StartedAt)
	report.Sessions = sessions
	report.TotalSessions = len(sessions)
	report.SuccessCount = int(successCount)
	report.FailureCount = int(failureCount)
	if report.TotalSessions > 0 {
		report.ErrorRate = float64(report.FailureCount) / float64(report.TotalSessions)
	}
	report.TotalTokensIn = totalTokensIn
	report.TotalTokensOut = totalTokensOut
	report.TotalCost = totalCost

	// Compute latency percentiles
	if len(sessions) > 0 {
		durations := make([]time.Duration, len(sessions))
		for i, s := range sessions {
			durations[i] = s.Duration
		}
		sort.Slice(durations, func(i, j int) bool {
			return durations[i] < durations[j]
		})

		var totalLatency time.Duration
		for _, d := range durations {
			totalLatency += d
		}
		report.AvgLatency = totalLatency / time.Duration(len(durations))
		report.MinLatency = durations[0]
		report.MaxLatency = durations[len(durations)-1]
		report.P50Latency = durations[len(durations)*50/100]
		report.P90Latency = durations[len(durations)*90/100]
		if len(durations) > 1 {
			report.P99Latency = durations[len(durations)*99/100]
		}
	}

	report.ThroughputRPS = float64(report.TotalSessions) / report.WallDuration.Seconds()
	report.PeakRPS = report.ThroughputRPS // simplified

	r.mu.Lock()
	r.reports[configID] = report
	r.saveReports()
	r.mu.Unlock()

	return report, nil
}

// GetReport returns a test report.
func (r *Runner) GetReport(configID string) (*TestReport, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rep, ok := r.reports[configID]
	if !ok {
		return nil, false
	}
	copy := *rep
	return &copy, true
}

// ListReports returns all test reports.
func (r *Runner) ListReports() []TestReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]TestReport, 0, len(r.reports))
	for _, rep := range r.reports {
		result = append(result, *rep)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	return result
}

// RenderReport renders a test report for display.
func RenderReport(report *TestReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Stress Test Report: %s\n", report.ConfigName)
	fmt.Fprintf(&b, "Type: %s\n", report.Type)
	fmt.Fprintf(&b, "Duration: %s\n", report.WallDuration.Round(time.Millisecond))
	fmt.Fprintf(&b, "\nSessions:\n")
	fmt.Fprintf(&b, "  Total: %d\n", report.TotalSessions)
	fmt.Fprintf(&b, "  Success: %d\n", report.SuccessCount)
	fmt.Fprintf(&b, "  Failed: %d\n", report.FailureCount)
	fmt.Fprintf(&b, "  Error Rate: %.2f%%\n", report.ErrorRate*100)

	fmt.Fprintf(&b, "\nLatency:\n")
	fmt.Fprintf(&b, "  Min: %s\n", report.MinLatency.Round(time.Microsecond))
	fmt.Fprintf(&b, "  Avg: %s\n", report.AvgLatency.Round(time.Microsecond))
	fmt.Fprintf(&b, "  P50: %s\n", report.P50Latency.Round(time.Microsecond))
	fmt.Fprintf(&b, "  P90: %s\n", report.P90Latency.Round(time.Microsecond))
	fmt.Fprintf(&b, "  P99: %s\n", report.P99Latency.Round(time.Microsecond))
	fmt.Fprintf(&b, "  Max: %s\n", report.MaxLatency.Round(time.Microsecond))

	fmt.Fprintf(&b, "\nThroughput:\n")
	fmt.Fprintf(&b, "  RPS: %.1f\n", report.ThroughputRPS)
	fmt.Fprintf(&b, "  Peak: %.1f\n", report.PeakRPS)

	fmt.Fprintf(&b, "\nResources:\n")
	fmt.Fprintf(&b, "  Tokens In: %d\n", report.TotalTokensIn)
	fmt.Fprintf(&b, "  Tokens Out: %d\n", report.TotalTokensOut)
	fmt.Fprintf(&b, "  Cost: $%.4f\n", report.TotalCost)

	return b.String()
}

// RenderConfig renders a test config for display.
func RenderConfig(cfg *TestConfig) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Stress Test Config: %s\n", cfg.Name)
	fmt.Fprintf(&b, "ID: %s\n", cfg.ID)
	fmt.Fprintf(&b, "Type: %s\n", cfg.Type)
	fmt.Fprintf(&b, "Concurrency: %d\n", cfg.Concurrency)
	fmt.Fprintf(&b, "Duration: %s\n", cfg.Duration)
	fmt.Fprintf(&b, "Ramp Up: %s\n", cfg.RampUpTime)
	fmt.Fprintf(&b, "Max RPS: %.1f\n", cfg.MaxRPS)
	fmt.Fprintf(&b, "Error Rate: %.2f%%\n", cfg.ErrorRate*100)
	fmt.Fprintf(&b, "Tokens In (avg): %d\n", cfg.TokensInAvg)
	fmt.Fprintf(&b, "Tokens Out (avg): %d\n", cfg.TokensOutAvg)
	fmt.Fprintf(&b, "Cost/Token: $%.6f\n", cfg.CostPerToken)

	return b.String()
}

// Simulation helpers

func simulateLatency(tokensIn, tokensOut int, testType TestType, startTime time.Time) time.Duration {
	baseLatency := time.Duration(tokensIn+tokensOut) * time.Microsecond * 10

	switch testType {
	case TypeRampUp:
		elapsed := time.Since(startTime)
		factor := 1.0 + float64(elapsed.Seconds())*0.1
		baseLatency = time.Duration(float64(baseLatency) * factor)
	case TypeSpike:
		if time.Since(startTime).Seconds() > 5 {
			baseLatency = time.Duration(float64(baseLatency) * 2.5)
		}
	case TypeWave:
		wave := float64(time.Since(startTime).Seconds()) * 0.5
		factor := 1.0 + 0.5*wave
		baseLatency = time.Duration(float64(baseLatency) * factor)
	}

	// Add jitter
	jitter := time.Duration(randomJitter(100)) * time.Microsecond
	return baseLatency + jitter
}

func simulateSuccess(errorRate float64) bool {
	return randomFloat() > errorRate
}

func randomJitter(max int) int {
	if max <= 0 {
		return 0
	}
	return int(randomFloat() * float64(max))
}

func randomFloat() float64 {
	// Simple deterministic pseudo-random based on time
	n := time.Now().UnixNano()
	// xorshift
	n ^= n << 13
	n ^= n >> 7
	n ^= n << 17
	if n < 0 {
		n = -n
	}
	return float64(n%10000) / 10000.0
}

func (r *Runner) save() {
	if r.dir == "" {
		return
	}
	os.MkdirAll(r.dir, 0755)
	data, _ := json.MarshalIndent(r.configs, "", "  ")
	os.WriteFile(filepath.Join(r.dir, "configs.json"), data, 0644)
}

func (r *Runner) saveReports() {
	if r.dir == "" {
		return
	}
	os.MkdirAll(r.dir, 0755)
	data, _ := json.MarshalIndent(r.reports, "", "  ")
	os.WriteFile(filepath.Join(r.dir, "reports.json"), data, 0644)
}

func (r *Runner) load() {
	if r.dir == "" {
		return
	}

	data, err := os.ReadFile(filepath.Join(r.dir, "configs.json"))
	if err == nil {
		json.Unmarshal(data, &r.configs)
	}

	data, err = os.ReadFile(filepath.Join(r.dir, "reports.json"))
	if err == nil {
		json.Unmarshal(data, &r.reports)
	}
}
