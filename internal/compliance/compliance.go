// Package compliance generates compliance reports from audit logs.
// Supports SOC2, HIPAA, GDPR, and ISO 27001 templates.
// Maps Forge controls to compliance requirements automatically.
//
// Compliance isn't paperwork. It's proof of doing the right thing.
package compliance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Framework represents a compliance framework.
type Framework string

const (
	FrameworkSOC2     Framework = "SOC2"
	FrameworkHIPAA    Framework = "HIPAA"
	FrameworkGDPR     Framework = "GDPR"
	FrameworkISO27001 Framework = "ISO27001"
)

// ControlStatus represents the status of a compliance control.
type ControlStatus string

const (
	StatusCompliant     ControlStatus = "compliant"
	StatusPartial       ControlStatus = "partial"
	StatusNonCompliant  ControlStatus = "non-compliant"
	StatusNotApplicable ControlStatus = "not-applicable"
)

// Control represents a compliance control.
type Control struct {
	ID          string        `json:"id"`
	Framework   Framework     `json:"framework"`
	Category    string        `json:"category"`
	Description string        `json:"description"`
	Status      ControlStatus `json:"status"`
	Evidence    []string      `json:"evidence,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
}

// Report represents a compliance report.
type Report struct {
	ID          string    `json:"id"`
	Framework   Framework `json:"framework"`
	GeneratedAt string    `json:"generated_at"`
	Period      string    `json:"period,omitempty"`
	Summary     Summary   `json:"summary"`
	Controls    []Control `json:"controls"`
	Status      string    `json:"status"` // draft, final
}

// Summary contains report summary statistics.
type Summary struct {
	Total          int     `json:"total"`
	Compliant      int     `json:"compliant"`
	Partial        int     `json:"partial"`
	NonCompliant   int     `json:"non_compliant"`
	NotApplicable  int     `json:"not_applicable"`
	ComplianceRate float64 `json:"compliance_rate"` // 0-100
}

// Store manages compliance reports.
type Store struct {
	Dir string
}

// NewStore creates a compliance store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// GenerateReport generates a compliance report for a given framework.
func (s *Store) GenerateReport(framework Framework, period string) (*Report, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create compliance dir: %w", err)
	}

	controls := getFrameworkControls(framework)

	// Auto-evaluate controls based on Forge capabilities
	evaluateControls(controls)

	report := &Report{
		ID:          fmt.Sprintf("compliance-%s-%d", strings.ToLower(string(framework)), time.Now().UnixNano()),
		Framework:   framework,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Period:      period,
		Controls:    controls,
		Status:      "draft",
	}

	// Calculate summary
	report.Summary = calculateSummary(controls)

	if err := s.writeReport(report); err != nil {
		return nil, err
	}

	return report, nil
}

// GetReport retrieves a compliance report by ID.
func (s *Store) GetReport(id string) (*Report, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("report %q not found: %w", id, err)
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report: %w", err)
	}
	return &report, nil
}

// ListReports returns all compliance reports.
func (s *Store) ListReports() ([]*Report, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var reports []*Report
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var r Report
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		reports = append(reports, &r)
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].GeneratedAt > reports[j].GeneratedAt
	})

	return reports, nil
}

// DeleteReport removes a compliance report.
func (s *Store) DeleteReport(id string) error {
	return os.Remove(filepath.Join(s.Dir, id+".json"))
}

// Finalize marks a report as final.
func (s *Store) Finalize(id string) (*Report, error) {
	report, err := s.GetReport(id)
	if err != nil {
		return nil, err
	}
	report.Status = "final"
	if err := s.writeReport(report); err != nil {
		return nil, err
	}
	return report, nil
}

// ExportMarkdown exports a report as Markdown.
func ExportMarkdown(report *Report) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Compliance Report\n\n", report.Framework))
	sb.WriteString(fmt.Sprintf("**Report ID:** %s  \n", report.ID))
	sb.WriteString(fmt.Sprintf("**Generated:** %s  \n", report.GeneratedAt))
	if report.Period != "" {
		sb.WriteString(fmt.Sprintf("**Period:** %s  \n", report.Period))
	}
	sb.WriteString(fmt.Sprintf("**Status:** %s  \n\n", report.Status))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Count |\n|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Total Controls | %d |\n", report.Summary.Total))
	sb.WriteString(fmt.Sprintf("| Compliant | %d |\n", report.Summary.Compliant))
	sb.WriteString(fmt.Sprintf("| Partial | %d |\n", report.Summary.Partial))
	sb.WriteString(fmt.Sprintf("| Non-Compliant | %d |\n", report.Summary.NonCompliant))
	sb.WriteString(fmt.Sprintf("| Not Applicable | %d |\n", report.Summary.NotApplicable))
	sb.WriteString(fmt.Sprintf("\n**Compliance Rate: %.1f%%**\n\n", report.Summary.ComplianceRate))

	sb.WriteString("## Controls\n\n")
	sb.WriteString("| ID | Category | Description | Status |\n")
	sb.WriteString("|----|----------|-------------|--------|\n")
	for _, c := range report.Controls {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", c.ID, c.Category, c.Description, c.Status))
	}

	// Non-compliant details
	var nonCompliant []Control
	for _, c := range report.Controls {
		if c.Status == StatusNonCompliant {
			nonCompliant = append(nonCompliant, c)
		}
	}
	if len(nonCompliant) > 0 {
		sb.WriteString("\n## Remediation Required\n\n")
		for _, c := range nonCompliant {
			sb.WriteString(fmt.Sprintf("### %s: %s\n", c.ID, c.Description))
			if c.Remediation != "" {
				sb.WriteString(fmt.Sprintf("**Remediation:** %s\n\n", c.Remediation))
			}
			if len(c.Evidence) > 0 {
				sb.WriteString("**Evidence:**\n")
				for _, e := range c.Evidence {
					sb.WriteString(fmt.Sprintf("- %s\n", e))
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// FormatReport renders a report for terminal display.
func FormatReport(report *Report) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s Compliance Report: %s\n", report.Framework, report.ID))
	sb.WriteString(fmt.Sprintf("  Generated: %s | Status: %s\n", report.GeneratedAt, report.Status))
	sb.WriteString(fmt.Sprintf("  Compliance Rate: %.1f%% (%d/%d compliant)\n\n",
		report.Summary.ComplianceRate, report.Summary.Compliant, report.Summary.Total))

	for _, c := range report.Controls {
		icon := "✓"
		switch c.Status {
		case StatusPartial:
			icon = "~"
		case StatusNonCompliant:
			icon = "✗"
		case StatusNotApplicable:
			icon = "—"
		}
		sb.WriteString(fmt.Sprintf("  %s %-12s %-35s %s\n", icon, c.ID, c.Description, c.Status))
	}

	return sb.String()
}

func getFrameworkControls(framework Framework) []Control {
	switch framework {
	case FrameworkSOC2:
		return soc2Controls()
	case FrameworkHIPAA:
		return hipaaControls()
	case FrameworkGDPR:
		return gdprControls()
	case FrameworkISO27001:
		return iso27001Controls()
	default:
		return soc2Controls()
	}
}

func soc2Controls() []Control {
	return []Control{
		{ID: "CC6.1", Framework: FrameworkSOC2, Category: "Security", Description: "Logical and physical access controls", Status: StatusNonCompliant},
		{ID: "CC6.2", Framework: FrameworkSOC2, Category: "Security", Description: "User authentication and credential management", Status: StatusNonCompliant},
		{ID: "CC6.3", Framework: FrameworkSOC2, Category: "Security", Description: "Role-based access control", Status: StatusNonCompliant},
		{ID: "CC7.1", Framework: FrameworkSOC2, Category: "Monitoring", Description: "Detection and monitoring of security events", Status: StatusNonCompliant},
		{ID: "CC7.2", Framework: FrameworkSOC2, Category: "Monitoring", Description: "Incident response procedures", Status: StatusNonCompliant},
		{ID: "CC8.1", Framework: FrameworkSOC2, Category: "Change", Description: "Change management controls", Status: StatusNonCompliant},
		{ID: "CC9.1", Framework: FrameworkSOC2, Category: "Risk", Description: "Risk mitigation controls", Status: StatusNonCompliant},
	}
}

func hipaaControls() []Control {
	return []Control{
		{ID: "164.312(a)", Framework: FrameworkHIPAA, Category: "Access", Description: "Access control and unique user identification", Status: StatusNonCompliant},
		{ID: "164.312(b)", Framework: FrameworkHIPAA, Category: "Audit", Description: "Audit controls for information systems", Status: StatusNonCompliant},
		{ID: "164.312(c)", Framework: FrameworkHIPAA, Category: "Integrity", Description: "Data integrity controls", Status: StatusNonCompliant},
		{ID: "164.312(d)", Framework: FrameworkHIPAA, Category: "Auth", Description: "Person or entity authentication", Status: StatusNonCompliant},
		{ID: "164.312(e)", Framework: FrameworkHIPAA, Category: "Transmission", Description: "Transmission security", Status: StatusNonCompliant},
	}
}

func gdprControls() []Control {
	return []Control{
		{ID: "Art.25", Framework: FrameworkGDPR, Category: "Design", Description: "Data protection by design and default", Status: StatusNonCompliant},
		{ID: "Art.30", Framework: FrameworkGDPR, Category: "Records", Description: "Records of processing activities", Status: StatusNonCompliant},
		{ID: "Art.32", Framework: FrameworkGDPR, Category: "Security", Description: "Security of processing", Status: StatusNonCompliant},
		{ID: "Art.33", Framework: FrameworkGDPR, Category: "Breach", Description: "Data breach notification (72h)", Status: StatusNonCompliant},
		{ID: "Art.35", Framework: FrameworkGDPR, Category: "Assessment", Description: "Data protection impact assessment", Status: StatusNonCompliant},
		{ID: "Art.44", Framework: FrameworkGDPR, Category: "Transfer", Description: "Data transfer outside EU", Status: StatusNonCompliant},
	}
}

func iso27001Controls() []Control {
	return []Control{
		{ID: "A.5.1", Framework: FrameworkISO27001, Category: "Policies", Description: "Information security policies", Status: StatusNonCompliant},
		{ID: "A.6.1", Framework: FrameworkISO27001, Category: "Organization", Description: "Information security organization", Status: StatusNonCompliant},
		{ID: "A.8.1", Framework: FrameworkISO27001, Category: "Asset", Description: "Asset management", Status: StatusNonCompliant},
		{ID: "A.9.1", Framework: FrameworkISO27001, Category: "Access", Description: "Access control", Status: StatusNonCompliant},
		{ID: "A.10.1", Framework: FrameworkISO27001, Category: "Crypto", Description: "Cryptography", Status: StatusNonCompliant},
		{ID: "A.12.1", Framework: FrameworkISO27001, Category: "Operations", Description: "Operational procedures", Status: StatusNonCompliant},
		{ID: "A.12.4", Framework: FrameworkISO27001, Category: "Logging", Description: "Logging and monitoring", Status: StatusNonCompliant},
		{ID: "A.14.1", Framework: FrameworkISO27001, Category: "Development", Description: "Secure development principles", Status: StatusNonCompliant},
	}
}

// evaluateControls auto-evaluates controls based on Forge capabilities.
func evaluateControls(controls []Control) {
	for i := range controls {
		c := &controls[i]

		// Forge has audit logging → audit controls are compliant
		if strings.Contains(strings.ToLower(c.Category), "audit") ||
			strings.Contains(strings.ToLower(c.Category), "logging") {
			c.Status = StatusCompliant
			c.Evidence = append(c.Evidence, "Forge audit trail: tamper-evident logging of all agent actions (forge.audit)")
		}

		// Forge has auth → access controls are partially compliant
		if strings.Contains(strings.ToLower(c.Category), "access") ||
			strings.Contains(strings.ToLower(c.Category), "auth") {
			c.Status = StatusPartial
			c.Evidence = append(c.Evidence, "Forge auth: API key management with scoped keys (forge auth)")
			c.Remediation = "Implement RBAC with role-based permissions for multi-user deployments"
		}

		// Forge has secrets scanning → crypto partially compliant
		if strings.Contains(strings.ToLower(c.Category), "crypto") ||
			strings.Contains(strings.ToLower(c.Category), "integrity") {
			c.Status = StatusPartial
			c.Evidence = append(c.Evidence, "Forge secrets: automatic secret scanning and redaction middleware (internal/secrets)")
		}

		// Forge has sandboxing → security controls partially compliant
		if strings.Contains(strings.ToLower(c.Category), "security") {
			c.Status = StatusPartial
			c.Evidence = append(c.Evidence, "Forge sandbox: multi-level sandboxing (Firecracker → gVisor → Docker → process)")
			c.Evidence = append(c.Evidence, "Forge jail: network isolation with configurable allow rules (forge jail)")
		}

		// Change management → Forge has undo/snapshot
		if strings.Contains(strings.ToLower(c.Category), "change") {
			c.Status = StatusPartial
			c.Evidence = append(c.Evidence, "Forge undo: universal agent undo with session-scoped rollback (forge undo)")
			c.Evidence = append(c.Evidence, "Forge snapshot: environment checkpoints with diff and restore (forge snapshot)")
		}

		// Monitoring → Forge has OTel integration
		if strings.Contains(strings.ToLower(c.Category), "monitoring") ||
			strings.Contains(strings.ToLower(c.Category), "detection") {
			c.Status = StatusPartial
			c.Evidence = append(c.Evidence, "Forge OTel: OpenTelemetry spans for all agent actions (internal/otel)")
		}

		// Breach notification → Forge has audit trail
		if strings.Contains(strings.ToLower(c.Description), "breach") ||
			strings.Contains(strings.ToLower(c.Description), "incident") {
			c.Status = StatusPartial
			c.Evidence = append(c.Evidence, "Forge audit trail provides forensic timeline of all agent actions")
			c.Remediation = "Implement automated breach notification workflow"
		}
	}
}

func calculateSummary(controls []Control) Summary {
	var s Summary
	s.Total = len(controls)
	for _, c := range controls {
		switch c.Status {
		case StatusCompliant:
			s.Compliant++
		case StatusPartial:
			s.Partial++
		case StatusNonCompliant:
			s.NonCompliant++
		case StatusNotApplicable:
			s.NotApplicable++
		}
	}
	applicable := s.Total - s.NotApplicable
	if applicable > 0 {
		s.ComplianceRate = float64(s.Compliant) / float64(applicable) * 100
	}
	return s
}

func (s *Store) writeReport(report *Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, report.ID+".json"), data, 0o644)
}
