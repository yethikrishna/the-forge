// Package govern provides composite governance scoring and auditor-ready reports.
// It aggregates signals from compliance, audit, cost, security, and agent behavior
// into a single governance score (0-100) with breakdowns and recommendations.
//
// Governance is not a checkbox. It's a measurable practice.
package govern

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/forge/sword/internal/persistence"
)

// Category represents a governance dimension.
type Category string

const (
	CatSecurity    Category = "security"     // Security posture
	CatCompliance  Category = "compliance"   // Regulatory compliance
	CatAudit       Category = "audit"        // Audit trail completeness
	CatCost        Category = "cost"         // Cost governance
	CatAgentTrust  Category = "agent_trust"  // Agent trust scores
	CatDataPrivacy Category = "data_privacy" // Data privacy & consent
	CatOps         Category = "operations"   // Operational health
	CatAccess      Category = "access"       // Access control
)

// Grade represents a letter grade for a governance score.
type Grade string

const (
	GradeA Grade = "A" // 90-100
	GradeB Grade = "B" // 80-89
	GradeC Grade = "C" // 70-79
	GradeD Grade = "D" // 60-69
	GradeF Grade = "F" // 0-59
)

// Score represents a governance score for a single category.
type Score struct {
	Category     Category        `json:"category"`
	Value        int             `json:"value"` // 0-100
	Grade        Grade           `json:"grade"`
	Weight       float64         `json:"weight"` // Relative weight
	Findings     []Finding       `json:"findings,omitempty"`
	LastAssessed time.Time       `json:"last_assessed"`
}

// Finding represents a specific governance finding.
type Finding struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"` // critical, high, medium, low, info
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Resource    string   `json:"resource,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
	Status      string   `json:"status"` // open, resolved, accepted, deferred
	Evidence    string   `json:"evidence,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// Assessment is a complete governance assessment.
type Assessment struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TenantID    string    `json:"tenant_id,omitempty"`
	Scores      []*Score  `json:"scores"`
	OverallScore int      `json:"overall_score"` // 0-100
	OverallGrade Grade    `json:"overall_grade"`
	Findings    []Finding `json:"findings"`
	Summary     string    `json:"summary"`
	CreatedAt   time.Time `json:"created_at"`
}

// ReportConfig defines how a governance report should be generated.
type ReportConfig struct {
	Name       string    `json:"name"`
	Framework  string    `json:"framework,omitempty"` // SOC2, HIPAA, GDPR, ISO27001
	Categories []Category `json:"categories"`
	Weights    map[Category]float64 `json:"weights,omitempty"`
	TenantID   string    `json:"tenant_id,omitempty"`
}

// Store manages governance assessments.
type Store struct {
	Dir         string
	mu          sync.RWMutex
	assessments map[string]*Assessment
	findings    map[string]*Finding
	pstore      *persistence.Store
}

// NewStore creates or loads a governance store.
func NewStore(dir string) (*Store, error) {
	s := &Store{
		Dir:         dir,
		assessments: make(map[string]*Assessment),
		findings:    make(map[string]*Finding),
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create govern dir: %w", err)
	}
	if err := s.load(); err != nil {
		return s, nil
	}

	ps, err := persistence.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("govern: open persistence store: %w", err)
	}
	s.pstore = ps
	ps.Register("assessments", func() ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return json.MarshalIndent(s.assessments, "", "  ")
	})
	ps.Register("findings", func() ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return json.MarshalIndent(s.findings, "", "  ")
	})
	return s, nil
}

// Close flushes pending writes and stops the background syncer.
func (s *Store) Close() error {
	if s.pstore != nil {
		return s.pstore.Close()
	}
	return nil
}

// Flush forces an immediate write of all dirty keys to disk.
func (s *Store) Flush() error {
	if s.pstore != nil {
		return s.pstore.Flush()
	}
	return nil
}

// ScoreToGrade converts a numeric score to a letter grade.
func ScoreToGrade(score int) Grade {
	switch {
	case score >= 90:
		return GradeA
	case score >= 80:
		return GradeB
	case score >= 70:
		return GradeC
	case score >= 60:
		return GradeD
	default:
		return GradeF
	}
}

