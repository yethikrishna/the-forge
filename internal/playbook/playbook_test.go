package playbook

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreatePlaybook(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pb := &Playbook{
		Name:        "Test Playbook",
		Description: "A test playbook",
		Version:     "1.0.0",
		Tags:        []string{"test"},
		Steps: []Step{
			{ID: "step-1", Name: "First step", Type: StepPrompt, Action: "Do something", Status: StatusPending},
		},
	}

	if err := store.Create(pb); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if pb.ID == "" {
		t.Error("Expected ID to be set")
	}

	// Verify persisted
	data, err := os.ReadFile(filepath.Join(dir, pb.ID+".json"))
	if err != nil {
		t.Fatalf("File not persisted: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty file")
	}
}

func TestGetPlaybook(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pb := &Playbook{Name: "Test", Description: "test"}
	store.Create(pb)

	retrieved, ok := store.Get(pb.ID)
	if !ok {
		t.Fatal("Expected to find playbook")
	}
	if retrieved.Name != "Test" {
		t.Errorf("Expected name 'Test', got %q", retrieved.Name)
	}
}

func TestListPlaybooks(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.Create(&Playbook{Name: "Alpha", Tags: []string{"security"}})
	store.Create(&Playbook{Name: "Beta", Tags: []string{"testing"}})
	store.Create(&Playbook{Name: "Gamma", Tags: []string{"security", "testing"}})

	// List all
	all := store.List("")
	if len(all) != 3 {
		t.Errorf("Expected 3 playbooks, got %d", len(all))
	}

	// Filter by tag
	security := store.List("security")
	if len(security) != 2 {
		t.Errorf("Expected 2 security playbooks, got %d", len(security))
	}
}

func TestUpdatePlaybook(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pb := &Playbook{Name: "Original", Description: "test"}
	store.Create(pb)

	pb.Name = "Updated"
	if err := store.Update(pb); err != nil {
		t.Fatalf("Update: %v", err)
	}

	retrieved, _ := store.Get(pb.ID)
	if retrieved.Name != "Updated" {
		t.Errorf("Expected 'Updated', got %q", retrieved.Name)
	}
}

func TestDeletePlaybook(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pb := &Playbook{Name: "ToDelete", Description: "test"}
	store.Create(pb)

	if err := store.Delete(pb.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, ok := store.Get(pb.ID)
	if ok {
		t.Error("Expected playbook to be deleted")
	}
}

func TestGenerateFromSession(t *testing.T) {
	session := Session{
		ID:      "sess-123",
		Prompt:  "Implement user authentication with OAuth2",
		Outcome: "success",
		Tags:    []string{"auth", "security"},
		Steps: []SessionStep{
			{Type: StepPrompt, Action: "Create OAuth2 provider struct", Status: StatusCompleted, Duration: 5 * time.Second},
			{Type: StepTool, Action: "Write file auth/oauth2.go", Status: StatusCompleted, Duration: 2 * time.Second},
			{Type: StepPrompt, Action: "Add tests for OAuth2 flow", Status: StatusCompleted, Duration: 8 * time.Second},
			{Type: StepPrompt, Action: "This step failed", Status: StatusFailed, Duration: 1 * time.Second},
		},
	}

	pb, err := GenerateFromSession(session)
	if err != nil {
		t.Fatalf("GenerateFromSession: %v", err)
	}

	if pb.Name == "" {
		t.Error("Expected non-empty name")
	}
	if pb.Source != "sess-123" {
		t.Errorf("Expected source sess-123, got %q", pb.Source)
	}
	// Should have 3 steps (skipping the failed one)
	if len(pb.Steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(pb.Steps))
	}
}

func TestGenerateFromSessionFailed(t *testing.T) {
	session := Session{
		ID:      "sess-456",
		Prompt:  "This failed",
		Outcome: "failed",
	}

	_, err := GenerateFromSession(session)
	if err == nil {
		t.Error("Expected error for failed session")
	}
}

