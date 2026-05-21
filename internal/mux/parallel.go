// Package mux runs multiple agents in parallel with split-pane coordination.
// Each agent gets its own workspace, and results are collected, deduplicated,
// and ranked before being returned.
package mux

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// AgentSpec defines an agent to run in parallel.
type AgentSpec struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Model   string            `json:"model,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// AgentResult is the output from one parallel agent.
type AgentResult struct {
	AgentName string        `json:"agent_name"`
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Tokens    int           `json:"tokens,omitempty"`
	Score     float64       `json:"score,omitempty"`
}

// ParallelConfig configures a parallel mux run.
type ParallelConfig struct {
	Agents      []AgentSpec `json:"agents"`
	Prompt      string      `json:"prompt"`
	MaxDuration time.Duration `json:"max_duration"`
	MinAgents   int           `json:"min_agents"`  // min agents to succeed
	RankBy      string        `json:"rank_by"`     // "score", "speed", "tokens"
}

// ParallelResult is the aggregate output of a parallel run.
type ParallelResult struct {
	Results    []AgentResult `json:"results"`
	Best       *AgentResult  `json:"best,omitempty"`
	Succeeded  int           `json:"succeeded"`
	Failed     int           `json:"failed"`
	TotalTime  time.Duration `json:"total_time"`
	Consensus  string        `json:"consensus,omitempty"`
}

// ParallelRunner executes agents in parallel.
type ParallelRunner struct {
	runFunc func(ctx context.Context, spec AgentSpec, prompt string) AgentResult
}

// NewParallelRunner creates a parallel runner with the given execution function.
func NewParallelRunner(runFunc func(ctx context.Context, spec AgentSpec, prompt string) AgentResult) *ParallelRunner {
	return &ParallelRunner{runFunc: runFunc}
}

// Run executes all agents in parallel and collects results.
func (pr *ParallelRunner) Run(ctx context.Context, config ParallelConfig) (*ParallelResult, error) {
	if len(config.Agents) == 0 {
		return nil, fmt.Errorf("mux: no agents specified")
	}

	timeout := config.MaxDuration
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	resultCh := make(chan AgentResult, len(config.Agents))
	var wg sync.WaitGroup

	for _, spec := range config.Agents {
		wg.Add(1)
		go func(s AgentSpec) {
			defer wg.Done()
			resultCh <- pr.runFunc(ctx, s, config.Prompt)
		}(spec)
	}

	// Close channel when all done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []AgentResult
	for r := range resultCh {
		results = append(results, r)
	}

	totalTime := time.Since(start)

	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}

	// Rank results
	rankResults(results, config.RankBy)

	prResult := &ParallelResult{
		Results:   results,
		Succeeded: succeeded,
		Failed:    failed,
		TotalTime: totalTime,
	}

	// Pick best
	if len(results) > 0 {
		prResult.Best = &results[0]
	}

	// Check minimum agents
	minAgents := config.MinAgents
	if minAgents == 0 {
		minAgents = 1
	}
	if succeeded < minAgents {
		return prResult, fmt.Errorf("mux: only %d/%d agents succeeded (min: %d)", succeeded, len(config.Agents), minAgents)
	}

	return prResult, nil
}

// RunFirstSuccess runs agents in parallel and returns the first successful result.
func (pr *ParallelRunner) RunFirstSuccess(ctx context.Context, agents []AgentSpec, prompt string, timeout time.Duration) (*AgentResult, error) {
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultCh := make(chan AgentResult, len(agents))

	for _, spec := range agents {
		go func(s AgentSpec) {
			resultCh <- pr.runFunc(ctx, s, prompt)
		}(spec)
	}

	for i := 0; i < len(agents); i++ {
		select {
		case r := <-resultCh:
			if r.Success {
				return &r, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("mux: all %d agents failed", len(agents))
}

func rankResults(results []AgentResult, rankBy string) {
	sort.Slice(results, func(i, j int) bool {
		switch rankBy {
		case "speed":
			return results[i].Duration < results[j].Duration
		case "tokens":
			return results[i].Tokens < results[j].Tokens
		case "score":
			return results[i].Score > results[j].Score
		default:
			// Default: success first, then by score
			if results[i].Success != results[j].Success {
				return results[i].Success
			}
			return results[i].Score > results[j].Score
		}
	})
}
