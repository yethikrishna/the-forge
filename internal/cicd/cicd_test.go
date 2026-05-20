package cicd

import (
	"strings"
	"testing"
)

func TestCreatePipeline(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{
		{Name: "build", Agent: "coder", Task: "build the project"},
		{Name: "test", Agent: "tester", Task: "run tests", DependsOn: []string{"build"}},
	}
	p, err := store.CreatePipeline("CI", "Build and test", stages, []string{"push", "pr"})
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if p.Name != "CI" {
		t.Errorf("name: %s", p.Name)
	}
	if len(p.Stages) != 2 {
		t.Errorf("stages: %d", len(p.Stages))
	}
}

func TestGetPipeline(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{{Name: "build", Agent: "coder", Task: "build"}}
	created, _ := store.CreatePipeline("CI", "", stages, nil)
	found, err := store.GetPipeline(created.ID)
	if err != nil {
		t.Fatalf("GetPipeline: %v", err)
	}
	if found.Name != "CI" {
		t.Errorf("name: %s", found.Name)
	}
}

func TestListPipelines(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{{Name: "build", Agent: "coder", Task: "build"}}
	store.CreatePipeline("P1", "", stages, nil)
	store.CreatePipeline("P2", "", stages, nil)
	pipes, err := store.ListPipelines()
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(pipes) != 2 {
		t.Errorf("count: %d", len(pipes))
	}
}

func TestDeletePipeline(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{{Name: "build", Agent: "coder", Task: "build"}}
	p, _ := store.CreatePipeline("CI", "", stages, nil)
	if err := store.DeletePipeline(p.ID); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}
	if _, err := store.GetPipeline(p.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestCreateRun(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{
		{Name: "build", Agent: "coder", Task: "build"},
		{Name: "test", Agent: "tester", Task: "test", DependsOn: []string{"build"}},
	}
	p, _ := store.CreatePipeline("CI", "", stages, nil)
	run, err := store.CreateRun(p.ID, "push", "abc123", "main")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.Status != "pending" {
		t.Errorf("status: %s", run.Status)
	}
	if len(run.StageRuns) != 2 {
		t.Errorf("stage_runs: %d", len(run.StageRuns))
	}
}

func TestGetRun(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{{Name: "build", Agent: "coder", Task: "build"}}
	p, _ := store.CreatePipeline("CI", "", stages, nil)
	created, _ := store.CreateRun(p.ID, "manual", "", "")
	found, err := store.GetRun(created.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if found.PipelineID != p.ID {
		t.Errorf("pipeline_id: %s", found.PipelineID)
	}
}

func TestUpdateStageRun(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{
		{Name: "build", Agent: "coder", Task: "build"},
		{Name: "test", Agent: "tester", Task: "test"},
	}
	p, _ := store.CreatePipeline("CI", "", stages, nil)
	run, _ := store.CreateRun(p.ID, "push", "abc", "main")

	updated, err := store.UpdateStageRun(run.ID, "build", "success", "built OK", 10)
	if err != nil {
		t.Fatalf("UpdateStageRun: %v", err)
	}
	if updated.StageRuns[0].Status != "success" {
		t.Errorf("stage status: %s", updated.StageRuns[0].Status)
	}
}

func TestRunCompletion(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{
		{Name: "build", Agent: "coder", Task: "build"},
		{Name: "test", Agent: "tester", Task: "test"},
	}
	p, _ := store.CreatePipeline("CI", "", stages, nil)
	run, _ := store.CreateRun(p.ID, "push", "abc", "main")

	store.UpdateStageRun(run.ID, "build", "success", "OK", 10)
	updated, _ := store.UpdateStageRun(run.ID, "test", "success", "passed", 20)

	if updated.Status != "success" {
		t.Errorf("expected success, got %s", updated.Status)
	}
	if updated.FinishedAt == nil {
		t.Error("expected finished_at")
	}
}

func TestRunFailure(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{{Name: "build", Agent: "coder", Task: "build"}}
	p, _ := store.CreatePipeline("CI", "", stages, nil)
	run, _ := store.CreateRun(p.ID, "push", "abc", "main")

	updated, _ := store.UpdateStageRun(run.ID, "build", "failed", "error", 5)
	if updated.Status != "failed" {
		t.Errorf("expected failed, got %s", updated.Status)
	}
}

func TestListRuns(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	stages := []Stage{{Name: "build", Agent: "coder", Task: "build"}}
	p, _ := store.CreatePipeline("CI", "", stages, nil)
	store.CreateRun(p.ID, "push", "a1", "main")
	store.CreateRun(p.ID, "push", "a2", "main")

	runs, err := store.ListRuns(p.ID)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("count: %d", len(runs))
	}
}

func TestFormatPipeline(t *testing.T) {
	p := &Pipeline{Name: "CI", ID: "p1", Stages: []Stage{
		{Name: "build", Agent: "coder", Task: "build"},
	}}
	out := FormatPipeline(p)
	if !strings.Contains(out, "CI") {
		t.Error("expected name")
	}
	if !strings.Contains(out, "build") {
		t.Error("expected stage name")
	}
}

func TestFormatRun(t *testing.T) {
	run := &Run{ID: "r1", PipelineID: "p1", Status: "running", StageRuns: []*StageRun{
		{StageName: "build", Status: "success"},
	}}
	out := FormatRun(run)
	if !strings.Contains(out, "r1") {
		t.Error("expected run ID")
	}
	if !strings.Contains(out, "build") {
		t.Error("expected stage name")
	}
}
