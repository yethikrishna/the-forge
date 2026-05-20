package forgeci

import (
	"testing"
	"time"
)

func TestNewCIEngine(t *testing.T) {
	e, err := NewCIEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestRegisterPipeline(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	p := &CIPipeline{Name: "test", Trigger: "push", Steps: []CIStep{
		{Name: "build", Type: StepBuild, Command: "echo building"},
	}}
	err := e.Register(p)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetPipeline(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{Name: "test", Trigger: "push", Steps: []CIStep{}})
	p, err := e.GetPipeline("test")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "test" {
		t.Errorf("expected test, got %s", p.Name)
	}
}

func TestGetPipelineNotFound(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	_, err := e.GetPipeline("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListPipelines(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{Name: "p1", Trigger: "push"})
	e.Register(&CIPipeline{Name: "p2", Trigger: "pr"})
	list := e.ListPipelines()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestRunPipeline(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{
		Name:    "test-ci",
		Trigger: "push",
		Steps: []CIStep{
			{Name: "build", Type: StepBuild, Command: "echo hello"},
			{Name: "test", Type: StepTest, Command: "echo testing", DependsOn: []string{"build"}},
		},
	})
	run, err := e.Run("test-ci", "push", "abc123", "main")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != PipelinePassed {
		t.Errorf("expected passed, got %s", run.Status)
	}
	if len(run.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(run.Results))
	}
}

func TestRunPipelineFailure(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{
		Name:    "fail-ci",
		Trigger: "push",
		Steps: []CIStep{
			{Name: "build", Type: StepBuild, Command: "exit 1"},
			{Name: "test", Type: StepTest, Command: "echo testing", DependsOn: []string{"build"}},
		},
	})
	run, _ := e.Run("fail-ci", "push", "abc", "main")
	if run.Status != PipelineFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
}

func TestRunPipelineWithContinueOnError(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{
		Name:    "continue-ci",
		Trigger: "push",
		Steps: []CIStep{
			{Name: "build", Type: StepBuild, Command: "exit 1", OnError: "continue"},
			{Name: "test", Type: StepTest, Command: "echo testing"},
		},
	})
	run, _ := e.Run("continue-ci", "push", "abc", "main")
	testResult, ok := run.Results["test"]
	if !ok {
		t.Fatal("expected test result")
	}
	if testResult.Status == StatusSkipped {
		t.Error("test should not be skipped with continue-on-error")
	}
}

func TestAgentStep(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{
		Name:    "agent-ci",
		Trigger: "push",
		Steps: []CIStep{
			{Name: "review", Type: StepAgent, Agent: "reviewer"},
		},
	})
	run, _ := e.Run("agent-ci", "push", "abc", "main")
	if run.Results["review"].Status != StatusPassed {
		t.Errorf("expected passed, got %s", run.Results["review"].Status)
	}
}

func TestReviewStep(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{
		Name:    "review-ci",
		Trigger: "pr",
		Steps: []CIStep{
			{Name: "review", Type: StepReview, Agent: "code-reviewer"},
		},
	})
	run, _ := e.Run("review-ci", "pr", "abc", "feature")
	if run.Results["review"].Status != StatusPassed {
		t.Errorf("expected passed, got %s", run.Results["review"].Status)
	}
}

func TestDefaultGoPipeline(t *testing.T) {
	p := DefaultGoPipeline()
	if p.Name != "go-ci" {
		t.Errorf("expected go-ci, got %s", p.Name)
	}
	if len(p.Steps) == 0 {
		t.Error("expected steps")
	}
}

func TestGetRun(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{Name: "test", Trigger: "push", Steps: []CIStep{
		{Name: "build", Type: StepBuild, Command: "echo hi"},
	}})
	run, _ := e.Run("test", "push", "abc", "main")
	got, err := e.GetRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != run.ID {
		t.Errorf("expected %s, got %s", run.ID, got.ID)
	}
}

func TestListRuns(t *testing.T) {
	e, _ := NewCIEngine(t.TempDir())
	e.Register(&CIPipeline{Name: "test", Trigger: "push", Steps: []CIStep{
		{Name: "build", Type: StepBuild, Command: "echo hi"},
	}})
	e.Run("test", "push", "abc1", "main")
	e.Run("test", "push", "abc2", "main")
	runs := e.ListRuns()
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}

func TestFormatPipeline(t *testing.T) {
	p := DefaultGoPipeline()
	output := FormatPipeline(p)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatRun(t *testing.T) {
	r := &PipelineRun{
		ID:        "run-1",
		Pipeline:  "test",
		Status:    PipelinePassed,
		StartedAt: time.Now(),
		Duration:  5 * time.Second,
		Results: map[string]*StepResult{
			"build": {StepName: "build", Status: StatusPassed, Duration: 2 * time.Second},
		},
	}
	output := FormatRun(r)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	e1, _ := NewCIEngine(dir)
	e1.Register(&CIPipeline{Name: "persist", Trigger: "push", Steps: []CIStep{}})

	e2, _ := NewCIEngine(dir)
	p, err := e2.GetPipeline("persist")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "persist" {
		t.Errorf("expected persist, got %s", p.Name)
	}
}
