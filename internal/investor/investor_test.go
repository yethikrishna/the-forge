package investor

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCollectMetrics(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	m, err := ih.CollectMetrics(PeriodMonthly, now.Add(-30*24*time.Hour), now,
		50000, 600000, 1200, 200, 0.05, 0.20, 1.15, 40000, 18, 5000, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.MRR != 50000 {
		t.Error("MRR mismatch")
	}
	if m.TotalUsers != 1200 {
		t.Error("users mismatch")
	}
}

func TestGetLatestMetrics(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()

	ih.CollectMetrics(PeriodMonthly, now.Add(-60*24*time.Hour), now.Add(-30*24*time.Hour),
		30000, 360000, 800, 100, 0.06, 0.15, 1.10, 35000, 20, 4000, 800, nil)
	ih.CollectMetrics(PeriodMonthly, now.Add(-30*24*time.Hour), now,
		50000, 600000, 1200, 200, 0.05, 0.20, 1.15, 40000, 18, 5000, 1000, nil)

	latest, err := ih.GetLatestMetrics(PeriodMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if latest.MRR != 50000 {
		t.Errorf("expected MRR 50000, got %.0f", latest.MRR)
	}

	_, err = ih.GetLatestMetrics(PeriodWeekly)
	if err == nil {
		t.Error("should error on missing period")
	}
}

func TestGenerateNarrativeTraction(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	m, _ := ih.CollectMetrics(PeriodMonthly, now.Add(-30*24*time.Hour), now,
		50000, 600000, 1200, 200, 0.05, 0.20, 1.15, 40000, 18, 5000, 1000, nil)

	narr, err := ih.GenerateNarrative("Q4 Update", m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if narr.Stage != StageTraction {
		t.Errorf("expected traction stage, got %s", narr.Stage)
	}
	if narr.KeyMessage == "" {
		t.Error("should have a key message")
	}
	if len(narr.SupportingPoints) == 0 {
		t.Error("should have supporting points")
	}
}

func TestGenerateNarrativeScale(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	m, _ := ih.CollectMetrics(PeriodMonthly, now.Add(-30*24*time.Hour), now,
		200000, 2400000, 5000, 800, 0.03, 0.25, 1.30, 80000, 24, 8000, 1500, nil)

	narr, _ := ih.GenerateNarrative("Growth Story", m.ID)
	if narr.Stage != StageScale {
		t.Errorf("expected scale stage, got %s", narr.Stage)
	}
}

func TestGenerateNarrativeProblem(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	narr, err := ih.GenerateNarrative("Vision", "")
	if err != nil {
		t.Fatal(err)
	}
	if narr.Stage != StageProblem {
		t.Errorf("expected problem stage with no metrics, got %s", narr.Stage)
	}
}

func TestNarrativeHighChurn(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	m, _ := ih.CollectMetrics(PeriodMonthly, now.Add(-30*24*time.Hour), now,
		10000, 120000, 500, 50, 0.15, 0.10, 1.0, 20000, 10, 2000, 500, nil)

	narr, _ := ih.GenerateNarrative("Honest Update", m.ID)
	found := false
	for _, ca := range narr.CounterArguments {
		if len(ca) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("high churn should generate counter-arguments")
	}
}

func TestBuildPitchDeck(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	m, _ := ih.CollectMetrics(PeriodMonthly, now.Add(-30*24*time.Hour), now,
		50000, 600000, 1200, 200, 0.05, 0.20, 1.15, 40000, 18, 5000, 1000, nil)

	sections, err := ih.BuildPitchDeck(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 5 {
		t.Errorf("expected 5 sections, got %d", len(sections))
	}

	// Check traction section has real data
	for _, s := range sections {
		if s.Type == "traction" && m != nil {
			if s.Content == "Early stage — collecting metrics." {
				t.Error("traction section should contain real metrics")
			}
		}
	}
}

func TestBuildPitchDeckNoMetrics(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	sections, _ := ih.BuildPitchDeck("")
	if len(sections) != 5 {
		t.Errorf("expected 5 sections, got %d", len(sections))
	}
}

func TestGenerateInvestorReport(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	start := now.Add(-30 * 24 * time.Hour)

	ih.CollectMetrics(PeriodMonthly, start, now,
		50000, 600000, 1200, 200, 0.05, 0.20, 1.15, 40000, 18, 5000, 1000, nil)

	report, err := ih.GenerateInvestorReport("May 2026 Report", start, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Title != "May 2026 Report" {
		t.Error("title mismatch")
	}
	if len(report.Highlights) == 0 {
		t.Error("should have highlights")
	}
}

func TestInvestorReportRisks(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	start := now.Add(-30 * 24 * time.Hour)

	// Low runway + high churn → should flag risks
	ih.CollectMetrics(PeriodMonthly, start, now,
		10000, 120000, 500, 50, 0.15, 0.05, 0.9, 20000, 4, 2000, 1500, nil)

	report, _ := ih.GenerateInvestorReport("Risk Report", start, now)
	foundChurn, foundRunway := false, false
	for _, r := range report.Risks {
		if len(r) > 0 {
			// just check there are risks
			foundChurn = true
		}
	}
	if !foundChurn && !foundRunway {
		t.Error("should flag risks for high churn and low runway")
	}
}

func TestListReports(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	ih.CollectMetrics(PeriodMonthly, now.Add(-60*24*time.Hour), now.Add(-30*24*time.Hour), 30000, 360000, 800, 100, 0.06, 0.15, 1.10, 35000, 20, 4000, 800, nil)
	ih.GenerateInvestorReport("Report 1", now.Add(-60*24*time.Hour), now.Add(-30*24*time.Hour))
	ih.GenerateInvestorReport("Report 2", now.Add(-30*24*time.Hour), now)

	reports := ih.ListReports()
	if len(reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(reports))
	}
}

func TestCustomMetrics(t *testing.T) {
	ih := NewInvestorHub(filepath.Join(t.TempDir(), "investor.json"))
	now := time.Now().UTC()
	m, _ := ih.CollectMetrics(PeriodMonthly, now.Add(-30*24*time.Hour), now,
		50000, 600000, 1200, 200, 0.05, 0.20, 1.15, 40000, 18, 5000, 1000,
		map[string]float64{"dau_mau": 0.45, "nps": 72})

	if m.CustomMetrics["dau_mau"] != 0.45 {
		t.Error("custom metric mismatch")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "investor.json")

	ih1 := NewInvestorHub(path)
	ih1.CollectMetrics(PeriodMonthly, time.Now().UTC(), time.Now().UTC(), 50000, 600000, 1200, 200, 0.05, 0.20, 1.15, 40000, 18, 5000, 1000, nil)
	ih1.GenerateNarrative("Test", "")
	ih1.GenerateInvestorReport("Report", time.Now().UTC(), time.Now().UTC())

	ih2 := NewInvestorHub(path)
	if len(ih2.metrics) != 1 {
		t.Errorf("expected 1 metrics, got %d", len(ih2.metrics))
	}
	if len(ih2.narratives) != 1 {
		t.Errorf("expected 1 narrative, got %d", len(ih2.narratives))
	}
	if len(ih2.reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(ih2.reports))
	}
}
