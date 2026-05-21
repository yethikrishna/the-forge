package experience_test

import (
	"testing"

	"github.com/forge/sword/internal/experience/achievement"
	"github.com/forge/sword/internal/experience/empath"
	"github.com/forge/sword/internal/experience/feedback"
)

func TestAchievementTracker(t *testing.T) {
	tracker := achievement.NewTracker(t.TempDir())

	// Should have defaults
	all := tracker.ListAll()
	if len(all) == 0 {
		t.Error("Should have built-in achievements")
	}

	// Unlock an achievement
	a, err := tracker.Unlock("first-chat")
	if err != nil {
		t.Fatalf("Unlock error: %v", err)
	}
	if !a.Unlocked {
		t.Error("Achievement should be unlocked after Unlock()")
	}
}

func TestAchievementGet(t *testing.T) {
	tracker := achievement.NewTracker(t.TempDir())
	a, ok := tracker.Get("first-chat")
	if !ok {
		t.Fatal("Get should find built-in achievement")
	}
	if a.ID != "first-chat" {
		t.Errorf("ID = %q, want %q", a.ID, "first-chat")
	}
}

func TestAchievementStats(t *testing.T) {
	tracker := achievement.NewTracker(t.TempDir())
	tracker.Unlock("first-chat")

	stats := tracker.Stats()
	if stats.UnlockedTotal < 1 {
		t.Error("Stats should show at least 1 unlocked")
	}
}

func TestAchievementProgress(t *testing.T) {
	tracker := achievement.NewTracker(t.TempDir())
	err := tracker.SetProgress("first-chat", 0.5)
	if err != nil {
		t.Fatalf("SetProgress error: %v", err)
	}
}

func TestEmpathAnalyzer(t *testing.T) {
	analyzer := empath.NewAnalyzer()

	// Normal input
	result := analyzer.Analyze("Hello, could you help me with this?")
	if result.Score > 50 {
		t.Error("Normal input should have low frustration score")
	}

	// Frustrated input
	result = analyzer.Analyze("WHY ISN'T THIS WORKING!!!")
	if result.Score < 30 {
		t.Error("Caps + exclamation should have higher frustration score")
	}
}

func TestEmpathTrend(t *testing.T) {
	analyzer := empath.NewAnalyzer()
	analyzer.Analyze("hello")
	analyzer.Analyze("error")
	analyzer.Analyze("ERROR AGAIN!!")

	trend := analyzer.Trend()
	if trend == "" {
		t.Error("Trend should not be empty")
	}
}

func TestEmpathHistory(t *testing.T) {
	analyzer := empath.NewAnalyzer()
	analyzer.Analyze("hello")
	analyzer.Analyze("frustrated!")

	history := analyzer.History()
	if len(history) < 2 {
		t.Errorf("History = %d entries, want at least 2", len(history))
	}
}

func TestFeedbackStoreRecord(t *testing.T) {
	store := feedback.NewStore(t.TempDir())

	sig, err := store.Record(feedback.Signal{
		Type:    feedback.SignalRating,
		Response: "Great experience",
		Rating:   5,
	})
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}
	if sig.ID == "" {
		t.Error("Recorded signal should have an ID")
	}
}

func TestFeedbackStoreList(t *testing.T) {
	store := feedback.NewStore(t.TempDir())
	store.Record(feedback.Signal{Type: feedback.SignalRating, Rating: 4})
	store.Record(feedback.Signal{Type: feedback.SignalBug, Rating: 1})

	list, err := store.List("", 10)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List = %d, want 2", len(list))
	}
}

func TestFeedbackStoreGet(t *testing.T) {
	store := feedback.NewStore(t.TempDir())
	sig, _ := store.Record(feedback.Signal{Type: feedback.SignalRating, Rating: 3})

	got, err := store.Get(sig.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.ID != sig.ID {
		t.Errorf("Get ID = %q, want %q", got.ID, sig.ID)
	}
}

func TestFeedbackStoreDelete(t *testing.T) {
	store := feedback.NewStore(t.TempDir())
	sig, _ := store.Record(feedback.Signal{Type: feedback.SignalRating, Rating: 3})

	if err := store.Delete(sig.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	_, err := store.Get(sig.ID)
	if err == nil {
		t.Error("Get after delete should return error")
	}
}
