package mux

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelRun(t *testing.T) {
	callCount := int32(0)
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		atomic.AddInt32(&callCount, 1)
		return AgentResult{
			AgentName: spec.Name,
			Success:   true,
			Output:    "result from " + spec.Name,
			Duration:  100 * time.Millisecond,
			Score:     0.9,
		}
	})

	result, err := runner.Run(context.Background(), ParallelConfig{
		Agents: []AgentSpec{
			{Name: "agent-a", Type: "claude"},
			{Name: "agent-b", Type: "codex"},
			{Name: "agent-c", Type: "gemini"},
		},
		Prompt: "explain recursion",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Succeeded != 3 {
		t.Errorf("expected 3 succeeded, got %d", result.Succeeded)
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&callCount))
	}
	if result.Best == nil {
		t.Error("expected best result")
	}
}

func TestParallelRunNoAgents(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		return AgentResult{}
	})

	_, err := runner.Run(context.Background(), ParallelConfig{})
	if err == nil {
		t.Error("expected error for no agents")
	}
}

func TestParallelRunPartialFailure(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		if spec.Name == "fail-agent" {
			return AgentResult{
				AgentName: spec.Name,
				Success:   false,
				Error:     "something went wrong",
			}
		}
		return AgentResult{
			AgentName: spec.Name,
			Success:   true,
			Output:    "ok",
			Score:     0.8,
		}
	})

	result, err := runner.Run(context.Background(), ParallelConfig{
		Agents: []AgentSpec{
			{Name: "ok-1"},
			{Name: "fail-agent"},
			{Name: "ok-2"},
		},
		Prompt: "test",
	})
	if err != nil {
		t.Fatalf("expected success with partial failure: %v", err)
	}
	if result.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
}

func TestParallelRunMinAgents(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		return AgentResult{
			AgentName: spec.Name,
			Success:   false,
			Error:     "failed",
		}
	})

	_, err := runner.Run(context.Background(), ParallelConfig{
		Agents:    []AgentSpec{{Name: "a"}, {Name: "b"}},
		Prompt:    "test",
		MinAgents: 2,
	})
	if err == nil {
		t.Error("expected error when min agents not met")
	}
}

func TestParallelRunRankBySpeed(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		dur := time.Duration(100 * time.Millisecond)
		if spec.Name == "fast" {
			dur = 10 * time.Millisecond
		}
		return AgentResult{
			AgentName: spec.Name,
			Success:   true,
			Output:    "ok",
			Duration:  dur,
			Score:     0.5,
		}
	})

	result, _ := runner.Run(context.Background(), ParallelConfig{
		Agents: []AgentSpec{
			{Name: "slow"},
			{Name: "fast"},
		},
		Prompt: "test",
		RankBy: "speed",
	})

	if result.Best.AgentName != "fast" {
		t.Errorf("expected fast as best by speed, got %s", result.Best.AgentName)
	}
}

func TestParallelRunRankByScore(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		score := 0.5
		if spec.Name == "best" {
			score = 0.99
		}
		return AgentResult{
			AgentName: spec.Name,
			Success:   true,
			Output:    "ok",
			Score:     score,
		}
	})

	result, _ := runner.Run(context.Background(), ParallelConfig{
		Agents: []AgentSpec{
			{Name: "ok"},
			{Name: "best"},
		},
		Prompt: "test",
		RankBy: "score",
	})

	if result.Best.AgentName != "best" {
		t.Errorf("expected best, got %s", result.Best.AgentName)
	}
}

func TestRunFirstSuccess(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		success := spec.Name == "winner"
		var dur time.Duration
		if spec.Name == "winner" {
			dur = 50 * time.Millisecond
		} else {
			dur = 200 * time.Millisecond
		}
		return AgentResult{
			AgentName: spec.Name,
			Success:   success,
			Output:    "result",
			Duration:  dur,
		}
	})

	result, err := runner.RunFirstSuccess(context.Background(),
		[]AgentSpec{{Name: "loser"}, {Name: "winner"}, {Name: "also-loser"}},
		"test", 0,
	)
	if err != nil {
		t.Fatalf("RunFirstSuccess failed: %v", err)
	}
	if result.AgentName != "winner" {
		t.Errorf("expected winner, got %s", result.AgentName)
	}
}

func TestRunFirstSuccessAllFail(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		return AgentResult{AgentName: spec.Name, Success: false, Error: "nope"}
	})

	_, err := runner.RunFirstSuccess(context.Background(),
		[]AgentSpec{{Name: "a"}, {Name: "b"}},
		"test", time.Second,
	)
	if err == nil {
		t.Error("expected error when all fail")
	}
}

func TestParallelRunTimeout(t *testing.T) {
	runner := NewParallelRunner(func(ctx context.Context, spec AgentSpec, prompt string) AgentResult {
		time.Sleep(5 * time.Second)
		return AgentResult{AgentName: spec.Name, Success: true}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := runner.Run(ctx, ParallelConfig{
		Agents:    []AgentSpec{{Name: "slow"}},
		Prompt:    "test",
		MaxDuration: 100 * time.Millisecond,
	})
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestAgentSpecSerialization(t *testing.T) {
	spec := AgentSpec{
		Name:  "test",
		Type:  "claude",
		Model: "claude-4",
		Env:   map[string]string{"KEY": "val"},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "claude-4") {
		t.Error("expected model in JSON")
	}
}

func TestParallelResultBestNil(t *testing.T) {
	pr := &ParallelResult{}
	if pr.Best != nil {
		t.Error("expected nil best for empty result")
	}
}
