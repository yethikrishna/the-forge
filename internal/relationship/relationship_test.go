package relationship

import (
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "relationships.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestBuildPartnership(t *testing.T) {
	s := tempStore(t)

	p, err := s.BuildPartnership("Acme Corp", PartnershipStrategic)
	if err != nil {
		t.Fatalf("BuildPartnership: %v", err)
	}
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.Name != "Acme Corp" {
		t.Errorf("expected 'Acme Corp', got %q", p.Name)
	}
	if p.TrustLevel != TrustInitial {
		t.Errorf("expected initial trust, got %s", p.TrustLevel)
	}
	if p.StartDate.IsZero() {
		t.Error("expected StartDate to be set")
	}
}

func TestMeasureTrust(t *testing.T) {
	s := tempStore(t)

	p, _ := s.BuildPartnership("TestCo", PartnershipClient)

	// Add positive records
	s.TrackRelationship(p.ID, "delivery", "On-time delivery", 0.8)
	s.TrackRelationship(p.ID, "meeting", "Productive meeting", 0.5)

	level, score, err := s.MeasureTrust(p.ID)
	if err != nil {
		t.Fatalf("MeasureTrust: %v", err)
	}
	if score <= 0.1 {
		t.Errorf("expected trust to increase, got %.2f", score)
	}
	if level == TrustInitial {
		t.Errorf("expected trust level to advance, got %s", level)
	}
}

func TestMeasureTrustNegative(t *testing.T) {
	s := tempStore(t)

	p, _ := s.BuildPartnership("BadCo", PartnershipVendor)

	// Add negative records
	s.TrackRelationship(p.ID, "issue", "Missed deadline", -0.7)
	s.TrackRelationship(p.ID, "issue", "Quality problems", -0.8)

	_, score, _ := s.MeasureTrust(p.ID)
	if score >= 0.5 {
		t.Errorf("expected low trust after negative events, got %.2f", score)
	}
}

func TestMeasureTrustNotFound(t *testing.T) {
	s := tempStore(t)
	_, _, err := s.MeasureTrust("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent partnership")
	}
}

func TestAssessHealth(t *testing.T) {
	s := tempStore(t)

	p, _ := s.BuildPartnership("HealthCo", PartnershipStrategic)
	s.TrackRelationship(p.ID, "meeting", "Quarterly review", 0.6)
	s.TrackRelationship(p.ID, "delivery", "Feature delivered", 0.8)
	s.TrackRelationship(p.ID, "delivery", "Another feature", 0.7)

	health, err := s.AssessHealth(p.ID)
	if err != nil {
		t.Fatalf("AssessHealth: %v", err)
	}
	if health.OverallScore <= 0 {
		t.Errorf("expected positive health score, got %.2f", health.OverallScore)
	}
	if health.CommunicationScore <= 0 {
		t.Error("expected positive communication score")
	}
	if health.DeliveryScore <= 0 {
		t.Error("expected positive delivery score")
	}
}

func TestAssessHealthWithIssues(t *testing.T) {
	s := tempStore(t)

	p, _ := s.BuildPartnership("StrainedCo", PartnershipVendor)
	s.TrackRelationship(p.ID, "issue", "Major outage", -0.9)
	s.TrackRelationship(p.ID, "issue", "Data breach", -0.8)
	s.TrackRelationship(p.ID, "issue", "SLA missed again", -0.6)
	s.TrackRelationship(p.ID, "issue", "Complaint filed", -0.7)

	health, _ := s.AssessHealth(p.ID)
	if len(health.Warnings) == 0 {
		t.Error("expected warnings for partnership with many issues")
	}
}

func TestAssessHealthNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.AssessHealth("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent partnership")
	}
}

