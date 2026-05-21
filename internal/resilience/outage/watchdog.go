package outage

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// WatchdogConfig configures the watchdog.
type WatchdogConfig struct {
	CheckInterval     time.Duration `json:"check_interval"`
	FailureThreshold  int           `json:"failure_threshold"`
	RecoveryThreshold int           `json:"recovery_threshold"`
	CooldownPeriod    time.Duration `json:"cooldown_period"`
}

// DefaultWatchdogConfig returns default config.
func DefaultWatchdogConfig() WatchdogConfig {
	return WatchdogConfig{
		CheckInterval:     30 * time.Second,
		FailureThreshold:  3,
		RecoveryThreshold: 2,
		CooldownPeriod:    5 * time.Minute,
	}
}

// ProviderResult represents a provider call result.
type ProviderResult struct {
	Provider string        `json:"provider"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
	Latency  time.Duration `json:"latency"`
}

// WatchdogProvider extends Provider with watchdog-specific fields.
type WatchdogProvider struct {
	Name          string         `json:"name"`
	Status        ProviderStatus `json:"status"`
	Priority      int            `json:"priority"`
	Latency       time.Duration  `json:"latency"`
	LastError     string         `json:"last_error,omitempty"`
	SuccessCount  int            `json:"success_count"`
	FailureCount  int            `json:"failure_count"`
	ConsecSuccess int            `json:"consec_success"`
	ConsecFailure int            `json:"consec_failure"`
	LastSuccessAt *time.Time     `json:"last_success_at,omitempty"`
	LastFailureAt *time.Time     `json:"last_failure_at,omitempty"`
}

// WatchdogIncident represents a tracked incident.
type WatchdogIncident struct {
	ID           string     `json:"id"`
	Provider     string     `json:"provider"`
	Status       string     `json:"status"` // open, resolved
	StartedAt    time.Time  `json:"started_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	FallbackUsed string     `json:"fallback_used,omitempty"`
	ErrorCount   int        `json:"error_count"`
}

// OutageReport represents a generated report.
type OutageReport struct {
	Status      string              `json:"status"`
	Summary     string              `json:"summary"`
	GeneratedAt time.Time           `json:"generated_at"`
	Providers   []*WatchdogProvider `json:"providers"`
	Incidents   []*WatchdogIncident `json:"incidents"`
	Suggestions []string            `json:"suggestions"`
}

// StatusRecovered is an alias for StatusHealthy.
var StatusRecovered = ProviderStatus("recovered")

// Watchdog monitors providers and handles fallbacks.
type Watchdog struct {
	config    WatchdogConfig
	providers map[string]*WatchdogProvider
	chains    map[string][]string // chain name -> ordered provider list
	incidents []*WatchdogIncident
}

// NewWatchdog creates a provider watchdog.
func NewWatchdog(config WatchdogConfig) *Watchdog {
	return &Watchdog{
		config:    config,
		providers: make(map[string]*WatchdogProvider),
		chains:    make(map[string][]string),
		incidents: make([]*WatchdogIncident, 0),
	}
}

// AddProvider adds a provider to monitor.
func (w *Watchdog) AddProvider(name string, priority int) {
	w.providers[name] = &WatchdogProvider{
		Name:     name,
		Status:   StatusUnknown,
		Priority: priority,
	}
}

// SetFallbackChain configures a fallback chain.
func (w *Watchdog) SetFallbackChain(name string, providers []string) {
	w.chains[name] = providers
}

// GetAllProviders returns all providers.
func (w *Watchdog) GetAllProviders() []*WatchdogProvider {
	result := make([]*WatchdogProvider, 0, len(w.providers))
	for _, p := range w.providers {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})
	return result
}

