package simulate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateScenario(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sc := &Scenario{
		Name:        "Test Bug Fix",
		Type:        ScenarioBugFix,
		Description: "Fix a nil pointer issue",
		Difficulty:  2,
		Context:     "handler doesn't check nil",
		Input: ScenarioInput{
			Prompt:   "Fix the nil pointer dereference",
			Language: "go",
		},
		Expected: ScenarioExpected{
			MinQualityScore: 60,
		},
	}

	if err := store.CreateScenario(sc); err != nil {
		t.Fatalf("CreateScenario: %v", err)
	}

	if sc.ID == "" {
		t.Error("Expected ID to be set")
	}

	// Verify persisted
	if _, err := os.Stat(filepath.Join(dir, sc.ID+".json")); err != nil {
		t.Fatalf("File not persisted: %v", err)
	}
}

func TestGetScenario(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sc := &Scenario{Name: "Test", Type: ScenarioFeature, Description: "test"}
	store.CreateScenario(sc)

	retrieved, ok := store.GetScenario(sc.ID)
	if !ok {
		t.Fatal("Expected to find scenario")
	}
	if retrieved.Name != "Test" {
		t.Errorf("Expected 'Test', got %q", retrieved.Name)
	}
}

func TestListScenarios(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.CreateScenario(&Scenario{Name: "Bug", Type: ScenarioBugFix, Description: "test"})
	store.CreateScenario(&Scenario{Name: "Feature", Type: ScenarioFeature, Description: "test"})
	store.CreateScenario(&Scenario{Name: "Security", Type: ScenarioSecurity, Description: "test"})

	all := store.ListScenarios("")
	if len(all) != 3 {
		t.Errorf("Expected 3 scenarios, got %d", len(all))
	}

	bugs := store.ListScenarios(ScenarioBugFix)
	if len(bugs) != 1 {
		t.Errorf("Expected 1 bug scenario, got %d", len(bugs))
	}
}

func TestRunScenario(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sc := &Scenario{
		Name:        "Test Run",
		Type:        ScenarioBugFix,
		Description: "test scenario",
		Difficulty:  2,
		Context:     "fix a bug",
		Expected: ScenarioExpected{
			MinQualityScore: 50,
		},
	}
	store.CreateScenario(sc)

	trial, err := store.RunScenario(t.Context(), sc.ID, "coder", "claude-sonnet-4")
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}

	if trial.Score <= 0 {
		t.Error("Expected positive score")
	}
	if trial.Cost < 0 {
		t.Error("Expected non-negative cost")
	}
	if trial.Duration <= 0 {
		t.Error("Expected positive duration")
	}
}

func TestRunScenarioNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, err = store.RunScenario(t.Context(), "nonexistent", "agent", "model")
	if err == nil {
		t.Error("Expected error for nonexistent scenario")
	}
}

func TestRecordTrial(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	trial := &Trial{
		ScenarioID: "sc-1",
		Agent:      "coder",
		Model:      "gpt-4.1",
		Score:      85.0,
		Pass:       true,
		Cost:       0.05,
		StartedAt:  time.Now(),
		CompletedAt: time.Now(),
	}
	store.RecordTrial(trial)

	trials := store.GetTrials("sc-1")
	if len(trials) != 1 {
		t.Errorf("Expected 1 trial, got %d", len(trials))
	}
}

func TestRunSimulation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Create some scenarios
	store.CreateScenario(&Scenario{
		Name: "Bug Fix", Type: ScenarioBugFix, Description: "test",
		Difficulty: 2, Context: "fix", Status: StatusReady,
		Expected: ScenarioExpected{MinQualityScore: 50},
	})
	store.CreateScenario(&Scenario{
		Name: "Feature", Type: ScenarioFeature, Description: "test",
		Difficulty: 3, Context: "add", Status: StatusReady,
		Expected: ScenarioExpected{MinQualityScore: 50},
	})

	report, err := store.RunSimulation(t.Context(),
		nil, // all types
		[]string{"agent-a", "agent-b"},
		[]string{"claude-sonnet-4", "gpt-4.1"},
	)
	if err != nil {
		t.Fatalf("RunSimulation: %v", err)
	}

	if report.ScenarioCount != 2 {
		t.Errorf("Expected 2 scenarios, got %d", report.ScenarioCount)
	}
	if report.TrialCount != 8 { // 2 scenarios × 2 agents × 2 models = 8
		t.Errorf("Expected 8 trials, got %d", report.TrialCount)
	}
	if report.Summary.BestPerformer == "" {
		t.Error("Expected best performer to be set")
	}
}

func TestGenerateFromGit(t *testing.T) {
	scenarios, err := GenerateFromGit("/tmp/nonexistent", 10)
	if err != nil {
		t.Fatalf("GenerateFromGit: %v", err)
	}

	if len(scenarios) == 0 {
		t.Error("Expected some scenarios")
	}

	for _, sc := range scenarios {
		if sc.ID == "" {
			t.Error("Expected non-empty ID")
		}
		if sc.Name == "" {
			t.Error("Expected non-empty name")
		}
		if sc.Difficulty < 1 || sc.Difficulty > 5 {
			t.Errorf("Expected difficulty 1-5, got %d", sc.Difficulty)
		}
	}
}

func TestFormatReport(t *testing.T) {
	report := &Report{
		Name:          "Test Report",
		CreatedAt:     time.Now(),
		ScenarioCount: 2,
		TrialCount:    4,
		Summary: ReportSummary{
			OverallPassRate: 75.0,
			AverageScore:    82.5,
			TotalCost:       0.15,
			BestPerformer:   "agent-a/claude-sonnet-4",
		},
		Results: []SimulationResult{
			{
				ScenarioName:  "Bug Fix",
				BestScore:     90,
				AverageScore:  85,
				PassRate:      100,
				AverageCost:   0.05,
				Winner:        "agent-a/claude-sonnet-4",
			},
		},
	}

	md := FormatReport(report)
	if md == "" {
		t.Error("Expected non-empty markdown")
	}
	if len(md) < 50 {
		t.Error("Expected detailed report")
	}
}

func TestLoadExistingScenarios(t *testing.T) {
	dir := t.TempDir()

	// Create a scenario file
	sc := Scenario{
		ID:          "test-sc",
		Name:        "Pre-existing",
		Type:        ScenarioBugFix,
		Description: "loaded from disk",
		CreatedAt:   time.Now(),
	}
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test-sc.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	retrieved, ok := store.GetScenario("test-sc")
	if !ok {
		t.Fatal("Expected to load pre-existing scenario")
	}
	if retrieved.Name != "Pre-existing" {
		t.Errorf("Expected 'Pre-existing', got %q", retrieved.Name)
	}
}
