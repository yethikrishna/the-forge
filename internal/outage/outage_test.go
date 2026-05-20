package outage

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestRegisterProvider(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	m.RegisterProvider("anthropic", "https://api.anthropic.com", 2)
	providers := m.ListProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestSetPlaybook(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	m.RegisterProvider("anthropic", "https://api.anthropic.com", 2)
	m.SetPlaybook(&Playbook{
		Provider:     "openai",
		Fallbacks:    []string{"anthropic"},
		AutoFallback: true,
	})
	pb, ok := m.playbooks["openai"]
	if !ok {
		t.Fatal("expected playbook")
	}
	if pb.RetryCount != 3 {
		t.Errorf("expected default retry_count=3, got %d", pb.RetryCount)
	}
}

func TestRecordCheckHealthy(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	m.RecordCheck(&CheckResult{
		Provider: "openai",
		Status:   StatusHealthy,
		Latency:  50 * time.Millisecond,
	})
	p, _ := m.GetProvider("openai")
	if p.Status != StatusHealthy {
		t.Errorf("expected healthy, got %s", p.Status)
	}
	if p.SuccessRate != 1.0 {
		t.Errorf("expected 1.0, got %.2f", p.SuccessRate)
	}
}

func TestRecordCheckOutage(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	m.RegisterProvider("anthropic", "https://api.anthropic.com", 2)
	m.SetPlaybook(&Playbook{
		Provider:     "openai",
		Fallbacks:    []string{"anthropic"},
		AutoFallback: true,
	})

	m.RecordCheck(&CheckResult{
		Provider: "openai",
		Status:   StatusOutage,
		Error:    "connection refused",
	})

	incidents := m.ListIncidents()
	if len(incidents) == 0 {
		t.Error("expected incident to be created")
	}
	if incidents[0].Status == "resolved" {
		t.Error("incident should not be resolved yet")
	}
}

func TestResolveIncident(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	m.RecordCheck(&CheckResult{
		Provider: "openai",
		Status:   StatusOutage,
		Error:    "timeout",
	})
	incidents := m.ListIncidents()
	if len(incidents) == 0 {
		t.Fatal("expected incident")
	}
	err := m.ResolveIncident(incidents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if incidents[0].Status != "resolved" {
		t.Errorf("expected resolved, got %s", incidents[0].Status)
	}
}

func TestGetFallback(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	m.RegisterProvider("anthropic", "https://api.anthropic.com", 2)
	m.SetPlaybook(&Playbook{
		Provider:  "openai",
		Fallbacks: []string{"anthropic"},
	})

	// Anthropic is healthy by default (unknown)
	fallback, err := m.GetFallback("openai")
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Name != "anthropic" {
		t.Errorf("expected anthropic, got %s", fallback.Name)
	}
}

func TestGetFallbackNoPlaybook(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	_, err := m.GetFallback("openai")
	if err == nil {
		t.Error("expected error: no playbook")
	}
}

func TestGenerateIncidentReport(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	m.RegisterProvider("openai", "https://api.openai.com", 1)
	m.RecordCheck(&CheckResult{
		Provider: "openai",
		Status:   StatusOutage,
		Error:    "503 Service Unavailable",
	})
	incidents := m.ListIncidents()
	report, err := m.GenerateIncidentReport(incidents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if report == "" {
		t.Error("expected non-empty report")
	}
	if !contains(report, "openai") {
		t.Error("report should mention provider")
	}
}

func TestFormatProvider(t *testing.T) {
	p := &Provider{Name: "openai", Status: StatusHealthy, Endpoint: "https://api.openai.com", Priority: 1, SuccessRate: 0.99}
	output := FormatProvider(p)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatIncident(t *testing.T) {
	i := &Incident{ID: "inc-1", Provider: "openai", Status: "investigating", Severity: "high", StartedAt: time.Now()}
	output := FormatIncident(i)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestProviderNotFound(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	_, err := m.GetProvider("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestIncidentNotFound(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	_, err := m.GenerateIncidentReport("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