// DefaultWeights returns the default category weights.
func DefaultWeights() map[Category]float64 {
	return map[Category]float64{
		CatSecurity:    0.20,
		CatCompliance:  0.15,
		CatAudit:       0.15,
		CatCost:        0.10,
		CatAgentTrust:  0.15,
		CatDataPrivacy: 0.10,
		CatOps:         0.10,
		CatAccess:      0.05,
	}
}

// Assess runs a governance assessment with given scores and produces a report.
func (s *Store) Assess(config ReportConfig, categoryScores map[Category]int, findings []Finding) (*Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	weights := config.Weights
	if len(weights) == 0 {
		weights = DefaultWeights()
	}

	// Build scores.
	var scores []*Score
	weightedSum := 0.0
	totalWeight := 0.0

	categories := config.Categories
	if len(categories) == 0 {
		for cat := range weights {
			categories = append(categories, cat)
		}
	}

	for _, cat := range categories {
		value, ok := categoryScores[cat]
		if !ok {
			value = 50 // Default to 50 if not provided.
		}
		weight := weights[cat]

		score := &Score{
			Category:     cat,
			Value:        value,
			Grade:        ScoreToGrade(value),
			Weight:       weight,
			LastAssessed: time.Now().UTC(),
		}

		// Attach findings for this category.
		for i := range findings {
			if findings[i].Category == cat {
				score.Findings = append(score.Findings, findings[i])
				if findings[i].ID == "" {
					findings[i].ID = fmt.Sprintf("GOV-%s-%d", cat, len(s.findings)+i+1)
				}
				if findings[i].DetectedAt.IsZero() {
					findings[i].DetectedAt = time.Now().UTC()
				}
				if findings[i].Status == "" {
					findings[i].Status = "open"
				}
			}
		}

		scores = append(scores, score)
		weightedSum += float64(value) * weight
		totalWeight += weight
	}

	overallScore := 0
	if totalWeight > 0 {
		overallScore = int(weightedSum / totalWeight)
	}

	assessment := &Assessment{
		ID:           fmt.Sprintf("gov-%d", nextID()),
		Name:         config.Name,
		TenantID:     config.TenantID,
		Scores:       scores,
		OverallScore: overallScore,
		OverallGrade: ScoreToGrade(overallScore),
		Findings:     findings,
		Summary:      generateSummary(overallScore, scores, findings),
		CreatedAt:    time.Now().UTC(),
	}

	s.assessments[assessment.ID] = assessment
	for i := range findings {
		if findings[i].ID != "" {
			s.findings[findings[i].ID] = &findings[i]
		}
	}
	s.markDirty()
	return assessment, nil
}

// Get retrieves an assessment by ID.
func (s *Store) Get(id string) (*Assessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.assessments[id]
	if !ok {
		return nil, fmt.Errorf("assessment %s not found", id)
	}
	return a, nil
}

// List returns all assessments, most recent first.
func (s *Store) List() ([]*Assessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Assessment
	for _, a := range s.assessments {
		results = append(results, a)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
	return results, nil
}

// ResolveFinding marks a finding as resolved.
func (s *Store) ResolveFinding(findingID string) (*Finding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.findings[findingID]
	if !ok {
		return nil, fmt.Errorf("finding %s not found", findingID)
	}
	now := time.Now().UTC()
	f.Status = "resolved"
	f.ResolvedAt = &now
	s.markDirty()
	return f, nil
}

// GetFindings returns findings, optionally filtered by status.
func (s *Store) GetFindings(status string) ([]*Finding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Finding
	for _, f := range s.findings {
		if status == "" || f.Status == status {
			results = append(results, f)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].DetectedAt.After(results[j].DetectedAt)
	})
	return results, nil
}

