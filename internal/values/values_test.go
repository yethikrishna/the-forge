package values

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "values.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestDefineValues(t *testing.T) {
	s := tempStore(t)

	err := s.DefineValues(
		Value{Name: "Privacy First", Description: "Protect user data above all", Weight: 0.9, Category: "privacy", NonNegotiable: true},
		Value{Name: "Move Fast", Description: "Ship quickly", Weight: 0.5, Category: "operational"},
	)
	if err != nil {
		t.Fatalf("DefineValues: %v", err)
	}

	vals := s.ListValues()
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d", len(vals))
	}
	if vals[0].Name != "Privacy First" {
		t.Errorf("expected 'Privacy First', got %q", vals[0].Name)
	}
	if vals[0].ID == "" {
		t.Error("expected non-empty ID")
	}
	if vals[0].CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestDefineValuesPersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "values.json")

	s1 := NewStore(fp)
	s1.Load()
	s1.DefineValues(Value{Name: "Test", Description: "desc", Weight: 0.5, Category: "ethical"})

	s2 := NewStore(fp)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	vals := s2.ListValues()
	if len(vals) != 1 {
		t.Fatalf("expected 1 value after reload, got %d", len(vals))
	}
	if vals[0].Name != "Test" {
		t.Errorf("expected 'Test', got %q", vals[0].Name)
	}
}

func TestMissionStatement(t *testing.T) {
	s := tempStore(t)

	err := s.AddMission(MissionStatement{
		Statement:  "Democratize AI for everyone",
		ApprovedBy: "founder",
		Version:    1,
	})
	if err != nil {
		t.Fatalf("AddMission: %v", err)
	}

	mission := s.GetActiveMission()
	if mission == nil {
		t.Fatal("expected active mission")
	}
	if mission.Statement != "Democratize AI for everyone" {
		t.Errorf("unexpected statement: %q", mission.Statement)
	}
	if !mission.Active {
		t.Error("expected mission to be active")
	}

	// Add a new mission — should deactivate the old one
	s.AddMission(MissionStatement{
		Statement:  "Build safe AI systems",
		ApprovedBy: "board",
		Version:    2,
	})

	mission2 := s.GetActiveMission()
	if mission2.Statement != "Build safe AI systems" {
		t.Errorf("expected new mission, got %q", mission2.Statement)
	}
}

func TestCheckMissionAlignment(t *testing.T) {
	s := tempStore(t)

	s.DefineValues(
		Value{Name: "Privacy", Description: "Protect user data", Weight: 0.9, Category: "privacy", NonNegotiable: true},
		Value{Name: "Safety", Description: "Ensure safe operations", Weight: 0.8, Category: "safety"},
	)

	s.AddMission(MissionStatement{Statement: "Build safe AI systems", Version: 1, ApprovedBy: "founder"})

	// Action that should align well
	check, err := s.CheckMissionAlignment("build safe AI system", "product")
	if err != nil {
		t.Fatalf("CheckMissionAlignment: %v", err)
	}
	if check.Score < 0.5 {
		t.Errorf("expected high alignment score, got %.2f", check.Score)
	}

	// Action that should conflict
	check2, _ := s.CheckMissionAlignment("expose user data to public", "database")
	if check2.Score >= 0.8 {
		t.Errorf("expected lower score for conflicting action, got %.2f", check2.Score)
	}
	if len(check2.Conflicts) == 0 {
		t.Error("expected conflicts for privacy-violating action")
	}
}

func TestApplyEthics(t *testing.T) {
	s := tempStore(t)

	s.AddFramework(EthicalFramework{
		Name: "Core Ethics",
		Principles: []EthicalRule{
			{ID: "r1", Description: "Never discriminate", Severity: "absolute", Category: "fairness"},
			{ID: "r2", Description: "Respect user privacy", Severity: "mandatory", Category: "privacy"},
		},
		RedLines: []string{"never sell user data"},
	})

	violations, blocked := s.ApplyEthics("discriminate against users")
	if len(violations) == 0 {
		t.Error("expected violations for discriminatory action")
	}
	if !blocked {
		t.Error("expected blocked=true for absolute violation")
	}

	violations2, blocked2 := s.ApplyEthics("ship feature normally")
	if len(violations2) > 0 {
		t.Error("expected no violations for normal action")
	}
	if blocked2 {
		t.Error("expected blocked=false for clean action")
	}

	violations3, blocked3 := s.ApplyEthics("sell user data to third party")
	if !blocked3 {
		t.Error("expected blocked for red line hit")
	}
	if len(violations3) == 0 {
		t.Error("expected violations for red line hit")
	}
}

