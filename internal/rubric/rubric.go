// Package rubric provides rubric-based output grading for Forge agents.
// Every blade is measured against the standard — only the sharpest pass.
package rubric

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Rubric defines a set of criteria for grading agent output.
type Rubric struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Criteria    []Criterion  `json:"criteria"`
	PassThreshold float64    `json:"pass_threshold"`
	MaxScore    float64      `json:"max_score"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Criterion is a single grading criterion.
type Criterion struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Weight      float64  `json:"weight"`
	MaxScore    float64  `json:"max_score"`
	Levels      []Level  `json:"levels"`
	Required    bool     `json:"required"`
	Category    string   `json:"category"`
}

// Level defines a scoring level for a criterion.
type Level struct {
	Score       float64 `json:"score"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
}

// Grade is the result of grading output against a rubric.
type Grade struct {
	ID           string         `json:"id"`
	RubricID     string         `json:"rubric_id"`
	AgentID      string         `json:"agent_id"`
	SessionID    string         `json:"session_id"`
	Output       string         `json:"output"`
	Scores       []CriterionScore `json:"scores"`
	TotalScore   float64        `json:"total_score"`
	MaxScore     float64        `json:"max_score"`
	Percentage   float64        `json:"percentage"`
	Passed       bool           `json:"passed"`
	Feedback     string         `json:"feedback,omitempty"`
	GradedAt     time.Time      `json:"graded_at"`
	TriggerRerun bool           `json:"trigger_rerun"`
}

// CriterionScore is the score for a single criterion.
type CriterionScore struct {
	CriterionID string  `json:"criterion_id"`
	CriterionName string `json:"criterion_name"`
	Score        float64 `json:"score"`
	MaxScore     float64 `json:"max_score"`
	Percentage   float64 `json:"percentage"`
	Level        string  `json:"level"`
	Notes        string  `json:"notes,omitempty"`
	Passed       bool    `json:"passed"`
}

// Grader grades agent output against rubrics.
type Grader struct {
	rubrics map[string]*Rubric
	grades  map[string]*Grade
	mu      sync.RWMutex
	storeDir string
}

// NewGrader creates a rubric grader.
func NewGrader(storeDir string) *Grader {
	g := &Grader{
		rubrics:  make(map[string]*Rubric),
		grades:   make(map[string]*Grade),
		storeDir: storeDir,
	}

	// Load built-in rubrics
	for _, r := range BuiltinRubrics() {
		g.rubrics[r.ID] = &r
	}

	os.MkdirAll(storeDir, 0o755)
	g.load()
	return g
}

