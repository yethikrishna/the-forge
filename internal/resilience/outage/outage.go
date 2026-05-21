// Package outage provides provider outage detection, auto-fallback,
// notification, and incident report generation.
//
// When the cloud breaks, Forge doesn't.
package outage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProviderStatus represents a provider's current state.
type ProviderStatus string

const (
	StatusHealthy    ProviderStatus = "healthy"
	StatusDegraded   ProviderStatus = "degraded"
	StatusOutage     ProviderStatus = "outage"
	StatusUnknown    ProviderStatus = "unknown"
	StatusMaintenance ProviderStatus = "maintenance"
)

// Provider represents an AI provider.
type Provider struct {
	Name       string         `json:"name"`
	Endpoint   string         `json:"endpoint"`
	Status     ProviderStatus `json:"status"`
	Priority   int            `json:"priority"` // lower = preferred
	LastCheck  *time.Time     `json:"last_check,omitempty"`
	LastError  string         `json:"last_error,omitempty"`
	Latency    time.Duration  `json:"latency,omitempty"`
	SuccessRate float64       `json:"success_rate"` // 0-1
}

// Incident represents a provider outage incident.
type Incident struct {
	ID          string         `json:"id"`
	Provider    string         `json:"provider"`
	Status      string         `json:"status"` // investigating, identified, monitoring, resolved
	Severity    string         `json:"severity"` // critical, high, medium, low
	StartedAt   time.Time      `json:"started_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
	Description string         `json:"description"`
	Timeline    []IncidentEvent `json:"timeline,omitempty"`
	Fallbacks   []string       `json:"fallbacks_used,omitempty"`
	AffectedAgents []string    `json:"affected_agents,omitempty"`
}

// IncidentEvent represents an event in an incident timeline.
type IncidentEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	Detail    string    `json:"detail,omitempty"`
}

// CheckResult represents the result of a provider health check.
type CheckResult struct {
	Provider   string         `json:"provider"`
	Status     ProviderStatus `json:"status"`
	Latency    time.Duration  `json:"latency"`
	Error      string         `json:"error,omitempty"`
	StatusCode int            `json:"status_code,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Playbook defines the fallback strategy for a provider.
type Playbook struct {
	Provider     string   `json:"provider"`
	Fallbacks    []string `json:"fallbacks"`     // ordered list of fallback providers
	RetryCount   int      `json:"retry_count"`   // default: 3
	RetryDelay   int      `json:"retry_delay"`   // seconds, default: 5
	NotifyEmail  []string `json:"notify_email,omitempty"`
	AutoFallback bool     `json:"auto_fallback"` // default: true
	CooldownSecs int      `json:"cooldown_secs"` // default: 300
}

// Manager manages provider health and outage response.
type Manager struct {
	mu        sync.RWMutex
	dir       string
	providers map[string]*Provider
	playbooks map[string]*Playbook
	incidents map[string]*Incident
	results   map[string][]*CheckResult
}

// NewManager creates an outage manager.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	m := &Manager{
		dir:       dir,
		providers: make(map[string]*Provider),
		playbooks: make(map[string]*Playbook),
		incidents: make(map[string]*Incident),
		results:   make(map[string][]*CheckResult),
	}
	m.load()
	return m, nil
}

func (m *Manager) load() {
	// Load incidents
	data, err := os.ReadFile(filepath.Join(m.dir, "incidents.json"))
	if err == nil {
		var incidents []*Incident
		if json.Unmarshal(data, &incidents) == nil {
			for _, i := range incidents {
				m.incidents[i.ID] = i
			}
		}
	}
}

func (m *Manager) saveIncidents() error {
	incidents := make([]*Incident, 0, len(m.incidents))
	for _, i := range m.incidents {
		incidents = append(incidents, i)
	}
	data, err := json.MarshalIndent(incidents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dir, "incidents.json"), data, 0o644)
}

