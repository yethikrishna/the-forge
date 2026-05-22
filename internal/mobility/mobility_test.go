package mobility

import (
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "mobility.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestPlanMigration(t *testing.T) {
	s := tempStore(t)

	plan, err := s.PlanMigration("Move to GCP", MigrationCloud, "aws-us-east-1")
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if plan.ID == "" {
		t.Error("expected non-empty plan ID")
	}
	if plan.Type != MigrationCloud {
		t.Errorf("expected cloud migration, got %s", plan.Type)
	}
	if plan.Status.Phase != PhasePlanning {
		t.Errorf("expected planning phase, got %s", plan.Status.Phase)
	}
	if len(plan.Steps) == 0 {
		t.Error("expected default steps")
	}
}

func TestPlanMigrationAllTypes(t *testing.T) {
	s := tempStore(t)

	types := []MigrationType{MigrationCloud, MigrationModel, MigrationJurisdiction, MigrationTechStack}
	for _, mt := range types {
		plan, err := s.PlanMigration("test-"+string(mt), mt, "source")
		if err != nil {
			t.Fatalf("PlanMigration %s: %v", mt, err)
		}
		if plan.Type != mt {
			t.Errorf("expected %s, got %s", mt, plan.Type)
		}
	}
}

func TestAssessPortability(t *testing.T) {
	s := tempStore(t)

	assess, err := s.AssessPortability("kubernetes docker terraform", MigrationCloud)
	if err != nil {
		t.Fatalf("AssessPortability: %v", err)
	}
	if assess.Score <= 0 {
		t.Error("expected positive portability score")
	}
	if assess.Score > 1 {
		t.Errorf("score should be <= 1.0, got %.2f", assess.Score)
	}
}

func TestAssessPortabilityVendorLockIn(t *testing.T) {
	s := tempStore(t)

	assess, _ := s.AssessPortability("proprietary vendor-specific locked", MigrationCloud)
	if assess.VendorLockInRisk < 0.5 {
		t.Errorf("expected high vendor lock-in risk, got %.2f", assess.VendorLockInRisk)
	}
	if len(assess.Blockers) == 0 {
		t.Error("expected blockers for proprietary source")
	}
}

func TestExecuteMigration(t *testing.T) {
	s := tempStore(t)

	plan, _ := s.PlanMigration("Test Migration", MigrationCloud, "source")

	status, err := s.ExecuteMigration(plan.ID)
	if err != nil {
		t.Fatalf("ExecuteMigration: %v", err)
	}
	if status.Phase != PhaseComplete {
		t.Errorf("expected complete phase, got %s", status.Phase)
	}
	if status.Progress != 1.0 {
		t.Errorf("expected 1.0 progress, got %.2f", status.Progress)
	}
	if status.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if status.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestExecuteMigrationNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.ExecuteMigration("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plan")
	}
}

func TestExecuteMigrationAlreadyComplete(t *testing.T) {
	s := tempStore(t)
	plan, _ := s.PlanMigration("Test", MigrationModel, "source")
	s.ExecuteMigration(plan.ID)
	_, err := s.ExecuteMigration(plan.ID)
	if err == nil {
		t.Error("expected error for already-complete plan")
	}
}

func TestCheckMigrationStatus(t *testing.T) {
	s := tempStore(t)
	plan, _ := s.PlanMigration("Test", MigrationCloud, "source")

	status, err := s.CheckMigrationStatus(plan.ID)
	if err != nil {
		t.Fatalf("CheckMigrationStatus: %v", err)
	}
	if status.Phase != PhasePlanning {
		t.Errorf("expected planning, got %s", status.Phase)
	}
}

func TestCheckMigrationStatusNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.CheckMigrationStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plan")
	}
}

func TestGenerateMobilityReport(t *testing.T) {
	s := tempStore(t)
	s.PlanMigration("Move to Azure", MigrationCloud, "aws")
	s.ExecuteMigration(s.ListPlans()[0].ID)

	report := s.GenerateMobilityReport()
	if report == "" {
		t.Error("expected non-empty report")
	}
	if !containsAny(report, "Mobility Report") {
		t.Error("expected 'Mobility Report' in report")
	}
}

func TestListPlans(t *testing.T) {
	s := tempStore(t)
	s.PlanMigration("P1", MigrationCloud, "src1")
	s.PlanMigration("P2", MigrationModel, "src2")

	plans := s.ListPlans()
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "mobility.json")

	s1 := NewStore(fp)
	s1.Load()
	s1.PlanMigration("Persistent", MigrationCloud, "src")

	s2 := NewStore(fp)
	s2.Load()
	plans := s2.ListPlans()
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan after reload, got %d", len(plans))
	}
	if plans[0].Name != "Persistent" {
		t.Errorf("expected 'Persistent', got %q", plans[0].Name)
	}
}

func TestDefaultSteps(t *testing.T) {
	steps := defaultSteps(MigrationCloud)
	if len(steps) < 4 {
		t.Errorf("expected at least 4 default steps, got %d", len(steps))
	}
	for i, step := range steps {
		if step.ID == "" {
			t.Errorf("step %d missing ID", i)
		}
	}
}

func TestTimeFieldsSet(t *testing.T) {
	s := tempStore(t)
	before := time.Now()
	plan, _ := s.PlanMigration("TimeTest", MigrationCloud, "src")
	after := time.Now()

	if plan.CreatedAt.Before(before) || plan.CreatedAt.After(after) {
		t.Error("CreatedAt not in expected range")
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello world", "hello") {
		t.Error("expected true for existing substring")
	}
	if containsAny("hello", "world") {
		t.Error("expected false for missing substring")
	}
	if !containsAny("kubernetes docker", "docker") {
		t.Error("expected true for docker")
	}
}

func TestAssessPortabilityScores(t *testing.T) {
	// Kubernetes should score higher than proprietary
	score1 := assessPortability("kubernetes docker terraform", MigrationCloud)
	score2 := assessPortability("proprietary vendor-specific", MigrationCloud)
	if score1 <= score2 {
		t.Errorf("expected kubernetes (%.2f) > proprietary (%.2f)", score1, score2)
	}
}

func TestIdentifyBlockers(t *testing.T) {
	blockers := identifyBlockers("proprietary custom fine-tuned", MigrationCloud)
	if len(blockers) < 2 {
		t.Errorf("expected at least 2 blockers, got %d", len(blockers))
	}
}

func TestGenerateRecommendations(t *testing.T) {
	recs := generateRecommendations(MigrationCloud, 0.3)
	if len(recs) == 0 {
		t.Error("expected recommendations")
	}
	recs2 := generateRecommendations(MigrationJurisdiction, 0.8)
	if len(recs2) == 0 {
		t.Error("expected recommendations for jurisdiction migration")
	}
}