// BuiltinRubrics returns pre-defined rubrics.
func BuiltinRubrics() []Rubric {
	now := time.Now().UTC()
	return []Rubric{
		{
			ID: "code-quality", Name: "Code Quality", Description: "Grade agent code output for quality, correctness, and style",
			PassThreshold: 70, MaxScore: 100, CreatedAt: now, UpdatedAt: now,
			Criteria: []Criterion{
				{ID: "correctness", Name: "Correctness", Description: "Code is logically correct and handles edge cases", Weight: 30, MaxScore: 30, Required: true, Category: "quality",
					Levels: []Level{
						{Score: 30, Label: "Excellent", Description: "All cases handled, no logical errors"},
						{Score: 20, Label: "Good", Description: "Mostly correct, minor edge case gaps"},
						{Score: 10, Label: "Needs Work", Description: "Logical errors present"},
						{Score: 0, Label: "Failing", Description: "Fundamentally incorrect"},
					}},
				{ID: "style", Name: "Style", Description: "Follows language conventions and project style", Weight: 15, MaxScore: 15, Category: "style",
					Levels: []Level{
						{Score: 15, Label: "Excellent", Description: "Perfectly follows conventions"},
						{Score: 10, Label: "Good", Description: "Minor style issues"},
						{Score: 5, Label: "Needs Work", Description: "Multiple style violations"},
						{Score: 0, Label: "Failing", Description: "Ignores style entirely"},
					}},
				{ID: "error-handling", Name: "Error Handling", Description: "Proper error handling and recovery", Weight: 20, MaxScore: 20, Required: true, Category: "reliability",
					Levels: []Level{
						{Score: 20, Label: "Excellent", Description: "All errors handled gracefully"},
						{Score: 14, Label: "Good", Description: "Most errors handled"},
						{Score: 7, Label: "Needs Work", Description: "Missing error handling"},
						{Score: 0, Label: "Failing", Description: "No error handling"},
					}},
				{ID: "tests", Name: "Tests", Description: "Test coverage and quality", Weight: 20, MaxScore: 20, Category: "quality",
					Levels: []Level{
						{Score: 20, Label: "Excellent", Description: "Comprehensive tests including edge cases"},
						{Score: 14, Label: "Good", Description: "Good coverage, some gaps"},
						{Score: 7, Label: "Needs Work", Description: "Minimal tests"},
						{Score: 0, Label: "Failing", Description: "No tests"},
					}},
				{ID: "documentation", Name: "Documentation", Description: "Code has appropriate comments and docs", Weight: 15, MaxScore: 15, Category: "maintainability",
					Levels: []Level{
						{Score: 15, Label: "Excellent", Description: "Well documented with examples"},
						{Score: 10, Label: "Good", Description: "Key functions documented"},
						{Score: 5, Label: "Needs Work", Description: "Sparse documentation"},
						{Score: 0, Label: "Failing", Description: "No documentation"},
					}},
			},
		},
		{
			ID: "review-quality", Name: "Review Quality", Description: "Grade code review output for thoroughness and actionability",
			PassThreshold: 60, MaxScore: 100, CreatedAt: now, UpdatedAt: now,
			Criteria: []Criterion{
				{ID: "thoroughness", Name: "Thoroughness", Description: "Reviews all relevant aspects", Weight: 30, MaxScore: 30, Required: true, Category: "completeness",
					Levels: []Level{
						{Score: 30, Label: "Comprehensive", Description: "All aspects reviewed"},
						{Score: 20, Label: "Adequate", Description: "Most aspects covered"},
						{Score: 10, Label: "Surface", Description: "Only surface-level review"},
						{Score: 0, Label: "Inadequate", Description: "Missed critical issues"},
					}},
				{ID: "actionability", Name: "Actionability", Description: "Suggestions are specific and actionable", Weight: 25, MaxScore: 25, Category: "usefulness",
					Levels: []Level{
						{Score: 25, Label: "Specific", Description: "Clear, actionable suggestions"},
						{Score: 17, Label: "Mostly Clear", Description: "Generally actionable"},
						{Score: 8, Label: "Vague", Description: "Suggestions lack specificity"},
						{Score: 0, Label: "Unhelpful", Description: "No actionable feedback"},
					}},
				{ID: "severity", Name: "Severity Classification", Description: "Issues properly classified by severity", Weight: 20, MaxScore: 20, Category: "accuracy",
					Levels: []Level{
						{Score: 20, Label: "Accurate", Description: "All severities correct"},
						{Score: 14, Label: "Mostly Accurate", Description: "Minor misclassifications"},
						{Score: 7, Label: "Inconsistent", Description: "Several misclassifications"},
						{Score: 0, Label: "Wrong", Description: "Severities completely off"},
					}},
				{ID: "constructiveness", Name: "Constructiveness", Description: "Tone is helpful, not destructive", Weight: 25, MaxScore: 25, Category: "communication",
					Levels: []Level{
						{Score: 25, Label: "Excellent", Description: "Helpful, encouraging tone"},
						{Score: 17, Label: "Good", Description: "Generally constructive"},
						{Score: 8, Label: "Neutral", Description: "Neither helpful nor harmful"},
						{Score: 0, Label: "Destructive", Description: "Harsh or dismissive"},
					}},
			},
		},
		{
			ID: "general", Name: "General Output", Description: "Grade general agent output quality",
			PassThreshold: 65, MaxScore: 100, CreatedAt: now, UpdatedAt: now,
			Criteria: []Criterion{
				{ID: "relevance", Name: "Relevance", Description: "Output addresses the prompt", Weight: 25, MaxScore: 25, Required: true, Category: "accuracy",
					Levels: []Level{
						{Score: 25, Label: "Directly Relevant", Description: "Perfectly addresses the prompt"},
						{Score: 17, Label: "Mostly Relevant", Description: "Minor tangents"},
						{Score: 8, Label: "Partially Relevant", Description: "Significant digressions"},
						{Score: 0, Label: "Off Topic", Description: "Does not address the prompt"},
					}},
				{ID: "completeness", Name: "Completeness", Description: "All aspects of the task are covered", Weight: 25, MaxScore: 25, Category: "completeness",
					Levels: []Level{
						{Score: 25, Label: "Complete", Description: "All aspects covered"},
						{Score: 17, Label: "Mostly Complete", Description: "Minor gaps"},
						{Score: 8, Label: "Partial", Description: "Significant gaps"},
						{Score: 0, Label: "Incomplete", Description: "Major parts missing"},
					}},
				{ID: "clarity", Name: "Clarity", Description: "Output is clear and well-structured", Weight: 25, MaxScore: 25, Category: "communication",
					Levels: []Level{
						{Score: 25, Label: "Crystal Clear", Description: "Exceptionally clear"},
						{Score: 17, Label: "Clear", Description: "Easy to understand"},
						{Score: 8, Label: "Confusing", Description: "Hard to follow"},
						{Score: 0, Label: "Incoherent", Description: "Unreadable"},
					}},
				{ID: "accuracy", Name: "Accuracy", Description: "Information is correct", Weight: 25, MaxScore: 25, Required: true, Category: "accuracy",
					Levels: []Level{
						{Score: 25, Label: "Fully Accurate", Description: "No factual errors"},
						{Score: 17, Label: "Mostly Accurate", Description: "Minor inaccuracies"},
						{Score: 8, Label: "Partially Accurate", Description: "Some errors"},
						{Score: 0, Label: "Inaccurate", Description: "Major errors"},
					}},
			},
		},
	}
}