// RegisterProvider adds a provider to monitor.
func (m *Manager) RegisterProvider(name, endpoint string, priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[name] = &Provider{
		Name:        name,
		Endpoint:    endpoint,
		Status:      StatusUnknown,
		Priority:    priority,
		SuccessRate: 1.0,
	}
}

// SetPlaybook configures a fallback playbook for a provider.
func (m *Manager) SetPlaybook(pb *Playbook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pb.RetryCount == 0 {
		pb.RetryCount = 3
	}
	if pb.RetryDelay == 0 {
		pb.RetryDelay = 5
	}
	if pb.CooldownSecs == 0 {
		pb.CooldownSecs = 300
	}
	m.playbooks[pb.Provider] = pb
}

// RecordCheck records a health check result.
func (m *Manager) RecordCheck(result *CheckResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result.Timestamp = time.Now()
	m.results[result.Provider] = append(m.results[result.Provider], result)
	if len(m.results[result.Provider]) > 100 {
		m.results[result.Provider] = m.results[result.Provider][50:]
	}

	// Update provider status
	if p, ok := m.providers[result.Provider]; ok {
		p.Status = result.Status
		p.Latency = result.Latency
		p.LastCheck = &result.Timestamp
		p.LastError = result.Error

		// Update success rate
		recent := m.results[result.Provider]
		if len(recent) > 0 {
			successes := 0
			for _, r := range recent {
				if r.Status == StatusHealthy {
					successes++
				}
			}
			p.SuccessRate = float64(successes) / float64(len(recent))
		}
	}

	// Check for outage
	if result.Status == StatusOutage || result.Status == StatusDegraded {
		m.handleOutage(result)
	}
}

// handleOutage manages outage detection and response.
func (m *Manager) handleOutage(result *CheckResult) {
	// Find or create incident
	var incident *Incident
	for _, i := range m.incidents {
		if i.Provider == result.Provider && i.Status != "resolved" {
			incident = i
			break
		}
	}

	if incident == nil {
		incident = &Incident{
			ID:          fmt.Sprintf("inc-%d", time.Now().UnixNano()),
			Provider:    result.Provider,
			Status:      "investigating",
			Severity:    "high",
			StartedAt:   time.Now(),
			Description: fmt.Sprintf("Provider %s %s: %s", result.Provider, result.Status, result.Error),
			Timeline:    []IncidentEvent{},
		}
		m.incidents[incident.ID] = incident
	}

	incident.Timeline = append(incident.Timeline, IncidentEvent{
		Timestamp: time.Now(),
		Event:     string(result.Status),
		Detail:    result.Error,
	})

	// Check if we should auto-fallback
	if pb, ok := m.playbooks[result.Provider]; ok && pb.AutoFallback {
		incident.Fallbacks = pb.Fallbacks
		incident.Timeline = append(incident.Timeline, IncidentEvent{
			Timestamp: time.Now(),
			Event:     "fallback_activated",
			Detail:    fmt.Sprintf("Switching to: %s", strings.Join(pb.Fallbacks, ", ")),
		})
	}

	m.saveIncidents()
}

// ResolveIncident marks an incident as resolved.
func (m *Manager) ResolveIncident(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	i, ok := m.incidents[id]
	if !ok {
		return fmt.Errorf("incident %q not found", id)
	}

	now := time.Now()
	i.ResolvedAt = &now
	i.Status = "resolved"
	i.Timeline = append(i.Timeline, IncidentEvent{
		Timestamp: now,
		Event:     "resolved",
	})

	if p, ok := m.providers[i.Provider]; ok {
		p.Status = StatusHealthy
	}

	return m.saveIncidents()
}

// GetFallback returns the best available fallback provider.
func (m *Manager) GetFallback(provider string) (*Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pb, ok := m.playbooks[provider]
	if !ok {
		return nil, fmt.Errorf("no playbook for provider %q", provider)
	}

	for _, fallback := range pb.Fallbacks {
		if p, ok := m.providers[fallback]; ok && (p.Status == StatusHealthy || p.Status == StatusUnknown) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no healthy fallback available for %q", provider)
}

// GetProvider returns a provider by name.
func (m *Manager) GetProvider(name string) (*Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

// ListProviders returns all providers.
func (m *Manager) ListProviders() []*Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Provider, 0, len(m.providers))
	for _, p := range m.providers {
		result = append(result, p)
	}
	return result
}

