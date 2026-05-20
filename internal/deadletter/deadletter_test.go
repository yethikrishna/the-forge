package deadletter

import (
	"strings"
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	entry, err := store.Add(Entry{
		AgentID: "coder-1",
		Task:    "Fix authentication bug",
		Reason:  ReasonTimeout,
		Error:   "request timed out after 60s",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if entry.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if entry.Status != "pending" {
		t.Errorf("expected pending, got %s", entry.Status)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	created, _ := store.Add(Entry{
		AgentID: "test",
		Task:    "test task",
		Reason:  ReasonProviderError,
		Error:   "openai 500",
	})

	found, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.AgentID != "test" {
		t.Errorf("expected test, got %s", found.AgentID)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Add(Entry{AgentID: "a1", Task: "t1", Reason: ReasonTimeout})
	store.Add(Entry{AgentID: "a2", Task: "t2", Reason: ReasonRateLimit})
	store.Add(Entry{AgentID: "a3", Task: "t3", Reason: ReasonCostCap})

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestListByReason(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Add(Entry{AgentID: "a1", Reason: ReasonTimeout})
	store.Add(Entry{AgentID: "a2", Reason: ReasonTimeout})
	store.Add(Entry{AgentID: "a3", Reason: ReasonRateLimit})

	entries, err := store.ListByReason(ReasonTimeout)
	if err != nil {
		t.Fatalf("ListByReason failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 timeout entries, got %d", len(entries))
	}
}

func TestListByStatus(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Add(Entry{AgentID: "a1", Reason: ReasonTimeout})
	store.Add(Entry{AgentID: "a2", Reason: ReasonTimeout, Status: "dismissed"})

	entries, err := store.ListByStatus("pending")
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 pending, got %d", len(entries))
	}
}

func TestRetry(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	created, _ := store.Add(Entry{
		AgentID:    "coder",
		Task:       "fix bug",
		Reason:     ReasonProviderError,
		Error:      "503 service unavailable",
		MaxRetries: 3,
	})

	retried, err := store.Retry(created.ID)
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if retried.RetryCount != 1 {
		t.Errorf("expected retry count 1, got %d", retried.RetryCount)
	}
	if retried.Status != "retried" {
		t.Errorf("expected retried, got %s", retried.Status)
	}
}

func TestRetryExceeded(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	created, _ := store.Add(Entry{
		AgentID:    "coder",
		Task:       "fix bug",
		Reason:     ReasonProviderError,
		MaxRetries: 1,
		RetryCount: 1,
	})

	_, err := store.Retry(created.ID)
	if err == nil {
		t.Error("expected error when max retries exceeded")
	}
}

func TestDismiss(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	created, _ := store.Add(Entry{
		AgentID: "coder",
		Task:    "fix bug",
		Reason:  ReasonCostCap,
	})

	dismissed, err := store.Dismiss(created.ID)
	if err != nil {
		t.Fatalf("Dismiss failed: %v", err)
	}
	if dismissed.Status != "dismissed" {
		t.Errorf("expected dismissed, got %s", dismissed.Status)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	created, _ := store.Add(Entry{AgentID: "a1", Reason: ReasonTimeout})
	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := store.Get(created.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestPurge(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Add expired entry
	expired := time.Now().Add(-1 * time.Hour)
	store.Add(Entry{
		AgentID:   "a1",
		Reason:    ReasonTimeout,
		ExpiresAt: &expired,
	})

	// Add non-expired entry
	future := time.Now().Add(24 * time.Hour)
	store.Add(Entry{
		AgentID:   "a2",
		Reason:    ReasonTimeout,
		ExpiresAt: &future,
	})

	purged, err := store.Purge()
	if err != nil {
		t.Fatalf("Purge failed: %v", err)
	}
	if purged != 1 {
		t.Errorf("expected 1 purged, got %d", purged)
	}

	entries, _ := store.List()
	if len(entries) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(entries))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Add(Entry{AgentID: "a1", Reason: ReasonTimeout})
	store.Add(Entry{AgentID: "a1", Reason: ReasonTimeout})
	store.Add(Entry{AgentID: "a2", Reason: ReasonRateLimit})

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("expected 3 total, got %d", stats.Total)
	}
	if stats.ByReason[ReasonTimeout] != 2 {
		t.Errorf("expected 2 timeout, got %d", stats.ByReason[ReasonTimeout])
	}
	if stats.ByAgent["a1"] != 2 {
		t.Errorf("expected 2 for a1, got %d", stats.ByAgent["a1"])
	}
}

func TestFormatEntry(t *testing.T) {
	entry := &Entry{
		ID:       "dl-test",
		AgentID:  "coder",
		Task:     "Fix auth bug",
		Reason:   ReasonTimeout,
		Error:    "timed out",
		Status:   "pending",
		Provider: "openai",
		Model:    "gpt-4",
		CostUSD:  0.05,
	}

	output := FormatEntry(entry)
	if !strings.Contains(output, "dl-test") {
		t.Error("expected ID in output")
	}
	if !strings.Contains(output, "coder") {
		t.Error("expected agent in output")
	}
	if !strings.Contains(output, "openai") {
		t.Error("expected provider in output")
	}
}

func TestFormatStats(t *testing.T) {
	stats := &Stats{
		Total:    5,
		ByReason: map[Reason]int{ReasonTimeout: 3, ReasonRateLimit: 2},
		ByStatus: map[string]int{"pending": 4, "dismissed": 1},
	}

	output := FormatStats(stats)
	if !strings.Contains(output, "5 entries") {
		t.Error("expected total in output")
	}
	if !strings.Contains(output, "timeout") {
		t.Error("expected reason in output")
	}
}

func TestAllReasons(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	reasons := []Reason{
		ReasonTimeout, ReasonProviderError, ReasonRateLimit,
		ReasonCostCap, ReasonSandboxEscape, ReasonValidation,
		ReasonCircuitOpen, ReasonUnknown,
	}

	for _, r := range reasons {
		_, err := store.Add(Entry{AgentID: "test", Reason: r})
		if err != nil {
			t.Errorf("Add with reason %s failed: %v", r, err)
		}
	}

	entries, _ := store.List()
	if len(entries) != len(reasons) {
		t.Errorf("expected %d entries, got %d", len(reasons), len(entries))
	}
}