// CreateRubric creates a custom rubric.
func (g *Grader) CreateRubric(rubric Rubric) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if rubric.ID == "" {
		rubric.ID = fmt.Sprintf("rubric-%d", time.Now().UnixNano())
	}
	rubric.CreatedAt = time.Now().UTC()
	rubric.UpdatedAt = rubric.CreatedAt

	g.rubrics[rubric.ID] = &rubric
	g.save()
	return nil
}

// GetRubric returns a rubric by ID.
func (g *Grader) GetRubric(id string) (*Rubric, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	r, ok := g.rubrics[id]
	if !ok {
		return nil, fmt.Errorf("rubric %s not found", id)
	}
	return r, nil
}

// ListRubrics returns all rubrics.
func (g *Grader) ListRubrics() []*Rubric {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var rubrics []*Rubric
	for _, r := range g.rubrics {
		rubrics = append(rubrics, r)
	}
	sort.Slice(rubrics, func(i, j int) bool {
		return rubrics[i].Name < rubrics[j].Name
	})
	return rubrics
}

// Grade grades output against a rubric.
func (g *Grader) Grade(rubricID, agentID, sessionID, output string, scores map[string]float64) (*Grade, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	rubric, ok := g.rubrics[rubricID]
	if !ok {
		return nil, fmt.Errorf("rubric %s not found", rubricID)
	}

	grade := &Grade{
		ID:        fmt.Sprintf("grade-%d", time.Now().UnixNano()),
		RubricID:  rubricID,
		AgentID:   agentID,
		SessionID: sessionID,
		Output:    output,
		GradedAt:  time.Now().UTC(),
	}

	var totalScore float64
	var totalMax float64
	allRequiredPassed := true

	for _, criterion := range rubric.Criteria {
		score, provided := scores[criterion.ID]
		if !provided {
			score = 0
		}
		if score > criterion.MaxScore {
			score = criterion.MaxScore
		}

		maxScore := criterion.MaxScore
		percentage := 0.0
		if maxScore > 0 {
			percentage = (score / maxScore) * 100
		}

		level := determineLevel(criterion, score)
		passed := percentage >= 60 // 60% is passing per criterion

		if criterion.Required && !passed {
			allRequiredPassed = false
		}

		cs := CriterionScore{
			CriterionID:   criterion.ID,
			CriterionName: criterion.Name,
			Score:         score,
			MaxScore:      maxScore,
			Percentage:    percentage,
			Level:         level,
			Passed:        passed,
		}

		grade.Scores = append(grade.Scores, cs)
		totalScore += score
		totalMax += maxScore
	}

	grade.TotalScore = totalScore
	grade.MaxScore = totalMax
	if totalMax > 0 {
		grade.Percentage = (totalScore / totalMax) * 100
	}
	grade.Passed = grade.Percentage >= rubric.PassThreshold && allRequiredPassed
	grade.TriggerRerun = !grade.Passed

	// Generate feedback
	grade.Feedback = generateFeedback(grade, rubric)

	g.grades[grade.ID] = grade
	g.save()

	return grade, nil
}

