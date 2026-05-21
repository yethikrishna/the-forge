package synthesis_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/synthesis"
)

func TestVoteStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "answer is 42", Confidence: 0.9},
		{AgentID: "a2", Output: "answer is 42", Confidence: 0.8},
		{AgentID: "a3", Output: "answer is 7", Confidence: 0.7},
	}

	result, err := engine.Synthesize(synthesis.StrategyVote, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "answer is 42" {
		t.Errorf("expected 'answer is 42', got %s", result.Output)
	}
	if !result.Consensus {
		t.Error("expected consensus with 2/3 votes")
	}
	if result.WinnerID != "a1" {
		t.Errorf("expected winner a1, got %s", result.WinnerID)
	}
	if result.Confidence != 2.0/3.0 {
		t.Errorf("expected confidence 0.67, got %.2f", result.Confidence)
	}
}

func TestBestStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "low confidence", Confidence: 0.5},
		{AgentID: "a2", Output: "high confidence", Confidence: 0.95},
	}

	result, err := engine.Synthesize(synthesis.StrategyBest, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "high confidence" {
		t.Errorf("expected 'high confidence', got %s", result.Output)
	}
	if result.WinnerID != "a2" {
		t.Errorf("expected winner a2, got %s", result.WinnerID)
	}
}

func TestMergeStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "Part A", Confidence: 0.8},
		{AgentID: "a2", Output: "Part B", Confidence: 0.7},
	}

	result, err := engine.Synthesize(synthesis.StrategyMerge, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output == "" {
		t.Error("expected non-empty merged output")
	}
	if result.Consensus {
		t.Error("different outputs should not have consensus")
	}
}

func TestCascadeStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "high conf", Confidence: 0.95},
		{AgentID: "a2", Output: "low conf", Confidence: 0.3},
	}

	result, err := engine.Synthesize(synthesis.StrategyCascade, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "high conf" {
		t.Errorf("expected 'high conf', got %s", result.Output)
	}
	if !result.Consensus {
		t.Error("cascade above threshold should have consensus")
	}
}

func TestCascadeFallback(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "low", Confidence: 0.3},
		{AgentID: "a2", Output: "lower", Confidence: 0.2},
	}

	result, err := engine.Synthesize(synthesis.StrategyCascade, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fallback to best available
	if result.Output != "low" {
		t.Errorf("expected fallback to 'low', got %s", result.Output)
	}
}

func TestEnsembleStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "best answer", Confidence: 0.9},
		{AgentID: "a2", Output: "ok answer", Confidence: 0.6},
		{AgentID: "a3", Output: "meh answer", Confidence: 0.3},
	}

	result, err := engine.Synthesize(synthesis.StrategyEnsemble, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "best answer" {
		t.Errorf("expected 'best answer', got %s", result.Output)
	}
	if len(result.Scores) != 3 {
		t.Errorf("expected 3 scores, got %d", len(result.Scores))
	}
}

func TestConsensusStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	// Test with consensus (60% threshold)
	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "agree", Confidence: 0.9},
		{AgentID: "a2", Output: "agree", Confidence: 0.8},
		{AgentID: "a3", Output: "disagree", Confidence: 0.7},
	}

	result, err := engine.Synthesize(synthesis.StrategyConsensus, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Consensus {
		t.Error("expected consensus with 2/3 agreement (66%)")
	}
}

func TestConsensusFailure(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "A", Confidence: 0.9},
		{AgentID: "a2", Output: "B", Confidence: 0.8},
		{AgentID: "a3", Output: "C", Confidence: 0.7},
	}

	_, err := engine.Synthesize(synthesis.StrategyConsensus, inputs)
	if err == nil {
		t.Error("expected error when no consensus")
	}
}

func TestMRRStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "best", Confidence: 0.9},
		{AgentID: "a2", Output: "second", Confidence: 0.7},
	}

	result, err := engine.Synthesize(synthesis.StrategyMRR, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "best" {
		t.Errorf("expected 'best', got %s", result.Output)
	}
	if len(result.Scores) != 2 {
		t.Errorf("expected 2 scores, got %d", len(result.Scores))
	}
}

func TestUnknownStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	_, err := engine.Synthesize(synthesis.Strategy("unknown"), nil)
	if err == nil {
		t.Error("expected error for unknown strategy")
	}
}

func TestEmptyInputs(t *testing.T) {
	engine := synthesis.NewEngine()

	_, err := engine.Synthesize(synthesis.StrategyVote, nil)
	if err == nil {
		t.Error("expected error for empty inputs")
	}
}

func TestHistory(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "test", Confidence: 0.9},
	}

	engine.Synthesize(synthesis.StrategyBest, inputs)
	engine.Synthesize(synthesis.StrategyBest, inputs)

	history := engine.History(10)
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
}

func TestStats(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "test", Confidence: 0.9, TokensUsed: 100, Cost: 0.01},
	}

	engine.Synthesize(synthesis.StrategyBest, inputs)
	engine.Synthesize(synthesis.StrategyVote, inputs)

	stats := engine.Stats()
	if stats.TotalSyntheses != 2 {
		t.Errorf("expected 2 syntheses, got %d", stats.TotalSyntheses)
	}
	if stats.ByStrategy["best"] != 1 {
		t.Errorf("expected 1 best, got %d", stats.ByStrategy["best"])
	}
	if stats.TotalTokens != 200 {
		t.Errorf("expected 200 total tokens, got %d", stats.TotalTokens)
	}
}

func TestCustomStrategy(t *testing.T) {
	engine := synthesis.NewEngine()

	customStrategy := synthesis.Strategy("custom")
	engine.RegisterStrategy(customStrategy, func(inputs []synthesis.AgentResponse) (*synthesis.SynthesisResult, error) {
		return &synthesis.SynthesisResult{
			Output:     "custom: " + inputs[0].Output,
			Confidence: 1.0,
		}, nil
	})

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "hello", Confidence: 0.8},
	}

	result, err := engine.Synthesize(customStrategy, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "custom: hello" {
		t.Errorf("expected 'custom: hello', got %s", result.Output)
	}
}

func TestSingleInput(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "only one", Confidence: 0.9},
	}

	result, err := engine.Synthesize(synthesis.StrategyVote, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "only one" {
		t.Errorf("expected 'only one', got %s", result.Output)
	}
	if result.AgentCount != 1 {
		t.Errorf("expected 1 agent, got %d", result.AgentCount)
	}
}

func TestResultMetadata(t *testing.T) {
	engine := synthesis.NewEngine()

	inputs := []synthesis.AgentResponse{
		{AgentID: "a1", Output: "test", Confidence: 0.9, TokensUsed: 100, Cost: 0.01, Latency: 50 * time.Millisecond},
		{AgentID: "a2", Output: "test2", Confidence: 0.7, TokensUsed: 200, Cost: 0.02, Latency: 100 * time.Millisecond},
	}

	result, err := engine.Synthesize(synthesis.StrategyBest, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalTokens != 300 {
		t.Errorf("expected 300 total tokens, got %d", result.TotalTokens)
	}
	if result.TotalCost != 0.03 {
		t.Errorf("expected 0.03 total cost, got %.4f", result.TotalCost)
	}
	if result.AgentCount != 2 {
		t.Errorf("expected 2 agents, got %d", result.AgentCount)
	}
}
