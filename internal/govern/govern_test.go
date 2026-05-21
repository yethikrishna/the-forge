package govern

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempGovernStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score int
		grade Grade
	}{
		{95, GradeA}, {90, GradeA},
		{89, GradeB}, {80, GradeB},
		{79, GradeC}, {70, GradeC},
		{69, GradeD}, {60, GradeD},
		{59, GradeF}, {0, GradeF},
	}
	for _, tt := range tests {
		got := ScoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("ScoreToGrade(%d) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAssess(t *testing.T) {
	s := tempGovernStore(t)

	scores := map[Category]int{
		CatSecurity:    85,
		CatCompliance:  90,
		CatAudit:       78,
		CatCost:        92,
		CatAgentTrust:  88,
		CatDataPrivacy: 95,
		CatOps:         80,
		CatAccess:      70,
	}

	findings := []Finding{
		{Severity: "medium", Title: "Audit log gap", Description: "Missing audit entries for agent deletions", Category: CatAudit, Remediation: "Enable audit for all agent lifecycle events"},
		{Severity: "low", Title: "Stale access policy", Description: "Access policy not updated in 30 days", Category: CatAccess},
	}

	config := ReportConfig{
		Name:      "Q1 Governance Review",
		TenantID:  "tenant-1",
	}

	a, err := s.Assess(config, scores, findings)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected ID")
	}
	if a.OverallScore < 70 || a.OverallScore > 100 {
		t.Fatalf("expected score 70-100, got %d", a.OverallScore)
	}
	if a.OverallGrade != GradeB {
		t.Fatalf("expected grade B, got %s", a.OverallGrade)
	}
	if len(a.Scores) != 8 {
		t.Fatalf("expected 8 category scores, got %d", len(a.Scores))
	}
	if len(a.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(a.Findings))
	}
	if a.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if a.Name != "Q1 Governance Review" {
		t.Fatal("wrong name")
	}
}

func TestAssessWithCustomWeights(t *testing.T) {
	s := tempGovernStore(t)

	scores := map[Category]int{
		CatSecurity: 100,
		CatCompliance: 50,
	}
	weights := map[Category]float64{
		CatSecurity:   0.80,
		CatCompliance: 0.20,
	}

	config := ReportConfig{
		Name:       "Security-Heavy",
		Categories: []Category{CatSecurity, CatCompliance},
		Weights:    weights,
	}

	a, _ := s.Assess(config, scores, nil)

	// Expected: 100*0.8 + 50*0.2 = 90
	if a.OverallScore != 90 {
		t.Fatalf("expected 90, got %d", a.OverallScore)
	}
	if a.OverallGrade != GradeA {
		t.Fatalf("expected A, got %s", a.OverallGrade)
	}
}

func TestGetAssessment(t *testing.T) {
	s := tempGovernStore(t)

	a, _ := s.Assess(ReportConfig{Name: "test"}, map[Category]int{CatSecurity: 80}, nil)

	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != a.ID {
		t.Fatal("ID mismatch")
	}
}

