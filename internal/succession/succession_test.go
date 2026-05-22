package succession

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "succession-test")
	os.MkdirAll(dir, 0755)
	return dir
}

func TestDistillationEngine(t *testing.T) {
	de := NewDistillationEngine()
	records := []TaskRecord{
		{ID: "t1", Type: "bug_fix", Outcome: "success", ToolCalls: []ToolCall{{Tool: "git", Success: true}, {Tool: "editor", Success: true}}, Duration: 120, Timestamp: time.Now()},
		{ID: "t2", Type: "bug_fix", Outcome: "success", ToolCalls: []ToolCall{{Tool: "git", Success: true}, {Tool: "editor", Success: true}}, Duration: 90, Timestamp: time.Now()},
		{ID: "t3", Type: "bug_fix", Outcome: "failure", Error: "test timeout", ToolCalls: []ToolCall{{Tool: "runner", Success: false}}, Duration: 300, Timestamp: time.Now()},
	}

	knowledge := de.Distill(records)
	if len(knowledge) == 0 {
		t.Fatal("expected extracted knowledge from task records")
	}

	found := false
	for _, k := range knowledge {
		if k.Category == CatDecisionMaking {
			found = true
			if k.Confidence <= 0 {
				t.Error("confidence should be positive for patterns")
			}
		}
	}
	if !found {
		t.Error("expected decision-making knowledge from successful patterns")
	}

	// Check failure lessons
	failureFound := false
	for _, k := range knowledge {
		if k.Category == CatFailureModes {
			failureFound = true
			if len(k.Failures) == 0 {
				t.Error("failure knowledge should have failure records")
			}
		}
	}
	if !failureFound {
		t.Error("expected failure mode knowledge from failed tasks")
	}
}

func TestBuildCapsule(t *testing.T) {
	knowledge := []*DistilledKnowledge{
		{ID: "dk1", Category: CatToolUsage, Title: "Test", Confidence: 0.8},
	}
	records := []TaskRecord{
		{ID: "t1", Type: "build", Outcome: "success", Duration: 100, Timestamp: time.Now()},
		{ID: "t2", Type: "build", Outcome: "failure", Duration: 200, Timestamp: time.Now()},
	}

	capsule := BuildCapsule("agent-1", "engineering", "senior", 24*time.Hour, knowledge, records)
	if capsule.AgentID != "agent-1" {
		t.Error("wrong agent ID")
	}
	if capsule.Version != "1.0" {
		t.Error("wrong version")
	}
	if len(capsule.KnowledgeUnits) != 1 {
		t.Error("should have 1 knowledge unit")
	}
	if capsule.Statistics.SuccessRate != 50.0 {
		t.Errorf("expected 50%% success rate, got %.1f", capsule.Statistics.SuccessRate)
	}
	if capsule.Hash == "" {
		t.Error("capsule should have integrity hash")
	}
}

func TestInstitutionalBank(t *testing.T) {
	dir := tempDir(t)
	bank := NewInstitutionalBank(dir)

	capsule := &KnowledgeCapsule{
		ID:       "cap-1",
		Version:  "1.0",
		AgentID:  "agent-1",
		Division: "engineering",
		Role:     "senior",
		KnowledgeUnits: []DistilledKnowledge{
			{ID: "dk1", Category: CatToolUsage, Title: "Use git for commits", Confidence: 0.9},
		},
		Statistics: CapsuleStats{SuccessRate: 95, KnowledgeUnitsCount: 1},
		CreatedAt: time.Now(),
	}

	// Build a valid hash
	data, _ := json.Marshal(capsule)
	capsule.Hash = fmt.Sprintf("%x", sha256.Sum256(data))

	err := bank.Deposit(capsule)
	if err != nil {
		t.Fatalf("deposit failed: %v", err)
	}

	// Retrieve
	retrieved, ok := bank.Withdraw("cap-1")
	if !ok {
		t.Fatal("capsule not found")
	}
	if retrieved.AgentID != "agent-1" {
		t.Error("wrong agent")
	}

	// Search
	results := bank.SearchKnowledge("git", 10)
	if len(results) == 0 {
		t.Error("should find knowledge about git")
	}

	// Stats
	stats := bank.GenerationalStats()
	if stats["total_capsules"] != 1 {
		t.Errorf("expected 1 capsule, got %d", stats["total_capsules"])
	}
}

func TestContinuityVerifier(t *testing.T) {
	records := []TaskRecord{
		{ID: "t1", Type: "bug_fix", Duration: 30, Timestamp: time.Now()},
		{ID: "t2", Type: "feature", Duration: 120, Timestamp: time.Now()},
		{ID: "t3", Type: "deploy", Duration: 60, Timestamp: time.Now()},
		{ID: "t4", Type: "test", Duration: 10, Timestamp: time.Now()},
	}

	cv := NewContinuityVerifier(records)
	result := cv.TestTask("bug_fix", func(tr TaskRecord) (bool, float64, []string) {
		return true, 85.0, nil
	})

	if !result.Passed {
		t.Error("should pass")
	}
	if result.Score != 85.0 {
		t.Errorf("expected score 85, got %.1f", result.Score)
	}

	score := cv.ContinuityScore()
	if score <= 0 {
		t.Error("continuity score should be positive")
	}
}

func TestSuccessionManager(t *testing.T) {
	dir := tempDir(t)
	sm := NewSuccessionManager(dir)

	session, err := sm.InitiateSuccession("senior-1", "junior-1")
	if err != nil {
		t.Fatalf("initiate failed: %v", err)
	}

	if session.Phase != PhaseExtract {
		t.Errorf("expected extract phase, got %s", session.Phase)
	}

	records := []TaskRecord{
		{ID: "t1", Type: "bug_fix", Outcome: "success", ToolCalls: []ToolCall{{Tool: "git", Success: true}}, Duration: 60, Timestamp: time.Now()},
	}

	capsule, err := sm.ExtractKnowledge(session.ID, records)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if capsule.AgentID != "senior-1" {
		t.Error("wrong agent in capsule")
	}

	score, err := sm.VerifyContinuity(session.ID, records, func(tr TaskRecord) (bool, float64, []string) {
		return true, 80.0, nil
	})
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	if score < 0.7 {
		t.Errorf("expected score >= 0.7, got %.2f", score)
	}
}

func TestIntegrityCheck(t *testing.T) {
	bank := NewInstitutionalBank(tempDir(t))

	// Create a capsule with a valid hash
	capsule := &KnowledgeCapsule{
		ID: "cap-bad", Version: "1.0", AgentID: "agent-1",
		KnowledgeUnits: []DistilledKnowledge{{ID: "dk1", Title: "test"}},
		Statistics:     CapsuleStats{KnowledgeUnitsCount: 1},
		CreatedAt:      time.Now(),
	}
	data, _ := json.Marshal(capsule)
	capsule.Hash = fmt.Sprintf("%x", sha256.Sum256(data))

	err := bank.Deposit(capsule)
	if err != nil {
		t.Fatalf("deposit should succeed with valid hash: %v", err)
	}

	// Tampered capsule should fail
	tampered := &KnowledgeCapsule{
		ID: "cap-tampered", Version: "1.0", AgentID: "agent-1",
		KnowledgeUnits: []DistilledKnowledge{{ID: "dk1", Title: "tampered content"}},
		Hash: "0000000000000000", // invalid hash
	}
	err = bank.Deposit(tampered)
	if err == nil {
		t.Error("deposit should fail with invalid hash")
	}
}
