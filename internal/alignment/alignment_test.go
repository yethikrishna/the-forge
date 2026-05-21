package alignment

import (
	"testing"
	"time"
)

func TestSetBaseline(t *testing.T) {
	am := NewAlignmentMonitor()
	err := am.SetBaseline("agent-1", Baseline{
		Measurements: map[Dimension]float64{
			DimDecisions: 0.9,
			DimQuality:   0.85,
			DimSpeed:     0.7,
			DimCost:      0.8,
			DimStyle:     0.9,
			DimRisk:      0.3,
		},
		Source: "original",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDriftScoreNoSamples(t *testing.T) {
	am := NewAlignmentMonitor()
	am.SetBaseline("agent-1", Baseline{
		Measurements: map[Dimension]float64{
			DimDecisions: 0.9,
			DimQuality:   0.85,
		},
	})

	report, err := am.DriftScore("agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if report.IsDrifted {
		t.Error("no samples should not indicate drift")
	}
}

func TestDriftScoreAligned(t *testing.T) {
	am := NewAlignmentMonitor()
	am.SetBaseline("agent-1", Baseline{
		Measurements: map[Dimension]float64{
			DimDecisions: 0.9,
			DimQuality:   0.85,
			DimSpeed:     0.7,
			DimCost:      0.8,
			DimStyle:     0.9,
			DimRisk:      0.3,
		},
	})

	// Well-aligned sample
	am.RecordSample(BehaviorSample{
		AgentID:   "agent-1",
		Timestamp: time.Now(),
		Decisions: []Decision{
			{ID: "d1", QualityScore: 0.88},
			{ID: "d2", QualityScore: 0.92},
		},
		Outputs: []Output{
			{ID: "o1", QualityScore: 0.84},
		},
		Metrics: SampleMetrics{
			StyleConsistency: 0.88,
			RiskScore:        0.32,
		},
	})

	report, err := am.DriftScore("agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if report.IsDrifted {
		t.Errorf("well-aligned sample should not indicate drift, composite=%.3f", report.Composite)
	}
	if report.Composite > 0.3 {
		t.Errorf("composite drift too high for aligned sample: %.3f", report.Composite)
	}
}

func TestDriftScoreDrifted(t *testing.T) {
	am := NewAlignmentMonitor()
	am.SetBaseline("agent-1", Baseline{
		Measurements: map[Dimension]float64{
			DimDecisions: 0.9,
			DimQuality:   0.85,
			DimSpeed:     0.5,
			DimCost:      0.8,
			DimStyle:     0.9,
			DimRisk:      0.3,
		},
	})

	// Heavily drifted sample — low quality, high risk
	am.RecordSample(BehaviorSample{
		AgentID:   "agent-1",
		Timestamp: time.Now(),
		Decisions: []Decision{
			{ID: "d1", QualityScore: 0.3},
			{ID: "d2", QualityScore: 0.2},
		},
		Outputs: []Output{
			{ID: "o1", QualityScore: 0.3},
		},
		Metrics: SampleMetrics{
			StyleConsistency: 0.3,
			RiskScore:        0.9,
			CostUSD:          0.01,
		},
	})

	report, err := am.DriftScore("agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if !report.IsDrifted {
		t.Error("heavily drifted sample should indicate drift")
	}
	if len(report.Actions) == 0 {
		t.Error("drifted agent should have correction actions")
	}
}

func TestGoodhartScan(t *testing.T) {
	am := NewAlignmentMonitor()
	am.SetBaseline("agent-1", Baseline{
		Measurements: map[Dimension]float64{
			DimDecisions: 0.9,
			DimQuality:   0.85,
		},
	})

	// Create enough drift history to trigger Goodhart detection
	for i := 0; i < 15; i++ {
		am.RecordSample(BehaviorSample{
			AgentID:   "agent-1",
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Decisions: []Decision{{QualityScore: 0.95}}, // very high decisions
			Outputs:   []Output{{QualityScore: 0.2}},     // very low quality
		})
		am.DriftScore("agent-1")
	}

	report, err := am.GoodhartScan("agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if report == nil {
		t.Fatal("expected Goodhart report")
	}
	// May or may not be suspicious depending on exact calculations
	// but should not error
}

func TestCorrectionLevels(t *testing.T) {
	am := NewAlignmentMonitor()
	am.SetBaseline("agent-1", Baseline{
		Measurements: map[Dimension]float64{
			DimDecisions: 0.9,
			DimQuality:   0.9,
			DimSpeed:     0.9,
			DimCost:      0.9,
			DimStyle:     0.9,
			DimRisk:      0.3,
		},
	})

	// Severe drift
	am.RecordSample(BehaviorSample{
		AgentID: "agent-1",
		Decisions: []Decision{{QualityScore: 0.1}, {QualityScore: 0.1}},
		Outputs:   []Output{{QualityScore: 0.1}},
		Metrics:   SampleMetrics{StyleConsistency: 0.1, RiskScore: 0.95, CostUSD: 5.0},
	})

	report, _ := am.DriftScore("agent-1")
	if len(report.Actions) == 0 {
		t.Fatal("expected correction actions")
	}

	highestLevel := CorrectionNudge
	for _, a := range report.Actions {
		if a.Level > highestLevel {
			highestLevel = a.Level
		}
	}
	if highestLevel < CorrectionEscalate {
		t.Errorf("severe drift should trigger at least escalate, got %s", highestLevel)
	}
}

func TestDriftHistory(t *testing.T) {
	am := NewAlignmentMonitor()
	am.SetBaseline("agent-1", Baseline{
		Measurements: map[Dimension]float64{
			DimDecisions: 0.9, DimQuality: 0.9, DimSpeed: 0.9,
			DimCost: 0.9, DimStyle: 0.9, DimRisk: 0.3,
		},
	})

	am.RecordSample(BehaviorSample{
		AgentID: "agent-1",
		Decisions: []Decision{{QualityScore: 0.8}},
		Outputs:   []Output{{QualityScore: 0.8}},
		Metrics:   SampleMetrics{StyleConsistency: 0.8, RiskScore: 0.4},
	})
	am.DriftScore("agent-1")

	history := am.DriftHistory("agent-1")
	if len(history) == 0 {
		t.Error("expected drift history after scoring")
	}
}