// ListIncidents returns all incidents.
func (m *Manager) ListIncidents() []*Incident {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Incident, 0, len(m.incidents))
	for _, i := range m.incidents {
		result = append(result, i)
	}
	return result
}

// GenerateIncidentReport creates a markdown incident report.
func (m *Manager) GenerateIncidentReport(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	i, ok := m.incidents[id]
	if !ok {
		return "", fmt.Errorf("incident %q not found", id)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Incident Report: %s\n\n", i.ID))
	sb.WriteString(fmt.Sprintf("## Overview\n"))
	sb.WriteString(fmt.Sprintf("- **Provider:** %s\n", i.Provider))
	sb.WriteString(fmt.Sprintf("- **Severity:** %s\n", i.Severity))
	sb.WriteString(fmt.Sprintf("- **Status:** %s\n", i.Status))
	sb.WriteString(fmt.Sprintf("- **Started:** %s\n", i.StartedAt.Format(time.RFC3339)))
	if i.ResolvedAt != nil {
		sb.WriteString(fmt.Sprintf("- **Resolved:** %s\n", i.ResolvedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("- **Duration:** %s\n", i.ResolvedAt.Sub(i.StartedAt).Round(time.Second)))
	}
	sb.WriteString(fmt.Sprintf("\n## Description\n%s\n", i.Description))

	if len(i.Fallbacks) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Fallbacks Used\n"))
		for _, f := range i.Fallbacks {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(i.Timeline) > 0 {
		sb.WriteString("\n## Timeline\n")
		for _, e := range i.Timeline {
			sb.WriteString(fmt.Sprintf("- **%s** — %s", e.Timestamp.Format("15:04:05"), e.Event))
			if e.Detail != "" {
				sb.WriteString(fmt.Sprintf(": %s", e.Detail))
			}
			sb.WriteString("\n")
		}
	}

	if len(i.AffectedAgents) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Affected Agents\n"))
		for _, a := range i.AffectedAgents {
			sb.WriteString(fmt.Sprintf("- %s\n", a))
		}
	}

	return sb.String(), nil
}

// FormatProvider renders a provider for display.
func FormatProvider(p *Provider) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n", p.Name))
	sb.WriteString(fmt.Sprintf("  Status:      %s\n", p.Status))
	sb.WriteString(fmt.Sprintf("  Endpoint:    %s\n", p.Endpoint))
	sb.WriteString(fmt.Sprintf("  Priority:    %d\n", p.Priority))
	sb.WriteString(fmt.Sprintf("  Success Rate: %.0f%%\n", p.SuccessRate*100))
	if p.Latency > 0 {
		sb.WriteString(fmt.Sprintf("  Latency:     %s\n", p.Latency.Round(time.Millisecond)))
	}
	if p.LastCheck != nil {
		sb.WriteString(fmt.Sprintf("  Last Check:  %s\n", p.LastCheck.Format(time.RFC3339)))
	}
	return sb.String()
}

// FormatIncident renders an incident for display.
func FormatIncident(i *Incident) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s [%s]\n", i.ID, i.Status))
	sb.WriteString(fmt.Sprintf("  Provider:  %s\n", i.Provider))
	sb.WriteString(fmt.Sprintf("  Severity:  %s\n", i.Severity))
	sb.WriteString(fmt.Sprintf("  Started:   %s\n", i.StartedAt.Format(time.RFC3339)))
	if i.ResolvedAt != nil {
		sb.WriteString(fmt.Sprintf("  Resolved:  %s\n", i.ResolvedAt.Format(time.RFC3339)))
	}
	return sb.String()
}