func TestGetNonexistent(t *testing.T) {
	s := tempGovernStore(t)
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListAssessments(t *testing.T) {
	s := tempGovernStore(t)

	s.Assess(ReportConfig{Name: "first"}, map[Category]int{CatSecurity: 80}, nil)
	s.Assess(ReportConfig{Name: "second"}, map[Category]int{CatSecurity: 90}, nil)

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
	// Most recent first.
	if list[0].Name != "second" {
		t.Fatalf("expected most recent first, got %s", list[0].Name)
	}
}

func TestResolveFinding(t *testing.T) {
	s := tempGovernStore(t)

	findings := []Finding{
		{ID: "F-001", Severity: "high", Title: "Test finding", Category: CatSecurity, Status: "open"},
	}

	s.Assess(ReportConfig{Name: "test"}, map[Category]int{CatSecurity: 80}, findings)

	resolved, err := s.ResolveFinding("F-001")
	if err != nil {
		t.Fatalf("ResolveFinding: %v", err)
	}
	if resolved.Status != "resolved" {
		t.Fatalf("expected resolved, got %s", resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Fatal("expected resolved_at")
	}
}

func TestGetFindings(t *testing.T) {
	s := tempGovernStore(t)

	findings := []Finding{
		{ID: "F-001", Severity: "high", Title: "Open finding", Category: CatSecurity, Status: "open"},
		{ID: "F-002", Severity: "low", Title: "Closed finding", Category: CatOps, Status: "resolved"},
	}

	s.Assess(ReportConfig{Name: "test"}, map[Category]int{CatSecurity: 80}, findings)

	all, _ := s.GetFindings("")
	if len(all) != 2 {
		t.Fatalf("expected 2 total findings, got %d", len(all))
	}

	open, _ := s.GetFindings("open")
	if len(open) != 1 {
		t.Fatalf("expected 1 open, got %d", len(open))
	}

	resolved, _ := s.GetFindings("resolved")
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
}

func TestExportMarkdown(t *testing.T) {
	s := tempGovernStore(t)

	findings := []Finding{
		{ID: "F-001", Severity: "critical", Title: "Missing auth", Description: "No auth on admin endpoint", Category: CatSecurity, Remediation: "Add auth middleware", Status: "open"},
	}

	a, _ := s.Assess(ReportConfig{Name: "Security Audit"}, map[Category]int{CatSecurity: 45, CatCompliance: 80}, findings)

	md, err := s.ExportMarkdown(a.ID)
	if err != nil {
		t.Fatalf("ExportMarkdown: %v", err)
	}
	if !strings.Contains(md, "# Governance Assessment") {
		t.Error("missing header")
	}
	if !strings.Contains(md, "Overall Score") {
		t.Error("missing overall score")
	}
	if !strings.Contains(md, "Missing auth") {
		t.Error("missing finding")
	}
	if !strings.Contains(md, "CRITICAL") {
		t.Error("missing severity")
	}
	if !strings.Contains(md, "Add auth middleware") {
		t.Error("missing remediation")
	}
}

func TestExportJSON(t *testing.T) {
	s := tempGovernStore(t)

	a, _ := s.Assess(ReportConfig{Name: "JSON Export Test"}, map[Category]int{CatSecurity: 85}, nil)

	data, err := s.ExportJSON(a.ID)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	var loaded Assessment
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.ID != a.ID {
		t.Fatal("ID mismatch in export")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s1, _ := NewStore(dir)

	findings := []Finding{
		{ID: "F-001", Severity: "high", Title: "Persist test", Category: CatSecurity, Status: "open"},
	}
	a1, _ := s1.Assess(ReportConfig{Name: "persist-test"}, map[Category]int{CatSecurity: 75}, findings)

	// Flush before reload so write-behind cache has written to disk.
	if err := s1.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	s1.Close()

	// Reload.
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	a2, err := s2.Get(a1.ID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if a2.Name != "persist-test" {
		t.Fatalf("expected persist-test, got %s", a2.Name)
	}
	if a2.OverallScore != a1.OverallScore {
		t.Fatal("score mismatch after reload")
	}

	// Verify files.
	if _, err := os.Stat(filepath.Join(dir, "assessments.json")); err != nil {
		t.Fatalf("assessments.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "findings.json")); err != nil {
		t.Fatalf("findings.json missing: %v", err)
	}
}

func TestGenerateSummary(t *testing.T) {
	summary := generateSummary(85, []*Score{
		{Category: CatSecurity, Value: 85, Grade: GradeB},
		{Category: CatAccess, Value: 55, Grade: GradeF},
	}, []Finding{
		{Severity: "high", Title: "test", Status: "open"},
		{Severity: "low", Title: "info", Status: "open"},
	})

	if !strings.Contains(summary, "85/100") {
		t.Error("missing score in summary")
	}
	if !strings.Contains(summary, "2 open findings") {
		t.Error("missing finding count")
	}
	if !strings.Contains(summary, "access (55)") {
		t.Error("missing weak category")
	}
}

func TestDefaultWeights(t *testing.T) {
	w := DefaultWeights()
	if len(w) != 8 {
		t.Fatalf("expected 8 categories, got %d", len(w))
	}
	total := 0.0
	for _, v := range w {
		total += v
	}
	if total < 0.99 || total > 1.01 {
		t.Fatalf("expected weights to sum to 1.0, got %.2f", total)
	}
}

func TestFindingsAutoID(t *testing.T) {
	s := tempGovernStore(t)

	findings := []Finding{
		{Severity: "medium", Title: "No ID finding", Category: CatOps, Status: "open"},
	}

	a, _ := s.Assess(ReportConfig{Name: "test"}, map[Category]int{CatOps: 70}, findings)

	if a.Findings[0].ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if a.Findings[0].DetectedAt.IsZero() {
		t.Fatal("expected auto-set detected_at")
	}
}

func TestTimestamps(t *testing.T) {
	s := tempGovernStore(t)
	before := time.Now().UTC()

	a, _ := s.Assess(ReportConfig{Name: "ts"}, map[Category]int{CatSecurity: 80}, nil)

	after := time.Now().UTC()
	if a.CreatedAt.Before(before) || a.CreatedAt.After(after) {
		t.Fatalf("created_at %v not between %v and %v", a.CreatedAt, before, after)
	}
}
