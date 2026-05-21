package workflow

import (
	"context"
	"testing"
	"time"
)

func TestNewWorkflow(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{Name: "test-workflow"})
	if wf == nil {
		t.Fatal("NewWorkflow should return a workflow")
	}
	if wf.ID() == "" {
		t.Error("Workflow should have an ID")
	}
}

func TestWorkflowState(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{Name: "test"})
	if wf.State() != WfPending {
		t.Errorf("Initial state = %q, want %q", wf.State(), WfPending)
	}
}

func TestWorkflowStart(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Do something"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := wf.Start(ctx)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if wf.State() != WfRunning {
		t.Errorf("State after start = %q, want %q", wf.State(), WfRunning)
	}
}

func TestWorkflowStartWrongState(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{Name: "test"})
	ctx := context.Background()
	wf.Start(ctx)

	err := wf.Start(ctx)
	if err == nil {
		t.Error("Starting already-started workflow should error")
	}
}

func TestWorkflowCancel(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Do something"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wf.Start(ctx)
	wf.Cancel()

	if wf.State() != WfCancelled {
		t.Errorf("State after cancel = %q, want %q", wf.State(), WfCancelled)
	}
}

func TestSubmitStepResult(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Do something"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	err := wf.SubmitStepResult("step1", StepResult{
		StepID:   "step1",
		State:    StepComplete,
		Output:   "done",
		Duration: 2 * time.Second,
		Exports:  map[string]string{"key1": "value1"},
		CostUSD:  0.05,
	})
	if err != nil {
		t.Fatalf("SubmitStepResult error: %v", err)
	}

	result, ok := wf.StepResult("step1")
	if !ok {
		t.Fatal("Step result should exist")
	}
	if result.State != StepComplete {
		t.Errorf("State = %q, want %q", result.State, StepComplete)
	}

	exports := wf.Exports()
	if exports["key1"] != "value1" {
		t.Errorf("Export key1 = %q, want %q", exports["key1"], "value1")
	}
}

func TestStepDependencies(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Step 1"},
			{ID: "step2", Name: "Second", Prompt: "Step 2", DependsOn: []string{"step1"}},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	// step2 should still be pending (depends on step1)
	result, _ := wf.StepResult("step2")
	if result.State != StepPending {
		t.Errorf("step2 should be pending before step1 completes, got %q", result.State)
	}

	// Complete step1
	wf.SubmitStepResult("step1", StepResult{StepID: "step1", State: StepComplete, Output: "done"})

	// Wait a moment for dispatch
	time.Sleep(100 * time.Millisecond)

	// step2 should now be running (dispatched)
	result, _ = wf.StepResult("step2")
	if result.State != StepRunning && result.State != StepPending {
		t.Errorf("step2 should be running or pending after step1 completes, got %q", result.State)
	}
}

func TestStepConditions(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Step 1"},
			{ID: "step2", Name: "Second", Prompt: "Step 2", DependsOn: []string{"step1"},
				Conditions: []Condition{
					{StepID: "step1", Field: "state", Operator: OpEquals, Value: "complete"},
				}},
			{ID: "step3", Name: "Third", Prompt: "Step 3", DependsOn: []string{"step1"},
				Conditions: []Condition{
					{StepID: "step1", Field: "state", Operator: OpEquals, Value: "failed"},
				}},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	// Complete step1
	wf.SubmitStepResult("step1", StepResult{StepID: "step1", State: StepComplete, Output: "ok"})
	time.Sleep(100 * time.Millisecond)

	// step2 should be running (condition met)
	result2, _ := wf.StepResult("step2")
	if result2.State == StepSkipped {
		t.Error("step2 should NOT be skipped (condition met)")
	}

	// step3 should be skipped (condition not met)
	result3, _ := wf.StepResult("step3")
	if result3.State != StepSkipped {
		t.Errorf("step3 should be skipped (condition not met), got %q", result3.State)
	}
}

func TestWorkflowStats(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Step 1"},
			{ID: "step2", Name: "Second", Prompt: "Step 2", DependsOn: []string{"step1"}},
		},
	})

	stats := wf.Stats()
	if stats.TotalSteps != 2 {
		t.Errorf("TotalSteps = %d, want 2", stats.TotalSteps)
	}
}

func TestWorkflowEvents(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Step 1"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	events := wf.Events()
	if len(events) == 0 {
		t.Error("Should have events after starting")
	}
}

func TestExportMarkdown(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test-workflow",
		Steps: []StepConfig{
			{ID: "step1", Name: "First Step", Prompt: "Do it"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	md := wf.ExportMarkdown()
	if md == "" {
		t.Error("ExportMarkdown should not be empty")
	}
	if !containsStr(md, "test-workflow") {
		t.Error("Markdown should contain workflow name")
	}
}

func TestWorkflowCost(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Step 1"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	wf.SubmitStepResult("step1", StepResult{
		StepID:  "step1",
		State:   StepComplete,
		CostUSD: 0.15,
	})

	if wf.Cost() != 0.15 {
		t.Errorf("Cost = %.4f, want 0.15", wf.Cost())
	}
}

func TestWorkflowCompletion(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Step 1"},
			{ID: "step2", Name: "Second", Prompt: "Step 2"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	wf.SubmitStepResult("step1", StepResult{StepID: "step1", State: StepComplete})
	wf.SubmitStepResult("step2", StepResult{StepID: "step2", State: StepComplete})
	time.Sleep(150 * time.Millisecond)

	if wf.State() != WfComplete {
		t.Errorf("State = %q, want %q", wf.State(), WfComplete)
	}
}

func TestWorkflowFailedCompletion(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{
		Name: "test",
		Steps: []StepConfig{
			{ID: "step1", Name: "First", Prompt: "Step 1"},
			{ID: "step2", Name: "Second", Prompt: "Step 2"},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wf.Start(ctx)

	wf.SubmitStepResult("step1", StepResult{StepID: "step1", State: StepComplete})
	wf.SubmitStepResult("step2", StepResult{StepID: "step2", State: StepFailed, Error: "something broke"})
	time.Sleep(150 * time.Millisecond)

	if wf.State() != WfFailed {
		t.Errorf("State = %q, want %q", wf.State(), WfFailed)
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	wf := NewWorkflow(WorkflowConfig{Name: "store-test"})
	wf.SubmitStepResult("s1", StepResult{StepID: "s1", State: StepComplete, CostUSD: 0.1})

	if err := store.Save(wf); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	stats, err := store.Load(wf.ID())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if stats.WorkflowID != wf.ID() {
		t.Errorf("Loaded ID = %q, want %q", stats.WorkflowID, wf.ID())
	}
}

func TestStoreList(t *testing.T) {
	store, _ := NewStore(t.TempDir())

	wf := NewWorkflow(WorkflowConfig{Name: "list-test"})
	wf.SubmitStepResult("s1", StepResult{StepID: "s1", State: StepComplete})
	store.Save(wf)

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("List = %d, want 1", len(ids))
	}
}

func TestStepResultNotFound(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{Name: "test"})
	_, ok := wf.StepResult("nonexistent")
	if ok {
		t.Error("StepResult should return false for nonexistent step")
	}
}

func TestSubmitStepResultNotFound(t *testing.T) {
	wf := NewWorkflow(WorkflowConfig{Name: "test"})
	err := wf.SubmitStepResult("nonexistent", StepResult{StepID: "nonexistent", State: StepComplete})
	if err == nil {
		t.Error("Should error for nonexistent step")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
