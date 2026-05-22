package time

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCreateClock(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, err := tc.CreateClock("Engineering", "team-eng", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if clock.Name != "Engineering" {
		t.Error("name mismatch")
	}
	if !clock.Active {
		t.Error("should be active")
	}
	clocks := tc.ListClocks()
	if len(clocks) != 1 {
		t.Errorf("expected 1 clock, got %d", len(clocks))
	}
}

func TestTrackDeadline(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Eng", "team-eng", "UTC")

	due := time.Now().UTC().Add(5 * 24 * time.Hour)
	dl, err := tc.TrackDeadline(clock.ID, "Ship v2", "Release version 2", "team-eng", due, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dl.Status != DeadlineActive {
		t.Error("should be active")
	}
	if dl.Urgency == UrgencyCritical {
		t.Error("5 days out should not be critical")
	}
	if dl.ProgressPct != 0 {
		t.Error("should start at 0")
	}
}

func TestDeadlineUrgency(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Test", "t1", "UTC")

	// Due in 1 hour → critical
	dl1, _ := tc.TrackDeadline(clock.ID, "Urgent", "", "t1", time.Now().UTC().Add(1*time.Hour), nil)
	if dl1.Urgency != UrgencyCritical {
		t.Errorf("expected critical, got %s", dl1.Urgency)
	}

	// Due in 2 days → high
	dl2, _ := tc.TrackDeadline(clock.ID, "Soon", "", "t1", time.Now().UTC().Add(48*time.Hour), nil)
	if dl2.Urgency != UrgencyHigh {
		t.Errorf("expected high, got %s", dl2.Urgency)
	}

	// Due in 10 days → low
	dl3, _ := tc.TrackDeadline(clock.ID, "Later", "", "t1", time.Now().UTC().Add(10*24*time.Hour), nil)
	if dl3.Urgency != UrgencyLow {
		t.Errorf("expected low, got %s", dl3.Urgency)
	}
}

func TestUpdateProgress(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Test", "t1", "UTC")
	dl, _ := tc.TrackDeadline(clock.ID, "Feature", "", "t1", time.Now().UTC().Add(7*24*time.Hour), nil)

	tc.UpdateProgress(dl.ID, 50)
	dl, _ = tc.deadlines[dl.ID]
	if dl.ProgressPct != 50 {
		t.Error("progress should be 50")
	}

	tc.UpdateProgress(dl.ID, 100)
	dl, _ = tc.deadlines[dl.ID]
	if dl.Status != DeadlineMet {
		t.Error("should be met at 100%")
	}
	if dl.MetAt == nil {
		t.Error("should have met_at")
	}
}

func TestEscalateUrgency(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Test", "t1", "UTC")
	dl, _ := tc.TrackDeadline(clock.ID, "Late", "", "t1", time.Now().UTC().Add(10*24*time.Hour), nil)

	escalated, err := tc.EscalateUrgency(dl.ID, []string{"vp-eng", "cto"}, "blocked by infra")
	if err != nil {
		t.Fatal(err)
	}
	if escalated.Status != DeadlineEscalated {
		t.Error("should be escalated")
	}
	if escalated.Urgency != UrgencyCritical {
		t.Error("escalated should be critical")
	}
	if len(escalated.EscalatedTo) != 2 {
		t.Error("should have 2 escalation targets")
	}
}

func TestBurnRate(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Eng", "t1", "UTC")

	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour)

	tc.RecordBurnSample(clock.ID, 45, 40) // over capacity
	tc.RecordBurnSample(clock.ID, 42, 40)
	tc.RecordBurnSample(clock.ID, 38, 40)
	tc.RecordBurnSample(clock.ID, 44, 40)

	report, err := tc.CalculateBurnRate(clock.ID, start, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if report.BurnRate <= 0 {
		t.Error("burn rate should be positive")
	}
	if report.Trend == "" {
		t.Error("trend should be set")
	}
	if report.HoursWorked != 45+42+38+44 {
		t.Errorf("expected %f hours worked", 45.0+42+38+44)
	}
}

func TestBurnRateAtRisk(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Burnout", "t1", "UTC")

	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour)

	// Record high burn samples
	tc.RecordBurnSample(clock.ID, 39, 40) // 0.975 per sample
	tc.RecordBurnSample(clock.ID, 39, 40)

	report, _ := tc.CalculateBurnRate(clock.ID, start, now.Add(time.Minute))
	if len(report.AtRiskMembers) == 0 {
		t.Error("high burn rate should flag at-risk members")
	}
}

func TestSuggestPacing(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Test", "t1", "UTC")

	// Deadline in 2 days, 0% progress → should suggest acceleration
	dl, _ := tc.TrackDeadline(clock.ID, "Rush", "", "t1", time.Now().UTC().Add(48*time.Hour), nil)
	s, err := tc.SuggestPacing(dl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.Suggestion == "" {
		t.Error("should have a suggestion")
	}
	if s.Confidence <= 0 {
		t.Error("confidence should be positive")
	}
	if s.Priority == UrgencyNone {
		t.Error("2-day deadline at 0% should have non-none priority")
	}
}

func TestSuggestPacingCompleted(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Test", "t1", "UTC")
	dl, _ := tc.TrackDeadline(clock.ID, "Done", "", "t1", time.Now().UTC().Add(7*24*time.Hour), nil)
	tc.UpdateProgress(dl.ID, 100)

	s, _ := tc.SuggestPacing(dl.ID)
	if s.Priority != UrgencyNone {
		t.Error("completed task should have none priority")
	}
}

func TestGenerateTimeReport(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Eng", "t1", "UTC")
	tc.TrackDeadline(clock.ID, "D1", "", "t1", time.Now().UTC().Add(5*24*time.Hour), nil)
	tc.TrackDeadline(clock.ID, "D2", "", "t1", time.Now().UTC().Add(2*24*time.Hour), nil)

	report, err := tc.GenerateTimeReport(clock.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report["total"].(int) != 2 {
		t.Errorf("expected 2 deadlines, got %v", report["total"])
	}
	if report["active"].(int) != 2 {
		t.Errorf("expected 2 active, got %v", report["active"])
	}
}

func TestListDeadlinesByStatus(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	clock, _ := tc.CreateClock("Test", "t1", "UTC")
	dl, _ := tc.TrackDeadline(clock.ID, "D1", "", "t1", time.Now().UTC().Add(2*24*time.Hour), nil)

	tc.EscalateUrgency(dl.ID, []string{"boss"}, "stuck")

	escalated := tc.ListDeadlines(clock.ID, DeadlineEscalated)
	if len(escalated) != 1 {
		t.Errorf("expected 1 escalated, got %d", len(escalated))
	}

	active := tc.ListDeadlines(clock.ID, DeadlineActive)
	if len(active) != 0 {
		t.Errorf("expected 0 active after escalation, got %d", len(active))
	}
}

func TestTrackDeadlineInvalidClock(t *testing.T) {
	tc := NewTimeConsciousness(filepath.Join(t.TempDir(), "time.json"))
	_, err := tc.TrackDeadline("nonexistent", "X", "", "t1", time.Now().UTC(), nil)
	if err == nil {
		t.Error("should error on invalid clock")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "time.json")

	tc1 := NewTimeConsciousness(path)
	clock, _ := tc1.CreateClock("Eng", "t1", "UTC")
	tc1.TrackDeadline(clock.ID, "Ship", "", "t1", time.Now().UTC().Add(5*24*time.Hour), nil)
	tc1.RecordBurnSample(clock.ID, 40, 40)

	tc2 := NewTimeConsciousness(path)
	if len(tc2.clocks) != 1 {
		t.Errorf("expected 1 clock, got %d", len(tc2.clocks))
	}
	if len(tc2.deadlines) != 1 {
		t.Errorf("expected 1 deadline, got %d", len(tc2.deadlines))
	}
	if len(tc2.burnSamples) != 1 {
		t.Errorf("expected 1 clock with burn samples, got %d", len(tc2.burnSamples))
	}
}
