package quantum_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/forge/sword/internal/quantum"
)

// mockExecutor returns results with predictable scores.
type mockExecutor struct {
	results map[string]*quantum.Result
}

func (m *mockExecutor) Execute(ctx context.Context, u *quantum.Universe) (*quantum.Result, error) {
	if r, ok := m.results[u.ID]; ok {
		return r, nil
	}
	return &quantum.Result{Output: "default", Score: 0.5, TokensUsed: 100, CostUSD: 0.01}, nil
}

func TestDefaultConfig(t *testing.T) {
	cfg := quantum.DefaultConfig()
	if cfg.NumUniverses != 3 {
		t.Errorf("expected 3 universes, got %d", cfg.NumUniverses)
	}
	if cfg.Criteria.Method != quantum.ScoreComposite {
		t.Errorf("expected ScoreComposite, got %d", cfg.Criteria.Method)
	}
}

func TestRunBasic(t *testing.T) {
	executor := &mockExecutor{
		results: map[string]*quantum.Result{
			"u-1": {Output: "result-1", Score: 0.8, TokensUsed: 100, CostUSD: 0.02},
			"u-2": {Output: "result-2", Score: 0.9, TokensUsed: 150, CostUSD: 0.03},
			"u-3": {Output: "result-3", Score: 0.7, TokensUsed: 80, CostUSD: 0.01},
		},
	}

	cfg := quantum.DefaultConfig()
	cfg.Criteria.Method = quantum.ScoreHighest

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "write a hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Winner == nil {
		t.Fatal("expected a winner")
	}
	if result.Winner.Result.Score != 0.9 {
		t.Errorf("expected winner score 0.9, got %.2f", result.Winner.Result.Score)
	}
	if len(result.AllUniverses) != 3 {
		t.Errorf("expected 3 universes, got %d", len(result.AllUniverses))
	}
}

func TestRunLowestCost(t *testing.T) {
	executor := &mockExecutor{
		results: map[string]*quantum.Result{
			"u-1": {Output: "expensive", Score: 0.95, TokensUsed: 500, CostUSD: 0.50},
			"u-2": {Output: "cheap", Score: 0.8, TokensUsed: 50, CostUSD: 0.01},
			"u-3": {Output: "mid", Score: 0.85, TokensUsed: 200, CostUSD: 0.10},
		},
	}

	cfg := quantum.DefaultConfig()
	cfg.Criteria.Method = quantum.ScoreLowestCost

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "optimize this")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Winner.Result.CostUSD != 0.01 {
		t.Errorf("expected cheapest ($0.01), got $%.4f", result.Winner.Result.CostUSD)
	}
}

func TestRunFastest(t *testing.T) {
	executor := quantum.ExecutorFunc(func(ctx context.Context, u *quantum.Universe) (*quantum.Result, error) {
		durations := map[string]time.Duration{
			"u-1": 3 * time.Second,
			"u-2": 1 * time.Second,
			"u-3": 5 * time.Second,
		}
		d, ok := durations[u.ID]
		if !ok {
			d = 2 * time.Second
		}
		// Simulate the duration
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &quantum.Result{Output: "result", Score: 0.8, TokensUsed: 100, CostUSD: 0.01}, nil
	})

	cfg := quantum.DefaultConfig()
	cfg.Criteria.Method = quantum.ScoreFastest
	cfg.MaxDuration = 10 * time.Second

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "fast task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Winner == nil {
		t.Fatal("expected a winner")
	}
}

func TestRunFirstSuccess(t *testing.T) {
	executor := &mockExecutor{
		results: map[string]*quantum.Result{
			"u-1": {Output: "first", Score: 0.5, TokensUsed: 100, CostUSD: 0.01},
			"u-2": {Output: "second", Score: 0.99, TokensUsed: 500, CostUSD: 0.50},
			"u-3": {Output: "third", Score: 0.6, TokensUsed: 150, CostUSD: 0.05},
		},
	}

	cfg := quantum.DefaultConfig()
	cfg.Criteria.Method = quantum.ScoreFirstSuccess

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "any task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FirstSuccess picks the first successful one (order may vary with goroutines)
	if result.Winner == nil {
		t.Fatal("expected a winner")
	}
}

func TestRunAllFail(t *testing.T) {
	executor := quantum.ExecutorFunc(func(ctx context.Context, u *quantum.Universe) (*quantum.Result, error) {
		return nil, fmt.Errorf("model error")
	})

	cfg := quantum.DefaultConfig()
	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "failing task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still return a result with a fallback
	if result.Winner == nil {
		t.Fatal("expected fallback winner")
	}
	if result.Consensus != 0.0 {
		t.Errorf("expected 0 consensus for all failures, got %.2f", result.Consensus)
	}
}

