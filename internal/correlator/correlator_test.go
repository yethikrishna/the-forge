package correlator_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/correlator"
)

func TestNewEngine(t *testing.T) {
	engine := correlator.NewEngine()
	if engine == nil {
		t.Fatal("engine should not be nil")
	}
	stats := engine.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("expected 0 events, got %d", stats.TotalEvents)
	}
}

func TestIngestEvent(t *testing.T) {
	engine := correlator.NewEngine()

	event := correlator.NewEvent(correlator.SourceCost, "spike", "cost spike detected")
	engine.Ingest(event)

	stats := engine.Stats()
	if stats.TotalEvents != 1 {
		t.Errorf("expected 1 event, got %d", stats.TotalEvents)
	}
	if stats.BySource[correlator.SourceCost] != 1 {
		t.Errorf("expected 1 cost event, got %d", stats.BySource[correlator.SourceCost])
	}
}

func TestIngestBatch(t *testing.T) {
	engine := correlator.NewEngine()

	events := []*correlator.Event{
		correlator.NewEvent(correlator.SourceCost, "spike", "cost spike"),
		correlator.NewEvent(correlator.SourceHealth, "timeout", "health check timeout"),
		correlator.NewEvent(correlator.SourceAgent, "error", "agent error"),
	}
	engine.IngestBatch(events)

	stats := engine.Stats()
	if stats.TotalEvents != 3 {
		t.Errorf("expected 3 events, got %d", stats.TotalEvents)
	}
}

func TestCorrelationRuleTrigger(t *testing.T) {
	engine := correlator.NewEngine()

	// Trigger the cost-retry-loop rule: needs cost spike + resilience error + agent retry
	events := []*correlator.Event{
		correlator.NewEvent(correlator.SourceCost, "spike", "cost increased 300%").WithSeverity(correlator.SeverityCritical).WithLabel("agent_id", "agent-1"),
		correlator.NewEvent(correlator.SourceResilience, "error", "provider 500 error").WithSeverity(correlator.SeverityWarning),
		correlator.NewEvent(correlator.SourceAgent, "retry", "agent retrying after failure").WithSeverity(correlator.SeverityWarning).WithLabel("agent_id", "agent-1"),
	}
	engine.IngestBatch(events)

	incidents := engine.ActiveIncidents()
	if len(incidents) == 0 {
		t.Fatal("expected at least one correlated incident")
	}

	// Should be the cost-retry-loop incident
	found := false
	for _, inc := range incidents {
		if inc.RootCause == "Provider errors are causing automatic retries, each consuming tokens without producing results" {
			found = true
			if inc.Severity != correlator.SeverityCritical {
				t.Errorf("expected critical severity, got %s", inc.Severity)
			}
			if len(inc.AffectedAgents) == 0 || inc.AffectedAgents[0] != "agent-1" {
				t.Errorf("expected affected agent 'agent-1', got %v", inc.AffectedAgents)
			}
			if len(inc.Recommendations) == 0 {
				t.Error("expected recommendations")
			}
		}
	}
	if !found {
		t.Error("cost-retry-loop incident not found")
	}
}

func TestNoCorrelationWithoutEnoughEvents(t *testing.T) {
	engine := correlator.NewEngine()

	// Only 1 event for cost-retry-loop (needs 3)
	events := []*correlator.Event{
		correlator.NewEvent(correlator.SourceCost, "spike", "cost spike"),
		correlator.NewEvent(correlator.SourceResilience, "error", "provider error"),
	}
	engine.IngestBatch(events)

	incidents := engine.ActiveIncidents()
	if len(incidents) != 0 {
		t.Errorf("expected no incidents with insufficient events, got %d", len(incidents))
	}
}

func TestIncidentStatusUpdate(t *testing.T) {
	engine := correlator.NewEngine()

	// Create incident
	events := []*correlator.Event{
		correlator.NewEvent(correlator.SourceCost, "spike", "cost spike").WithLabel("agent_id", "agent-1"),
		correlator.NewEvent(correlator.SourceResilience, "error", "error"),
		correlator.NewEvent(correlator.SourceAgent, "retry", "retry").WithLabel("agent_id", "agent-1"),
	}
	engine.IngestBatch(events)

	incidents := engine.ActiveIncidents()
	if len(incidents) == 0 {
		t.Fatal("expected at least one incident")
	}

	incID := incidents[0].ID
	err := engine.UpdateIncidentStatus(incID, correlator.IncidentInvestigating)
	if err != nil {
		t.Fatalf("status update error: %v", err)
	}

	incidents = engine.ActiveIncidents()
	if len(incidents) == 0 {
		t.Fatal("incident should still be active (investigating)")
	}

	err = engine.UpdateIncidentStatus(incID, correlator.IncidentResolved)
	if err != nil {
		t.Fatalf("status update error: %v", err)
	}

	// Should no longer be active
	incidents = engine.ActiveIncidents()
	for _, inc := range incidents {
		if inc.ID == incID {
			t.Error("resolved incident should not be in active list")
		}
	}
}

