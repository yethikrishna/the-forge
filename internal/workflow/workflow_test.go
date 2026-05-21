package workflow

import (
	"strings"
	"testing"
)

func TestDefine(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("CI Pipeline", "Run tests and deploy", []Step{
		{ID: "lint", Name: "Lint", Agent: "linter", Prompt: "Check code quality"},
		{ID: "test", Name: "Test", Agent: "tester", Prompt: "Run tests", DependsOn: []string{"lint"}},
		{ID: "deploy", Name: "Deploy", Agent: "deployer", Prompt: "Deploy to prod", DependsOn: []string{"test"}},
	})

	if wf.ID == "" {
		t.Error("expected non-empty ID")
	}
	if wf.Status != WFStatusPending {
		t.Errorf("expected pending, got %s", wf.Status)
	}
	if len(wf.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(wf.Steps))
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("Simple", "Simple workflow", []Step{
		{ID: "step1", Name: "Step 1", Agent: "a1", Prompt: "Do something"},
		{ID: "step2", Name: "Step 2", Agent: "a2", Prompt: "Do more", DependsOn: []string{"step1"}},
	})

	err := e.Run(wf.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := e.Get(wf.ID)
	if got.Status != WFStatusSuccess {
		t.Errorf("expected success, got %s", got.Status)
	}
	for _, step := range got.Steps {
		if step.Status != StepSuccess {
			t.Errorf("step %s: expected success, got %s", step.ID, step.Status)
		}
	}
}

func TestRunNotFound(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	err := e.Run("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestRunAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("Test", "Test", []Step{
		{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"},
	})
	e.Run(wf.ID)

	err := e.Run(wf.ID)
	if err == nil {
		t.Error("expected error for already completed workflow")
	}
}

func TestPause(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("Test", "Test", []Step{
		{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"},
	})

	// Manually set to running for pause test
	wf.Status = WFStatusRunning

	err := e.Pause(wf.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := e.Get(wf.ID)
	if got.Status != WFStatusPaused {
		t.Errorf("expected paused, got %s", got.Status)
	}
}

func TestResume(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("Test", "Test", []Step{
		{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"},
	})

	wf.Status = WFStatusRunning
	e.Pause(wf.ID)
	e.Resume(wf.ID)

	got, _ := e.Get(wf.ID)
	if got.Status != WFStatusRunning {
		t.Errorf("expected running, got %s", got.Status)
	}
}

func TestCancel(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("Test", "Test", []Step{
		{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"},
	})

	e.Cancel(wf.ID)

	got, _ := e.Get(wf.ID)
	if got.Status != WFStatusCancelled {
		t.Errorf("expected cancelled, got %s", got.Status)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("Test", "Test", []Step{
		{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"},
	})

	e.Delete(wf.ID)

	_, ok := e.Get(wf.ID)
	if ok {
		t.Error("expected workflow to be deleted")
	}
}

func TestParallelSteps(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	// Steps with no dependencies run in sequence here, but the DAG supports parallelism
	wf := e.Define("Parallel", "Parallel steps", []Step{
		{ID: "setup", Name: "Setup", Agent: "setup", Prompt: "Setup env"},
		{ID: "lint", Name: "Lint", Agent: "linter", Prompt: "Lint", DependsOn: []string{"setup"}},
		{ID: "test", Name: "Test", Agent: "tester", Prompt: "Test", DependsOn: []string{"setup"}},
		{ID: "deploy", Name: "Deploy", Agent: "deployer", Prompt: "Deploy", DependsOn: []string{"lint", "test"}},
	})

	e.Run(wf.ID)

	got, _ := e.Get(wf.ID)
	if got.Status != WFStatusSuccess {
		t.Errorf("expected success, got %s", got.Status)
	}
}

func TestValidate(t *testing.T) {
	wf := &Workflow{
		Steps: []Step{
			{ID: "step1", Name: "Step 1", Agent: "a1", Prompt: "P"},
			{ID: "step2", Name: "Step 2", Agent: "a2", Prompt: "P", DependsOn: []string{"step1"}},
		},
	}

	errors := Validate(wf)
	if len(errors) != 0 {
		t.Errorf("expected no errors, got: %v", errors)
	}
}

func TestValidateMissingDep(t *testing.T) {
	wf := &Workflow{
		Steps: []Step{
			{ID: "step1", Name: "Step 1", Agent: "a1", Prompt: "P", DependsOn: []string{"nonexistent"}},
		},
	}

	errors := Validate(wf)
	if len(errors) == 0 {
		t.Error("expected validation error for missing dependency")
	}
}

func TestValidateEmptyID(t *testing.T) {
	wf := &Workflow{
		Steps: []Step{
			{ID: "", Name: "No ID", Agent: "a1", Prompt: "P"},
		},
	}

	errors := Validate(wf)
	if len(errors) == 0 {
		t.Error("expected validation error for empty step ID")
	}
}

func TestWorkflowReport(t *testing.T) {
	wf := &Workflow{
		ID:     "wf-test",
		Name:   "CI Pipeline",
		Status: WFStatusSuccess,
		Steps: []Step{
			{ID: "lint", Name: "Lint", Status: StepSuccess, Agent: "linter"},
			{ID: "test", Name: "Test", Status: StepSuccess, Agent: "tester", DependsOn: []string{"lint"}},
		},
		Duration: "5.2s",
	}

	report := WorkflowReport(wf)
	if !strings.Contains(report, "CI Pipeline") {
		t.Error("expected workflow name in report")
	}
	if !strings.Contains(report, "success") && !strings.Contains(report, "Success") && !strings.Contains(report, "✅") {
		t.Error("expected success status in report")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.Define("WF1", "First", []Step{{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"}})
	e.Define("WF2", "Second", []Step{{ID: "s2", Name: "S2", Agent: "a2", Prompt: "P"}})

	list := e.List()
	if len(list) != 2 {
		t.Errorf("expected 2 workflows, got %d", len(list))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.Define("WF1", "First", []Step{{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"}})

	stats := e.Stats()
	if stats["total_workflows"] != 1 {
		t.Errorf("expected 1 workflow, got %v", stats["total_workflows"])
	}
}

func TestRetryStep(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	wf := e.Define("Test", "Test", []Step{
		{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P", MaxRetries: 1},
	})

	// Manually fail a step
	got, _ := e.Get(wf.ID)
	got.Steps[0].Status = StepFailed

	err := e.RetryStep(wf.ID, "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got2, _ := e.Get(wf.ID)
	if got2.Steps[0].Status != StepSuccess {
		t.Errorf("expected success after retry, got %s", got2.Steps[0].Status)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	e1 := NewEngine(dir)
	wf := e1.Define("Persistent", "Survives restart", []Step{
		{ID: "s1", Name: "S1", Agent: "a1", Prompt: "P"},
	})

	e2 := NewEngine(dir)
	list := e2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 workflow after reload, got %d", len(list))
	}
	if list[0].ID != wf.ID {
		t.Errorf("expected %s, got %s", wf.ID, list[0].ID)
	}
}