func TestExecutePlaybook(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pb := &Playbook{
		Name:        "Exec Test",
		Description: "test execution",
		Variables: map[string]Variable{
			"name": {Name: "name", Type: "string", Required: true},
		},
		Steps: []Step{
			{ID: "step-1", Name: "Greet", Type: StepPrompt, Action: "Hello {{.name}}", Status: StatusPending},
			{ID: "step-2", Name: "Bye", Type: StepPrompt, Action: "Goodbye {{.name}}", Status: StatusPending, DependsOn: []string{"step-1"}},
		},
	}
	store.Create(pb)

	run, err := store.Execute(t.Context(), pb.ID, map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if run.Status != StatusCompleted {
		t.Errorf("Expected completed status, got %s", run.Status)
	}
	if len(run.StepResults) != 2 {
		t.Errorf("Expected 2 step results, got %d", len(run.StepResults))
	}
}

func TestExecuteMissingVariable(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pb := &Playbook{
		Name:        "Missing Var",
		Description: "test",
		Variables: map[string]Variable{
			"required_var": {Name: "required_var", Type: "string", Required: true},
		},
	}
	store.Create(pb)

	_, err = store.Execute(t.Context(), pb.ID, map[string]string{})
	if err == nil {
		t.Error("Expected error for missing required variable")
	}
}

func TestRecordRun(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pb := &Playbook{Name: "Run Test", Description: "test"}
	store.Create(pb)

	run := &Run{
		PlaybookID: pb.ID,
		Status:     StatusCompleted,
		StartedAt:  time.Now(),
	}
	store.RecordRun(run)

	runs := store.GetRuns(pb.ID)
	if len(runs) != 1 {
		t.Errorf("Expected 1 run, got %d", len(runs))
	}

	// Check playbook stats updated
	updated, _ := store.Get(pb.ID)
	if updated.RunCount != 1 {
		t.Errorf("Expected run count 1, got %d", updated.RunCount)
	}
}

func TestExportMarkdown(t *testing.T) {
	pb := &Playbook{
		Name:        "Test Playbook",
		Description: "A test",
		Version:     "1.0.0",
		Tags:        []string{"test"},
		Variables: map[string]Variable{
			"name": {Name: "name", Type: "string", Required: true, Description: "The name"},
		},
		Steps: []Step{
			{ID: "step-1", Name: "First", Type: StepPrompt, Action: "Hello {{.name}}", Status: StatusPending},
		},
	}

	md := ExportMarkdown(pb)
	if md == "" {
		t.Error("Expected non-empty markdown")
	}
	if !contains(md, "# Playbook: Test Playbook") {
		t.Error("Expected title in markdown")
	}
	if !contains(md, "Variables") {
		t.Error("Expected variables section")
	}
	if !contains(md, "Steps") {
		t.Error("Expected steps section")
	}
}

func TestExportYAML(t *testing.T) {
	pb := &Playbook{
		Name:        "YAML Test",
		Description: "A test",
		Version:     "1.0.0",
		Steps: []Step{
			{ID: "step-1", Name: "First", Type: StepPrompt, Action: "Do thing", Status: StatusPending},
		},
	}

	yaml := ExportYAML(pb)
	if yaml == "" {
		t.Error("Expected non-empty YAML")
	}
	if !contains(yaml, "name:") {
		t.Error("Expected name field in YAML")
	}
	if !contains(yaml, "steps:") {
		t.Error("Expected steps field in YAML")
	}
}

func TestLoadExistingPlaybooks(t *testing.T) {
	dir := t.TempDir()

	// Create a playbook file manually
	pb := &Playbook{
		ID:          "test-pb",
		Name:        "Pre-existing",
		Description: "Loaded from disk",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	data, _ := marshalJSON(pb)
	os.WriteFile(filepath.Join(dir, "test-pb.json"), data, 0644)

	// Load
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	retrieved, ok := store.Get("test-pb")
	if !ok {
		t.Fatal("Expected to load pre-existing playbook")
	}
	if retrieved.Name != "Pre-existing" {
		t.Errorf("Expected 'Pre-existing', got %q", retrieved.Name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func marshalJSON(v interface{}) ([]byte, error) {
	import_json_pkg := func() ([]byte, error) {
		b, err := (&struct{ V interface{}}{V: v}).V, error(nil)
		_ = b
		return nil, err
	}
	_ = import_json_pkg
	return nil, nil
}
