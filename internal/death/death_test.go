package death

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "death.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.ShutdownPlans = append(s.ShutdownPlans, ShutdownPlan{ID: "sp_1", Reason: "financial"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.ShutdownPlans) != 1 || s2.ShutdownPlans[0].Reason != "financial" {
		t.Errorf("unexpected after load")
	}
}

func TestPlanShutdown(t *testing.T) {
	sp := PlanShutdown("org1", "strategic", "6 months",
		[]string{"notify", "wind_down", "archive"}, true)
	if sp.Status != "draft" {
		t.Errorf("expected draft, got %s", sp.Status)
	}
	if !sp.DataMigration {
		t.Error("expected data_migration true")
	}
	if len(sp.Phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(sp.Phases))
	}
}

func TestPlanSuccession(t *testing.T) {
	scp := PlanSuccession("org1", "CTO", "Alice", "Bob", "6-month overlap", 0.6)
	if scp.Readiness != 0.6 {
		t.Errorf("expected 0.6 readiness, got %.2f", scp.Readiness)
	}
	if scp.Status != "planned" {
		t.Errorf("expected planned, got %s", scp.Status)
	}
	if scp.Successor != "Bob" {
		t.Errorf("expected Bob, got %s", scp.Successor)
	}
}

func TestDefineLegacy(t *testing.T) {
	li := DefineLegacy("org1", "knowledge", "Architecture Docs", "Core system design", "wiki", "critical")
	if li.Type != "knowledge" || li.Priority != "critical" {
		t.Errorf("unexpected legacy item: %+v", li)
	}
}

func TestCreateArchive(t *testing.T) {
	ar := CreateArchive("org1", "code", "s3://archive/org1", "abc123", 1024000, time.Now().AddDate(7, 0, 0))
	if ar.ContentType != "code" || ar.SizeBytes != 1024000 {
		t.Errorf("unexpected archive: %+v", ar)
	}
}

func TestExecuteDirective(t *testing.T) {
	dd := ExecuteDirective("org1", "financial_threshold", "initiate_shutdown",
		[]string{"cash < 3_months", "board_approval"}, 1)
	if !dd.IsActive {
		t.Error("expected directive to be active")
	}
	if dd.Priority != 1 {
		t.Errorf("expected priority 1, got %d", dd.Priority)
	}
	if len(dd.Conditions) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(dd.Conditions))
	}
}

func TestGenerateDeathReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.LegacyItems = append(s.LegacyItems, LegacyItem{ID: "li_1"})
	s.DeathDirectives = append(s.DeathDirectives, DeathDirective{ID: "dd_1"})
	report := GenerateDeathReport(s)
	if len(report.LegacyItems) != 1 || len(report.DeathDirectives) != 1 {
		t.Errorf("unexpected report contents")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
