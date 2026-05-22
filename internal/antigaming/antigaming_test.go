package antigaming

import (
	"path/filepath"
	"testing"
)

func tempAGStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "antigaming.json")
	s := NewStore(fp)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestRunAudit(t *testing.T) {
	s := tempAGStore(t)
	audit := MetricAudit{
		ID:     "a1",
		Target: "sprint_velocity",
		Type:   "metric",
		Auditor: "internal_audit",
		Score:  0.95,
	}
	result := s.RunAudit(audit)
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if result.Status != AuditScheduled {
		t.Errorf("expected scheduled, got %s", result.Status)
	}
	if s.Audits["a1"].Target != "sprint_velocity" {
		t.Error("audit not stored")
	}
}

func TestDetectGaming(t *testing.T) {
	s := tempAGStore(t)
	detection := GamingDetection{
		ID:             "d1",
		MetricName:     "code_coverage",
		Severity:       GamingHigh,
		Description:    "Coverage increased but test quality decreased",
		Evidence:       []string{"Tests with no assertions", "Empty test methods"},
		SuspectedCause: "Incentive tied to coverage percentage only",
	}
	result := s.DetectGaming(detection)
	if result.DetectedAt.IsZero() {
		t.Error("DetectedAt should be set")
	}
	if s.Detections["d1"].Severity != GamingHigh {
		t.Error("detection not stored")
	}
}

func TestScanGoodhart(t *testing.T) {
	s := tempAGStore(t)
	// High correlation - no divergence
	scan1 := GoodhartScan{
		ID:              "gs1",
		TargetMetric:    "customer_satisfaction",
		ProxyMetric:     "nps_score",
		CorrelationScore: 0.85,
	}
	result1 := s.ScanGoodhart(scan1)
	if result1.DivergenceDetected {
		t.Error("high correlation should not trigger divergence")
	}
	// Low correlation - divergence detected
	scan2 := GoodhartScan{
		ID:              "gs2",
		TargetMetric:    "engineering_quality",
		ProxyMetric:     "lines_of_code",
		CorrelationScore: 0.2,
	}
	result2 := s.ScanGoodhart(scan2)
	if !result2.DivergenceDetected {
		t.Error("low correlation should trigger divergence")
	}
}

func TestGenerateAntiGamingReport(t *testing.T) {
	s := tempAGStore(t)
	s.RunAudit(MetricAudit{ID: "a1", Target: "t1", Score: 0.9, Status: AuditComplete})
	s.RunAudit(MetricAudit{ID: "a2", Target: "t2", Score: 0.7, Status: AuditComplete})
	s.DetectGaming(GamingDetection{ID: "d1", Severity: GamingCritical, Resolved: false})
	s.DetectGaming(GamingDetection{ID: "d2", Severity: GamingLow, Resolved: true})
	s.ScanGoodhart(GoodhartScan{ID: "gs1", CorrelationScore: 0.3, DivergenceDetected: true})
	s.ScanGoodhart(GoodhartScan{ID: "gs2", CorrelationScore: 0.9})
	// Add a red team finding
	s.RedTeamFindings["rt1"] = RedTeamFinding{ID: "rt1", IsFixed: false}
	report := s.GenerateAntiGamingReport()
	if report["total_audits"] != 2 {
		t.Errorf("expected 2 audits, got %v", report["total_audits"])
	}
	avgScore := report["avg_audit_score"].(float64)
	if avgScore != 0.8 {
		t.Errorf("expected 0.8 avg, got %v", avgScore)
	}
	if report["unresolved_detections"] != 1 {
		t.Errorf("expected 1 unresolved, got %v", report["unresolved_detections"])
	}
	if report["critical_detections"] != 1 {
		t.Errorf("expected 1 critical, got %v", report["critical_detections"])
	}
	if report["goodhart_violations"] != 1 {
		t.Errorf("expected 1 Goodhart violation, got %v", report["goodhart_violations"])
	}
	if report["unfixed_red_team"] != 1 {
		t.Errorf("expected 1 unfixed, got %v", report["unfixed_red_team"])
	}
}

func TestAntiGamingLoadRoundTrip(t *testing.T) {
	s := tempAGStore(t)
	s.RunAudit(MetricAudit{ID: "a1", Target: "test", Score: 0.5})
	s.DetectGaming(GamingDetection{ID: "d1", MetricName: "test", Severity: GamingMedium})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s2.Audits["a1"].Target != "test" {
		t.Error("audit not persisted")
	}
	if s2.Detections["d1"].Severity != GamingMedium {
		t.Error("detection not persisted")
	}
}
