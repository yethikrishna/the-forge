package consensus

import (
	"strings"
	"testing"
)

func TestStartRound(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Review this code", StrategyMajority, []string{"agent-1", "agent-2", "agent-3"}, 0.6)

	if round.ID == "" {
		t.Error("expected non-empty round ID")
	}
	if round.Strategy != StrategyMajority {
		t.Errorf("expected majority, got %s", round.Strategy)
	}
	if round.Status != RoundRunning {
		t.Errorf("expected running, got %s", round.Status)
	}
	if len(round.Agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(round.Agents))
	}
}

func TestAddResponse(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyMajority, []string{"agent-1", "agent-2"}, 0.5)

	err := e.AddResponse(round.ID, AgentResponse{
		AgentID: "agent-1",
		Model:   "gpt-4",
		Output:  "Result 1",
		Score:   90,
		Trust:   0.9,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := e.GetRound(round.ID)
	if len(got.Responses) != 1 {
		t.Errorf("expected 1 response, got %d", len(got.Responses))
	}
}

func TestAddResponseNotRunning(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyMajority, []string{"agent-1"}, 0.5)
	e.ResolveWithSimulatedResponses(round.ID) // completes the round

	err := e.AddResponse(round.ID, AgentResponse{
		AgentID: "agent-1",
		Model:   "gpt-4",
		Output:  "Late response",
		Score:   80,
	})
	if err == nil {
		t.Error("expected error for completed round")
	}
}

func TestResolveMajority(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyMajority, []string{"a1", "a2", "a3"}, 0.5)

	e.AddResponse(round.ID, AgentResponse{AgentID: "a1", Output: "same result", Score: 85, Trust: 0.9})
	e.AddResponse(round.ID, AgentResponse{AgentID: "a2", Output: "same result", Score: 88, Trust: 0.85})
	e.AddResponse(round.ID, AgentResponse{AgentID: "a3", Output: "different", Score: 75, Trust: 0.8})

	result, err := e.Resolve(round.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.WinnerID == "" {
		t.Error("expected a winner")
	}
	if result.Agreement < 0.5 {
		t.Errorf("expected high agreement, got %.2f", result.Agreement)
	}
}

func TestResolveWeighted(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyWeighted, []string{"a1", "a2"}, 0.5)

	e.AddResponse(round.ID, AgentResponse{AgentID: "a1", Output: "Result A", Score: 80, Trust: 0.95})
	e.AddResponse(round.ID, AgentResponse{AgentID: "a2", Output: "Result B", Score: 90, Trust: 0.7})

	result, err := e.Resolve(round.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// a1: 0.95 * 80 = 76, a2: 0.7 * 90 = 63 → a1 wins
	if result.WinnerID != "a1" {
		t.Errorf("expected a1 to win with weighted strategy, got %s", result.WinnerID)
	}
}

func TestResolveAdversarial(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyAdversarial, []string{"a1", "a2", "a3"}, 0.5)

	e.AddResponse(round.ID, AgentResponse{AgentID: "a1", Output: "A", Score: 85})
	e.AddResponse(round.ID, AgentResponse{AgentID: "a2", Output: "B", Score: 95})
	e.AddResponse(round.ID, AgentResponse{AgentID: "a3", Output: "C", Score: 80})

	result, err := e.Resolve(round.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.WinnerID != "a2" {
		t.Errorf("expected a2 (highest score) to win, got %s", result.WinnerID)
	}
}

func TestResolveFirstOK(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyFirstOK, []string{"a1", "a2"}, 0.5)

	e.AddResponse(round.ID, AgentResponse{AgentID: "a1", Output: "First", Score: 75})
	e.AddResponse(round.ID, AgentResponse{AgentID: "a2", Output: "Second", Score: 95})

	result, err := e.Resolve(round.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// a1 is first with score >= 70
	if result.WinnerID != "a1" {
		t.Errorf("expected a1 (first acceptable) to win, got %s", result.WinnerID)
	}
}

func TestResolveNoResponses(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyMajority, []string{"a1"}, 0.5)

	_, err := e.Resolve(round.ID)
	if err == nil {
		t.Error("expected error for no responses")
	}
}

func TestSimulatedResponses(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Test task", StrategyMajority, []string{"a1", "a2", "a3"}, 0.5)

	result, err := e.ResolveWithSimulatedResponses(round.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.WinnerID == "" {
		t.Error("expected a winner")
	}
	if len(result.Responses) != 3 {
		t.Errorf("expected 3 responses, got %d", len(result.Responses))
	}
}

func TestListRounds(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.StartRound("Task 1", StrategyMajority, []string{"a1"}, 0.5)
	e.StartRound("Task 2", StrategyAdversarial, []string{"a1", "a2"}, 0.5)

	rounds := e.ListRounds()
	if len(rounds) != 2 {
		t.Errorf("expected 2 rounds, got %d", len(rounds))
	}
}

func TestDeleteRound(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Task", StrategyMajority, []string{"a1"}, 0.5)
	e.DeleteRound(round.ID)

	if len(e.ListRounds()) != 0 {
		t.Error("expected round to be deleted")
	}
}

func TestRoundReport(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	round := e.StartRound("Review this code for security", StrategyMajority, []string{"a1", "a2", "a3"}, 0.6)
	e.ResolveWithSimulatedResponses(round.ID)

	got, _ := e.GetRound(round.ID)
	report := RoundReport(got)

	if !strings.Contains(report, "majority") && !strings.Contains(report, "Majority") {
		t.Error("expected strategy in report")
	}
	if !strings.Contains(report, "Responses:") {
		t.Error("expected responses section")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.StartRound("Task 1", StrategyMajority, []string{"a1"}, 0.5)
	e.StartRound("Task 2", StrategyAdversarial, []string{"a1"}, 0.5)

	stats := e.Stats()
	if stats["total_rounds"] != 2 {
		t.Errorf("expected 2 rounds, got %v", stats["total_rounds"])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	e1 := NewEngine(dir)
	round := e1.StartRound("Persistent task", StrategyMajority, []string{"a1", "a2"}, 0.5)
	e1.AddResponse(round.ID, AgentResponse{AgentID: "a1", Output: "Hello", Score: 90, Trust: 0.9})

	e2 := NewEngine(dir)
	rounds := e2.ListRounds()
	if len(rounds) != 1 {
		t.Fatalf("expected 1 round after reload, got %d", len(rounds))
	}
}

func TestStrategies(t *testing.T) {
	strategies := []Strategy{StrategyMajority, StrategyWeighted, StrategyUnanimous, StrategyAdversarial, StrategyFirstOK}
	for _, s := range strategies {
		if s == "" {
			t.Error("empty strategy")
		}
	}
}
