package achievement

import (
	"testing"
)

func TestNewTracker(t *testing.T) {
	tr, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if tr == nil {
		t.Fatal("expected non-nil tracker")
	}
}

func TestDefaultAchievements(t *testing.T) {
	achs := DefaultAchievements()
	if len(achs) < 10 {
		t.Errorf("expected at least 10 achievements, got %d", len(achs))
	}
}

func TestTrackEventFirstChat(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	unlocked := tr.TrackEvent("chat", "agent1", "sess1")
	found := false
	for _, a := range unlocked {
		if a.ID == "first_chat" {
			found = true
		}
	}
	if !found {
		t.Error("expected first_chat to unlock")
	}
}

func TestTrackEventFirstAgent(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	unlocked := tr.TrackEvent("agent_create", "agent1", "sess1")
	found := false
	for _, a := range unlocked {
		if a.ID == "first_agent" {
			found = true
		}
	}
	if !found {
		t.Error("expected first_agent to unlock")
	}
}

func TestTrackEventTenChats(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	for i := 0; i < 9; i++ {
		tr.TrackEvent("chat", "agent1", "sess1")
	}
	unlocked := tr.TrackEvent("chat", "agent1", "sess1")
	found := false
	for _, a := range unlocked {
		if a.ID == "ten_chats" {
			found = true
		}
	}
	if !found {
		t.Error("expected ten_chats to unlock on 10th chat")
	}
}

func TestPrerequisite(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	// hundred_chats requires ten_chats
	// Even if we send 100 chats, hundred_chats should unlock after ten_chats
	for i := 0; i < 100; i++ {
		tr.TrackEvent("chat", "agent1", "sess1")
	}
	if !tr.IsUnlocked("hundred_chats") {
		t.Error("expected hundred_chats to unlock after 100 chats")
	}
}

func TestNoDoubleUnlock(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	tr.TrackEvent("chat", "agent1", "sess1")
	unlocked := tr.TrackEvent("chat", "agent1", "sess1")
	for _, a := range unlocked {
		if a.ID == "first_chat" {
			t.Error("first_chat should not unlock twice")
		}
	}
}

func TestGetProfile(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	tr.TrackEvent("chat", "agent1", "sess1")
	tr.TrackEvent("agent_create", "agent1", "sess1")
	profile := tr.GetProfile()
	if profile.TotalPoints <= 0 {
		t.Error("expected positive points")
	}
	if profile.Level < 1 {
		t.Error("expected level >= 1")
	}
}

func TestListAchievements(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	list := tr.ListAchievements()
	if len(list) == 0 {
		t.Error("expected achievements")
	}
}

func TestGetStats(t *testing.T) {
	tr, _ := NewTracker(t.TempDir())
	tr.TrackEvent("chat", "agent1", "sess1")
	stats := tr.GetStats()
	if stats["unlocked"].(int) < 1 {
		t.Error("expected at least 1 unlocked")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	tr1, _ := NewTracker(dir)
	tr1.TrackEvent("chat", "agent1", "sess1")

	tr2, _ := NewTracker(dir)
	if !tr2.IsUnlocked("first_chat") {
		t.Error("expected first_chat to persist")
	}
}

func TestFormatAchievement(t *testing.T) {
	ach := &Achievement{ID: "test", Name: "Test", Description: "Test ach", Icon: "🏆", Points: 10, Rarity: RarityCommon}
	output := FormatAchievement(ach, true)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatProfile(t *testing.T) {
	p := &Profile{Level: 5, Title: "Artisan", TotalPoints: 420, Unlocks: make([]Unlock, 3)}
	output := FormatProfile(p)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestLevelTitle(t *testing.T) {
	tests := []struct{ level int; want string }{
		{1, "Apprentice"},
		{2, "Journeyman"},
		{5, "Expert"},
		{100, "Legendary Smith"},
	}
	for _, tt := range tests {
		got := levelTitle(tt.level)
		if got != tt.want {
			t.Errorf("levelTitle(%d) = %s, want %s", tt.level, got, tt.want)
		}
	}
}