// GetGrade returns a grade by ID.
func (g *Grader) GetGrade(id string) (*Grade, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	grade, ok := g.grades[id]
	if !ok {
		return nil, fmt.Errorf("grade %s not found", id)
	}
	return grade, nil
}

// ListGrades returns grades, optionally filtered.
func (g *Grader) ListGrades(agentID, rubricID string) []*Grade {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var grades []*Grade
	for _, grade := range g.grades {
		if agentID != "" && grade.AgentID != agentID {
			continue
		}
		if rubricID != "" && grade.RubricID != rubricID {
			continue
		}
		grades = append(grades, grade)
	}

	sort.Slice(grades, func(i, j int) bool {
		return grades[i].GradedAt.After(grades[j].GradedAt)
	})

	return grades
}

// Stats returns grader statistics.
func (g *Grader) Stats() GraderStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := GraderStats{
		TotalGrades: len(g.grades),
		TotalRubrics: len(g.rubrics),
	}

	var totalPct float64
	for _, grade := range g.grades {
		totalPct += grade.Percentage
		if grade.Passed {
			stats.Passed++
		} else {
			stats.Failed++
		}
	}

	if len(g.grades) > 0 {
		stats.AvgPercentage = totalPct / float64(len(g.grades))
	}

	return stats
}

// GraderStats holds grader statistics.
type GraderStats struct {
	TotalGrades  int     `json:"total_grades"`
	TotalRubrics int     `json:"total_rubrics"`
	Passed       int     `json:"passed"`
	Failed       int     `json:"failed"`
	AvgPercentage float64 `json:"avg_percentage"`
}

func determineLevel(criterion Criterion, score float64) string {
	for i, level := range criterion.Levels {
		if score >= level.Score {
			return level.Label
		}
		// Levels should be sorted high to low
		if i < len(criterion.Levels)-1 && score >= criterion.Levels[i+1].Score {
			return level.Label
		}
	}
	if len(criterion.Levels) > 0 {
		return criterion.Levels[len(criterion.Levels)-1].Label
	}
	return "Unknown"
}

