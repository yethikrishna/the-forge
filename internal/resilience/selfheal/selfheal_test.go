package selfheal

import (
	"strings"
	"testing"
)

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()
	if len(rules) < 5 {
		t.Errorf("expected at least 5 default rules, got %d", len(rules))
	}
	for _, r := range rules {
		if r.ID == "" {
			t.Error("expected non-empty rule ID")
		}
		if !r.Enabled {
			t.Errorf("expected default rule %s to be enabled", r.Name)
		}
	}
}

func TestReportIncident(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureTimeout, "Request timed out after 30s")

	if inc.ID == "" {
		t.Error("expected non-empty ID")
	}
	if inc.Status != IncidentOpen {
		t.Errorf("expected open, got %s", inc.Status)
	}
	if inc.FailureType != FailureTimeout {
		t.Errorf("expected timeout, got %s", inc.FailureType)
	}
}

func TestRemediateRetry(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureTimeout, "Timeout")

	result, err := e.Remediate(inc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Attempt != 1 {
		t.Errorf("expected attempt 1, got %d", result.Attempt)
	}
	if result.Status != IncidentRemediating {
		t.Errorf("expected remediating, got %s", result.Status)
	}
}

func TestRemediateMaxAttempts(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureTimeout, "Timeout")

	// Exhaust max attempts (default 3 for timeout)
	e.Remediate(inc.ID)
	e.Remediate(inc.ID)
	result, _ := e.Remediate(inc.ID)

	if result.Attempt != 3 {
		t.Errorf("expected attempt 3, got %d", result.Attempt)
	}
	if result.Status != IncidentResolved {
		t.Errorf("expected resolved after max attempts, got %s", result.Status)
	}
}

func TestRemediateEscalate(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureAuth, "Invalid API key")

	result, err := e.Remediate(inc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != IncidentEscalated {
		t.Errorf("expected escalated, got %s", result.Status)
	}
}

func TestRemediateAlreadyResolved(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureAuth, "Auth error")
	e.Remediate(inc.ID) // This escalates

	_, err := e.Remediate(inc.ID)
	if err == nil {
		t.Error("expected error for already resolved incident")
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureCrash, "Agent crashed")

	err := e.Resolve(inc.ID, "admin", "Manually restarted agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := e.GetIncident(inc.ID)
	if got.Status != IncidentResolved {
		t.Errorf("expected resolved, got %s", got.Status)
	}
	if got.ResolvedBy != "admin" {
		t.Errorf("expected admin, got %s", got.ResolvedBy)
	}
}

func TestGetIncident(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureTimeout, "Timeout")

	got, ok := e.GetIncident(inc.ID)
	if !ok {
		t.Fatal("expected to find incident")
	}
	if got.ID != inc.ID {
		t.Errorf("expected %s, got %s", inc.ID, got.ID)
	}
}

func TestListIncidents(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.ReportIncident("agent-1", FailureTimeout, "T1")
	e.ReportIncident("agent-2", FailureCrash, "C1")

	list := e.ListIncidents()
	if len(list) != 2 {
		t.Errorf("expected 2 incidents, got %d", len(list))
	}
}

func TestListByAgent(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.ReportIncident("agent-1", FailureTimeout, "T1")
	e.ReportIncident("agent-1", FailureCrash, "C1")
	e.ReportIncident("agent-2", FailureOOM, "O1")

	agent1 := e.ListByAgent("agent-1")
	if len(agent1) != 2 {
		t.Errorf("expected 2 for agent-1, got %d", len(agent1))
	}
}

func TestListByStatus(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.ReportIncident("agent-1", FailureTimeout, "T1")
	inc := e.ReportIncident("agent-1", FailureAuth, "A1")
	e.Remediate(inc.ID) // Escalates

	open := e.ListByStatus(IncidentOpen)
	if len(open) != 1 {
		t.Errorf("expected 1 open, got %d", len(open))
	}

	escalated := e.ListByStatus(IncidentEscalated)
	if len(escalated) != 1 {
		t.Errorf("expected 1 escalated, got %d", len(escalated))
	}
}

func TestAddRule(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.AddRule(Rule{
		ID:          "custom-rule",
		Name:        "Custom Handler",
		FailureType: FailureUnknown,
		Action:      ActionIgnore,
		MaxAttempts: 1,
		Priority:    1,
		Enabled:     true,
	})

	rules := e.ListRules()
	found := false
	for _, r := range rules {
		if r.ID == "custom-rule" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find custom rule")
	}
}

func TestRemoveRule(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.AddRule(Rule{ID: "temp-rule", Name: "Temp", FailureType: FailureUnknown, Action: ActionIgnore, MaxAttempts: 1, Priority: 1, Enabled: true})
	e.RemoveRule("temp-rule")

	for _, r := range e.ListRules() {
		if r.ID == "temp-rule" {
			t.Error("expected rule to be removed")
		}
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.ReportIncident("agent-1", FailureTimeout, "T1")
	e.ReportIncident("agent-1", FailureCrash, "C1")

	stats := e.Stats()
	if stats["total_incidents"] != 2 {
		t.Errorf("expected 2 incidents, got %v", stats["total_incidents"])
	}
}

func TestIncidentReport(t *testing.T) {
	inc := &Incident{
		ID:           "inc-test",
		AgentID:      "agent-1",
		FailureType:  FailureTimeout,
		Status:       IncidentResolved,
		Message:      "Request timed out",
		Remediation:  ActionRetry,
		Attempt:      2,
		MaxAttempts:  3,
		Resolution:   "Succeeded on retry",
		Duration:     "45s",
	}

	report := IncidentReport(inc)
	if !strings.Contains(report, "timeout") {
		t.Error("expected failure type in report")
	}
	if !strings.Contains(report, "Succeeded on retry") {
		t.Error("expected resolution in report")
	}
}

func TestFallbackRemediation(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureModel, "Model returned 500")

	result, _ := e.Remediate(inc.ID)
	if !strings.Contains(result.Resolution, "Switched to") || !strings.Contains(result.Resolution, "gpt-4") {
		t.Errorf("expected fallback resolution, got: %s", result.Resolution)
	}
}

func TestOOMRemediation(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	inc := e.ReportIncident("agent-1", FailureOOM, "Out of memory")

	result, _ := e.Remediate(inc.ID)
	if !strings.Contains(result.Resolution, "scaled down") {
		t.Errorf("expected scale down resolution, got: %s", result.Resolution)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	e1 := NewEngine(dir)
	inc := e1.ReportIncident("agent-1", FailureTimeout, "Timeout")
	e1.Remediate(inc.ID)

	e2 := NewEngine(dir)
	list := e2.ListIncidents()
	if len(list) != 1 {
		t.Fatalf("expected 1 incident after reload, got %d", len(list))
	}
	if list[0].Attempt != 1 {
		t.Errorf("expected attempt 1, got %d", list[0].Attempt)
	}
}

func TestRulePriority(t *testing.T) {
	rules := DefaultRules()
	// Just verify rules have different priorities
	priorities := make(map[int]bool)
	for _, r := range rules {
		if priorities[r.Priority] {
			t.Errorf("duplicate priority %d", r.Priority)
		}
		priorities[r.Priority] = true
	}
}
