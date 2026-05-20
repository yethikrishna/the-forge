package migrate

import (
	"strings"
	"testing"
	"time"
)

func TestStartMigration(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	mig, err := m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mig.ID == "" {
		t.Error("expected non-empty migration ID")
	}
	if mig.FromModel != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", mig.FromModel)
	}
	if mig.ToModel != "claude-sonnet-4" {
		t.Errorf("expected claude-sonnet-4, got %s", mig.ToModel)
	}
	if mig.Status != StatusInProgress {
		t.Errorf("expected in_progress, got %s", mig.Status)
	}
}

func TestCompleteMigration(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	mig, _ := m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	err := m.CompleteMigration(mig.ID, 0.03, 92.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.GetMigration(mig.ID)
	if got.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", got.Status)
	}
	if got.QualityAfter != 92.5 {
		t.Errorf("expected 92.5, got %f", got.QualityAfter)
	}
	if got.Duration == "" {
		t.Error("expected non-empty duration")
	}
}

func TestFailMigration(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	mig, _ := m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	m.FailMigration(mig.ID, "context window exceeded")

	got, _ := m.GetMigration(mig.ID)
	if got.Status != StatusFailed {
		t.Errorf("expected failed, got %s", got.Status)
	}
	if !strings.Contains(got.Notes, "context window exceeded") {
		t.Errorf("expected notes to contain error, got: %s", got.Notes)
	}
}

func TestRollbackMigration(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	mig, _ := m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	m.CompleteMigration(mig.ID, 0.03, 92.5)

	rollback, err := m.RollbackMigration(mig.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rollback.FromModel != "claude-sonnet-4" {
		t.Errorf("expected reverse from, got %s", rollback.FromModel)
	}
	if rollback.ToModel != "gpt-4" {
		t.Errorf("expected reverse to, got %s", rollback.ToModel)
	}

	original, _ := m.GetMigration(mig.ID)
	if original.Status != StatusRolledBack {
		t.Errorf("expected rolled_back, got %s", original.Status)
	}
}

func TestRollbackNotCompleted(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	mig, _ := m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	_, err := m.RollbackMigration(mig.ID)
	if err == nil {
		t.Error("expected error for non-completed migration")
	}
}

func TestGetMigrationNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	_, ok := m.GetMigration("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestListMigrations(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	m.StartMigration("agent-2", "gpt-4.1-mini", "deepseek-v3")

	list := m.ListMigrations()
	if len(list) != 2 {
		t.Errorf("expected 2 migrations, got %d", len(list))
	}
}

func TestListByAgent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	m.StartMigration("agent-2", "gpt-4.1-mini", "deepseek-v3")
	m.StartMigration("agent-1", "claude-sonnet-4", "gpt-4.1")

	agent1 := m.ListByAgent("agent-1")
	if len(agent1) != 2 {
		t.Errorf("expected 2 migrations for agent-1, got %d", len(agent1))
	}
}

func TestStartABTest(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	test := m.StartABTest("agent-1", "gpt-4", "claude-sonnet-4", "Write a REST API handler")

	if test.ID == "" {
		t.Error("expected non-empty test ID")
	}
	if test.Winner == "" {
		t.Error("expected a winner to be determined")
	}
	if test.CostA <= 0 || test.CostB <= 0 {
		t.Error("expected positive costs")
	}
}

func TestListABTests(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.StartABTest("agent-1", "gpt-4", "claude-sonnet-4", "Test 1")
	m.StartABTest("agent-1", "gpt-4.1", "deepseek-v3", "Test 2")

	tests := m.ListABTests()
	if len(tests) != 2 {
		t.Errorf("expected 2 AB tests, got %d", len(tests))
	}
}

func TestGetABTest(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	test := m.StartABTest("agent-1", "gpt-4", "claude-sonnet-4", "Test")

	got, ok := m.GetABTest(test.ID)
	if !ok {
		t.Fatal("expected to find AB test")
	}
	if got.ModelA != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", got.ModelA)
	}
}

func TestMigrationReport(t *testing.T) {
	now := time.Now()
	mig := &Migration{
		ID:            "mig-test",
		AgentID:       "agent-1",
		FromModel:     "gpt-4",
		ToModel:       "claude-sonnet-4",
		Status:        StatusCompleted,
		StartedAt:     &now,
		CompletedAt:   &now,
		Duration:      "2s",
		ContextTokens: 4096,
		MemoryEntries: 42,
		QualityBefore: 80.0,
		QualityAfter:  92.5,
		CostBefore:    0.05,
		CostAfter:     0.03,
	}

	report := MigrationReport(mig)
	if !strings.Contains(report, "gpt-4") || !strings.Contains(report, "claude-sonnet-4") {
		t.Error("expected model names in report")
	}
	if !strings.Contains(report, "completed") {
		t.Error("expected status in report")
	}
}

func TestABTestReport(t *testing.T) {
	test := &ABTest{
		ID:       "ab-test",
		AgentID:  "agent-1",
		ModelA:   "gpt-4",
		ModelB:   "claude-sonnet-4",
		Prompt:   "Write code",
		CostA:    0.05,
		CostB:    0.03,
		QualityA: 85.0,
		QualityB: 88.0,
		LatencyA: "1.2s",
		LatencyB: "0.8s",
		Winner:   "claude-sonnet-4",
	}

	report := ABTestReport(test)
	if !strings.Contains(report, "claude-sonnet-4") {
		t.Error("expected model name in report")
	}
	if !strings.Contains(report, "Winner") {
		t.Error("expected winner in report")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	mig, _ := m.StartMigration("agent-2", "gpt-4", "deepseek-v3")
	m.CompleteMigration(mig.ID, 0.02, 90.0)
	m.StartABTest("agent-1", "gpt-4", "claude-sonnet-4", "Test")

	stats := m.Stats()
	if stats["total_migrations"] != 2 {
		t.Errorf("expected 2 total, got %v", stats["total_migrations"])
	}
	if stats["completed"] != 1 {
		t.Errorf("expected 1 completed, got %v", stats["completed"])
	}
	if stats["ab_tests"] != 1 {
		t.Errorf("expected 1 AB test, got %v", stats["ab_tests"])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	mig, _ := m1.StartMigration("agent-1", "gpt-4", "claude-sonnet-4")
	m1.CompleteMigration(mig.ID, 0.03, 92.5)

	m2 := NewManager(dir)
	migrations := m2.ListMigrations()
	if len(migrations) != 1 {
		t.Fatalf("expected 1 migration after reload, got %d", len(migrations))
	}
	if migrations[0].Status != StatusCompleted {
		t.Errorf("expected completed, got %s", migrations[0].Status)
	}
}
