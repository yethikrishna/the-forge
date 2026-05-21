package refactor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a simple Go project
	mainGo := `package main

import "fmt"

func main() {
	greet("world")
}

func greet(name string) {
	fmt.Println("Hello, " + name)
}
`
	helperGo := `package main

import "strings"

func upper(s string) string {
	return strings.ToUpper(s)
}

func greetUpper(name string) string {
	return upper(name)
}
`
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644)
	os.WriteFile(filepath.Join(dir, "helper.go"), []byte(helperGo), 0o644)

	return dir
}

func TestNewEngine(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.baseDir != dir {
		t.Errorf("expected baseDir %s, got %s", dir, engine.baseDir)
	}
}

func TestScanDependencies(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)

	err := engine.ScanDependencies(context.Background())
	if err != nil {
		t.Fatalf("ScanDependencies failed: %v", err)
	}

	stats := engine.Stats()
	deps, ok := stats["dependencies"].(int)
	if !ok || deps == 0 {
		t.Error("expected dependencies to be scanned")
	}
}

func TestAnalyzeImpact(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)
	engine.ScanDependencies(context.Background())

	analysis, err := engine.AnalyzeImpact(context.Background(), "main.greet", RenameSymbol)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	if analysis.Target != "main.greet" {
		t.Errorf("expected target main.greet, got %s", analysis.Target)
	}
	if analysis.Type != RenameSymbol {
		t.Errorf("expected type RenameSymbol, got %s", analysis.Type)
	}
}

func TestCreatePlan(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)
	engine.ScanDependencies(context.Background())

	plan, err := engine.CreatePlan(context.Background(), "rename-greet", RenameSymbol, "main.greet")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan.Name != "rename-greet" {
		t.Errorf("expected name rename-greet, got %s", plan.Name)
	}
	if plan.Status != "draft" {
		t.Errorf("expected status draft, got %s", plan.Status)
	}
	if plan.TotalSteps == 0 {
		t.Error("expected at least one step in plan")
	}
}

func TestCreatePlanWithDryRun(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)
	engine.ScanDependencies(context.Background())

	plan, err := engine.CreatePlan(context.Background(), "dry-run-test", RenameSymbol, "main.greet", WithDryRun(true))
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if !plan.DryRun {
		t.Error("expected dry run to be true")
	}
}

func TestExecutePlanDryRun(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)
	engine.ScanDependencies(context.Background())

	plan, err := engine.CreatePlan(context.Background(), "exec-dry", RenameSymbol, "main.greet", WithDryRun(true))
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	err = engine.ExecutePlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	updated, ok := engine.GetPlan(plan.ID)
	if !ok {
		t.Fatal("plan not found")
	}
	if updated.Status != "completed" {
		t.Errorf("expected completed, got %s", updated.Status)
	}
}

func TestGetPlanNotFound(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)

	_, ok := engine.GetPlan("nonexistent")
	if ok {
		t.Error("expected plan not to be found")
	}
}

func TestListPlans(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)
	engine.ScanDependencies(context.Background())

	engine.CreatePlan(context.Background(), "plan1", RenameSymbol, "main.greet")
	engine.CreatePlan(context.Background(), "plan2", MovePackage, "main")

	plans := engine.ListPlans()
	if len(plans) != 2 {
		t.Errorf("expected 2 plans, got %d", len(plans))
	}
}

func TestExportPlan(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)
	engine.ScanDependencies(context.Background())

	plan, _ := engine.CreatePlan(context.Background(), "export-test", RenameSymbol, "main.greet")

	outPath := filepath.Join(dir, "plan.json")
	err := engine.ExportPlan(plan.ID, outPath)
	if err != nil {
		t.Fatalf("ExportPlan failed: %v", err)
	}

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("export file was not created")
	}
}

func TestRiskCalculation(t *testing.T) {
	dir := setupTestDir(t)
	engine := NewEngine(dir)

	// Safe: 0 files
	analysis := &ImpactAnalysis{DirectFiles: []string{}, IndirectFiles: []string{}}
	if engine.calculateRisk(analysis) != RiskSafe {
		t.Error("expected RiskSafe for 0 files")
	}

	// Low: 1-3 files, non-breaking
	analysis = &ImpactAnalysis{DirectFiles: []string{"a.go"}, IndirectFiles: []string{}, BreakingChange: false}
	if engine.calculateRisk(analysis) != RiskLow {
		t.Error("expected RiskLow for 1 file, non-breaking")
	}

	// Medium: 4-10 files
	files := make([]string, 5)
	for i := range files {
		files[i] = fmt.Sprintf("file%d.go", i)
	}
	analysis = &ImpactAnalysis{DirectFiles: files, IndirectFiles: []string{}}
	if engine.calculateRisk(analysis) != RiskMedium {
		t.Error("expected RiskMedium for 5 files")
	}

	// High: 11-30 files
	dangerFiles := make([]string, 15)
	for i := range dangerFiles {
		dangerFiles[i] = fmt.Sprintf("file%d.go", i)
	}
	analysis = &ImpactAnalysis{DirectFiles: dangerFiles, IndirectFiles: []string{}}
	if engine.calculateRisk(analysis) != RiskHigh {
		t.Error("expected RiskHigh for 15 files")
	}
}

func TestIsBreakingChange(t *testing.T) {
	engine := NewEngine(".")

	tests := []struct {
		refType   RefactorType
		breaking  bool
	}{
		{ChangeSignature, true},
		{RemoveParam, true},
		{ChangeType, true},
		{MovePackage, true},
		{MergePackage, true},
		{RenameSymbol, false},
		{ExtractFunc, false},
		{InlineFunc, false},
		{AddParam, false},
		{SplitPackage, false},
	}

	for _, tt := range tests {
		result := engine.isBreakingChange(tt.refType)
		if result != tt.breaking {
			t.Errorf("isBreakingChange(%s) = %v, want %v", tt.refType, result, tt.breaking)
		}
	}
}

func TestEngineStats(t *testing.T) {
	engine := NewEngine(".")

	stats := engine.Stats()
	if stats["plans"] != 0 {
		t.Errorf("expected 0 plans, got %v", stats["plans"])
	}
	if stats["dependencies"] != 0 {
		t.Errorf("expected 0 dependencies, got %v", stats["dependencies"])
	}
}
