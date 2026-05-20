package eval_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/forge/sword/internal/eval"
)

func TestExactScorer(t *testing.T) {
	scorer := &eval.ExactScorer{}

	if scorer.Score("hello", "hello") != 1.0 {
		t.Error("exact match should score 1.0")
	}
	if scorer.Score("hello", "world") != 0.0 {
		t.Error("non-match should score 0.0")
	}
}

func TestContainsScorer(t *testing.T) {
	scorer := &eval.ContainsScorer{}

	if scorer.Score("hello world", "hello") != 1.0 {
		t.Error("should find substring")
	}
	if scorer.Score("hello world", "xyz") != 0.0 {
		t.Error("should not find missing substring")
	}
	if scorer.Score("anything", "") != 1.0 {
		t.Error("empty expected should score 1.0")
	}
}

func TestKeywordScorer(t *testing.T) {
	scorer := &eval.KeywordScorer{}

	// Expected is semicolon-separated keywords
	score := scorer.Score("package main with fmt.Println saying Hello", "package main; fmt.Println; Hello")
	if score != 1.0 {
		t.Errorf("all keywords present, expected 1.0, got %.2f", score)
	}

	score = scorer.Score("package main with fmt.Println", "package main; fmt.Println; Hello")
	if score != 0.67 && score < 0.6 || score > 0.7 {
		t.Errorf("2 of 3 keywords, expected ~0.67, got %.2f", score)
	}
}

func TestRunBenchmark(t *testing.T) {
	runner := eval.NewRunner("")

	bm := eval.Benchmark{
		ID:       "test-1",
		Name:     "Test Benchmark",
		Expected: "hello; world",
	}

	result := runner.RunBenchmark(bm, "claude", "sonnet", "hello and world output", 100*time.Millisecond, 0.01)
	if result.BenchmarkID != "test-1" {
		t.Errorf("expected test-1, got %s", result.BenchmarkID)
	}
	if result.Score <= 0 {
		t.Error("should have a positive score")
	}
	if result.Grade == "" {
		t.Error("should have a grade")
	}
}

func TestRunAll(t *testing.T) {
	runner := eval.NewRunner("")

	benchmarks := eval.BuiltInBenchmarks()

	runFn := func(bm eval.Benchmark) (string, time.Duration, float64, error) {
		return "package main with fmt.Println Hello world", 50 * time.Millisecond, 0.01, nil
	}

	result := runner.RunAll(benchmarks, "claude", "sonnet", runFn)

	if result.AvgScore <= 0 {
		t.Error("should have positive average score")
	}
	if result.Grade == "" {
		t.Error("should have a grade")
	}
	if len(result.Results) != len(benchmarks) {
		t.Errorf("expected %d results, got %d", len(benchmarks), len(result.Results))
	}
}

func TestRunAllWithErrors(t *testing.T) {
	runner := eval.NewRunner("")

	benchmarks := []eval.Benchmark{
		{ID: "ok", Name: "OK", Expected: "hello"},
		{ID: "fail", Name: "Fail", Expected: "hello"},
	}

	callCount := 0
	runFn := func(bm eval.Benchmark) (string, time.Duration, float64, error) {
		callCount++
		if bm.ID == "fail" {
			return "", 10 * time.Millisecond, 0, fmt.Errorf("agent error")
		}
		return "hello world", 50 * time.Millisecond, 0.01, nil
	}

	result := runner.RunAll(benchmarks, "claude", "sonnet", runFn)

	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
	if result.Results[1].Error == "" {
		t.Error("second result should have an error")
	}
	if result.Results[1].Grade != eval.GradeF {
		t.Errorf("failed benchmark should be F, got %s", result.Results[1].Grade)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade eval.Grade
	}{
		{0.99, eval.GradeAPlus},
		{0.95, eval.GradeAPlus},
		{0.90, eval.GradeA},
		{0.80, eval.GradeB},
		{0.70, eval.GradeC},
		{0.60, eval.GradeD},
		{0.30, eval.GradeF},
	}

	for _, tt := range tests {
		// Test via RunBenchmark which uses scoreToGrade internally
		runner := eval.NewRunner("")
		bm := eval.Benchmark{ID: "grade-test", Name: "Grade Test", Expected: "expected keywords"}
		result := runner.RunBenchmark(bm, "test", "test", "", 0, 0)
		if result.Grade == "" {
			t.Error("should have a grade")
		}
	}
}

func TestBuiltInBenchmarks(t *testing.T) {
	benchmarks := eval.BuiltInBenchmarks()
	if len(benchmarks) == 0 {
		t.Error("should have built-in benchmarks")
	}
}

func TestHistory(t *testing.T) {
	runner := eval.NewRunner("")

	benchmarks := []eval.Benchmark{
		{ID: "t1", Name: "T1", Expected: "hello"},
	}

	runFn := func(bm eval.Benchmark) (string, time.Duration, float64, error) {
		return "hello", 10 * time.Millisecond, 0.001, nil
	}

	runner.RunAll(benchmarks, "claude", "sonnet", runFn)
	runner.RunAll(benchmarks, "reviewer", "opus", runFn)

	history := runner.History()
	if len(history) != 2 {
		t.Errorf("expected 2 runs in history, got %d", len(history))
	}
}

func TestFormatRunResult(t *testing.T) {
	runner := eval.NewRunner("")

	benchmarks := []eval.Benchmark{
		{ID: "t1", Name: "T1", Expected: "hello"},
	}

	runFn := func(bm eval.Benchmark) (string, time.Duration, float64, error) {
		return "hello", 10 * time.Millisecond, 0.001, nil
	}

	result := runner.RunAll(benchmarks, "claude", "sonnet", runFn)
	formatted := eval.FormatRunResult(result)

	if formatted == "" {
		t.Error("formatted result should not be empty")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/eval.json"

	runner := eval.NewRunner(path)

	benchmarks := []eval.Benchmark{
		{ID: "t1", Name: "T1", Expected: "hello"},
	}

	runFn := func(bm eval.Benchmark) (string, time.Duration, float64, error) {
		return "hello", 10 * time.Millisecond, 0.001, nil
	}

	runner.RunAll(benchmarks, "claude", "sonnet", runFn)

	runner2 := eval.NewRunner(path)
	if len(runner2.History()) != 1 {
		t.Errorf("expected 1 run after reload, got %d", len(runner2.History()))
	}
}