func TestPlanRepair(t *testing.T) {
	s := tempStore(t)

	p, _ := s.BuildPartnership("RepairCo", PartnershipClient)
	s.TrackRelationship(p.ID, "issue", "Serious issue", -0.9)

	// Force low trust for repair
	p.TrustScore = 0.2

	actions, err := s.PlanRepair(p.ID)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected repair actions")
	}
	for _, a := range actions {
		if a.PartnershipID != p.ID {
			t.Errorf("wrong partnership ID: %s", a.PartnershipID)
		}
		if a.Status != "pending" {
			t.Errorf("expected pending status, got %s", a.Status)
		}
	}
}

func TestPlanRepairHealthy(t *testing.T) {
	s := tempStore(t)

	p, _ := s.BuildPartnership("HealthyCo", PartnershipStrategic)
	actions, _ := s.PlanRepair(p.ID)
	// Should still get a low-priority check-in
	if len(actions) == 0 {
		t.Error("expected at least a check-in action for healthy partnership")
	}
}

func TestTrackRelationship(t *testing.T) {
	s := tempStore(t)

	p, _ := s.BuildPartnership("TrackCo", PartnershipTechnology)

	rec, err := s.TrackRelationship(p.ID, "meeting", "Kickoff meeting", 0.5)
	if err != nil {
		t.Fatalf("TrackRelationship: %v", err)
	}
	if rec.ID == "" {
		t.Error("expected non-empty record ID")
	}
	if rec.EventType != "meeting" {
		t.Errorf("expected 'meeting', got %q", rec.EventType)
	}

	partnerships := s.ListPartnerships()
	if partnerships[0].Interactions != 1 {
		t.Errorf("expected 1 interaction, got %d", partnerships[0].Interactions)
	}
}

func TestTrackRelationshipNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.TrackRelationship("nonexistent", "meeting", "test", 0.5)
	if err == nil {
		t.Error("expected error for nonexistent partnership")
	}
}

func TestGenerateRelationshipReport(t *testing.T) {
	s := tempStore(t)

	s.BuildPartnership("ReportCo", PartnershipStrategic)
	s.BuildPartnership("ReportCo2", PartnershipClient)

	report := s.GenerateRelationshipReport()
	if report == "" {
		t.Error("expected non-empty report")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "relationships.json")

	s1 := NewStore(fp)
	s1.Load()
	s1.BuildPartnership("PersistCo", PartnershipVendor)

	s2 := NewStore(fp)
	s2.Load()
	pList := s2.ListPartnerships()
	if len(pList) != 1 {
		t.Fatalf("expected 1 partnership after reload, got %d", len(pList))
	}
	if pList[0].Name != "PersistCo" {
		t.Errorf("expected 'PersistCo', got %q", pList[0].Name)
	}
}

func TestScoreToTrustLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  TrustLevel
	}{
		{0.9, TrustStrategic},
		{0.7, TrustDeep},
		{0.5, TrustEstablished},
		{0.3, TrustBuilding},
		{0.1, TrustInitial},
		{-0.1, TrustBroken},
	}
	for _, tt := range tests {
		got := scoreToTrustLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreToTrustLevel(%.1f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestScoreToHealthStatus(t *testing.T) {
	tests := []struct {
		score float64
		want  HealthStatus
	}{
		{0.9, HealthThriving},
		{0.7, HealthHealthy},
		{0.5, HealthStable},
		{0.3, HealthStrained},
		{0.15, HealthAtRisk},
		{0.05, HealthBroken},
	}
	for _, tt := range tests {
		got := scoreToHealthStatus(tt.score)
		if got != tt.want {
			t.Errorf("scoreToHealthStatus(%.1f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestMinHelper(t *testing.T) {
	if min(3, 5) != 3 {
		t.Error("min(3,5) should be 3")
	}
	if min(5, 3) != 3 {
		t.Error("min(5,3) should be 3")
	}
}

func TestLastContactUpdated(t *testing.T) {
	s := tempStore(t)
	p, _ := s.BuildPartnership("ContactCo", PartnershipClient)

	before := time.Now()
	s.TrackRelationship(p.ID, "meeting", "Test", 0.5)

	partnerships := s.ListPartnerships()
	if partnerships[0].LastContact.Before(before) {
		t.Error("LastContact should be updated")
	}
}
