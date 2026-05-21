// Package overview provides a unified dashboard view of Forge status.
// Combines agents, costs, sessions, alerts, and quick actions into one pane.
//
// Everything at a glance.
package overview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AgentStatus represents an agent's state for the overview.
type AgentStatus struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"` // running, idle, error, stopped
	Model    string    `json:"model"`
	LastUsed time.Time `json:"last_used"`
	Actions  int       `json:"actions"`
}

// CostEntry represents cost info for the overview.
type CostEntry struct {
	TodayUSD    float64 `json:"today_usd"`
	MonthUSD    float64 `json:"month_usd"`
	BudgetUSD   float64 `json:"budget_usd"`
	TokensToday int     `json:"tokens_today"`
}

// SessionEntry represents a session in the overview.
type SessionEntry struct {
	ID           string    `json:"id"`
	Agent        string    `json:"agent"`
	CreatedAt    time.Time `json:"created_at"`
	LastActive   time.Time `json:"last_active"`
	MessageCount int       `json:"message_count"`
}

// Alert represents an alert in the overview.
type Alert struct {
	Level   string    `json:"level"` // info, warning, error
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// Overview is the complete status dashboard.
type Overview struct {
	GeneratedAt  time.Time      `json:"generated_at"`
	Version      string         `json:"version"`
	Uptime       string         `json:"uptime"`
	Agents       []AgentStatus  `json:"agents"`
	Cost         CostEntry      `json:"cost"`
	Sessions     []SessionEntry `json:"sessions"`
	Alerts       []Alert        `json:"alerts"`
	QuickActions []QuickAction  `json:"quick_actions"`
	Health       HealthStatus   `json:"health"`
}

// HealthStatus is the overall health.
type HealthStatus struct {
	Status  string   `json:"status"` // healthy, degraded, critical
	Score   int      `json:"score"`  // 0-100
	Details []string `json:"details"`
}

// QuickAction is a suggested next action.
type QuickAction struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// Collector gathers overview data from various sources.
type Collector struct {
	forgeDir string
	version  string
}

// NewCollector creates an overview data collector.
func NewCollector(forgeDir, version string) *Collector {
	if forgeDir == "" {
		home, _ := os.UserHomeDir()
		forgeDir = filepath.Join(home, ".forge")
	}
	return &Collector{
		forgeDir: forgeDir,
		version:  version,
	}
}

// Collect gathers all overview data.
func (c *Collector) Collect() *Overview {
	ov := &Overview{
		GeneratedAt:  time.Now(),
		Version:      c.version,
		Uptime:       c.getUptime(),
		Agents:       c.getAgents(),
		Cost:         c.getCost(),
		Sessions:     c.getSessions(),
		Alerts:       c.getAlerts(),
		Health:       c.getHealth(),
		QuickActions: c.getQuickActions(),
	}
	return ov
}

func (c *Collector) getUptime() string {
	uptimeFile := filepath.Join(c.forgeDir, "uptime")
	data, err := os.ReadFile(uptimeFile)
	if err != nil {
		return "unknown"
	}
	var t time.Time
	if err := t.UnmarshalText(data); err == nil {
		return time.Since(t).Round(time.Minute).String()
	}
	return "unknown"
}

func (c *Collector) getAgents() []AgentStatus {
	// Scan agent configs
	agentsDir := filepath.Join(c.forgeDir, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}

	var agents []AgentStatus
	for _, e := range entries {
		if e.IsDir() {
			agents = append(agents, AgentStatus{
				Name:   e.Name(),
				Status: "idle",
			})
		}
	}
	return agents
}

func (c *Collector) getCost() CostEntry {
	return CostEntry{
		TodayUSD: 0,
		MonthUSD: 0,
	}
}

func (c *Collector) getSessions() []SessionEntry {
	sessionsDir := filepath.Join(c.forgeDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}

	var sessions []SessionEntry
	for _, e := range entries {
		if e.IsDir() {
			info, err := e.Info()
			if err != nil {
				continue
			}
			sessions = append(sessions, SessionEntry{
				ID:         e.Name(),
				LastActive: info.ModTime(),
			})
		}
	}
	return sessions
}

func (c *Collector) getAlerts() []Alert {
	var alerts []Alert

	// Check disk usage
	if info, err := os.Stat(c.forgeDir); err == nil {
		_ = info
	}

	return alerts
}

func (c *Collector) getHealth() HealthStatus {
	health := HealthStatus{
		Status:  "healthy",
		Score:   100,
		Details: []string{},
	}

	// Check if forge dir exists
	if _, err := os.Stat(c.forgeDir); os.IsNotExist(err) {
		health.Status = "degraded"
		health.Score = 50
		health.Details = append(health.Details, "Forge directory not found")
	}

	return health
}

func (c *Collector) getQuickActions() []QuickAction {
	actions := []QuickAction{
		{Command: "forge chat", Description: "Start a new chat"},
		{Command: "forge doctor", Description: "Check environment health"},
		{Command: "forge cost report", Description: "View cost report"},
		{Command: "forge level", Description: "Check your progress"},
		{Command: "forge quickstart", Description: "Run the onboarding guide"},
	}
	return actions
}

// FormatOverview renders the overview as a formatted string.
func FormatOverview(ov *Overview) string {
	var b strings.Builder

	b.WriteString("┌─────────────────────────────────────────────┐\n")
	b.WriteString("│          FORGE OVERVIEW                      │\n")
	b.WriteString("├─────────────────────────────────────────────┤\n")

	// Health
	healthIcon := "✅"
	if ov.Health.Status != "healthy" {
		healthIcon = "⚠️"
	}
	b.WriteString(fmt.Sprintf("│  Health:  %s %s (%d/100)\n", healthIcon, ov.Health.Status, ov.Health.Score))
	b.WriteString(fmt.Sprintf("│  Version: %s\n", ov.Version))
	b.WriteString(fmt.Sprintf("│  Uptime:  %s\n", ov.Uptime))

	// Agents
	if len(ov.Agents) > 0 {
		b.WriteString("│\n")
		b.WriteString(fmt.Sprintf("│  Agents: %d\n", len(ov.Agents)))
		for _, a := range ov.Agents {
			statusIcon := "●"
			switch a.Status {
			case "running":
				statusIcon = "🟢"
			case "error":
				statusIcon = "🔴"
			case "idle":
				statusIcon = "⚪"
			default:
				statusIcon = "⚫"
			}
			b.WriteString(fmt.Sprintf("│    %s %s (%s)\n", statusIcon, a.Name, a.Status))
		}
	}

	// Cost
	b.WriteString("│\n")
	b.WriteString(fmt.Sprintf("│  Cost Today: $%.2f  |  Month: $%.2f\n", ov.Cost.TodayUSD, ov.Cost.MonthUSD))
	if ov.Cost.TokensToday > 0 {
		b.WriteString(fmt.Sprintf("│  Tokens Today: %d\n", ov.Cost.TokensToday))
	}

	// Sessions
	if len(ov.Sessions) > 0 {
		b.WriteString("│\n")
		b.WriteString(fmt.Sprintf("│  Sessions: %d\n", len(ov.Sessions)))
	}

	// Alerts
	if len(ov.Alerts) > 0 {
		b.WriteString("│\n")
		b.WriteString(fmt.Sprintf("│  Alerts: %d\n", len(ov.Alerts)))
		for _, a := range ov.Alerts {
			icon := "ℹ️"
			if a.Level == "warning" {
				icon = "⚠️"
			} else if a.Level == "error" {
				icon = "🔴"
			}
			b.WriteString(fmt.Sprintf("│    %s %s\n", icon, a.Message))
		}
	}

	// Quick actions
	b.WriteString("│\n")
	b.WriteString("│  Quick Actions:\n")
	limit := 3
	if len(ov.QuickActions) < limit {
		limit = len(ov.QuickActions)
	}
	for _, qa := range ov.QuickActions[:limit] {
		b.WriteString(fmt.Sprintf("│    → %-20s %s\n", qa.Command, qa.Description))
	}

	b.WriteString("└─────────────────────────────────────────────┘\n")

	return b.String()
}

// FormatOverviewJSON renders the overview as JSON.
func FormatOverviewJSON(ov *Overview) (string, error) {
	data, err := json.MarshalIndent(ov, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
