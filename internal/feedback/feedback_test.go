package feedback

import (
	"testing"
	"time"
)

func TestIngestAndRoute(t *testing.T) {
	fl := NewFeedbackLoop()
	fl.SetOwnership("api/auth", "engineering")

	sig := Signal{
		Type: SignalError, Severity: SeverityHigh,
		Source: "api/auth", Message: "nil pointer dereference",
	}
	fl.Ingest(sig)

	div, err := fl.Route(sig)
	if err != nil {
		t.Fatal(err)
	}
	if div != "engineering" {
		t.Errorf("expected engineering, got %s", div)
	}
}

func TestCorrelation(t *testing.T) {
	fl := NewFeedbackLoop()
	fl.SetOwnership("api/users", "engineering")

	// Ingest multiple signals from same source
	for i := 0; i < 5; i++ {
		fl.Ingest(Signal{
			Type: SignalError, Severity: SeverityHigh,
			Source: "api/users", Message: "timeout",
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	incidents := fl.Correlate()
	if len(incidents) == 0 {
		t.Error("expected incidents to be created from correlated signals")
	}
}

func TestSLAStatus(t *testing.T) {
	fl := NewFeedbackLoop()
	fl.SetOwnership("api/auth", "security")

	fl.Ingest(Signal{Type: SignalError, Severity: SeverityLow, Source: "api/auth"})
	fl.Ingest(Signal{Type: SignalError, Severity: SeverityHigh, Source: "api/auth"})
	fl.Ingest(Signal{Type: SignalLatency, Severity: SeverityMedium, Source: "api/auth"})

	report := fl.SLAStatus("security")
	if report.TotalSignals != 3 {
		t.Errorf("expected 3 signals, got %d", report.TotalSignals)
	}
	if report.ByType[SignalError] != 2 {
		t.Errorf("expected 2 error signals, got %d", report.ByType[SignalError])
	}
}

func TestTrends(t *testing.T) {
	fl := NewFeedbackLoop()

	now := time.Now()
	for i := 0; i < 10; i++ {
		fl.Ingest(Signal{
			Type: SignalLatency, Source: "api/test",
			Timestamp: now.Add(-time.Duration(10-i) * time.Minute),
		})
	}

	points := fl.Trends(SignalLatency, 30*time.Minute)
	if len(points) == 0 {
		t.Error("expected trend points")
	}
}

func TestIncidentsSince(t *testing.T) {
	fl := NewFeedbackLoop()
	fl.SetOwnership("svc/a", "ops")

	for i := 0; i < 3; i++ {
		fl.Ingest(Signal{Type: SignalAvailability, Source: "svc/a", Timestamp: time.Now()})
	}
	fl.Correlate()

	recent := fl.Incidents(time.Now().Add(-1 * time.Hour))
	if len(recent) == 0 {
		t.Error("expected recent incidents")
	}
}
