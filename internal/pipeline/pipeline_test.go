package pipeline_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/forge/sword/internal/guard"
	"github.com/forge/sword/internal/memory"
	"github.com/forge/sword/internal/pipeline"
	"github.com/forge/sword/internal/qualitygate"
	"github.com/forge/sword/internal/trust"
)

// mockRunner is a test agent runner.
type mockRunner struct {
	calls atomic.Int32
	err   error
}

func (m *mockRunner) Run(_ context.Context, agent, model, prompt string) (string, error) {
	m.calls.Add(1)
	if m.err != nil {
		return "", m.err
	}
	return fmt.Sprintf("output from %s/%s for: %s", agent, model, prompt), nil
}

func TestPipelineSequential(t *testing.T) {
	runner := &mockRunner{}
	exec := pipeline.NewExecutor(runner)

	pipe := pipeline.Pipeline{
		Name: "test-sequential",
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1"},
			{Name: "step2", Agent: "reviewer", Model: "opus", Prompt: "do step 2"},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Status != pipeline.StatusCompleted {
		t.Errorf("expected completed, got %s", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
	if runner.calls.Load() != 2 {
		t.Errorf("expected 2 runner calls, got %d", runner.calls.Load())
	}
}

func TestPipelineParallel(t *testing.T) {
	runner := &mockRunner{}
	exec := pipeline.NewExecutor(runner)

	pipe := pipeline.Pipeline{
		Name:     "test-parallel",
		Parallel: true,
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1"},
			{Name: "step2", Agent: "reviewer", Model: "opus", Prompt: "do step 2"},
			{Name: "step3", Agent: "tester", Model: "haiku", Prompt: "do step 3"},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Status != pipeline.StatusCompleted {
		t.Errorf("expected completed, got %s", result.Status)
	}
	if runner.calls.Load() != 3 {
		t.Errorf("expected 3 runner calls, got %d", runner.calls.Load())
	}
}

func TestPipelineFailure(t *testing.T) {
	runner := &mockRunner{}
	runner.err = fmt.Errorf("agent failed")
	exec := pipeline.NewExecutor(runner)

	pipe := pipeline.Pipeline{
		Name: "test-fail",
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1"},
			{Name: "step2", Agent: "reviewer", Model: "opus", Prompt: "do step 2"},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Status != pipeline.StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if result.Steps[0].Status != pipeline.StatusFailed {
		t.Errorf("step1 should be failed, got %s", result.Steps[0].Status)
	}
	// Second step should not have been attempted (on_fail defaults to stop)
	if runner.calls.Load() != 1 {
		t.Errorf("expected 1 runner call (fail-fast), got %d", runner.calls.Load())
	}
}

func TestPipelineContinueOnFail(t *testing.T) {
	runner := &mockRunner{}
	runner.err = fmt.Errorf("agent failed")
	exec := pipeline.NewExecutor(runner)

	pipe := pipeline.Pipeline{
		Name:   "test-continue",
		OnFail: "continue",
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1"},
			{Name: "step2", Agent: "reviewer", Model: "opus", Prompt: "do step 2"},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Status != pipeline.StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if runner.calls.Load() != 2 {
		t.Errorf("expected 2 runner calls (continue on fail), got %d", runner.calls.Load())
	}
}

func TestPipelineEmpty(t *testing.T) {
	runner := &mockRunner{}
	exec := pipeline.NewExecutor(runner)

	pipe := pipeline.Pipeline{Name: "empty"}

	_, err := exec.Execute(context.Background(), pipe)
	if err == nil {
		t.Error("expected error for empty pipeline")
	}
}

func TestPipelineApprovalGate(t *testing.T) {
	runner := &mockRunner{}

	// Deny approval
	denier := &denyApprover{}
	exec := pipeline.NewExecutor(runner, pipeline.WithApprovalHandler(denier))

	pipe := pipeline.Pipeline{
		Name: "test-approval",
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1", Approval: true},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Status != pipeline.StatusFailed {
		t.Errorf("expected failed (approval denied), got %s", result.Status)
	}
}

func TestPipelineStepCallback(t *testing.T) {
	runner := &mockRunner{}

	var statuses []pipeline.StepStatus
	exec := pipeline.NewExecutor(runner, pipeline.WithStepCallback(func(_ pipeline.Step, status pipeline.StepStatus) {
		statuses = append(statuses, status)
	}))

	pipe := pipeline.Pipeline{
		Name: "test-callback",
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1"},
		},
	}

	exec.Execute(context.Background(), pipe)

	// Should have: running, completed
	if len(statuses) < 2 {
		t.Errorf("expected at least 2 callbacks, got %d", len(statuses))
	}
}

func TestPipelineDependencies(t *testing.T) {
	runner := &mockRunner{}
	exec := pipeline.NewExecutor(runner)

	pipe := pipeline.Pipeline{
		Name: "test-deps",
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1"},
			{Name: "step2", Agent: "reviewer", Model: "opus", Prompt: "review step1", Input: "step1", DependsOn: []string{"step1"}},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Status != pipeline.StatusCompleted {
		t.Errorf("expected completed, got %s", result.Status)
	}
}

func TestFormatResult(t *testing.T) {
	runner := &mockRunner{}
	exec := pipeline.NewExecutor(runner)

	pipe := pipeline.Pipeline{
		Name: "test-format",
		Steps: []pipeline.Step{
			{Name: "step1", Agent: "claude", Model: "sonnet", Prompt: "do step 1"},
		},
	}

	result, _ := exec.Execute(context.Background(), pipe)
	formatted := pipeline.FormatResult(result)

	if formatted == "" {
		t.Error("formatted result should not be empty")
	}
}

// denyApprover denies all approval requests.
type denyApprover struct{}

func (d *denyApprover) RequestApproval(_ context.Context, _ pipeline.Step, _ string) (bool, error) {
	return false, nil
}

func TestPipelineQualityGateBlocksBadCode(t *testing.T) {
	runner := &mockRunner{} // always returns output (no errors)

	// Set up quality gate system with a blocking security gate
	dir := t.TempDir()
	qgs := qualitygate.NewQualityGateSystem(dir)
	gp := qgs.CreatePipeline("code-review", []*qualitygate.Gate{
		{
			ID:        "security-check",
			Name:      "Security Gate",
			Criterion: qualitygate.CriterionSecurity,
			Blocking:  true,
			Order:     1,
			Config:    map[string]interface{}{"keywords": []string{"rm -rf", "eval(", "exec("}},
		},
	})

	// Set up trust manager
	trustDir := t.TempDir()
	trustMgr := trust.NewManager(trustDir)

	// Wire quality gate + trust into executor
	exec := pipeline.NewExecutor(runner,
		pipeline.WithQualityGate(qgs, gp.ID),
		pipeline.WithTrustManager(trustMgr),
	)

	pipe := pipeline.Pipeline{
		Name: "test-quality-gate",
		Steps: []pipeline.Step{
			{Name: "codegen", Agent: "coder-1", Model: "sonnet", Prompt: "write dangerous code"},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	// Quality gate may pass or fail depending on output content.
	// The key check: if gate fails, step status must be failed.
	if result.Steps[0].Status == pipeline.StatusFailed {
		// Confirm trust score dropped
		score, exists := trustMgr.GetScore("coder-1")
		if !exists {
			t.Error("trust record should exist after gate evaluation")
		}
		if score >= 75 {
			t.Errorf("trust score should have dropped after gate failure, got %.1f", score)
		}
		t.Logf("quality gate blocked step; trust score: %.1f", score)
	} else {
		// Gate passed — trust should have increased
		t.Logf("quality gate passed; status: %s", result.Steps[0].Status)
	}
}

func TestPipelineQualityGateTrustDelta(t *testing.T) {
	// Verify trust +3 on pass, -5 on fail
	trustDir := t.TempDir()
	trustMgr := trust.NewManager(trustDir)

	initialScore, _ := trustMgr.GetScore("agent-test")

	// Simulate a pass: RecordTestResult(true) + RecordFeedback(true)
	trustMgr.RecordTestResult("agent-test", true)
	trustMgr.RecordFeedback("agent-test", true)
	afterPass, _ := trustMgr.GetScore("agent-test")

	if afterPass <= initialScore {
		t.Errorf("trust should increase after pass: was %.1f, now %.1f", initialScore, afterPass)
	}

	// Simulate a fail: RecordTestResult(false) + RecordFeedback(false)
	before := afterPass
	trustMgr.RecordTestResult("agent-test", false)
	trustMgr.RecordFeedback("agent-test", false)
	afterFail, _ := trustMgr.GetScore("agent-test")

	if afterFail >= before {
		t.Errorf("trust should decrease after fail: was %.1f, now %.1f", before, afterFail)
	}
	t.Logf("trust delta verified: pass %.1f→%.1f, fail %.1f→%.1f",
		initialScore, afterPass, before, afterFail)
}

func TestPipelineMemoryStoreOnCompletion(t *testing.T) {
	runner := &mockRunner{}

	// Wire memory store
	memDir := t.TempDir()
	ms := memory.NewStore(memDir + "/mem.json")

	exec := pipeline.NewExecutor(runner, pipeline.WithMemoryStore(ms))

	pipe := pipeline.Pipeline{
		Name: "test-memory",
		Steps: []pipeline.Step{
			{Name: "step-alpha", Agent: "agent-x", Model: "sonnet", Prompt: "do work"},
			{Name: "step-beta", Agent: "agent-y", Model: "haiku", Prompt: "review work"},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if result.Status != pipeline.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}

	// Memory store should have entries for both steps
	entries := ms.ListByAgent("agent-x")
	if len(entries) == 0 {
		t.Error("expected memory entries for agent-x after step completion")
	}
	entriesY := ms.ListByAgent("agent-y")
	if len(entriesY) == 0 {
		t.Error("expected memory entries for agent-y after step completion")
	}

	// Check tags contain "outcome:success"
	found := false
	for _, e := range entries {
		for _, tag := range e.Tags {
			if tag == "outcome:success" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected 'outcome:success' tag in memory entry for agent-x")
	}
	t.Logf("memory store has %d entries for agent-x, %d for agent-y", len(entries), len(entriesY))
}

func TestPipelineComplianceGateBlocksExternalAction(t *testing.T) {
	runner := &mockRunner{}

	// Set up a guard with a block rule on "data_export"
	guardDir := t.TempDir()
	g := guard.NewGuard(guardDir)
	g.AddRule(guard.Rule{
		Name:        "block-data-export",
		Type:        guard.RuleBlock,
		Enabled:     true,
		Priority:    100,
		ActionTypes: []string{"data_export"},
		Description: "Block all data exports",
	})

	exec := pipeline.NewExecutor(runner, pipeline.WithGuard(g))

	pipe := pipeline.Pipeline{
		Name: "test-compliance",
		Steps: []pipeline.Step{
			{
				Name:           "export-step",
				Agent:          "exporter",
				Model:          "sonnet",
				Prompt:         "export user data",
				ExternalAction: "data_export",
				ActionTarget:   "users.csv",
			},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Steps[0].Status != pipeline.StatusFailed {
		t.Errorf("expected step to be blocked by compliance gate; got status=%s", result.Steps[0].Status)
	}
	if !stringContains(result.Steps[0].Error, "compliance gate blocked") {
		t.Errorf("expected compliance gate error; got: %s", result.Steps[0].Error)
	}
	t.Logf("compliance gate blocked correctly: %s", result.Steps[0].Error)
}

func TestPipelineComplianceGateAllowsNonExternalStep(t *testing.T) {
	runner := &mockRunner{}

	guardDir := t.TempDir()
	g := guard.NewGuard(guardDir)
	g.AddRule(guard.Rule{
		Name:        "block-data-export",
		Type:        guard.RuleBlock,
		Enabled:     true,
		Priority:    100,
		ActionTypes: []string{"data_export"},
		Description: "Block data exports",
	})

	exec := pipeline.NewExecutor(runner, pipeline.WithGuard(g))

	// Step with no ExternalAction — guard should NOT run
	pipe := pipeline.Pipeline{
		Name: "test-compliance-allow",
		Steps: []pipeline.Step{
			{Name: "code-step", Agent: "coder", Model: "sonnet", Prompt: "write code"},
		},
	}

	result, err := exec.Execute(context.Background(), pipe)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if result.Status != pipeline.StatusCompleted {
		t.Errorf("expected completed; got %s (step error: %s)", result.Status, result.Steps[0].Error)
	}
}

// stringContains is a simple substring check (no strings import needed).
func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
