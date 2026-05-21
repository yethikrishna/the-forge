package qualitycorpus

import (
	"strings"
	"testing"
	"time"
)

func TestNewCorpus(t *testing.T) {
	c := NewCorpus("", true)
	if c == nil {
		t.Fatal("expected corpus")
	}
}

func TestNotOptedIn(t *testing.T) {
	c := NewCorpus("", false)
	_, err := c.Record(Outcome{AgentID: "a1", TaskType: "code"})
	if err == nil {
		t.Error("should reject when not opted in")
	}
}

func TestIsOptedIn(t *testing.T) {
	c := NewCorpus("", true)
	if !c.IsOptedIn() {
		t.Error("should be opted in")
	}
}

func TestSetOptIn(t *testing.T) {
	c := NewCorpus("", false)
	c.SetOptIn(true)
	if !c.IsOptedIn() {
		t.Error("should be opted in after set")
	}
}

func TestRecord(t *testing.T) {
	c := NewCorpus("", true)
	out, err := c.Record(Outcome{
		AgentID:      "agent-1",
		TaskType:     "code_gen",
		Model:        "gpt-4",
		Success:      true,
		QualityScore: 0.9,
		Duration:     5 * time.Second,
		TokensUsed:   1000,
		Cost:         0.05,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID == "" {
		t.Error("expected ID")
	}
	if !out.Anonymized {
		t.Error("should be anonymized")
	}
	if out.AgentID == "agent-1" {
		t.Error("agent ID should be anonymized")
	}
}

func TestRecordStripsFeedback(t *testing.T) {
	c := NewCorpus("", true)
	out, _ := c.Record(Outcome{
		AgentID:  "a1",
		Feedback: "sensitive user feedback",
	})
	if out.Feedback != "" {
		t.Error("feedback should be stripped")
	}
}

func TestCount(t *testing.T) {
	c := NewCorpus("", true)
	c.Record(Outcome{AgentID: "a1"})
	c.Record(Outcome{AgentID: "a2"})
	if c.Count() != 2 {
		t.Errorf("expected 2, got %d", c.Count())
	}
}

func TestMetricsEmpty(t *testing.T) {
	c := NewCorpus("", true)
	metrics := c.Metrics()
	if len(metrics) != 0 {
		t.Error("empty corpus should have no metrics")
	}
}

func TestMetrics(t *testing.T) {
	c := NewCorpus("", true)
	c.Record(Outcome{AgentID: "a1", Model: "gpt-4", Success: true, QualityScore: 0.9, Duration: 10 * time.Second, TokensUsed: 500, Cost: 0.02})
	c.Record(Outcome{AgentID: "a2", Model: "gpt-4", Success: false, QualityScore: 0.5, Duration: 20 * time.Second, TokensUsed: 1500, Cost: 0.08})

	metrics := c.Metrics()
	if len(metrics) < 4 {
		t.Fatalf("expected 4+ metrics, got %d", len(metrics))
	}

	// Check success rate = 0.5
	for _, m := range metrics {
		if m.Name == "success_rate" && m.Value != 0.5 {
			t.Errorf("success rate should be 0.5, got %.2f", m.Value)
		}
	}
}

func TestMetricsByModel(t *testing.T) {
	c := NewCorpus("", true)
	c.Record(Outcome{Model: "gpt-4", Success: true})
	c.Record(Outcome{Model: "gpt-4", Success: false})
	c.Record(Outcome{Model: "claude", Success: true})

	byModel := c.MetricsByModel()
	if len(byModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(byModel))
	}
	if byModel["gpt-4"].Value != 0.5 {
		t.Error("gpt-4 success rate should be 0.5")
	}
	if byModel["claude"].Value != 1.0 {
		t.Error("claude success rate should be 1.0")
	}
}

func TestMetricsByTaskType(t *testing.T) {
	c := NewCorpus("", true)
	c.Record(Outcome{TaskType: "code_gen", Success: true})
	c.Record(Outcome{TaskType: "code_gen", Success: true})
	c.Record(Outcome{TaskType: "review", Success: false})

	byType := c.MetricsByTaskType()
	if len(byType) != 2 {
		t.Fatalf("expected 2 types, got %d", len(byType))
	}
	if byType["code_gen"].Value != 1.0 {
		t.Error("code_gen should be 1.0")
	}
	if byType["review"].Value != 0.0 {
		t.Error("review should be 0.0")
	}
}

func TestRecent(t *testing.T) {
	c := NewCorpus("", true)
	for i := 0; i < 10; i++ {
		c.Record(Outcome{AgentID: "a1"})
	}

	recent := c.Recent(3)
	if len(recent) != 3 {
		t.Errorf("expected 3, got %d", len(recent))
	}
}

func TestRecentMoreThanAvailable(t *testing.T) {
	c := NewCorpus("", true)
	c.Record(Outcome{AgentID: "a1"})

	recent := c.Recent(10)
	if len(recent) != 1 {
		t.Errorf("expected 1, got %d", len(recent))
	}
}

func TestClear(t *testing.T) {
	c := NewCorpus("", true)
	c.Record(Outcome{AgentID: "a1"})
	c.Clear()
	if c.Count() != 0 {
		t.Error("should be empty after clear")
	}
}

func TestExport(t *testing.T) {
	c := NewCorpus("", true)
	c.Record(Outcome{AgentID: "a1", TaskType: "test"})

	data, err := c.Export()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test") {
		t.Error("should contain task type")
	}
}

func TestFormatMetric(t *testing.T) {
	m := Metric{Name: "success_rate", Value: 0.85, Count: 100, Period: "all"}
	s := FormatMetric(m)
	if !strings.Contains(s, "0.85") {
		t.Error("should show value")
	}
	if !strings.Contains(s, "100") {
		t.Error("should show count")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	c1 := NewCorpus(dir, true)
	c1.Record(Outcome{AgentID: "a1", TaskType: "persist-test"})

	c2 := NewCorpus(dir, true)
	if c2.Count() != 1 {
		t.Fatalf("should persist, got %d", c2.Count())
	}
}

func TestTrimLargeCorpus(t *testing.T) {
	c := NewCorpus("", true)
	for i := 0; i < 10001; i++ {
		c.Record(Outcome{AgentID: "a1"})
	}
	if c.Count() > 10000 {
		t.Error("should trim to 10000")
	}
}

func TestAnonymize(t *testing.T) {
	tests := []struct {
		input string
		short bool
	}{
		{"agent-12345", false},
		{"ab", true},
	}
	for _, tt := range tests {
		result := anonymize(tt.input)
		if tt.short && result != "****" {
			t.Errorf("short input should be masked: %q", result)
		}
		if !tt.short && !strings.HasPrefix(result, tt.input[:2]) {
			t.Error("should preserve first 2 chars")
		}
	}
}