func TestRunBelowMinScore(t *testing.T) {
	executor := &mockExecutor{
		results: map[string]*quantum.Result{
			"u-1": {Output: "bad", Score: 0.1, TokensUsed: 100, CostUSD: 0.01},
			"u-2": {Output: "mediocre", Score: 0.2, TokensUsed: 100, CostUSD: 0.01},
			"u-3": {Output: "ok", Score: 0.35, TokensUsed: 100, CostUSD: 0.01},
		},
	}

	cfg := quantum.DefaultConfig()
	cfg.Criteria.MinScore = 0.5
	cfg.Criteria.Method = quantum.ScoreHighest

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "hard task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fallback to best available even below min score
	if result.Winner == nil {
		t.Fatal("expected fallback winner")
	}
}

func TestCompositeScoring(t *testing.T) {
	executor := &mockExecutor{
		results: map[string]*quantum.Result{
			"u-1": {Output: "best quality", Score: 0.95, TokensUsed: 1000, CostUSD: 0.50},
			"u-2": {Output: "balanced", Score: 0.80, TokensUsed: 300, CostUSD: 0.10},
			"u-3": {Output: "cheap", Score: 0.60, TokensUsed: 50, CostUSD: 0.01},
		},
	}

	cfg := quantum.DefaultConfig()
	cfg.Criteria.Method = quantum.ScoreComposite
	cfg.Criteria.QualityWeight = 0.5
	cfg.Criteria.CostWeight = 0.3
	cfg.Criteria.SpeedWeight = 0.2

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "balanced task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Winner == nil {
		t.Fatal("expected a winner")
	}
}

func TestConsensusScoring(t *testing.T) {
	executor := &mockExecutor{
		results: map[string]*quantum.Result{
			"u-1": {Output: "similar-1", Score: 0.80, TokensUsed: 100, CostUSD: 0.01},
			"u-2": {Output: "similar-2", Score: 0.82, TokensUsed: 100, CostUSD: 0.01},
			"u-3": {Output: "outlier", Score: 0.40, TokensUsed: 100, CostUSD: 0.01},
		},
	}

	cfg := quantum.DefaultConfig()
	cfg.Criteria.Method = quantum.ScoreConsensus

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "consensus task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Consensus should be moderate (two similar, one outlier)
	if result.Consensus < 0.5 {
		t.Errorf("expected moderate consensus, got %.2f", result.Consensus)
	}
}

func TestExperimentStore(t *testing.T) {
	store := quantum.NewStore()

	exp1 := &quantum.Experiment{
		ID:        "qe-test-1",
		Task:      "task 1",
		Config:    quantum.DefaultConfig(),
		CreatedAt: time.Now(),
	}
	exp2 := &quantum.Experiment{
		ID:        "qe-test-2",
		Task:      "task 2",
		Config:    quantum.DefaultConfig(),
		CreatedAt: time.Now().Add(time.Second),
	}

	if err := store.Save(exp1); err != nil {
		t.Fatalf("save error: %v", err)
	}
	if err := store.Save(exp2); err != nil {
		t.Fatalf("save error: %v", err)
	}

	got, err := store.Get("qe-test-1")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if got.Task != "task 1" {
		t.Errorf("expected task 1, got %s", got.Task)
	}

	list := store.List()
	if len(list) != 2 {
		t.Errorf("expected 2 experiments, got %d", len(list))
	}

	comp, err := store.Compare("qe-test-1", "qe-test-2")
	if err != nil {
		t.Fatalf("compare error: %v", err)
	}
	if comp.Experiment1.Task != "task 1" || comp.Experiment2.Task != "task 2" {
		t.Error("comparison mismatch")
	}
}

func TestNewExperimentID(t *testing.T) {
	id1 := quantum.NewExperimentID()
	id2 := quantum.NewExperimentID()
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
	if len(id1) < 5 {
		t.Errorf("ID too short: %s", id1)
	}
}

func TestUniverseGeneration(t *testing.T) {
	cfg := quantum.DefaultConfig()
	cfg.NumUniverses = 5
	cfg.Models = []string{"model-a", "model-b"}
	cfg.Temperatures = []float64{0.0, 0.5, 1.0}
	cfg.Strategies = []string{"direct", "chain-of-thought"}

	executor := &mockExecutor{results: map[string]*quantum.Result{
		"u-1": {Output: "r1", Score: 0.5, TokensUsed: 100, CostUSD: 0.01},
		"u-2": {Output: "r2", Score: 0.6, TokensUsed: 100, CostUSD: 0.01},
		"u-3": {Output: "r3", Score: 0.7, TokensUsed: 100, CostUSD: 0.01},
		"u-4": {Output: "r4", Score: 0.8, TokensUsed: 100, CostUSD: 0.01},
		"u-5": {Output: "r5", Score: 0.9, TokensUsed: 100, CostUSD: 0.01},
	}}

	engine := quantum.NewEngine(cfg, executor)
	result, err := engine.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.AllUniverses) != 5 {
		t.Errorf("expected 5 universes, got %d", len(result.AllUniverses))
	}

	// Verify diversity
	models := make(map[string]bool)
	for _, u := range result.AllUniverses {
		models[u.Model] = true
	}
	if len(models) < 2 {
		t.Error("expected at least 2 different models across universes")
	}
}
