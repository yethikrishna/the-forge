package overview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	dir := t.TempDir()
	c := NewCollector(dir, "1.0.0")
	if c == nil {
		t.Fatal("expected collector")
	}
}

func TestCollect(t *testing.T) {
	dir := t.TempDir()
	c := NewCollector(dir, "1.0.0")
	ov := c.Collect()

	if ov == nil {
		t.Fatal("expected overview")
	}
	if ov.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", ov.Version)
	}
	if ov.GeneratedAt.IsZero() {
		t.Error("expected timestamp")
	}
}

func TestCollectHealth(t *testing.T) {
	dir := t.TempDir()
	c := NewCollector(dir, "1.0.0")
	ov := c.Collect()

	// With no forge dir, should be degraded
	if ov.Health.Status != "degraded" && ov.Health.Status != "healthy" {
		t.Errorf("unexpected health status: %s", ov.Health.Status)
	}
}

func TestCollectWithAgentsDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "agents", "my-agent"), 0755)

	c := NewCollector(dir, "1.0.0")
	ov := c.Collect()

	if len(ov.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(ov.Agents))
	}
	if ov.Agents[0].Name != "my-agent" {
		t.Errorf("expected my-agent, got %s", ov.Agents[0].Name)
	}
}

func TestCollectWithSessions(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sessions", "sess-1"), 0755)
	os.MkdirAll(filepath.Join(dir, "sessions", "sess-2"), 0755)

	c := NewCollector(dir, "1.0.0")
	ov := c.Collect()

	if len(ov.Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(ov.Sessions))
	}
}

func TestQuickActions(t *testing.T) {
	dir := t.TempDir()
	c := NewCollector(dir, "1.0.0")
	ov := c.Collect()

	if len(ov.QuickActions) == 0 {
		t.Error("expected quick actions")
	}
}

func TestFormatOverview(t *testing.T) {
	ov := &Overview{
		GeneratedAt: time.Now(),
		Version:     "1.0.0",
		Uptime:      "2h30m",
		Health:      HealthStatus{Status: "healthy", Score: 100},
		Agents: []AgentStatus{
			{Name: "coder", Status: "idle"},
		},
		Cost: CostEntry{TodayUSD: 1.50, MonthUSD: 25.00, TokensToday: 50000},
		Sessions: []SessionEntry{
			{ID: "sess-1", Agent: "coder"},
		},
		QuickActions: []QuickAction{
			{Command: "forge chat", Description: "Start a chat"},
		},
	}

	s := FormatOverview(ov)
	if !strings.Contains(s, "FORGE OVERVIEW") {
		t.Error("should contain title")
	}
	if !strings.Contains(s, "coder") {
		t.Error("should mention agents")
	}
	if !strings.Contains(s, "$1.50") {
		t.Error("should show cost")
	}
}

func TestFormatOverviewJSON(t *testing.T) {
	ov := &Overview{
		GeneratedAt: time.Now(),
		Version:     "1.0.0",
		Health:      HealthStatus{Status: "healthy", Score: 100},
	}

	s, err := FormatOverviewJSON(ov)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "healthy") {
		t.Error("JSON should contain health status")
	}

	// Verify it's valid JSON
	var parsed Overview
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}

func TestFormatOverviewWithAlerts(t *testing.T) {
	ov := &Overview{
		GeneratedAt: time.Now(),
		Version:     "1.0.0",
		Health:      HealthStatus{Status: "degraded", Score: 60},
		Alerts: []Alert{
			{Level: "warning", Message: "Budget 80% used", Time: time.Now()},
			{Level: "error", Message: "Agent crashed", Time: time.Now()},
		},
	}

	s := FormatOverview(ov)
	if !strings.Contains(s, "Budget 80%") {
		t.Error("should show alerts")
	}
}

func TestDefaultForgeDir(t *testing.T) {
	c := NewCollector("", "1.0.0")
	if c.forgeDir == "" {
		t.Error("should default forge dir")
	}
}

func TestAgentStatusStates(t *testing.T) {
	ov := &Overview{
		GeneratedAt: time.Now(),
		Agents: []AgentStatus{
			{Name: "a1", Status: "running"},
			{Name: "a2", Status: "idle"},
			{Name: "a3", Status: "error"},
			{Name: "a4", Status: "stopped"},
		},
	}

	s := FormatOverview(ov)
	if !strings.Contains(s, "a1") || !strings.Contains(s, "a3") {
		t.Error("should list all agents")
	}
}

func TestCostDisplay(t *testing.T) {
	ov := &Overview{
		GeneratedAt: time.Now(),
		Cost: CostEntry{
			TodayUSD:    5.67,
			MonthUSD:    123.45,
			TokensToday: 100000,
		},
	}

	s := FormatOverview(ov)
	if !strings.Contains(s, "$5.67") {
		t.Error("should show today's cost")
	}
	if !strings.Contains(s, "$123.45") {
		t.Error("should show month cost")
	}
}

func TestEmptyOverview(t *testing.T) {
	ov := &Overview{
		GeneratedAt: time.Now(),
	}

	s := FormatOverview(ov)
	if !strings.Contains(s, "FORGE OVERVIEW") {
		t.Error("should render even when empty")
	}
}
