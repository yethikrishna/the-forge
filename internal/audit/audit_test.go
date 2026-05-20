package audit_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/audit"
)

func TestLog(t *testing.T) {
	logger := audit.NewLogger("", 100)

	e := logger.Log(audit.ActionAgentStart, "claude", "s1", "", "Agent started")
	if e.ID == "" {
		t.Error("entry should have an ID")
	}
	if e.Action != audit.ActionAgentStart {
		t.Errorf("expected agent.start, got %s", e.Action)
	}
	if !e.Success {
		t.Error("should be successful by default")
	}
}

func TestLogWithOptions(t *testing.T) {
	logger := audit.NewLogger("", 100)

	e := logger.Log(audit.ActionModelCall, "claude", "s1", "sonnet", "Model call",
		audit.WithCost(0.05),
		audit.WithDuration("2s"),
		audit.WithMetadata(map[string]string{"tokens": "1500"}),
	)

	if e.Cost != 0.05 {
		t.Errorf("expected 0.05, got %f", e.Cost)
	}
	if e.Duration != "2s" {
		t.Errorf("expected 2s, got %s", e.Duration)
	}
	if e.Metadata["tokens"] != "1500" {
		t.Error("metadata not set")
	}
}

func TestLogWithError(t *testing.T) {
	logger := audit.NewLogger("", 100)

	e := logger.Log(audit.ActionExec, "claude", "s1", "bash", "Command failed",
		audit.WithError("exit code 1"),
	)

	if e.Success {
		t.Error("should not be successful with error")
	}
	if e.Error != "exit code 1" {
		t.Errorf("expected 'exit code 1', got %s", e.Error)
	}
}

func TestSearch(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "Started")
	logger.Log(audit.ActionModelCall, "claude", "s1", "sonnet", "Called model")
	logger.Log(audit.ActionAgentStart, "reviewer", "s2", "", "Started")
	logger.Log(audit.ActionFileWrite, "claude", "s1", "main.go", "Wrote file")

	// Search by agent
	results := logger.Search(audit.Query{Agent: "claude"})
	if len(results) != 3 {
		t.Errorf("expected 3, got %d", len(results))
	}

	// Search by action
	results = logger.Search(audit.Query{Action: audit.ActionAgentStart})
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}

	// Search by session
	results = logger.Search(audit.Query{Session: "s1"})
	if len(results) != 3 {
		t.Errorf("expected 3, got %d", len(results))
	}

	// Search with limit
	results = logger.Search(audit.Query{Agent: "claude", Limit: 1})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestSearchByTime(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "First")

	from := time.Now().UTC().Add(-time.Hour)
	to := time.Now().UTC().Add(time.Hour)

	results := logger.Search(audit.Query{From: from, To: to})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestSearchBySuccess(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionExec, "claude", "s1", "", "OK")
	logger.Log(audit.ActionExec, "claude", "s1", "", "FAIL", audit.WithError("bad"))

	failed := false
	results := logger.Search(audit.Query{Success: &failed})
	if len(results) != 1 {
		t.Errorf("expected 1 failure, got %d", len(results))
	}
}

func TestCount(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "")
	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "")
	logger.Log(audit.ActionModelCall, "claude", "s1", "", "")

	if logger.Count() != 3 {
		t.Errorf("expected 3, got %d", logger.Count())
	}
}

func TestCountByAction(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "")
	logger.Log(audit.ActionAgentStart, "reviewer", "s2", "", "")
	logger.Log(audit.ActionModelCall, "claude", "s1", "", "")

	counts := logger.CountByAction()
	if counts[audit.ActionAgentStart] != 2 {
		t.Errorf("expected 2, got %d", counts[audit.ActionAgentStart])
	}
	if counts[audit.ActionModelCall] != 1 {
		t.Errorf("expected 1, got %d", counts[audit.ActionModelCall])
	}
}

func TestCountByAgent(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "")
	logger.Log(audit.ActionAgentStart, "reviewer", "s2", "", "")
	logger.Log(audit.ActionModelCall, "claude", "s1", "", "")

	counts := logger.CountByAgent()
	if counts["claude"] != 2 {
		t.Errorf("expected 2, got %d", counts["claude"])
	}
	if counts["reviewer"] != 1 {
		t.Errorf("expected 1, got %d", counts["reviewer"])
	}
}

func TestRecent(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "1")
	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "2")
	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "3")

	results := logger.Recent(2)
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
	// Most recent first
	if results[0].Detail != "3" {
		t.Errorf("expected most recent first, got %s", results[0].Detail)
	}
}

func TestExport(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "test")

	data, err := logger.Export()
	if err != nil {
		t.Fatalf("export error: %v", err)
	}
	if len(data) == 0 {
		t.Error("export should return data")
	}
}

func TestExportCSV(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "test")

	csv := logger.ExportCSV()
	if len(csv) == 0 {
		t.Error("CSV export should return data")
	}
}

func TestClear(t *testing.T) {
	logger := audit.NewLogger("", 100)

	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "")
	if logger.Count() != 1 {
		t.Error("should have 1 entry")
	}

	logger.Clear()
	if logger.Count() != 0 {
		t.Error("should be 0 after clear")
	}
}

func TestRotation(t *testing.T) {
	logger := audit.NewLogger("", 5) // max 5 entries

	for i := 0; i < 10; i++ {
		logger.Log(audit.ActionAgentStart, "claude", "s1", "", fmt.Sprintf("entry-%d", i))
	}

	if logger.Count() != 5 {
		t.Errorf("expected 5 after rotation, got %d", logger.Count())
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.json"

	logger := audit.NewLogger(path, 100)
	logger.Log(audit.ActionAgentStart, "claude", "s1", "", "persisted")

	logger2 := audit.NewLogger(path, 100)
	if logger2.Count() != 1 {
		t.Errorf("expected 1 after reload, got %d", logger2.Count())
	}
}