func TestRecentEvents(t *testing.T) {
	engine := correlator.NewEngine()

	events := []*correlator.Event{
		correlator.NewEvent(correlator.SourceCost, "spike", "recent event"),
	}
	engine.IngestBatch(events)

	recent := engine.RecentEvents(1 * time.Minute)
	if len(recent) != 1 {
		t.Errorf("expected 1 recent event, got %d", len(recent))
	}

	recent = engine.RecentEvents(1 * time.Nanosecond)
	// The event was just created, so with a nanosecond window it may not appear
	// depending on timing. This is a timing-sensitive test; just verify no panic.
}

func TestAddCustomRule(t *testing.T) {
	engine := correlator.NewEngine()

	rule := &correlator.CorrelationRule{
		ID:          "custom-rule",
		Name:        "Custom Test Rule",
		Description: "Test custom rule",
		Sources:     []correlator.Source{correlator.SourceAudit, correlator.SourceMemory},
		Types:       []string{"alert", "growth"},
		Window:      5 * time.Minute,
		MinEvents:   2,
		Severity:    correlator.SeverityWarning,
		Title:       "Custom incident",
		RootCause:   "Custom root cause",
		Recommendations: []string{"Do something"},
	}
	engine.AddRule(rule)

	// Trigger it
	events := []*correlator.Event{
		correlator.NewEvent(correlator.SourceAudit, "alert", "audit alert"),
		correlator.NewEvent(correlator.SourceMemory, "growth", "memory growing"),
	}
	engine.IngestBatch(events)

	incidents := engine.ActiveIncidents()
	found := false
	for _, inc := range incidents {
		if inc.RootCause == "Custom root cause" {
			found = true
		}
	}
	if !found {
		t.Error("custom rule should have triggered an incident")
	}
}

func TestEventBuilder(t *testing.T) {
	event := correlator.NewEvent(correlator.SourceCost, "spike", "test").
		WithSeverity(correlator.SeverityCritical).
		WithLabel("agent_id", "test-agent").
		WithValue(42.5)

	if event.Severity != correlator.SeverityCritical {
		t.Errorf("expected critical severity")
	}
	if event.Labels["agent_id"] != "test-agent" {
		t.Errorf("label not set")
	}
	if event.Value != 42.5 {
		t.Errorf("expected value 42.5, got %.2f", event.Value)
	}
}

func TestFormatIncident(t *testing.T) {
	inc := &correlator.Incident{
		ID:        "inc-123",
		Title:     "Test Incident",
		Severity:  correlator.SeverityCritical,
		Status:    correlator.IncidentOpen,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		RootCause: "Test root cause",
		AffectedAgents: []string{"agent-1", "agent-2"},
		Events:    make([]*correlator.Event, 3),
		Recommendations: []string{"Fix it", "Restart"},
	}

	output := correlator.FormatIncident(inc)
	if output == "" {
		t.Error("expected non-empty output")
	}
	// Verify key content appears
	if !contains(output, "Test Incident") {
		t.Error("missing title in output")
	}
	if !contains(output, "agent-1") {
		t.Error("missing affected agent in output")
	}
	if !contains(output, "Fix it") {
		t.Error("missing recommendation in output")
	}
}

func TestDefaultRules(t *testing.T) {
	rules := correlator.DefaultRules()
	if len(rules) < 3 {
		t.Errorf("expected at least 3 default rules, got %d", len(rules))
	}

	for _, rule := range rules {
		if rule.ID == "" {
			t.Error("rule missing ID")
		}
		if len(rule.Sources) == 0 {
			t.Error("rule missing sources")
		}
		if rule.MinEvents < 1 {
			t.Error("rule MinEvents should be at least 1")
		}
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s     correlator.Severity
		want  string
	}{
		{correlator.SeverityInfo, "info"},
		{correlator.SeverityWarning, "warning"},
		{correlator.SeverityCritical, "critical"},
		{correlator.SeverityEmergency, "emergency"},
	}
	for _, tt := range tests {
		if tt.s.String() != tt.want {
			t.Errorf("expected %s, got %s", tt.want, tt.s.String())
		}
	}
}

func TestIncidentStatusString(t *testing.T) {
	tests := []struct {
		s     correlator.IncidentStatus
		want  string
	}{
		{correlator.IncidentOpen, "open"},
		{correlator.IncidentInvestigating, "investigating"},
		{correlator.IncidentMitigated, "mitigated"},
		{correlator.IncidentResolved, "resolved"},
	}
	for _, tt := range tests {
		if tt.s.String() != tt.want {
			t.Errorf("expected %s, got %s", tt.want, tt.s.String())
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || 
		(len(s) > 0 && len(sub) > 0 && findSubstring(s, sub)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