func generateFeedback(grade *Grade, rubric *Rubric) string {
	var sb strings.Builder

	if grade.Passed {
		sb.WriteString(fmt.Sprintf("PASSED (%.1f%%, threshold: %.1f%%)\n\n", grade.Percentage, rubric.PassThreshold))
	} else {
		sb.WriteString(fmt.Sprintf("FAILED (%.1f%%, threshold: %.1f%%) — RERUN RECOMMENDED\n\n", grade.Percentage, rubric.PassThreshold))
	}

	for _, cs := range grade.Scores {
		status := "✅"
		if !cs.Passed {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s %-20s: %.1f/%.1f (%.0f%%) — %s\n",
			status, cs.CriterionName, cs.Score, cs.MaxScore, cs.Percentage, cs.Level))
	}

	// Identify weakest areas
	var weakest []CriterionScore
	for _, cs := range grade.Scores {
		if cs.Percentage < 60 {
			weakest = append(weakest, cs)
		}
	}
	if len(weakest) > 0 {
		sb.WriteString("\nAreas for improvement:\n")
		for _, w := range weakest {
			sb.WriteString(fmt.Sprintf("  - %s: only %.0f%% (%s)\n", w.CriterionName, w.Percentage, w.Level))
		}
	}

	return sb.String()
}

// FormatGrade renders a grade for display.
func FormatGrade(grade *Grade) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Grade: %s\n", grade.ID))
	sb.WriteString(fmt.Sprintf("  Agent:     %s\n", grade.AgentID))
	sb.WriteString(fmt.Sprintf("  Rubric:    %s\n", grade.RubricID))
	sb.WriteString(fmt.Sprintf("  Score:     %.1f/%.1f (%.1f%%)\n", grade.TotalScore, grade.MaxScore, grade.Percentage))
	sb.WriteString(fmt.Sprintf("  Passed:    %v\n", grade.Passed))
	sb.WriteString(fmt.Sprintf("  Rerun:     %v\n", grade.TriggerRerun))
	sb.WriteString(fmt.Sprintf("  Graded:    %s\n", grade.GradedAt.Format(time.RFC3339)))
	sb.WriteString("\n")
	sb.WriteString(grade.Feedback)
	return sb.String()
}

// FormatRubric renders a rubric for display.
func FormatRubric(r *Rubric) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Rubric: %s (%s)\n", r.Name, r.ID))
	sb.WriteString(fmt.Sprintf("  Description: %s\n", r.Description))
	sb.WriteString(fmt.Sprintf("  Pass Threshold: %.1f%%\n", r.PassThreshold))
	sb.WriteString(fmt.Sprintf("  Max Score: %.1f\n", r.MaxScore))
	sb.WriteString("\n  Criteria:\n")
	for _, c := range r.Criteria {
		req := ""
		if c.Required {
			req = " [REQUIRED]"
		}
		sb.WriteString(fmt.Sprintf("    %-20s (weight: %.0f, max: %.0f)%s\n", c.Name, c.Weight, c.MaxScore, req))
		for _, l := range c.Levels {
			sb.WriteString(fmt.Sprintf("      %-12s (%.0f pts): %s\n", l.Label, l.Score, l.Description))
		}
	}
	return sb.String()
}

// FormatStats renders grader stats.
func FormatStats(stats GraderStats) string {
	return fmt.Sprintf("Rubric Grader Stats:\n  Total Grades:  %d\n  Total Rubrics: %d\n  Passed:        %d\n  Failed:        %d\n  Avg Score:     %.1f%%\n",
		stats.TotalGrades, stats.TotalRubrics, stats.Passed, stats.Failed, stats.AvgPercentage)
}

func (g *Grader) save() {
	rubricData, _ := json.MarshalIndent(g.rubrics, "", "  ")
	os.WriteFile(filepath.Join(g.storeDir, "rubrics.json"), rubricData, 0o644)

	gradeData, _ := json.MarshalIndent(g.grades, "", "  ")
	os.WriteFile(filepath.Join(g.storeDir, "grades.json"), gradeData, 0o644)
}

func (g *Grader) load() {
	data, err := os.ReadFile(filepath.Join(g.storeDir, "rubrics.json"))
	if err == nil {
		json.Unmarshal(data, &g.rubrics)
	}

	data, err = os.ReadFile(filepath.Join(g.storeDir, "grades.json"))
	if err == nil {
		json.Unmarshal(data, &g.grades)
	}
}
