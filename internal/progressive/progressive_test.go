package progressive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLadder(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	if l.UserLevel != Level0 {
		t.Errorf("expected Level0, got %s", l.UserLevel)
	}
	if len(l.Milestones) < 20 {
		t.Errorf("expected at least 20 milestones, got %d", len(l.Milestones))
	}
}

func TestCompleteMilestone(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	m, err := l.Complete("install")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.Completed {
		t.Error("expected milestone to be completed")
	}
	if l.XP <= 0 {
		t.Error("expected XP to increase")
	}
}

func TestCompleteMilestoneTwice(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	l.Complete("install")
	xp1 := l.XP
	l.Complete("install")
	xp2 := l.XP

	if xp2 != xp1 {
		t.Error("should not award XP twice for same milestone")
	}
}

func TestCompleteNotFound(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	_, err := l.Complete("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent milestone")
	}
}

func TestLevelUp(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	// Complete all Level0 milestones
	for _, m := range l.Milestones {
		if m.Level == Level0 {
			l.Complete(m.ID)
		}
	}

	if l.UserLevel < Level1 {
		t.Errorf("expected at least Level1 after completing Level0 milestones, got %s", l.UserLevel)
	}
}

func TestMilestonesForLevel(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	level1 := l.MilestonesForLevel(Level1)
	if len(level1) == 0 {
		t.Error("expected milestones for Level1")
	}

	for _, m := range level1 {
		if m.Level != Level1 {
			t.Errorf("expected Level1, got %s", m.Level)
		}
	}
}

func TestNextSteps(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	steps := l.NextSteps()
	if len(steps) == 0 {
		t.Error("expected next steps")
	}

	// All should be incomplete
	for _, s := range steps {
		if s.Completed {
			t.Errorf("expected incomplete milestone, got %s", s.ID)
		}
	}
}

func TestProgress(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	progress := l.Progress()
	if len(progress) != 6 {
		t.Errorf("expected 6 levels, got %d", len(progress))
	}

	// Level0 should have some milestones
	if progress[Level0].Total == 0 {
		t.Error("expected milestones for Level0")
	}
}

func TestOverallProgress(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	pct := l.OverallProgress()
	if pct != 0 {
		t.Errorf("expected 0%% progress initially, got %.1f%%", pct)
	}

	// Complete one milestone
	for _, m := range l.Milestones {
		if m.Level == Level0 {
			l.Complete(m.ID)
			break
		}
	}

	pct = l.OverallProgress()
	if pct <= 0 {
		t.Error("expected positive progress after completing a milestone")
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	path := l.Path()
	if !strings.Contains(path, "Level 0") {
		t.Error("path should contain Level 0")
	}
	if !strings.Contains(path, "Curious") {
		t.Error("path should contain level name")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	l := NewLadder(filepath.Join(dir, "ladder.json"))

	stats := l.Stats()
	if stats["level"] != "Curious" {
		t.Errorf("expected Curious, got %v", stats["level"])
	}
	if stats["xp"] != 0 {
		t.Errorf("expected 0 XP initially, got %v", stats["xp"])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ladder.json")

	l1 := NewLadder(path)
	l1.Complete("install")
	l1.Complete("first-chat")
	level1 := l1.UserLevel
	xp1 := l1.XP

	// Create new ladder instance — should load from file
	l2 := NewLadder(path)
	if l2.UserLevel != level1 {
		t.Errorf("expected level %s, got %s", level1, l2.UserLevel)
	}
	if l2.XP != xp1 {
		t.Errorf("expected XP %d, got %d", xp1, l2.XP)
	}

	// Verify milestone state persisted
	m, ok := l2.Milestones["install"]
	if !ok || !m.Completed {
		t.Error("expected install milestone to be persisted as completed")
	}
}

func TestLevelStrings(t *testing.T) {
	tests := []struct {
		level Level
		name  string
		icon  string
	}{
		{Level0, "Curious", "🌱"},
		{Level1, "Explorer", "🧭"},
		{Level2, "Builder", "🔨"},
		{Level3, "Architect", "🏛️"},
		{Level4, "Operator", "⚙️"},
		{Level5, "Master", "👑"},
	}

	for _, tt := range tests {
		if tt.level.String() != tt.name {
			t.Errorf("Level%d.String() = %q, want %q", tt.level, tt.level.String(), tt.name)
		}
		if tt.level.Icon() != tt.icon {
			t.Errorf("Level%d.Icon() = %q, want %q", tt.level, tt.level.Icon(), tt.icon)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ladder.json")

	l := NewLadder(path)
	l.Complete("install")

	// Read the file and unmarshal
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if raw["user_level"] == nil {
		t.Error("expected user_level in JSON")
	}
	if raw["xp"] == nil {
		t.Error("expected xp in JSON")
	}
}

func TestXPForLevel(t *testing.T) {
	tests := []struct {
		level Level
		xp    int
	}{
		{Level0, 10},
		{Level1, 25},
		{Level2, 50},
		{Level3, 100},
		{Level4, 200},
		{Level5, 500},
	}

	for _, tt := range tests {
		got := xpForLevel(tt.level)
		if got != tt.xp {
			t.Errorf("xpForLevel(%d) = %d, want %d", tt.level, got, tt.xp)
		}
	}
}
