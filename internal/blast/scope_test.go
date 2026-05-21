package blast_test

import (
	"testing"

	"github.com/forge/sword/internal/blast"
)

func TestAnalyzerCreation(t *testing.T) {
	a := blast.NewAnalyzer(t.TempDir())
	if a == nil {
		t.Error("expected non-nil analyzer")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	a := blast.NewAnalyzer(t.TempDir())
	result := a.Analyze([]blast.Change{
		{Path: "foo.go", Package: "internal/foo", Symbol: "Bar"},
	})

	if result == nil {
		t.Error("expected non-nil result")
	}
	if len(result.Changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(result.Changes))
	}
}

func TestStats(t *testing.T) {
	a := blast.NewAnalyzer(t.TempDir())
	stats := a.Stats()
	if stats["packages"].(int) < 0 {
		t.Error("expected non-negative package count")
	}
}

func TestPackageDependencies(t *testing.T) {
	a := blast.NewAnalyzer(t.TempDir())
	// Empty project has no dependencies
	deps := a.PackageDependencies("nonexistent")
	if deps != nil {
		t.Error("expected nil for nonexistent package")
	}
}

func TestPackageDependents(t *testing.T) {
	a := blast.NewAnalyzer(t.TempDir())
	dependents := a.PackageDependents("nonexistent", 3)
	if len(dependents) != 0 {
		t.Error("expected no dependents for nonexistent package")
	}
}

func TestRenderResult(t *testing.T) {
	result := &blast.ScopeResult{
		Changes: []blast.Change{
			{Path: "foo.go", Package: "internal/foo", Symbol: "Bar"},
		},
		Stats: blast.ScopeStats{
			ChangedFiles:    1,
			ChangedPackages: 1,
			ImpactedFiles:   0,
			ImpactedPackages: 0,
			MaxDepth:        0,
			TestFiles:       0,
		},
	}
	text := blast.RenderResult(result)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestAnalyzeWithImpacts(t *testing.T) {
	a := blast.NewAnalyzer(t.TempDir())
	result := a.Analyze([]blast.Change{
		{Path: "cmd/main.go", Package: "cmd"},
	})

	// In a temp dir with no Go files, should still produce a valid result
	if result.Stats.ChangedFiles != 1 {
		t.Errorf("expected 1 changed file, got %d", result.Stats.ChangedFiles)
	}
}
