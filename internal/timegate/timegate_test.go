package timegate

import (
	"testing"
	"time"
)

func TestNewTimeBudget(t *testing.T) {
	tb := NewTimeBudget("task-1", 2*time.Hour)
	if tb.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", tb.TaskID)
	}
	if tb.Allocated != 2*time.Hour {
		t.Errorf("expected 2h allocated")
	}
	if tb.Remaining() > 2*time.Hour {
		t.Errorf("remaining should be <= 2h")
	}
}

func TestPaceReportOnTrack(t *testing.T) {
	tb := NewTimeBudget("task-1", 2*time.Hour)
	tb.UpdateProgress(0.5, "execute", "halfway done")
	report := tb.CheckPace(0.5)

	// With ~0% consumed and 50% progress, we're ahead
	if report.ProgressPct != 0.5 {
		t.Errorf("expected 0.5 progress, got %f", report.ProgressPct)
	}
}

func TestUrgencyLevels(t *testing.T) {
	tg := NewTimeGate()

	tests := []struct {
		offset   time.Duration
		expected UrgencyLevel
	}{
		{24 * time.Hour, UrgencyRoutine},
		{4 * time.Hour, UrgencyNormal},
		{1 * time.Hour, UrgencyElevated},
		{15 * time.Minute, UrgencyCritical},
		{2 * time.Minute, UrgencyEmergency},
	}

	for _, tt := range tests {
		deadline := time.Now().Add(tt.offset)
		got := tg.UrgencyLevel(deadline)
		if got != tt.expected {
			t.Errorf("urgency for %v offset: expected %s, got %s", tt.offset, tt.expected, got)
		}
	}
}

func TestTimeAccounting(t *testing.T) {
	tg := NewTimeGate()
	tg.RecordTime("agent-1", "task-1", 30*time.Minute)
	tg.RecordTime("agent-1", "task-1", 15*time.Minute)
	tg.RecordTime("agent-1", "task-2", 45*time.Minute)

	acc, ok := tg.TimeAccounting("agent-1")
	if !ok {
		t.Fatal("expected account to exist")
	}
	if acc.TotalTime != 90*time.Minute {
		t.Errorf("expected 90m total, got %v", acc.TotalTime)
	}
	if acc.TaskTimes["task-1"] != 45*time.Minute {
		t.Errorf("expected 45m for task-1, got %v", acc.TaskTimes["task-1"])
	}
}

func TestPredictCompletion(t *testing.T) {
	tg := NewTimeGate()
	tg.CreateBudget("task-1", 2*time.Hour)

	pred, err := tg.PredictCompletion("task-1", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if !pred.WillFinish {
		t.Error("expected task to finish with 50% progress just started")
	}
}

func TestPauseResume(t *testing.T) {
	tb := NewTimeBudget("task-1", 1*time.Hour)
	tb.Pause()
	if !tb.paused {
		t.Error("expected budget to be paused")
	}
	tb.Resume()
	if tb.paused {
		t.Error("expected budget to be resumed")
	}
}

func TestQualityModifier(t *testing.T) {
	tests := []struct {
		urgency  UrgencyLevel
		expected float64
	}{
		{UrgencyRoutine, 1.0},
		{UrgencyNormal, 0.95},
		{UrgencyElevated, 0.85},
		{UrgencyCritical, 0.7},
		{UrgencyEmergency, 0.4},
	}
	for _, tt := range tests {
		got := tt.urgency.QualityModifier()
		if got != tt.expected {
			t.Errorf("quality modifier for %s: expected %f, got %f", tt.urgency, tt.expected, got)
		}
	}
}

func TestCommunicationFrequency(t *testing.T) {
	if UrgencyEmergency.CommunicationFrequency() != 1*time.Minute {
		t.Error("emergency should check in every minute")
	}
	if UrgencyRoutine.CommunicationFrequency() != 60*time.Minute {
		t.Error("routine should check in hourly")
	}
}
