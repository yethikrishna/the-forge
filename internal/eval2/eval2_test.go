package eval2_test

import (
	"testing"

	"github.com/forge/sword/internal/eval2/abtest"
	"github.com/forge/sword/internal/eval2/agenttest"
	"github.com/forge/sword/internal/eval2/benchmark"
)

func TestABTestCreate(t *testing.T) {
	store := abtest.NewStore(t.TempDir())

	exp, err := store.Create("test-exp", "Test prompt", []abtest.Variant{
		{Name: "A", Model: "gpt-4"},
		{Name: "B", Model: "claude"},
	}, 100)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if exp.Name != "test-exp" {
		t.Errorf("Name = %q, want %q", exp.Name, "test-exp")
	}
}

func TestABTestStart(t *testing.T) {
	store := abtest.NewStore(t.TempDir())
	exp, _ := store.Create("start-test", "prompt", []abtest.Variant{
		{Name: "A"}, {Name: "B"},
	}, 10)

	started, err := store.Start(exp.ID)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if started.Status != "running" {
		t.Errorf("Status = %q, want %q", started.Status, "running")
	}
}

func TestABTestRecordResult(t *testing.T) {
	store := abtest.NewStore(t.TempDir())
	exp, _ := store.Create("result-test", "prompt", []abtest.Variant{
		{Name: "A"}, {Name: "B"},
	}, 10)
	store.Start(exp.ID)

	_, err := store.RecordResult(exp.ID, abtest.Result{
		Variant:   "A",
		Score:     0.85,
		LatencyMS: 2500,
		CostUSD:   0.05,
		Success:   true,
	})
	if err != nil {
		t.Fatalf("RecordResult error: %v", err)
	}
}

func TestABTestAnalyze(t *testing.T) {
	exp := &abtest.Experiment{
		Name:      "analyze-test",
		Variants:  []abtest.Variant{{Name: "A"}, {Name: "B"}},
		Results: []abtest.Result{
			{Variant: "A", Score: 0.9, Success: true},
			{Variant: "B", Score: 0.6, Success: true},
			{Variant: "A", Score: 0.85, Success: true},
			{Variant: "B", Score: 0.55, Success: true},
		},
	}
	analysis := abtest.Analyze(exp)
	if analysis == nil {
		t.Fatal("Analyze should return analysis")
	}

	formatted := abtest.FormatAnalysis(analysis)
	if formatted == "" {
		t.Error("FormatAnalysis should not be empty")
	}
}

func TestABTestList(t *testing.T) {
	store := abtest.NewStore(t.TempDir())
	store.Create("exp-1", "prompt", []abtest.Variant{{Name: "A"}}, 10)
	store.Create("exp-2", "prompt", []abtest.Variant{{Name: "A"}}, 10)

	exps, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(exps) != 2 {
		t.Errorf("List = %d, want 2", len(exps))
	}
}

func TestAgentTestEvaluateAssertion(t *testing.T) {
	assertion := agenttest.Assertion{
		Type:   "contains",
		Value:  "hello",
		Negate: false,
	}

	result := agenttest.EvaluateAssertion(assertion, "say hello world")
	if !result.Passed {
		t.Error("Contains assertion should pass")
	}
}

func TestAgentTestEvaluateAssertionNegate(t *testing.T) {
	assertion := agenttest.Assertion{
		Type:   "contains",
		Value:  "error",
		Negate: true,
	}

	result := agenttest.EvaluateAssertion(assertion, "success message")
	if !result.Passed {
		t.Error("Negated contains should pass when value is absent")
	}
}

func TestAgentTestEvaluateTestCase(t *testing.T) {
	tc := agenttest.TestCase{
		Name:   "test-greeting",
		Prompt: "Say hello",
		Assertions: []agenttest.Assertion{
			{Type: "contains", Value: "hello"},
		},
	}

	results := agenttest.EvaluateTestCase(tc, "hello there")
	allPassed := true
	for _, r := range results {
		if !r.Passed {
			allPassed = false
		}
	}
	if !allPassed {
		t.Error("All assertions should pass for valid output")
	}
}

func TestAgentTestSuiteResult(t *testing.T) {
	sr := &agenttest.SuiteResult{
		SuiteName: "test-suite",
		Results: []agenttest.Result{
			{TestCaseName: "test1", Status: agenttest.StatusPass},
			{TestCaseName: "test2", Status: agenttest.StatusFail, Error: "assertion failed"},
		},
	}

	summary := sr.Summary()
	if summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestBenchmarkRunner(t *testing.T) {
	runner := benchmark.NewRunner(t.TempDir())
	runner.WithScorer(&benchmark.ExactScorer{})
	runner.WithScorer(&benchmark.ContainsScorer{})
	runner.WithScorer(&benchmark.KeywordScorer{})
}

func TestBenchmarkBuiltins(t *testing.T) {
	benchmarks := benchmark.BuiltInBenchmarks()
	if len(benchmarks) == 0 {
		t.Error("BuiltInBenchmarks should return some benchmarks")
	}
}

func TestBenchmarkRun(t *testing.T) {
	runner := benchmark.NewRunner(t.TempDir())
	runner.WithScorer(&benchmark.ContainsScorer{})

	bm := benchmark.Benchmark{
		Name:     "test-bench",
		Prompt:   "Write a hello world function",
		Expected: "hello",
	}

	result := runner.RunBenchmark(bm, "test-agent", "test-model", "hello world", 1.0, 0.01)
	if result.Score < 0 {
		t.Error("Score should be non-negative")
	}
}

func TestBenchmarkHistory(t *testing.T) {
	runner := benchmark.NewRunner(t.TempDir())
	runner.WithScorer(&benchmark.ContainsScorer{})

	bm := benchmark.Benchmark{Name: "test", Prompt: "test", Expected: "test"}
	runner.RunBenchmark(bm, "agent", "model", "test", 1.0, 0.01)

	history := runner.History()
	if len(history) < 1 {
		t.Error("History should have at least one entry")
	}
}

func TestBenchmarkCompare(t *testing.T) {
	runner := benchmark.NewRunner(t.TempDir())
	runner.WithScorer(&benchmark.ContainsScorer{})

	bm := benchmark.Benchmark{Name: "cmp", Prompt: "test", Expected: "test"}

	r1 := runner.RunBenchmark(bm, "agent-a", "model-a", "test output", 1.0, 0.01)
	r2 := runner.RunBenchmark(bm, "agent-b", "model-b", "test", 0.5, 0.005)

	comparison := runner.Compare(r1, r2)
	if comparison == "" {
		t.Error("Compare should not be empty")
	}
}
