package compliance

import (
	"strings"
	"testing"
)

func TestGenerateSOC2Report(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, err := store.GenerateReport(FrameworkSOC2, "2026-Q1")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report.Framework != FrameworkSOC2 {
		t.Errorf("expected SOC2, got %s", report.Framework)
	}
	if len(report.Controls) != 7 {
		t.Errorf("expected 7 SOC2 controls, got %d", len(report.Controls))
	}
	if report.Status != "draft" {
		t.Errorf("expected draft, got %s", report.Status)
	}
}

func TestGenerateHIPAAReport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, err := store.GenerateReport(FrameworkHIPAA, "")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if len(report.Controls) != 5 {
		t.Errorf("expected 5 HIPAA controls, got %d", len(report.Controls))
	}
}

func TestGenerateGDPRReport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, err := store.GenerateReport(FrameworkGDPR, "")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if len(report.Controls) != 6 {
		t.Errorf("expected 6 GDPR controls, got %d", len(report.Controls))
	}
}

func TestGenerateISO27001Report(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, err := store.GenerateReport(FrameworkISO27001, "")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if len(report.Controls) != 8 {
		t.Errorf("expected 8 ISO 27001 controls, got %d", len(report.Controls))
	}
}

func TestAutoEvaluation(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, _ := store.GenerateReport(FrameworkSOC2, "")

	// Security controls should be at least partial (Forge has sandboxing)
	for _, c := range report.Controls {
		if c.Category == "Security" && c.Status == StatusNonCompliant {
			t.Errorf("Security control %s should not be non-compliant (Forge has sandboxing)", c.ID)
		}
	}

	// Audit/monitoring controls should be compliant (Forge has audit trail)
	for _, c := range report.Controls {
		if c.Category == "Monitoring" && c.Status == StatusNonCompliant {
			t.Errorf("Monitoring control %s should not be non-compliant (Forge has OTel)", c.ID)
		}
	}
}

func TestComplianceRate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, _ := store.GenerateReport(FrameworkSOC2, "")

	if report.Summary.ComplianceRate < 0 || report.Summary.ComplianceRate > 100 {
		t.Errorf("compliance rate should be 0-100, got %.1f", report.Summary.ComplianceRate)
	}

	total := report.Summary.Compliant + report.Summary.Partial + report.Summary.NonCompliant + report.Summary.NotApplicable
	if total != report.Summary.Total {
		t.Errorf("summary counts don't add up: %d != %d", total, report.Summary.Total)
	}
}

func TestGetReport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	created, _ := store.GenerateReport(FrameworkSOC2, "")

	found, err := store.GetReport(created.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if found.Framework != FrameworkSOC2 {
		t.Errorf("expected SOC2, got %s", found.Framework)
	}
}

func TestGetReportNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.GetReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestListReports(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.GenerateReport(FrameworkSOC2, "")
	store.GenerateReport(FrameworkHIPAA, "")
	store.GenerateReport(FrameworkGDPR, "")

	reports, err := store.ListReports()
	if err != nil {
		t.Fatalf("ListReports failed: %v", err)
	}
	if len(reports) != 3 {
		t.Errorf("expected 3 reports, got %d", len(reports))
	}
}

func TestDeleteReport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, _ := store.GenerateReport(FrameworkSOC2, "")
	if err := store.DeleteReport(report.ID); err != nil {
		t.Fatalf("DeleteReport failed: %v", err)
	}

	if _, err := store.GetReport(report.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestFinalize(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, _ := store.GenerateReport(FrameworkSOC2, "")
	if report.Status != "draft" {
		t.Errorf("expected draft, got %s", report.Status)
	}

	final, err := store.Finalize(report.ID)
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}
	if final.Status != "final" {
		t.Errorf("expected final, got %s", final.Status)
	}
}

func TestExportMarkdown(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, _ := store.GenerateReport(FrameworkSOC2, "2026-Q1")
	md := ExportMarkdown(report)

	if !strings.Contains(md, "SOC2") {
		t.Error("expected SOC2 in markdown")
	}
	if !strings.Contains(md, "Compliance Rate") {
		t.Error("expected compliance rate in markdown")
	}
	if !strings.Contains(md, "| ID |") {
		t.Error("expected controls table in markdown")
	}
}

func TestFormatReport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, _ := store.GenerateReport(FrameworkSOC2, "")
	output := FormatReport(report)

	if !strings.Contains(output, "SOC2") {
		t.Error("expected SOC2 in output")
	}
	if !strings.Contains(output, "Compliance Rate") {
		t.Error("expected compliance rate in output")
	}
}

func TestControlHasEvidence(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, _ := store.GenerateReport(FrameworkSOC2, "")

	hasEvidence := false
	for _, c := range report.Controls {
		if len(c.Evidence) > 0 {
			hasEvidence = true
			break
		}
	}
	if !hasEvidence {
		t.Error("expected at least some controls to have evidence after auto-evaluation")
	}
}
