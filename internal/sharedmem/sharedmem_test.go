package sharedmem

import (
	"strings"
	"testing"
)

func TestNewStore(t *testing.T) {
	s := NewStore("")
	if s == nil {
		t.Fatal("expected store")
	}
}

func TestDefaultPatterns(t *testing.T) {
	s := NewStore("")
	patterns := s.List("", "")
	if len(patterns) < 3 {
		t.Errorf("expected 3+ defaults, got %d", len(patterns))
	}
}

func TestContribute(t *testing.T) {
	s := NewStore("")
	p, err := s.Contribute(Pattern{
		Name:        "Custom pattern",
		Category:    "testing",
		Description: "My pattern",
		Steps:       []string{"Step 1", "Step 2"},
		Privacy:     PrivacyTeam,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" {
		t.Error("expected ID")
	}
	if p.Privacy != PrivacyTeam {
		t.Error("privacy mismatch")
	}
}

func TestContributeNoName(t *testing.T) {
	s := NewStore("")
	_, err := s.Contribute(Pattern{})
	if err == nil {
		t.Error("should require name")
	}
}

func TestGet(t *testing.T) {
	s := NewStore("")
	patterns := s.List("", "")
	if len(patterns) == 0 {
		t.Fatal("no patterns")
	}
	got, ok := s.Get(patterns[0].ID)
	if !ok {
		t.Fatal("should find")
	}
	if got.Name != patterns[0].Name {
		t.Error("name mismatch")
	}
}

func TestGetNotFound(t *testing.T) {
	s := NewStore("")
	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestListByCategory(t *testing.T) {
	s := NewStore("")
	s.Contribute(Pattern{Name: "Test", Category: "testing"})

	testing := s.List("testing", "")
	for _, p := range testing {
		if p.Category != "testing" {
			t.Error("should only return testing category")
		}
	}
}

func TestListByPrivacy(t *testing.T) {
	s := NewStore("")
	s.Contribute(Pattern{Name: "Team only", Category: "test", Privacy: PrivacyTeam})

	public := s.List("", PrivacyPublic)
	for _, p := range public {
		if p.Privacy != PrivacyPublic {
			t.Error("should only return public patterns")
		}
	}
}

func TestSearch(t *testing.T) {
	s := NewStore("")
	results := s.Search("test")
	if len(results) == 0 {
		t.Error("should find 'test after edit' pattern")
	}
}

func TestSearchByTag(t *testing.T) {
	s := NewStore("")
	results := s.Search("workflow")
	if len(results) == 0 {
		t.Error("should find patterns with workflow tag")
	}
}

func TestRecordUse(t *testing.T) {
	s := NewStore("")
	patterns := s.List("", "")
	err := s.RecordUse(patterns[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(patterns[0].ID)
	if got.Uses != 1 {
		t.Errorf("expected 1 use, got %d", got.Uses)
	}
}

func TestRecordUseUpdatesSuccessRate(t *testing.T) {
	s := NewStore("")
	p, _ := s.Contribute(Pattern{Name: "Test", Category: "test", SuccessRate: 1.0})
	s.RecordUse(p.ID, true)
	s.RecordUse(p.ID, false)

	got, _ := s.Get(p.ID)
	if got.Uses != 2 {
		t.Errorf("expected 2 uses, got %d", got.Uses)
	}
	if got.SuccessRate >= 1.0 {
		t.Errorf("rate should decrease after failure: %.2f", got.SuccessRate)
	}
}

func TestRecordUseNotFound(t *testing.T) {
	s := NewStore("")
	err := s.RecordUse("nonexistent", true)
	if err == nil {
		t.Error("should error")
	}
}

func TestAddInsight(t *testing.T) {
	s := NewStore("")
	patterns := s.List("", "")
	ins, err := s.AddInsight(patterns[0].ID, "tip", "Always test", "Run tests after changes", "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if ins.ID == "" {
		t.Error("expected ID")
	}
}

func TestGetInsights(t *testing.T) {
	s := NewStore("")
	patterns := s.List("", "")
	s.AddInsight(patterns[0].ID, "tip", "Tip 1", "Content", "a1")
	s.AddInsight(patterns[0].ID, "warning", "Warning", "Watch out", "a2")

	insights := s.GetInsights(patterns[0].ID)
	if len(insights) != 2 {
		t.Errorf("expected 2 insights, got %d", len(insights))
	}
}

func TestVoteUp(t *testing.T) {
	s := NewStore("")
	patterns := s.List("", "")
	ins, _ := s.AddInsight(patterns[0].ID, "tip", "Tip", "Content", "a1")
	s.VoteUp(ins.ID)
	s.VoteUp(ins.ID)

	insights := s.GetInsights(patterns[0].ID)
	if insights[0].Votes != 2 {
		t.Errorf("expected 2 votes, got %d", insights[0].Votes)
	}
}

func TestVoteUpNotFound(t *testing.T) {
	s := NewStore("")
	err := s.VoteUp("nonexistent")
	if err == nil {
		t.Error("should error")
	}
}

func TestStats(t *testing.T) {
	s := NewStore("")
	stats := s.Stats()
	if stats["patterns"].(int) < 3 {
		t.Error("should have patterns")
	}
}

func TestTop(t *testing.T) {
	s := NewStore("")
	top := s.Top(2)
	if len(top) > 2 {
		t.Error("should limit results")
	}
	// Should be sorted by success rate descending
	for i := 1; i < len(top); i++ {
		if top[i].SuccessRate > top[i-1].SuccessRate {
			t.Error("should be sorted by success rate")
		}
	}
}

func TestFormatPattern(t *testing.T) {
	p := &Pattern{ID: "pat-1", Category: "workflow", SuccessRate: 0.9, Uses: 10, Name: "Test after edit"}
	s := FormatPattern(p)
	if !strings.Contains(s, "90%") {
		t.Error("should show success rate")
	}
}

func TestFormatInsight(t *testing.T) {
	ins := &Insight{Type: "tip", Title: "Test always", Votes: 5, Content: "Run tests"}
	s := FormatInsight(ins)
	if !strings.Contains(s, "5 votes") {
		t.Error("should show votes")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s1 := NewStore(dir)
	s1.Contribute(Pattern{Name: "persist-test", Category: "test"})

	s2 := NewStore(dir)
	results := s2.Search("persist-test")
	if len(results) == 0 {
		t.Fatal("pattern should persist")
	}
}

func TestListSortedBySuccessRate(t *testing.T) {
	s := NewStore("")
	s.Contribute(Pattern{Name: "Low", Category: "test", SuccessRate: 0.3})
	s.Contribute(Pattern{Name: "High", Category: "test", SuccessRate: 0.95})

	list := s.List("test", "")
	if len(list) < 2 {
		t.Fatal("need 2 patterns")
	}
	if list[0].SuccessRate < list[1].SuccessRate {
		t.Error("should be sorted by success rate descending")
	}
}
