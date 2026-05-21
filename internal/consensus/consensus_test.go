package consensus

import (
	"strings"
	"testing"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine("")
	if e == nil {
		t.Fatal("expected engine")
	}
}

func TestNewRound(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Should we use Go?", StrategyMajority)
	if r.ID == "" {
		t.Error("expected ID")
	}
	if r.Strategy != StrategyMajority {
		t.Error("strategy mismatch")
	}
}

func TestCastVote(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Question?", StrategyMajority)
	err := e.CastVote(r.ID, "agent-1", "yes", "Go is great", 1.0, 0.9)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := e.Get(r.ID)
	if len(got.Votes) != 1 {
		t.Error("should have 1 vote")
	}
	if got.Votes[0].Answer != "yes" {
		t.Error("answer mismatch")
	}
}

func TestDuplicateVote(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Q?", StrategyMajority)
	e.CastVote(r.ID, "agent-1", "yes", "", 1.0, 0.9)
	err := e.CastVote(r.ID, "agent-1", "no", "", 1.0, 0.5)
	if err == nil {
		t.Error("should reject duplicate vote")
	}
}

func TestMajorityConsensus(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Go or Rust?", StrategyMajority)
	e.CastVote(r.ID, "a1", "Go", "", 1.0, 0.9)
	e.CastVote(r.ID, "a2", "Go", "", 1.0, 0.8)
	e.CastVote(r.ID, "a3", "Rust", "", 1.0, 0.7)

	resolved, err := e.Resolve(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Winner != "Go" {
		t.Errorf("expected Go, got %s", resolved.Winner)
	}
	if !resolved.Consensus {
		t.Error("majority should reach consensus")
	}
	if resolved.Strength != 2.0/3.0 {
		t.Errorf("strength should be %.2f, got %.2f", 2.0/3.0, resolved.Strength)
	}
}

func TestMajorityTie(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Tie?", StrategyMajority)
	e.CastVote(r.ID, "a1", "yes", "", 1.0, 0.5)
	e.CastVote(r.ID, "a2", "no", "", 1.0, 0.5)

	resolved, _ := e.Resolve(r.ID)
	// Tie: first one found wins (both have 1 vote)
	if resolved.Winner == "" {
		t.Error("should pick a winner even in tie")
	}
}

func TestWeightedConsensus(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Deploy?", StrategyWeighted)
	e.CastVote(r.ID, "senior", "yes", "", 2.0, 0.9)
	e.CastVote(r.ID, "junior", "no", "", 0.5, 0.6)

	resolved, _ := e.Resolve(r.ID)
	if resolved.Winner != "yes" {
		t.Errorf("weighted should pick yes (2.0*0.9=1.8 > 0.5*0.6=0.3), got %s", resolved.Winner)
	}
}

func TestUnanimousConsensus(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("All agree?", StrategyUnanimous)
	e.CastVote(r.ID, "a1", "yes", "", 1.0, 0.9)
	e.CastVote(r.ID, "a2", "yes", "", 1.0, 0.8)
	e.CastVote(r.ID, "a3", "yes", "", 1.0, 0.7)

	resolved, _ := e.Resolve(r.ID)
	if resolved.Winner != "yes" {
		t.Error("unanimous should agree")
	}
	if resolved.Strength != 1.0 {
		t.Error("unanimous strength should be 1.0")
	}
}

func TestUnanimousFail(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Not unanimous?", StrategyUnanimous)
	e.CastVote(r.ID, "a1", "yes", "", 1.0, 0.9)
	e.CastVote(r.ID, "a2", "no", "", 1.0, 0.8)

	resolved, _ := e.Resolve(r.ID)
	if resolved.Consensus {
		t.Error("should not reach consensus")
	}
	if resolved.Winner != "" {
		t.Error("unanimous fail should have no winner")
	}
}

func TestAdversarialConsensus(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Best approach?", StrategyAdversarial)
	e.CastVote(r.ID, "a1", "microservices", "", 1.0, 0.9)
	e.CastVote(r.ID, "a2", "microservices", "", 1.0, 0.85)
	e.CastVote(r.ID, "a3", "monolith", "", 1.0, 0.6)

	resolved, _ := e.Resolve(r.ID)
	if resolved.Winner != "microservices" {
		t.Errorf("adversarial should pick microservices, got %s", resolved.Winner)
	}
}

func TestResolveNoVotes(t *testing.T) {
	e := NewEngine("")
	r := e.NewRound("Q?", StrategyMajority)
	_, err := e.Resolve(r.ID)
	if err == nil {
		t.Error("should error with no votes")
	}
}

func TestResolveNotFound(t *testing.T) {
	e := NewEngine("")
	_, err := e.Resolve("nonexistent")
	if err == nil {
		t.Error("should error")
	}
}

func TestGetNotFound(t *testing.T) {
	e := NewEngine("")
	_, ok := e.Get("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestListRounds(t *testing.T) {
	e := NewEngine("")
	e.NewRound("Q1?", StrategyMajority)
	e.NewRound("Q2?", StrategyWeighted)

	rounds := e.ListRounds()
	if len(rounds) != 2 {
		t.Errorf("expected 2, got %d", len(rounds))
	}
}

func TestFormatRound(t *testing.T) {
	r := &Round{
		ID:       "round-1",
		Question: "Deploy to prod?",
		Strategy: StrategyMajority,
		Votes: []Vote{
			{AgentID: "a1", Answer: "yes", Confidence: 0.9, Weight: 1.0},
		},
		Winner:    "yes",
		Consensus: true,
		Strength:  0.67,
	}

	s := FormatRound(r)
	if !strings.Contains(s, "yes") {
		t.Error("should show winner")
	}
	if !strings.Contains(s, "67%") {
		t.Error("should show strength")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	e1 := NewEngine(dir)
	r := e1.NewRound("Persist?", StrategyMajority)
	e1.CastVote(r.ID, "a1", "yes", "", 1.0, 0.9)

	e2 := NewEngine(dir)
	got, ok := e2.Get(r.ID)
	if !ok {
		t.Fatal("round should persist")
	}
	if len(got.Votes) != 1 {
		t.Error("votes should persist")
	}
}

func TestCastVoteNotFound(t *testing.T) {
	e := NewEngine("")
	err := e.CastVote("nonexistent", "a1", "yes", "", 1.0, 0.9)
	if err == nil {
		t.Error("should error")
	}
}
