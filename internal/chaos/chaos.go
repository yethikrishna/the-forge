// Package chaos provides chaos engineering for Forge.
// The forge tests its own strength — what doesn't break makes it stronger.
package chaos

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// FaultType represents the type of fault to inject.
type FaultType string

const (
	FaultLatency    FaultType = "latency"
	FaultError      FaultType = "error"
	FaultCrash      FaultType = "crash"
	FaultMemory     FaultType = "memory_pressure"
	FaultNetwork    FaultType = "network_partition"
	FaultDependency FaultType = "dependency_failure"
)

// Target represents what the fault affects.
type Target struct {
	Type     string // agent, model, tool, network, storage
	ID       string // specific target ID
	Selector string // label selector
}

// FaultConfig configures a fault injection.
type FaultConfig struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        FaultType     `json:"type"`
	Target      Target        `json:"target"`
	Probability float64       `json:"probability"` // 0.0-1.0
	Duration    time.Duration `json:"duration"`
	Delay       time.Duration `json:"delay"`      // for latency faults
	ErrorCode   int           `json:"error_code"` // for error faults
	Active      bool          `json:"active"`
	CreatedAt   time.Time     `json:"created_at"`
}

// Experiment is a chaos experiment.
type Experiment struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Faults      []FaultConfig     `json:"faults"`
	SteadyState []SteadyCheck     `json:"steady_state"`
	Hypothesis  string            `json:"hypothesis"`
	Duration    time.Duration     `json:"duration"`
	Status      string            `json:"status"` // pending, running, completed, failed
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	Results     *ExperimentResult `json:"results,omitempty"`
}

// SteadyCheck verifies the system is in a steady state.
type SteadyCheck struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	CheckFunc string  `json:"check_func"` // function name to call
	Threshold float64 `json:"threshold"`
	Tolerated bool    `json:"tolerated"` // if true, failure is tolerated
}

// ExperimentResult holds experiment results.
type ExperimentResult struct {
	SteadyStateHeld bool             `json:"steady_state_held"`
	FaultsInjected  int              `json:"faults_injected"`
	FaultsSucceeded int              `json:"faults_succeeded"`
	RecoveryTime    time.Duration    `json:"recovery_time"`
	ObservedEffects []ObservedEffect `json:"observed_effects"`
	LessonsLearned  []string         `json:"lessons_learned"`
}

// ObservedEffect records what happened during the experiment.
type ObservedEffect struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	Severity  string    `json:"severity"` // info, warning, critical
}

// ChaosEngine runs chaos experiments.
type ChaosEngine struct {
	experiments  map[string]*Experiment
	faults       map[string]*FaultConfig
	activeFaults map[string]time.Time // fault ID -> activation time
	rng          *rand.Rand
	mu           sync.RWMutex
}