func TestPersonality(t *testing.T) {
	s := tempStore(t)

	p := s.AssessPersonality()
	if p.Tone != "balanced" {
		t.Errorf("expected default balanced tone, got %q", p.Tone)
	}

	err := s.SetPersonality(OrgPersonality{
		Tone:           "technical",
		RiskAppetite:   0.7,
		InnovationBias: 0.8,
		Collaboration:  0.6,
		Transparency:   0.9,
		SpeedVsQuality: 0.4,
	})
	if err != nil {
		t.Fatalf("SetPersonality: %v", err)
	}

	p2 := s.AssessPersonality()
	if p2.Tone != "technical" {
		t.Errorf("expected technical tone, got %q", p2.Tone)
	}
	if p2.RiskAppetite != 0.7 {
		t.Errorf("expected risk 0.7, got %.2f", p2.RiskAppetite)
	}
}

func TestRunValuesAudit(t *testing.T) {
	s := tempStore(t)

	s.DefineValues(
		Value{Name: "Privacy", Description: "d", Weight: 0.9, Category: "privacy", NonNegotiable: true},
		Value{Name: "Speed", Description: "d", Weight: 0.5, Category: "operational"},
		Value{Name: "Fairness", Description: "d", Weight: 0.8, Category: "fairness"},
	)

	report, err := s.RunValuesAudit()
	if err != nil {
		t.Fatalf("RunValuesAudit: %v", err)
	}
	if report.TotalValues != 3 {
		t.Errorf("expected 3 values, got %d", report.TotalValues)
	}
	if report.NonNegotiables != 1 {
		t.Errorf("expected 1 non-negotiable, got %d", report.NonNegotiables)
	}
	if report.ValuesByCategory["privacy"] != 1 {
		t.Errorf("expected 1 privacy value, got %d", report.ValuesByCategory["privacy"])
	}
}

func TestGenerateValuesReport(t *testing.T) {
	s := tempStore(t)
	s.DefineValues(Value{Name: "V1", Description: "d", Weight: 0.5, Category: "ethical"})
	s.AddMission(MissionStatement{Statement: "Test Mission", Version: 1, ApprovedBy: "test"})

	report := s.GenerateValuesReport()
	if report == "" {
		t.Error("expected non-empty report")
	}
	if !containsSubstring(report, "Values Report") {
		t.Error("expected 'Values Report' in output")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "values.json")

	s := NewStore(fp)
	s.Load()
	s.DefineValues(Value{Name: "Persist", Description: "d", Weight: 1.0, Category: "ethical"})
	s.AddMission(MissionStatement{Statement: "M", Version: 1, ApprovedBy: "a"})

	// Verify file exists
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		t.Fatal("expected values.json to exist")
	}

	s2 := NewStore(fp)
	s2.Load()

	if len(s2.ListValues()) != 1 {
		t.Error("expected 1 value after reload")
	}
	if s2.GetActiveMission() == nil {
		t.Error("expected active mission after reload")
	}
}

func TestScoreToAlignment(t *testing.T) {
	tests := []struct {
		score    float64
		expected AlignmentLevel
	}{
		{0.9, AlignmentFull},
		{0.7, AlignmentPartial},
		{0.5, AlignmentMinimal},
		{0.3, AlignmentConflict},
		{0.1, AlignmentNone},
	}
	for _, tt := range tests {
		got := scoreToAlignment(tt.score)
		if got != tt.expected {
			t.Errorf("scoreToAlignment(%.1f) = %q, want %q", tt.score, got, tt.expected)
		}
	}
}

func TestContainsSubstring(t *testing.T) {
	if !containsSubstring("hello world", "world") {
		t.Error("expected true")
	}
	if containsSubstring("hello", "world") {
		t.Error("expected false")
	}
	if !containsSubstring("abc", "abc") {
		t.Error("expected true for equal strings")
	}
}

func TestTimeNowNotZero(t *testing.T) {
	before := time.Now()
	s := tempStore(t)
	s.DefineValues(Value{Name: "T", Description: "d", Weight: 0.5, Category: "ethical"})
	after := time.Now()

	vals := s.ListValues()
	if vals[0].CreatedAt.Before(before) || vals[0].CreatedAt.After(after) {
		t.Error("CreatedAt not in expected range")
	}
}