// ExportMarkdown generates an auditor-ready markdown report.
func (s *Store) ExportMarkdown(assessmentID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.assessments[assessmentID]
	if !ok {
		return "", fmt.Errorf("assessment %s not found", assessmentID)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Governance Assessment: %s\n\n", a.Name))
	b.WriteString(fmt.Sprintf("**Date:** %s\n", a.CreatedAt.Format("2006-01-02 15:04 MST")))
	b.WriteString(fmt.Sprintf("**Overall Score:** %d/100 (%s)\n\n", a.OverallScore, a.OverallGrade))

	// Score breakdown table.
	b.WriteString("## Score Breakdown\n\n")
	b.WriteString("| Category | Score | Grade | Weight | Findings |\n")
	b.WriteString("|----------|-------|-------|--------|----------|\n")
	for _, sc := range a.Scores {
		openCount := 0
		for _, f := range sc.Findings {
			if f.Status == "open" {
				openCount++
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %d | %s | %.0f%% | %d open |\n",
			sc.Category, sc.Value, sc.Grade, sc.Weight*100, openCount))
	}

	// Findings.
	if len(a.Findings) > 0 {
		b.WriteString("\n## Findings\n\n")
		for i, f := range a.Findings {
			b.WriteString(fmt.Sprintf("### %d. [%s] %s\n\n", i+1, strings.ToUpper(f.Severity), f.Title))
			if f.Description != "" {
				b.WriteString(fmt.Sprintf("%s\n\n", f.Description))
			}
			if f.Remediation != "" {
				b.WriteString(fmt.Sprintf("**Remediation:** %s\n\n", f.Remediation))
			}
			b.WriteString(fmt.Sprintf("- **Status:** %s\n", f.Status))
			b.WriteString(fmt.Sprintf("- **Detected:** %s\n", f.DetectedAt.Format("2006-01-02")))
			if f.ResolvedAt != nil {
				b.WriteString(fmt.Sprintf("- **Resolved:** %s\n", f.ResolvedAt.Format("2006-01-02")))
			}
			b.WriteString("\n")
		}
	}

	// Summary.
	b.WriteString("## Summary\n\n")
	b.WriteString(a.Summary + "\n")

	return b.String(), nil
}

// ExportJSON exports an assessment as JSON.
func (s *Store) ExportJSON(assessmentID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.assessments[assessmentID]
	if !ok {
		return nil, fmt.Errorf("assessment %s not found", assessmentID)
	}
	return json.MarshalIndent(a, "", "  ")
}

	var counter int64

func nextID() int64 {
	counter++
	return time.Now().UnixMilli() + counter
}

func generateSummary(overallScore int, scores []*Score, findings []Finding) string {
	var parts []string

	grade := ScoreToGrade(overallScore)
	parts = append(parts, fmt.Sprintf("Overall governance score: %d/100 (Grade %s).", overallScore, grade))

	// Count findings by severity.
	critical, high, medium, low := 0, 0, 0, 0
	openCount := 0
	for _, f := range findings {
		if f.Status != "resolved" {
			openCount++
			switch f.Severity {
			case "critical":
				critical++
			case "high":
				high++
			case "medium":
				medium++
			case "low":
				low++
			}
		}
	}

	if openCount > 0 {
		parts = append(parts, fmt.Sprintf("%d open findings: %d critical, %d high, %d medium, %d low.",
			openCount, critical, high, medium, low))
	} else {
		parts = append(parts, "No open findings.")
	}

	// Identify weakest categories.
	var weak []string
	for _, sc := range scores {
		if sc.Value < 70 {
			weak = append(weak, fmt.Sprintf("%s (%d)", sc.Category, sc.Value))
		}
	}
	if len(weak) > 0 {
		parts = append(parts, fmt.Sprintf("Categories needing attention: %s.", strings.Join(weak, ", ")))
	}

	return strings.Join(parts, " ")
}

// --- persistence ---

func (s *Store) load() error {
	assessPath := filepath.Join(s.Dir, "assessments.json")
	findPath := filepath.Join(s.Dir, "findings.json")

	if data, err := os.ReadFile(assessPath); err == nil {
		if err := json.Unmarshal(data, &s.assessments); err != nil {
			return fmt.Errorf("unmarshal assessments: %w", err)
		}
	}
	if data, err := os.ReadFile(findPath); err == nil {
		if err := json.Unmarshal(data, &s.findings); err != nil {
			return fmt.Errorf("unmarshal findings: %w", err)
		}
	}
	return nil
}

// markDirty tells the persistence store that both assessments and findings need flushing.
// Must be called with s.mu held (write lock).
func (s *Store) markDirty() {
	if s.pstore != nil {
		s.pstore.Dirty("assessments")
		s.pstore.Dirty("findings")
	}
}
