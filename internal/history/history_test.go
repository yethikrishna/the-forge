package history

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "history.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.CaseStudies = append(s.CaseStudies, CaseStudy{ID: "cs_1", Title: "Netscape"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.CaseStudies) != 1 || s2.CaseStudies[0].Title != "Netscape" {
		t.Errorf("unexpected after load: %+v", s2.CaseStudies)
	}
}

func TestRecordCaseStudy(t *testing.T) {
	cs := RecordCaseStudy("Yahoo's Missed Acquisitions", "tech", "2000s",
		"Yahoo missed buying Google and Facebook", "failure",
		[]string{"act on opportunities", "don't overthink valuations"})
	if cs.Title != "Yahoo's Missed Acquisitions" {
		t.Errorf("unexpected title: %s", cs.Title)
	}
	if cs.Outcome != "failure" {
		t.Errorf("expected failure, got %s", cs.Outcome)
	}
	if len(cs.Lessons) != 2 {
		t.Errorf("expected 2 lessons, got %d", len(cs.Lessons))
	}
}

func TestSearchHistory_Found(t *testing.T) {
	studies := []CaseStudy{
		{Title: "Netscape vs IE", Summary: "Browser wars of the 90s"},
		{Title: "Google's Rise", Summary: "Search dominance"},
	}
	results := SearchHistory(studies, "Netscape")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchHistory_ByLesson(t *testing.T) {
	studies := []CaseStudy{
		{Title: "Case A", Lessons: []string{"pivot early"}},
		{Title: "Case B", Lessons: []string{"stay focused"}},
	}
	results := SearchHistory(studies, "pivot")
	if len(results) != 1 || results[0].Title != "Case A" {
		t.Errorf("expected Case A, got %+v", results)
	}
}

func TestSearchHistory_NotFound(t *testing.T) {
	studies := []CaseStudy{
		{Title: "Boring Case", Summary: "Nothing exciting"},
	}
	results := SearchHistory(studies, "quantum")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRecognizePatterns(t *testing.T) {
	studies := []CaseStudy{
		{Title: "A", Era: "1990s", Outcome: "failure"},
		{Title: "B", Era: "2000s", Outcome: "failure"},
		{Title: "C", Era: "2010s", Outcome: "success"},
	}
	patterns := RecognizePatterns(studies)
	if len(patterns) == 0 {
		t.Error("expected at least one pattern from recurring failures")
	}
	found := false
	for _, p := range patterns {
		if p.Name == "recurring_failure" {
			found = true
			if p.Frequency != 2 {
				t.Errorf("expected frequency 2, got %d", p.Frequency)
			}
		}
	}
	if !found {
		t.Error("expected recurring_failure pattern")
	}
}

func TestRecognizePatterns_NoRecurring(t *testing.T) {
	studies := []CaseStudy{
		{Title: "A", Era: "1990s", Outcome: "success"},
		{Title: "B", Era: "2000s", Outcome: "failure"},
		{Title: "C", Era: "2010s", Outcome: "mixed"},
	}
	patterns := RecognizePatterns(studies)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns with no recurring outcomes, got %d", len(patterns))
	}
}

func TestDetectCollapseSignals_Financial(t *testing.T) {
	signals := DetectCollapseSignals(map[string]float64{"burn_rate_multiplier": 3.5})
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].SignalType != "financial" {
		t.Errorf("expected financial, got %s", signals[0].SignalType)
	}
	if signals[0].Urgency != "critical" {
		t.Errorf("expected critical urgency for 3.5x burn, got %s", signals[0].Urgency)
	}
}

func TestDetectCollapseSignals_Cultural(t *testing.T) {
	signals := DetectCollapseSignals(map[string]float64{"key_talent_departure_rate": 0.2})
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].SignalType != "cultural" {
		t.Errorf("expected cultural, got %s", signals[0].SignalType)
	}
}

func TestDetectCollapseSignals_None(t *testing.T) {
	signals := DetectCollapseSignals(map[string]float64{
		"burn_rate_multiplier":    1.0,
		"key_talent_departure_rate": 0.05,
	})
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for healthy metrics, got %d", len(signals))
	}
}

func TestGenerateHistoryReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.CollapseSignals = append(s.CollapseSignals, CollapseSignal{ID: "col_1"})
	report := GenerateHistoryReport(s)
	if len(report.CollapseSignals) != 1 {
		t.Errorf("expected 1 collapse signal in report")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
