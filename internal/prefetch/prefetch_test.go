package prefetch

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewPredictor(t *testing.T) {
	p := NewPredictor("")
	if p == nil {
		t.Fatal("expected predictor")
	}
}

func TestRecordAndHistory(t *testing.T) {
	p := NewPredictor("")
	p.Record(ContextFile, "main.go", "forge build")

	history := p.GetHistory(0)
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Target != "main.go" {
		t.Errorf("expected main.go, got %s", history[0].Target)
	}
}

func TestLearnPattern(t *testing.T) {
	p := NewPredictor("")
	p.Record(ContextFile, "main.go", "forge build")
	p.Record(ContextFile, "main.go", "forge build")
	p.Record(ContextFile, "main.go", "forge build")

	patterns := p.GetPatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].Count != 3 {
		t.Errorf("expected count 3, got %d", patterns[0].Count)
	}
}

func TestPredictFromPattern(t *testing.T) {
	p := NewPredictor("")

	// Learn: forge build → main.go
	for i := 0; i < 5; i++ {
		p.Record(ContextFile, "main.go", "forge build")
	}

	entries := p.Predict("forge build", 5)
	if len(entries) == 0 {
		t.Fatal("expected predictions")
	}
	found := false
	for _, e := range entries {
		if e.Target == "main.go" {
			found = true
			if e.Priority <= 0 {
				t.Error("priority should be positive")
			}
		}
	}
	if !found {
		t.Error("should predict main.go for forge build")
	}
}

func TestPredictFuzzyMatch(t *testing.T) {
	p := NewPredictor("")
	p.Record(ContextFile, "go.mod", "forge build")

	entries := p.Predict("forge build --verbose", 5)
	found := false
	for _, e := range entries {
		if e.Target == "go.mod" {
			found = true
		}
	}
	if !found {
		t.Error("fuzzy match should work for prefix")
	}
}

func TestPredictMaxEntries(t *testing.T) {
	p := NewPredictor("")

	for i := 0; i < 20; i++ {
		p.Record(ContextFile, fmt.Sprintf("file%d.go", i), "forge test")
	}

	entries := p.Predict("forge test", 5)
	if len(entries) > 5 {
		t.Errorf("expected max 5, got %d", len(entries))
	}
}

func TestPredictEmpty(t *testing.T) {
	p := NewPredictor("")
	entries := p.Predict("unknown", 5)
	if len(entries) != 0 {
		t.Error("unknown command with no history should have no predictions")
	}
}

func TestRecentHistoryBoostsPriority(t *testing.T) {
	p := NewPredictor("")

	p.Record(ContextFile, "recent.go", "forge edit")
	entries := p.Predict("forge edit", 5)

	found := false
	for _, e := range entries {
		if e.Target == "recent.go" {
			found = true
		}
	}
	if !found {
		t.Error("recently accessed files should be predicted")
	}
}

func TestPriorityNormalization(t *testing.T) {
	p := NewPredictor("")

	for i := 0; i < 50; i++ {
		p.Record(ContextFile, "frequent.go", "forge test")
	}

	entries := p.Predict("forge test", 1)
	if len(entries) > 0 && entries[0].Priority > 1.0 {
		t.Errorf("priority should be normalized to 0-1, got %.2f", entries[0].Priority)
	}
}

func TestClearHistory(t *testing.T) {
	p := NewPredictor("")
	p.Record(ContextFile, "main.go", "forge build")
	p.ClearHistory()

	if len(p.GetHistory(0)) != 0 {
		t.Error("history should be empty")
	}
	if len(p.GetPatterns()) != 0 {
		t.Error("patterns should be empty")
	}
}

func TestHistoryLimit(t *testing.T) {
	p := NewPredictor("")

	for i := 0; i < 600; i++ {
		p.Record(ContextFile, fmt.Sprintf("f%d.go", i), "cmd")
	}

	history := p.GetHistory(0)
	if len(history) > p.maxHistory {
		t.Errorf("history should be trimmed to %d, got %d", p.maxHistory, len(history))
	}
}

func TestHistoryWithLimit(t *testing.T) {
	p := NewPredictor("")

	for i := 0; i < 20; i++ {
		p.Record(ContextFile, fmt.Sprintf("f%d.go", i), "cmd")
	}

	history := p.GetHistory(5)
	if len(history) != 5 {
		t.Errorf("expected 5, got %d", len(history))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	p1 := NewPredictor(dir)
	p1.Record(ContextFile, "main.go", "forge build")

	p2 := NewPredictor(dir)
	history := p2.GetHistory(0)
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry after reload, got %d", len(history))
	}
	if history[0].Target != "main.go" {
		t.Error("persisted target mismatch")
	}
}

func TestPatternPersistence(t *testing.T) {
	dir := t.TempDir()

	p1 := NewPredictor(dir)
	for i := 0; i < 3; i++ {
		p1.Record(ContextFile, "main.go", "forge build")
	}

	p2 := NewPredictor(dir)
	patterns := p2.GetPatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern after reload, got %d", len(patterns))
	}
	if patterns[0].Count != 3 {
		t.Errorf("expected count 3, got %d", patterns[0].Count)
	}
}

func TestStats(t *testing.T) {
	p := NewPredictor("")
	p.Record(ContextFile, "main.go", "forge build")

	stats := p.Stats()
	if stats["patterns"].(int) != 1 {
		t.Errorf("expected 1 pattern")
	}
	if stats["history"].(int) != 1 {
		t.Errorf("expected 1 history")
	}
}

func TestFormatEntry(t *testing.T) {
	e := &PrefetchEntry{
		Type:     ContextFile,
		Target:   "main.go",
		Priority: 0.85,
		Reason:   "Pattern match",
	}
	s := FormatEntry(e)
	if !strings.Contains(s, "main.go") {
		t.Error("should show target")
	}
	if !strings.Contains(s, "0.85") {
		t.Error("should show priority")
	}
}

func TestFormatPattern(t *testing.T) {
	p := Pattern{
		Trigger:  "forge build",
		Prefetch: "go.mod",
		Count:    10,
		Recency:  0.8,
	}
	s := FormatPattern(&p)
	if !strings.Contains(s, "forge build") {
		t.Error("should show trigger")
	}
	if !strings.Contains(s, "10") {
		t.Error("should show count")
	}
}

func TestMultipleCommands(t *testing.T) {
	p := NewPredictor("")
	p.Record(ContextFile, "main.go", "forge build")
	p.Record(ContextFile, "main_test.go", "forge test")
	p.Record(ContextFile, "README.md", "forge doc")

	entries := p.Predict("forge build", 5)
	found := false
	for _, e := range entries {
		if e.Target == "main.go" {
			found = true
		}
	}
	if !found {
		t.Error("forge build should predict main.go")
	}
}

func TestPatternTrimming(t *testing.T) {
	p := NewPredictor("")

	for i := 0; i < 250; i++ {
		p.Record(ContextFile, fmt.Sprintf("f%d.go", i), fmt.Sprintf("cmd%d", i))
	}

	if len(p.GetPatterns()) > 200 {
		t.Error("patterns should be trimmed to 200")
	}
}