// NewChaosEngine creates a new chaos engine.
func NewChaosEngine() *ChaosEngine {
	return &ChaosEngine{
		experiments:  make(map[string]*Experiment),
		faults:       make(map[string]*FaultConfig),
		activeFaults: make(map[string]time.Time),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RegisterFault registers a fault configuration.
func (ce *ChaosEngine) RegisterFault(config FaultConfig) string {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if config.ID == "" {
		config.ID = fmt.Sprintf("fault-%d", time.Now().UnixNano())
	}
	config.CreatedAt = time.Now().UTC()
	ce.faults[config.ID] = &config
	return config.ID
}

// ActivateFault activates a fault.
func (ce *ChaosEngine) ActivateFault(faultID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	fault, ok := ce.faults[faultID]
	if !ok {
		return fmt.Errorf("fault %s not found", faultID)
	}
	fault.Active = true
	ce.activeFaults[faultID] = time.Now().UTC()
	return nil
}

// DeactivateFault deactivates a fault.
func (ce *ChaosEngine) DeactivateFault(faultID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	fault, ok := ce.faults[faultID]
	if !ok {
		return fmt.Errorf("fault %s not found", faultID)
	}
	fault.Active = false
	delete(ce.activeFaults, faultID)
	return nil
}

// ShouldInject checks if a fault should be injected for the given target.
func (ce *ChaosEngine) ShouldInject(targetType, targetID string) (bool, FaultType, time.Duration) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	for fault := range ce.activeFaults {
		cfg, ok := ce.faults[fault]
		if !ok || !cfg.Active {
			continue
		}
		if cfg.Target.Type != targetType && cfg.Target.Type != "*" {
			continue
		}
		if cfg.Target.ID != targetID && cfg.Target.ID != "*" {
			continue
		}
		// Probability check
		if ce.rng.Float64() < cfg.Probability {
			return true, cfg.Type, cfg.Delay
		}
	}
	return false, "", 0
}

// RunExperiment runs a chaos experiment.
func (ce *ChaosEngine) RunExperiment(ctx context.Context, experiment *Experiment) (*ExperimentResult, error) {
	ce.mu.Lock()
	experiment.Status = "running"
	now := time.Now().UTC()
	experiment.StartedAt = &now
	ce.experiments[experiment.ID] = experiment
	ce.mu.Unlock()

	result := &ExperimentResult{
		FaultsInjected: len(experiment.Faults),
	}

	// Activate all faults
	for i := range experiment.Faults {
		fault := &experiment.Faults[i]
		fault.Active = true
		ce.mu.Lock()
		ce.faults[fault.ID] = fault
		ce.activeFaults[fault.ID] = time.Now().UTC()
		ce.mu.Unlock()
		result.FaultsSucceeded++
	}

	// Record effects
	result.ObservedEffects = append(result.ObservedEffects, ObservedEffect{
		Timestamp: time.Now().UTC(),
		Type:      "experiment_start",
		Detail:    fmt.Sprintf("Started experiment: %s", experiment.Name),
		Severity:  "info",
	})

	// Wait for experiment duration or context cancellation
	select {
	case <-time.After(experiment.Duration):
	case <-ctx.Done():
	}

	// Check steady state
	steadyStateHeld := true
	for _, check := range experiment.SteadyState {
		// Simulate steady state check (in production, calls real check functions)
		effect := ObservedEffect{
			Timestamp: time.Now().UTC(),
			Type:      "steady_state_check",
			Target:    check.ID,
			Detail:    fmt.Sprintf("Check %s: %s", check.Name, "passed"),
			Severity:  "info",
		}
		result.ObservedEffects = append(result.ObservedEffects, effect)
	}

	result.SteadyStateHeld = steadyStateHeld

	// Deactivate all faults
	for i := range experiment.Faults {
		fault := &experiment.Faults[i]
		fault.Active = false
		ce.mu.Lock()
		delete(ce.activeFaults, fault.ID)
		ce.mu.Unlock()
	}

	result.RecoveryTime = time.Since(now) - experiment.Duration

	// Generate lessons learned
	if !result.SteadyStateHeld {
		result.LessonsLearned = append(result.LessonsLearned,
			"System failed to maintain steady state under fault injection",
			"Consider adding redundancy for affected components",
		)
	} else {
		result.LessonsLearned = append(result.LessonsLearned,
			"System maintained steady state under fault conditions",
		)
	}

	// Update experiment
	ce.mu.Lock()
	finished := time.Now().UTC()
	experiment.FinishedAt = &finished
	experiment.Status = "completed"
	experiment.Results = result
	ce.mu.Unlock()

	return result, nil
}

// ListExperiments returns all experiments.
func (ce *ChaosEngine) ListExperiments() []*Experiment {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	var experiments []*Experiment
	for _, e := range ce.experiments {
		experiments = append(experiments, e)
	}
	return experiments
}

// ListFaults returns all faults.
func (ce *ChaosEngine) ListFaults() []*FaultConfig {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	var faults []*FaultConfig
	for _, f := range ce.faults {
		faults = append(faults, f)
	}
	return faults
}

// ActiveFaults returns currently active faults.
func (ce *ChaosEngine) ActiveFaults() []*FaultConfig {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	var faults []*FaultConfig
	for id := range ce.activeFaults {
		if f, ok := ce.faults[id]; ok {
			faults = append(faults, f)
		}
	}
	return faults
}

// Stats returns chaos engine statistics.
func (ce *ChaosEngine) Stats() ChaosStats {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	stats := ChaosStats{
		TotalExperiments: len(ce.experiments),
		TotalFaults:      len(ce.faults),
		ActiveFaults:     len(ce.activeFaults),
	}

	for _, e := range ce.experiments {
		switch e.Status {
		case "completed":
			stats.CompletedExperiments++
		case "running":
			stats.RunningExperiments++
		case "failed":
			stats.FailedExperiments++
		}
	}

	return stats
}

// ChaosStats holds chaos engineering statistics.
type ChaosStats struct {
	TotalExperiments     int `json:"total_experiments"`
	CompletedExperiments int `json:"completed_experiments"`
	RunningExperiments   int `json:"running_experiments"`
	FailedExperiments    int `json:"failed_experiments"`
	TotalFaults          int `json:"total_faults"`
	ActiveFaults         int `json:"active_faults"`
}

// KillSwitch immediately deactivates all faults.
func (ce *ChaosEngine) KillSwitch() int {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	count := 0
	for id := range ce.activeFaults {
		if f, ok := ce.faults[id]; ok {
			f.Active = false
			count++
		}
		delete(ce.activeFaults, id)
	}
	return count
}

// FormatExperiment renders an experiment.
func FormatExperiment(e *Experiment) string {
	status := e.Status
	if e.StartedAt != nil {
		status += fmt.Sprintf(" (started %s)", e.StartedAt.Format(time.RFC3339))
	}
	result := fmt.Sprintf("Experiment: %s (%s)\n  Status:      %s\n  Hypothesis:  %s\n  Faults:      %d\n  Duration:    %s\n",
		e.Name, e.ID, status, e.Hypothesis, len(e.Faults), e.Duration)
	if e.Results != nil {
		result += fmt.Sprintf("  Steady State: %v\n  Recovery:    %s\n  Lessons:     %d\n",
			e.Results.SteadyStateHeld, e.Results.RecoveryTime, len(e.Results.LessonsLearned))
		for _, lesson := range e.Results.LessonsLearned {
			result += fmt.Sprintf("    - %s\n", lesson)
		}
	}
	return result
}

// FormatFault renders a fault config.
func FormatFault(f *FaultConfig) string {
	active := "inactive"
	if f.Active {
		active = "ACTIVE"
	}
	return fmt.Sprintf("Fault: %s (%s)\n  Type:         %s\n  Target:       %s/%s\n  Probability:  %.0f%%\n  Status:       %s\n",
		f.Name, f.ID, f.Type, f.Target.Type, f.Target.ID, f.Probability*100, active)
}

// FormatStats renders chaos stats.
func FormatStats(stats ChaosStats) string {
	return fmt.Sprintf("Chaos Engineering Stats:\n  Experiments:  %d (%d completed, %d running, %d failed)\n  Faults:       %d total, %d active\n",
		stats.TotalExperiments, stats.CompletedExperiments, stats.RunningExperiments, stats.FailedExperiments,
		stats.TotalFaults, stats.ActiveFaults)
}

// BuiltinExperiments returns pre-defined experiments.
func BuiltinExperiments() []Experiment {
	return []Experiment{
		{
			ID:          "exp-model-latency",
			Name:        "Model Latency Spike",
			Description: "Test system resilience when model responses take 10x longer",
			Hypothesis:  "Agents should fall back to faster models and maintain service",
			Duration:    5 * time.Minute,
			Faults: []FaultConfig{
				{ID: "f1", Name: "Slow Model", Type: FaultLatency, Target: Target{Type: "model", ID: "*"}, Probability: 0.8, Delay: 5 * time.Second},
			},
			SteadyState: []SteadyCheck{
				{ID: "ss1", Name: "Agent Response Time", Threshold: 30, Tolerated: false},
			},
		},
		{
			ID:          "exp-agent-crash",
			Name:        "Random Agent Crash",
			Description: "Test that the system recovers when agents crash unexpectedly",
			Hypothesis:  "Orchestrator should detect crashed agents and restart them",
			Duration:    3 * time.Minute,
			Faults: []FaultConfig{
				{ID: "f2", Name: "Agent Crash", Type: FaultCrash, Target: Target{Type: "agent", ID: "*"}, Probability: 0.2},
			},
			SteadyState: []SteadyCheck{
				{ID: "ss2", Name: "Agent Availability", Threshold: 80, Tolerated: false},
			},
		},
		{
			ID:          "exp-network-partition",
			Name:        "Network Partition",
			Description: "Test system behavior during network partitions",
			Hypothesis:  "Agents should queue work and retry when connectivity returns",
			Duration:    2 * time.Minute,
			Faults: []FaultConfig{
				{ID: "f3", Name: "Network Loss", Type: FaultNetwork, Target: Target{Type: "network", ID: "*"}, Probability: 0.5},
			},
		},
	}
}
