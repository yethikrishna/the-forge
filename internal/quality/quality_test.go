package quality

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Weights should sum to ~1.0
	total := 0.0
	for _, w := range cfg.Weights {
		total += w
	}
	if total < 0.99 || total > 1.01 {
		t.Errorf("weights should sum to 1.0, got %.4f", total)
	}

	if cfg.Threshold <= 0 || cfg.Threshold > 1 {
		t.Errorf("threshold should be 0-1, got %.2f", cfg.Threshold)
	}
}

func TestScorerCreation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoreDir = t.TempDir()

	scorer, err := NewScorer(cfg)
	if err != nil {
		t.Fatalf("NewScorer: %v", err)
	}
	if scorer == nil {
		t.Fatal("expected non-nil scorer")
	}
}

func TestEvaluateBasic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoreDir = t.TempDir()

	scorer, err := NewScorer(cfg)
	if err != nil {
		t.Fatalf("NewScorer: %v", err)
	}

	report, err := scorer.Evaluate(EvaluateInput{
		AgentID:   "test-agent",
		SessionID: "sess-123",
		Model:     "gpt-4",
		Prompt:    "Write a hello world function",
		Output:    "func hello() { fmt.Println(\"Hello, World!\") }",
		Cost:      0.02,
		TokensUsed: 150,
		Duration:   2.5,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if report.ID == "" {
		t.Error("expected non-empty report ID")
	}
	if report.Composite < 0 || report.Composite > 1 {
		t.Errorf("composite should be 0-1, got %.2f", report.Composite)
	}
	if len(report.Scores) == 0 {
		t.Error("expected non-empty scores")
	}
}

func TestEvaluateWithProvidedScores(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoreDir = t.TempDir()

	scorer, _ := NewScorer(cfg)

	report, err := scorer.Evaluate(EvaluateInput{
		AgentID:   "test-agent",
		SessionID: "sess-456",
		Model:     "claude-sonnet-4",
		Prompt:    "Fix the bug",
		Output:    "Fixed the bug by adding nil check",
		Scores: map[Dimension]Score{
			DimensionCorrectness:  {Dimension: DimensionCorrectness, Value: 0.9},
			DimensionCompleteness: {Dimension: DimensionCompleteness, Value: 0.8},
			DimensionStyle:        {Dimension: DimensionStyle, Value: 0.7},
			DimensionSecurity:     {Dimension: DimensionSecurity, Value: 0.85},
			DimensionEfficiency:   {Dimension: DimensionEfficiency, Value: 0.6},
			DimensionReadability:  {Dimension: DimensionReadability, Value: 0.75},
			DimensionTestability:  {Dimension: DimensionTestability, Value: 0.5},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if report.Composite < 0.5 {
		t.Errorf("composite should be high with good scores, got %.2f", report.Composite)
	}
	if !report.Passed {
		t.Error("expected report to pass with high scores")
	}
}

func TestEvaluateSecurityFailure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoreDir = t.TempDir()
	cfg.StrictSecurity = true

	scorer, _ := NewScorer(cfg)

	report, err := scorer.Evaluate(EvaluateInput{
		AgentID:   "test-agent",
		SessionID: "sess-789",
		Model:     "gpt-4",
		Output:    "os.system('rm -rf /')",
		Scores: map[Dimension]Score{
			DimensionSecurity: {Dimension: DimensionSecurity, Value: 0.3},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if report.Passed {
		t.Error("expected report to fail with low security score")
	}
}

func TestEvaluateCompletenessFailure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoreDir = t.TempDir()
	cfg.MinCompleteness = 0.4

	scorer, _ := NewScorer(cfg)

	report, _ := scorer.Evaluate(EvaluateInput{
		AgentID:   "test-agent",
		SessionID: "sess-comp",
		Model:     "gpt-4",
		Output:    "ok",
		Scores: map[Dimension]Score{
			DimensionCompleteness: {Dimension: DimensionCompleteness, Value: 0.2},
		},
	})

	if report.Passed {
		t.Error("expected report to fail with low completeness")
	}
}

func TestSecurityHeuristic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoreDir = t.TempDir()

	scorer, _ := NewScorer(cfg)

	// Output with dangerous patterns
	report, _ := scorer.Evaluate(EvaluateInput{
		AgentID:   "test-agent",
		SessionID: "sess-sec",
		Model:     "gpt-4",
		Output:    "rm -rf / && sudo chmod 777 /etc/passwd",
	})

	// Find security score
	var secScore float64
	for _, sc := range report.Scores {
		if sc.Dimension == DimensionSecurity {
			secScore = sc.Value
		}
	}

	if secScore >= 0.8 {
		t.Errorf("security score should be low for dangerous output, got %.2f", secScore)
	}
}

func TestEfficiencyHeuristic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoreDir = t.TempDir()

	scorer, _ := NewScorer(cfg)

	// Expensive output
	report, _ := scorer.Evaluate(EvaluateInput{
		AgentID:    "test-agent",
		SessionID:  "sess-eff",
		Model:      "gpt-4",
		Output:     "result",
		Cost:       5.0,
		TokensUsed: 50000,
		Duration:   120,
	})

	var effScore float64
	for _, sc := range report.Scores {
		if sc.Dimension == DimensionEfficiency {
			effScore = sc.Value
		}
	}

	if effScore >= 0.8 {
		t.Errorf("efficiency should be low for expensive output, got %.2f", effScore)
	}
}

func TestReportPersistence(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.StoreDir = dir

	scorer, _ := NewScorer(cfg)

	report, _ := scorer.Evaluate(EvaluateInput{
		AgentID:   "persist-agent",
		SessionID: "sess-persist",
		Model:     "gpt-4",
		Output:    "Hello world",
	})

	// Retrieve
	got, err := scorer.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("expected ID %s, got %s", report.ID, got.ID)
	}
	if got.Composite != report.Composite {
		t.Errorf("expected composite %.4f, got %.4f", report.Composite, got.Composite)
	}
}

func TestListReports(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.StoreDir = dir

	scorer, _ := NewScorer(cfg)

	for i := 0; i < 5; i++ {
		scorer.Evaluate(EvaluateInput{
			AgentID:   "list-agent",
			SessionID: "sess-list",
			Model:     "gpt-4",
			Output:    "output",
		})
	}

	reports, err := scorer.ListReports("list-agent", 10)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(reports) < 5 {
		t.Errorf("expected at least 5 reports, got %d", len(reports))
	}
}

func TestTrend(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.StoreDir = dir

	scorer, _ := NewScorer(cfg)

	// Create several reports
	for i := 0; i < 10; i++ {
		scorer.Evaluate(EvaluateInput{
			AgentID:   "trend-agent",
			SessionID: "sess-trend",
			Model:     "gpt-4",
			Output:    "output",
			Scores: map[Dimension]Score{
				DimensionCorrectness: {Dimension: DimensionCorrectness, Value: 0.5 + float64(i)*0.05},
			},
		})
	}

	trend, err := scorer.Trend("trend-agent", 10)
	if err != nil {
		t.Fatalf("Trend: %v", err)
	}
	if trend.SampleSize == 0 {
		t.Error("expected non-zero sample size")
	}
	if trend.Direction == "" {
		t.Error("expected non-empty direction")
	}
}

func TestFormatReport(t *testing.T) {
	report := &Report{
		ID:        "qr-test",
		AgentID:   "test-agent",
		Model:     "gpt-4",
		Composite: 0.85,
		Threshold: 0.6,
		Passed:    true,
		Cost:      0.03,
		Scores: []Score{
			{Dimension: DimensionCorrectness, Value: 0.9, Weight: 0.25},
			{Dimension: DimensionSecurity, Value: 0.8, Weight: 0.2},
		},
	}

	output := FormatReport(report)
	if output == "" {
		t.Error("expected non-empty formatted report")
	}
}

func TestFormatTrend(t *testing.T) {
	tr := &TrendReport{
		AgentID:      "test-agent",
		SampleSize:   10,
		AvgComposite: 0.75,
		PassRate:     0.8,
		AvgCost:      0.05,
		Direction:    "improving",
		DimensionAverages: map[Dimension]float64{
			DimensionCorrectness: 0.8,
			DimensionSecurity:    0.9,
		},
	}

	output := FormatTrend(tr)
	if output == "" {
		t.Error("expected non-empty formatted trend")
	}
}

func TestAllDimensions(t *testing.T) {
	dims := AllDimensions()
	if len(dims) != 7 {
		t.Errorf("expected 7 dimensions, got %d", len(dims))
	}
}

func TestScoreBar(t *testing.T) {
	bar := scoreBar(0.5)
	if len(bar) != 20 { // 10 filled + 10 empty chars
		t.Errorf("expected bar length 20, got %d", len(bar))
	}
}

func TestClamp01(t *testing.T) {
	tests := []struct {
		input, expected float64
	}{
		{-0.5, 0},
		{0.0, 0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}
	for _, tt := range tests {
		got := clamp01(tt.input)
		if got != tt.expected {
			t.Errorf("clamp01(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
