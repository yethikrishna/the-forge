package achievement

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker("")
	if tracker.TotalCount() == 0 {
		t.Error("expected default achievements")
	}
}

func TestUnlock(t *testing.T) {
	tracker := NewTracker("")

	a, err := tracker.Unlock("first-chat")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Unlocked {
		t.Error("should be unlocked")
	}
	if a.UnlockedAt.IsZero() {
		t.Error("should have unlock time")
	}
}

func TestUnlockNotFound(t *testing.T) {
	tracker := NewTracker("")
	_, err := tracker.Unlock("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent achievement")
	}
}

func TestUnlockIdempotent(t *testing.T) {
	tracker := NewTracker("")
	tracker.Unlock("first-chat")
	a, _ := tracker.Unlock("first-chat")
	if !a.Unlocked {
		t.Error("should stay unlocked")
	}
}

func TestSetProgress(t *testing.T) {
	tracker := NewTracker("")

	err := tracker.SetProgress("power-user", 0.5)
	if err != nil {
		t.Fatal(err)
	}

	a, _ := tracker.Get("power-user")
	if a.Progress != 0.5 {
		t.Errorf("expected 0.5, got %.1f", a.Progress)
	}
}

func TestSetProgressAutoUnlock(t *testing.T) {
	tracker := NewTracker("")

	tracker.SetProgress("power-user", 1.0)

	a, _ := tracker.Get("power-user")
	if !a.Unlocked {
		t.Error("should auto-unlock at 100%")
	}
}

func TestSetProgressClamps(t *testing.T) {
	tracker := NewTracker("")
	tracker.SetProgress("power-user", 2.0)

	a, _ := tracker.Get("power-user")
	if a.Progress != 1.0 {
		t.Errorf("expected clamped to 1.0, got %.1f", a.Progress)
	}
}

func TestGet(t *testing.T) {
	tracker := NewTracker("")

	a, ok := tracker.Get("first-chat")
	if !ok {
		t.Error("expected to find first-chat")
	}
	if a.Name != "Hello World" {
		t.Errorf("expected Hello World, got %s", a.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	tracker := NewTracker("")
	_, ok := tracker.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent")
	}
}

func TestListHidesHidden(t *testing.T) {
	tracker := NewTracker("")

	visible := tracker.List()
	for _, a := range visible {
		if a.Hidden && !a.Unlocked {
			t.Errorf("hidden achievement %s should not appear", a.ID)
		}
	}
}

func TestListShowsUnlockedHidden(t *testing.T) {
	tracker := NewTracker("")
	tracker.Unlock("power-user")

	visible := tracker.List()
	found := false
	for _, a := range visible {
		if a.ID == "power-user" {
			found = true
		}
	}
	if !found {
		t.Error("unlocked hidden achievement should appear in list")
	}
}

func TestListAll(t *testing.T) {
	tracker := NewTracker("")
	all := tracker.ListAll()
	if len(all) != tracker.TotalCount() {
		t.Error("ListAll should return all achievements including hidden")
	}
}

func TestUnlockedCount(t *testing.T) {
	tracker := NewTracker("")
	tracker.Unlock("first-chat")
	tracker.Unlock("first-agent")

	if tracker.UnlockedCount() != 2 {
		t.Errorf("expected 2, got %d", tracker.UnlockedCount())
	}
}

func TestStats(t *testing.T) {
	tracker := NewTracker("")
	tracker.Unlock("first-chat")

	stats := tracker.Stats()
	if stats.UnlockedTotal != 1 {
		t.Errorf("expected 1 unlocked, got %d", stats.UnlockedTotal)
	}
	if stats.Total == 0 {
		t.Error("expected total > 0")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "achievements.json")

	t1 := NewTracker(path)
	t1.Unlock("first-chat")
	t1.Unlock("first-agent")

	t2 := NewTracker(path)
	if t2.UnlockedCount() != 2 {
		t.Errorf("expected 2 unlocked after reload, got %d", t2.UnlockedCount())
	}

	a, _ := t2.Get("first-chat")
	if !a.Unlocked {
		t.Error("first-chat should persist as unlocked")
	}
}

func TestForgeMaster(t *testing.T) {
	tracker := NewTracker("")

	// Unlock all non-hidden achievements
	for _, a := range tracker.ListAll() {
		if a.ID != "forge-master" && !a.Hidden {
			tracker.Unlock(a.ID)
		}
	}

	fm, _ := tracker.Get("forge-master")
	if !fm.Unlocked {
		t.Error("forge-master should unlock when all visible achievements are done")
	}
}

func TestDefaultAchievements(t *testing.T) {
	tracker := NewTracker("")
	all := tracker.ListAll()

	expected := []string{"first-chat", "first-agent", "first-pipeline", "forge-master"}
	for _, id := range expected {
		found := false
		for _, a := range all {
			if a.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected achievement %s", id)
		}
	}
}

func TestFormatAchievement(t *testing.T) {
	a := Achievement{
		Name:        "Hello World",
		Description: "Start your first chat",
		Tier:        TierCommon,
		Icon:        "💬",
		Unlocked:    true,
	}

	s := FormatAchievement(a)
	if !strings.Contains(s, "Hello World") {
		t.Error("should contain name")
	}
	if !strings.Contains(s, "💬") {
		t.Error("should contain icon")
	}
}

func TestTierOrder(t *testing.T) {
	if tierOrder(TierCommon) >= tierOrder(TierLegendary) {
		t.Error("common should have lower order than legendary")
	}
}
