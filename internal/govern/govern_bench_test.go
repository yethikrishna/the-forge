package govern

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func newBenchStore(b *testing.B) *Store {
	b.Helper()
	dir, err := os.MkdirTemp("", "govern-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })
	s, err := NewStore(dir)
	if err != nil {
		b.Fatal(err)
	}
	return s
}

func defaultCategoryScores() map[Category]int {
	return map[Category]int{
		CatSecurity:    85,
		CatCompliance:  72,
		CatAudit:       90,
		CatCost:        68,
		CatAgentTrust:  78,
		CatDataPrivacy: 81,
		CatOps:         74,
		CatAccess:      95,
	}
}

func sampleFindings(n int) []Finding {
	severities := []string{"critical", "high", "medium", "low", "info"}
	cats := []Category{CatSecurity, CatCompliance, CatAudit, CatCost}
	findings := make([]Finding, n)
	for i := range findings {
		findings[i] = Finding{
			Severity:    severities[i%len(severities)],
			Title:       fmt.Sprintf("Finding %d", i),
			Description: "Benchmark finding",
			Category:    cats[i%len(cats)],
			Status:      "open",
			DetectedAt:  time.Now().UTC(),
		}
	}
	return findings
}

// BenchmarkAssess measures governance assessment computation.
func BenchmarkAssess(b *testing.B) {
	s := newBenchStore(b)
	cfg := ReportConfig{
		Name:      "bench-report",
		Framework: "SOC2",
	}
	scores := defaultCategoryScores()
	findings := sampleFindings(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Name = fmt.Sprintf("bench-report-%d", i) // avoid duplicate ID
		s.Assess(cfg, scores, findings) //nolint:errcheck
	}
}

// BenchmarkAssess_NoFindings measures assessment with zero findings (fast path).
func BenchmarkAssess_NoFindings(b *testing.B) {
	s := newBenchStore(b)
	cfg := ReportConfig{Name: "no-findings"}
	scores := defaultCategoryScores()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Name = fmt.Sprintf("no-findings-%d", i)
		s.Assess(cfg, scores, nil) //nolint:errcheck
	}
}

// BenchmarkAssess_ManyFindings measures assessment with a large findings set.
func BenchmarkAssess_ManyFindings(b *testing.B) {
	s := newBenchStore(b)
	cfg := ReportConfig{Name: "many-findings"}
	scores := defaultCategoryScores()
	findings := sampleFindings(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Name = fmt.Sprintf("many-findings-%d", i)
		s.Assess(cfg, scores, findings) //nolint:errcheck
	}
}

// BenchmarkScoreToGrade measures the grade conversion function.
func BenchmarkScoreToGrade(b *testing.B) {
	scores := []int{0, 30, 59, 60, 70, 80, 90, 100}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScoreToGrade(scores[i%len(scores)])
	}
}

// BenchmarkExportMarkdown measures markdown report generation.
func BenchmarkExportMarkdown(b *testing.B) {
	s := newBenchStore(b)
	cfg := ReportConfig{Name: "export-bench"}
	a, err := s.Assess(cfg, defaultCategoryScores(), sampleFindings(20))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExportMarkdown(a.ID) //nolint:errcheck
	}
}

// BenchmarkList_LargeStore measures list performance with many assessments.
func BenchmarkList_LargeStore(b *testing.B) {
	s := newBenchStore(b)
	scores := defaultCategoryScores()
	for i := 0; i < 200; i++ {
		cfg := ReportConfig{Name: fmt.Sprintf("report-%d", i)}
		s.Assess(cfg, scores, nil) //nolint:errcheck
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.List() //nolint:errcheck
	}
}

// BenchmarkGetFindings_FilteredStatus measures finding retrieval by status.
func BenchmarkGetFindings_FilteredStatus(b *testing.B) {
	s := newBenchStore(b)
	cfg := ReportConfig{Name: "findings-bench"}
	s.Assess(cfg, defaultCategoryScores(), sampleFindings(50)) //nolint:errcheck
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.GetFindings("open") //nolint:errcheck
	}
}

// BenchmarkDefaultWeights measures default weight map creation.
func BenchmarkDefaultWeights(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DefaultWeights()
	}
}
