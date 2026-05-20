package debate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateDebate(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debaters := []Debater{
		{ID: "d1", Name: "Pro", Agent: "advocate", Position: PositionFor},
		{ID: "d2", Name: "Con", Agent: "critic", Position: PositionAgainst},
	}

	debate, err := store.Create("Should we use microservices?", "Architecture decision", debaters, 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if debate.ID == "" {
		t.Error("expected non-empty ID")
	}
	if debate.Topic != "Should we use microservices?" {
		t.Errorf("unexpected topic: %s", debate.Topic)
	}
	if len(debate.Debaters) != 2 {
		t.Errorf("expected 2 debaters, got %d", len(debate.Debaters))
	}
	if debate.MaxRounds != 3 {
		t.Errorf("expected 3 rounds, got %d", debate.MaxRounds)
	}
	if debate.Status != "open" {
		t.Errorf("expected status open, got %s", debate.Status)
	}
}

func TestCreateDebateDefaultRounds(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debate, _ := store.Create("Test topic", "", nil, 0)
	if debate.MaxRounds != 3 {
		t.Errorf("expected default 3 rounds, got %d", debate.MaxRounds)
	}
}

func TestAddArgument(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debaters := []Debater{
		{ID: "d1", Name: "Pro", Position: PositionFor},
		{ID: "d2", Name: "Con", Position: PositionAgainst},
	}
	debate, _ := store.Create("Test debate", "", debaters, 3)

	arg := Argument{
		DebaterID: "d1",
		Position:  PositionFor,
		Claim:     "Microservices enable independent deployment",
		Evidence:  "Netflix migrated to microservices in 2015",
		Reasoning: "Independent deployment reduces blast radius",
	}

	updated, err := store.AddArgument(debate.ID, arg)
	if err != nil {
		t.Fatalf("AddArgument failed: %v", err)
	}

	if len(updated.Arguments) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(updated.Arguments))
	}
	if updated.Arguments[0].ID == "" {
		t.Error("expected auto-generated argument ID")
	}
	if updated.Arguments[0].Round != 1 {
		t.Errorf("expected round 1, got %d", updated.Arguments[0].Round)
	}
}

func TestAddMultipleRounds(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debate, _ := store.Create("Test", "", []Debater{
		{ID: "d1", Position: PositionFor},
	}, 3)

	store.AddArgument(debate.ID, Argument{DebaterID: "d1", Claim: "Round 1"})
	store.AddArgument(debate.ID, Argument{DebaterID: "d1", Claim: "Round 2"})
	store.AddArgument(debate.ID, Argument{DebaterID: "d1", Claim: "Round 3"})

	updated, _ := store.Get(debate.ID)
	if len(updated.Arguments) != 3 {
		t.Errorf("expected 3 arguments, got %d", len(updated.Arguments))
	}
	if updated.Rounds != 3 {
		t.Errorf("expected 3 rounds, got %d", updated.Rounds)
	}
	if updated.Status != "concluded" {
		t.Errorf("expected status concluded, got %s", updated.Status)
	}
}

func TestConcludeDebate(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debate, _ := store.Create("Test", "", []Debater{
		{ID: "d1", Position: PositionFor},
		{ID: "d2", Position: PositionAgainst},
	}, 2)

	store.AddArgument(debate.ID, Argument{DebaterID: "d1", Claim: "For"})
	store.AddArgument(debate.ID, Argument{DebaterID: "d2", Claim: "Against"})

	verdict := Verdict{
		Winner:     "d1",
		Reasoning:  "Stronger evidence",
		Confidence: 0.85,
		KeyPoints:  []string{"Deployment independence", "Team autonomy"},
		Consensus:  false,
	}

	concluded, err := store.Conclude(debate.ID, verdict)
	if err != nil {
		t.Fatalf("Conclude failed: %v", err)
	}

	if concluded.Status != "concluded" {
		t.Errorf("expected concluded, got %s", concluded.Status)
	}
	if concluded.Verdict.Winner != "d1" {
		t.Errorf("expected winner d1, got %s", concluded.Verdict.Winner)
	}
	if concluded.Verdict.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", concluded.Verdict.Confidence)
	}
}

func TestGetByTopic(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	store.Create("Should we use Go?", "", nil, 3)

	found, err := store.Get("should we use go?")
	if err != nil {
		t.Fatalf("Get by topic failed: %v", err)
	}
	if found.Topic != "Should we use Go?" {
		t.Errorf("unexpected topic: %s", found.Topic)
	}
}

func TestGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent debate")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	store.Create("Debate 1", "", nil, 3)
	store.Create("Debate 2", "", nil, 3)
	store.Create("Debate 3", "", nil, 3)

	debates, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(debates) != 3 {
		t.Errorf("expected 3 debates, got %d", len(debates))
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debates, err := store.List()
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(debates) != 0 {
		t.Errorf("expected 0 debates, got %d", len(debates))
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debate, _ := store.Create("To delete", "", nil, 3)
	if err := store.Delete(debate.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(debate.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestEvaluateArguments(t *testing.T) {
	debate := &Debate{
		Arguments: []Argument{
			{Claim: "No evidence", Evidence: "", Reasoning: ""},
			{Claim: "With evidence", Evidence: "data point", Reasoning: ""},
			{Claim: "Full argument", Evidence: "data", Reasoning: "logic", RebuttalTo: "arg-1"},
		},
	}

	EvaluateArguments(debate)

	if debate.Arguments[0].Score >= debate.Arguments[1].Score {
		t.Error("arguments with evidence should score higher")
	}
	if debate.Arguments[1].Score >= debate.Arguments[2].Score {
		t.Error("arguments with evidence+reasoning+rebuttal should score highest")
	}
	if debate.Arguments[2].Score > 100 {
		t.Errorf("score should be capped at 100, got %f", debate.Arguments[2].Score)
	}
}

func TestFormatDebate(t *testing.T) {
	debate := &Debate{
		Topic:     "Test Topic",
		Status:    "concluded",
		Rounds:    1,
		MaxRounds: 3,
		Debaters: []Debater{
			{ID: "d1", Name: "Pro", Position: PositionFor},
		},
		Arguments: []Argument{
			{DebaterID: "d1", Position: PositionFor, Claim: "Yes", Round: 1},
		},
		Verdict: &Verdict{
			Winner:     "d1",
			Reasoning:  "Compelling",
			Confidence: 0.9,
			Consensus:  true,
		},
	}

	output := FormatDebate(debate)
	if !strings.Contains(output, "Test Topic") {
		t.Error("expected topic in output")
	}
	if !strings.Contains(output, "Verdict") {
		t.Error("expected verdict in output")
	}
	if !strings.Contains(output, "Consensus") {
		t.Error("expected consensus in output")
	}
}

func TestDebateSerialization(t *testing.T) {
	debate := &Debate{
		ID:        "debate-test",
		Topic:     "Serialize test",
		Status:    "open",
		MaxRounds: 3,
		Debaters:  []Debater{{ID: "d1", Position: PositionFor}},
		Arguments: []Argument{{ID: "a1", Claim: "Test", Round: 1}},
		CreatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(debate, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var d2 Debate
	if err := json.Unmarshal(data, &d2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if d2.Topic != "Serialize test" {
		t.Errorf("expected topic 'Serialize test', got %s", d2.Topic)
	}
	if len(d2.Debaters) != 1 {
		t.Errorf("expected 1 debater, got %d", len(d2.Debaters))
	}
}

func TestDebatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debate, _ := store.Create("Persist test", "desc", []Debater{
		{ID: "d1", Position: PositionFor},
	}, 2)

	store.AddArgument(debate.ID, Argument{DebaterID: "d1", Claim: "Point"})

	// Verify file exists
	path := filepath.Join(tmpDir, debate.ID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("debate file should exist")
	}

	// Reload
	data, _ := os.ReadFile(path)
	var loaded Debate
	json.Unmarshal(data, &loaded)

	if loaded.Topic != "Persist test" {
		t.Errorf("expected 'Persist test', got %s", loaded.Topic)
	}
	if len(loaded.Arguments) != 1 {
		t.Errorf("expected 1 argument, got %d", len(loaded.Arguments))
	}
}

func TestRebuttalArgument(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	debate, _ := store.Create("Rebuttal test", "", []Debater{
		{ID: "d1", Position: PositionFor},
		{ID: "d2", Position: PositionAgainst},
	}, 3)

	arg1, _ := store.AddArgument(debate.ID, Argument{
		DebaterID: "d1",
		Position:  PositionFor,
		Claim:     "Microservices scale better",
	})

	arg2, err := store.AddArgument(debate.ID, Argument{
		DebaterID:  "d2",
		Position:   PositionAgainst,
		Claim:      "Monoliths are simpler",
		RebuttalTo: arg1.Arguments[0].ID,
	})
	if err != nil {
		t.Fatalf("AddArgument with rebuttal failed: %v", err)
	}

	if arg2.Arguments[1].RebuttalTo != arg1.Arguments[0].ID {
		t.Error("rebuttal should reference first argument")
	}
}
