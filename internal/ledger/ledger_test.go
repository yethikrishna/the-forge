package ledger

import (
	"testing"
	"time"
)

func TestNewLedger(t *testing.T) {
	l := NewLedger(t.TempDir())
	if l == nil {
		t.Fatal("NewLedger should return a ledger")
	}
}

func TestRecordUsage(t *testing.T) {
	l := NewLedger(t.TempDir())
	err := l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	if err != nil {
		t.Fatalf("RecordUsage error: %v", err)
	}
	if l.TotalCost() != 0.05 {
		t.Errorf("TotalCost = %.4f, want 0.05", l.TotalCost())
	}
	if l.AgentCost("agent-1") != 0.05 {
		t.Errorf("AgentCost = %.4f, want 0.05", l.AgentCost("agent-1"))
	}
	if l.ModelCost("gpt-4") != 0.05 {
		t.Errorf("ModelCost = %.4f, want 0.05", l.ModelCost("gpt-4"))
	}
}

func TestRecordMultipleUsage(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	l.RecordUsage("agent-2", "sess-1", "claude", 2000, 1000, 0.10)
	l.RecordUsage("agent-1", "sess-2", "gpt-4", 500, 200, 0.03)

	if l.TotalCost() < 0.179 || l.TotalCost() > 0.181 {
		t.Errorf("TotalCost = %.4f, want 0.18", l.TotalCost())
	}
	if l.AgentCost("agent-1") != 0.08 {
		t.Errorf("AgentCost(agent-1) = %.4f, want 0.08", l.AgentCost("agent-1"))
	}
}

func TestRecordAction(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordAction("agent-1", "sess-1", "file_read", "main.go", 100*time.Millisecond)

	entries := l.EntriesByType(EntryAction)
	if len(entries) != 1 {
		t.Errorf("Action entries = %d, want 1", len(entries))
	}
	if entries[0].Action != "file_read" {
		t.Errorf("Action = %q, want %q", entries[0].Action, "file_read")
	}
}

func TestRecordRefund(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.10)
	l.RecordRefund("agent-1", "sess-1", 0.05, "overcharge")

	if l.TotalCost() != 0.05 {
		t.Errorf("TotalCost after refund = %.4f, want 0.05", l.TotalCost())
	}
	if l.AgentCost("agent-1") != 0.05 {
		t.Errorf("AgentCost after refund = %.4f, want 0.05", l.AgentCost("agent-1"))
	}
}

func TestBudget(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.SetBudget(0.10)

	err := l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	if err != nil {
		t.Fatalf("First usage should succeed: %v", err)
	}

	err = l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.10)
	if err == nil {
		t.Error("Should exceed budget")
	}

	remaining := l.BudgetRemaining()
	if remaining < 0 || remaining > 0.05 {
		t.Errorf("BudgetRemaining = %.4f, want ~0.05", remaining)
	}
}

func TestNoBudget(t *testing.T) {
	l := NewLedger(t.TempDir())
	if l.BudgetRemaining() != -1 {
		t.Error("No budget set should return -1")
	}
}

func TestEntries(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 100, 50, 0.01)
	l.RecordAction("agent-1", "sess-1", "file_read", "main.go", 50*time.Millisecond)

	entries := l.Entries()
	if len(entries) != 2 {
		t.Errorf("Entries = %d, want 2", len(entries))
	}
}

func TestEntriesByAgent(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 100, 50, 0.01)
	l.RecordUsage("agent-2", "sess-1", "gpt-4", 200, 100, 0.02)
	l.RecordUsage("agent-1", "sess-2", "gpt-4", 50, 25, 0.005)

	entries := l.EntriesByAgent("agent-1")
	if len(entries) != 2 {
		t.Errorf("Agent-1 entries = %d, want 2", len(entries))
	}
}

func TestEntriesByType(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 100, 50, 0.01)
	l.RecordAction("agent-1", "sess-1", "file_read", "main.go", 50*time.Millisecond)
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 200, 100, 0.02)

	tokenEntries := l.EntriesByType(EntryTokenUsage)
	if len(tokenEntries) != 2 {
		t.Errorf("Token entries = %d, want 2", len(tokenEntries))
	}
}

func TestVerifyIntegrity(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	l.RecordAction("agent-1", "sess-1", "file_read", "main.go", 50*time.Millisecond)
	l.RecordUsage("agent-2", "sess-1", "claude", 2000, 1000, 0.10)

	if err := l.Verify(); err != nil {
		t.Errorf("Verify error: %v", err)
	}
}

func TestStats(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	l.RecordUsage("agent-2", "sess-1", "claude", 2000, 1000, 0.10)

	stats := l.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("TotalEntries = %d, want 2", stats.TotalEntries)
	}
	if stats.TotalCost < 0.149 || stats.TotalCost > 0.151 {
		t.Errorf("TotalCost = %.4f, want 0.15", stats.TotalCost)
	}
	if stats.TotalTokensIn != 3000 {
		t.Errorf("TotalTokensIn = %d, want 3000", stats.TotalTokensIn)
	}
	if stats.AgentCount != 2 {
		t.Errorf("AgentCount = %d, want 2", stats.AgentCount)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	l := NewLedger(dir)
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	l.RecordAction("agent-1", "sess-1", "file_read", "main.go", 50*time.Millisecond)

	if err := l.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	l2 := NewLedger(dir)
	if err := l2.Load(); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if l2.TotalCost() != l.TotalCost() {
		t.Errorf("Loaded TotalCost = %.4f, want %.4f", l2.TotalCost(), l.TotalCost())
	}
	if len(l2.Entries()) != len(l.Entries()) {
		t.Errorf("Loaded entries = %d, want %d", len(l2.Entries()), len(l.Entries()))
	}

	if err := l2.Verify(); err != nil {
		t.Errorf("Verify after load error: %v", err)
	}
}

func TestExportMarkdown(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	l.RecordUsage("agent-2", "sess-1", "claude", 2000, 1000, 0.10)

	md := l.ExportMarkdown()
	if md == "" {
		t.Error("ExportMarkdown should not be empty")
	}
}

func TestHashChain(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 500, 250, 0.03)

	entries := l.Entries()
	if entries[0].Hash == "" {
		t.Error("First entry should have a hash")
	}
	if entries[1].PrevHash != entries[0].Hash {
		t.Error("Second entry should chain to first")
	}
}

func TestRunningTotal(t *testing.T) {
	l := NewLedger(t.TempDir())
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 1000, 500, 0.05)
	l.RecordUsage("agent-1", "sess-1", "gpt-4", 500, 250, 0.03)

	entries := l.Entries()
	if entries[0].TotalCost != 0.05 {
		t.Errorf("First TotalCost = %.4f, want 0.05", entries[0].TotalCost)
	}
	if entries[1].TotalCost != 0.08 {
		t.Errorf("Second TotalCost = %.4f, want 0.08", entries[1].TotalCost)
	}
}