// GetBestProvider returns the best available provider for a chain.
func (w *Watchdog) GetBestProvider(chain string) (string, error) {
	providers, ok := w.chains[chain]
	if !ok {
		// Fall back to priority ordering
		all := w.GetAllProviders()
		for _, p := range all {
			if p.Status == StatusHealthy || p.Status == StatusUnknown {
				return p.Name, nil
			}
		}
		return "", fmt.Errorf("no healthy provider available")
	}

	for _, name := range providers {
		if p, ok := w.providers[name]; ok {
			if p.Status == StatusHealthy || p.Status == StatusUnknown || p.Status == StatusRecovered {
				return p.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no healthy provider in chain %q", chain)
}

// RecordResult records a provider call result.
func (w *Watchdog) RecordResult(result ProviderResult) {
	p, ok := w.providers[result.Provider]
	if !ok {
		return
	}

	now := time.Now()

	if result.Success {
		p.SuccessCount++
		p.ConsecSuccess++
		p.ConsecFailure = 0
		p.LastSuccessAt = &now
		p.Latency = result.Latency
		p.LastError = ""

		// Check for recovery
		if p.Status == StatusOutage && p.ConsecSuccess >= w.config.RecoveryThreshold {
			p.Status = StatusRecovered
			// Resolve any open incidents
			for _, inc := range w.incidents {
				if inc.Provider == result.Provider && inc.Status == "open" {
					inc.Status = "resolved"
					inc.ResolvedAt = &now
				}
			}
		} else if p.Status != StatusOutage {
			p.Status = StatusHealthy
		}
	} else {
		p.FailureCount++
		p.ConsecFailure++
		p.ConsecSuccess = 0
		p.LastFailureAt = &now
		p.LastError = result.Error

		// Check for outage
		if p.ConsecFailure >= w.config.FailureThreshold {
			if p.Status != StatusOutage {
				p.Status = StatusOutage
				w.incidents = append(w.incidents, &WatchdogIncident{
					ID:        fmt.Sprintf("inc-%d", now.UnixNano()),
					Provider:  result.Provider,
					Status:    "open",
					StartedAt: now,
				})
			}
			w.incidents[len(w.incidents)-1].ErrorCount++
		} else if p.ConsecFailure > 0 {
			p.Status = StatusDegraded
		}
	}
}

// GetIncidentHistory returns all incidents.
func (w *Watchdog) GetIncidentHistory() []*WatchdogIncident {
	return w.incidents
}

// GenerateReport creates an outage report.
func (w *Watchdog) GenerateReport() *OutageReport {
	report := &OutageReport{
		GeneratedAt: time.Now(),
		Providers:   w.GetAllProviders(),
		Incidents:   w.incidents,
		Suggestions: make([]string, 0),
	}

	// Determine overall status
	healthyCount := 0
	outageCount := 0
	for _, p := range report.Providers {
		switch p.Status {
		case StatusHealthy, StatusRecovered, StatusUnknown:
			healthyCount++
		case StatusOutage:
			outageCount++
		}
	}

	if outageCount == 0 {
		report.Status = "healthy"
		report.Summary = fmt.Sprintf("All %d providers operational", healthyCount)
	} else if outageCount < len(report.Providers) {
		report.Status = "degraded"
		report.Summary = fmt.Sprintf("%d of %d providers in outage", outageCount, len(report.Providers))
		report.Suggestions = append(report.Suggestions, "Consider switching to a healthy provider")
	} else {
		report.Status = "critical"
		report.Summary = "All providers in outage"
		report.Suggestions = append(report.Suggestions, "Check network connectivity")
		report.Suggestions = append(report.Suggestions, "Enable local model fallback")
	}

	// Add provider-specific suggestions
	for _, p := range report.Providers {
		if p.Status == StatusOutage {
			best, err := w.GetBestProvider("default")
			if err == nil {
				report.Suggestions = append(report.Suggestions,
					fmt.Sprintf("Switch from %s to %s", p.Name, best))
			}
		}
		if p.Status == StatusDegraded {
			report.Suggestions = append(report.Suggestions,
				fmt.Sprintf("Monitor %s — degraded performance", p.Name))
		}
	}

	// Deduplicate suggestions
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, s := range report.Suggestions {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	report.Suggestions = unique

	// Count open incidents
	openCount := 0
	for _, inc := range report.Incidents {
		if inc.Status == "open" {
			openCount++
		}
	}
	if openCount > 0 {
		report.Summary += fmt.Sprintf(" (%d open incidents)", openCount)
	}

	_ = strings.Contains // avoid unused import

	return report
}
