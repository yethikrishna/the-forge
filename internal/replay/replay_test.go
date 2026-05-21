package replay_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/forge/sword/internal/replay"
)

func TestRecorder(t *testing.T) {
	rec := replay.NewRecorder("test-session", "agent-1", "")

	sess := rec.Session()
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if sess.Status != "recording" {
		t.Errorf("expected recording status, got %s", sess.Status)
	}
}

func TestRecordPrompt(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")

	event := rec.RecordPrompt("agent-1", "gpt-4.1", "Hello world", 10)
	if event.Type != replay.EventPrompt {
		t.Errorf("expected prompt type, got %s", event.Type)
	}
	if event.Sequence != 1 {
		t.Errorf("expected seq 1, got %d", event.Sequence)
	}
}

func TestRecordResponse(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")

	event := rec.RecordResponse("agent-1", "gpt-4.1", "Hello!", 20, 0.003, 500*time.Millisecond)
	if event.Type != replay.EventResponse {
		t.Errorf("expected response type, got %s", event.Type)
	}
	if event.CostUSD != 0.003 {
		t.Errorf("expected cost 0.003, got %f", event.CostUSD)
	}
}

func TestRecordToolCall(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")

	prompt := rec.RecordPrompt("agent-1", "gpt-4.1", "Run tests", 5)
	toolCall := rec.RecordToolCall("bash", map[string]interface{}{"cmd": "go test ./..."}, prompt.ID)
	toolResult := rec.RecordToolResult("bash", "PASS", 2*time.Second)

	if toolCall.Type != replay.EventToolCall {
		t.Errorf("expected tool_call type, got %s", toolCall.Type)
	}
	if toolCall.ParentID != prompt.ID {
		t.Errorf("expected parent ID %s, got %s", prompt.ID, toolCall.ParentID)
	}
	if toolResult.Type != replay.EventToolResult {
		t.Errorf("expected tool_result type, got %s", toolResult.Type)
	}
}

func TestRecordError(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")

	event := rec.RecordError(fmt.Errorf("connection refused"), true)
	if event.Type != replay.EventError {
		t.Errorf("expected error type, got %s", event.Type)
	}
}

func TestCheckpoint(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")

	rec.RecordPrompt("agent-1", "gpt-4.1", "Step 1", 5)
	rec.RecordPrompt("agent-1", "gpt-4.1", "Step 2", 5)

	cp := rec.Checkpoint("before-step3", "About to try the risky operation")
	if cp.Name != "before-step3" {
		t.Errorf("expected name before-step3, got %s", cp.Name)
	}
	if cp.Sequence != 2 {
		t.Errorf("expected seq 2, got %d", cp.Sequence)
	}
}

func TestStop(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")

	rec.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	err := rec.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess := rec.Session()
	if sess.Status != "completed" {
		t.Errorf("expected completed status, got %s", sess.Status)
	}
	if sess.EndedAt.IsZero() {
		t.Error("expected non-zero ended_at")
	}
}

func TestBranch(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")

	rec.RecordPrompt("agent-1", "gpt-4.1", "Step 1", 5)
	rec.RecordPrompt("agent-1", "gpt-4.1", "Step 2", 5)

	branch := rec.Branch(2)

	branchSess := branch.Session()
	if branchSess.BranchFrom == "" {
		t.Error("expected branch_from to be set")
	}
	if branchSess.BranchAt != 2 {
		t.Errorf("expected branch_at 2, got %d", branchSess.BranchAt)
	}
	if len(branchSess.Events) != 2 {
		t.Errorf("expected 2 events in branch, got %d", len(branchSess.Events))
	}

	// Add different events to branch
	branch.RecordPrompt("agent-1", "claude-sonnet-4", "Step 3 (alt)", 5)
	if len(branch.Session().Events) != 3 {
		t.Errorf("expected 3 events after adding to branch, got %d", len(branch.Session().Events))
	}
}

func TestPlayer(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")
	rec.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	rec.RecordResponse("agent-1", "gpt-4.1", "Hi!", 10, 0.001, 100*time.Millisecond)
	rec.RecordPrompt("agent-1", "gpt-4.1", "How are you?", 8)

	player := replay.NewPlayer(rec.Session())

	// Step through
	e1 := player.Next()
	if e1 == nil || e1.Type != replay.EventPrompt {
		t.Error("expected first event to be prompt")
	}

	e2 := player.Next()
	if e2 == nil || e2.Type != replay.EventResponse {
		t.Error("expected second event to be response")
	}

	// Go back
	e1b := player.Prev()
	if e1b == nil || e1b.Sequence != 1 {
		t.Error("expected to go back to first event")
	}

	// Position
	pos, total := player.Position()
	if total != 3 {
		t.Errorf("expected 3 total events, got %d", total)
	}
	_ = pos
}

func TestPlayerSeekToType(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")
	rec.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	rec.RecordResponse("agent-1", "gpt-4.1", "Hi!", 10, 0.001, 100*time.Millisecond)
	rec.RecordToolCall("bash", map[string]interface{}{"cmd": "ls"}, "")
	rec.RecordPrompt("agent-1", "gpt-4.1", "Next", 5)

	player := replay.NewPlayer(rec.Session())

	// Find first tool call
	e := player.JumpToType(replay.EventToolCall)
	if e == nil {
		t.Error("expected to find a tool call")
	}
	if e.Type != replay.EventToolCall {
		t.Errorf("expected tool_call, got %s", e.Type)
	}
}

func TestPlayerFilter(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")
	rec.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	rec.RecordResponse("agent-1", "gpt-4.1", "Hi!", 10, 0.001, 100*time.Millisecond)
	rec.RecordError(fmt.Errorf("timeout"), false)
	rec.RecordPrompt("agent-1", "gpt-4.1", "Retry", 5)

	player := replay.NewPlayer(rec.Session())

	errors := player.Filter(func(e replay.Event) bool {
		return e.Type == replay.EventError
	})
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestPlayerSummary(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")
	rec.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	rec.RecordResponse("agent-1", "gpt-4.1", "Hi!", 10, 0.001, 100*time.Millisecond)

	player := replay.NewPlayer(rec.Session())
	summary := player.Summary()

	if summary.TotalEvents != 2 {
		t.Errorf("expected 2 events, got %d", summary.TotalEvents)
	}
	if summary.ByType["prompt"] != 1 {
		t.Errorf("expected 1 prompt, got %d", summary.ByType["prompt"])
	}
	if summary.ByType["response"] != 1 {
		t.Errorf("expected 1 response, got %d", summary.ByType["response"])
	}
}

func TestStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store := replay.NewStore(dir)

	rec := replay.NewRecorder("test", "agent-1", dir)
	rec.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	rec.Stop()

	err := store.Save(rec.Session())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a new store to test persistence
	store2 := replay.NewStore(dir)
	got, err := store2.Get(rec.Session().ID)
	if err != nil {
		t.Fatalf("unexpected error loading session: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("expected name 'test', got %s", got.Name)
	}
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	store := replay.NewStore(dir)

	rec1 := replay.NewRecorder("test1", "agent-1", dir)
	rec1.Stop()
	store.Save(rec1.Session())

	time.Sleep(2 * time.Millisecond)

	rec2 := replay.NewRecorder("test2", "agent-2", dir)
	rec2.Stop()
	store.Save(rec2.Session())

	sessions := store.List()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestStoreCompare(t *testing.T) {
	dir := t.TempDir()
	store := replay.NewStore(dir)

	rec1 := replay.NewRecorder("baseline", "agent-1", "")
	rec1.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	rec1.RecordResponse("agent-1", "gpt-4.1", "Hi!", 10, 0.001, 100*time.Millisecond)
	rec1.Stop()
	store.Save(rec1.Session())

	rec2 := replay.NewRecorder("optimized", "agent-2", "")
	rec2.RecordPrompt("agent-2", "claude-sonnet-4", "Hello", 5)
	rec2.RecordResponse("agent-2", "claude-sonnet-4", "Hi there!", 15, 0.002, 150*time.Millisecond)
	rec2.Stop()
	store.Save(rec2.Session())

	cmp, err := store.Compare(rec1.Session().ID, rec2.Session().ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmp.EventsA != 2 || cmp.EventsB != 2 {
		t.Errorf("expected 2 events each, got %d and %d", cmp.EventsA, cmp.EventsB)
	}
}

func TestEventTypeString(t *testing.T) {
	types := map[replay.EventType]string{
		replay.EventPrompt:     "prompt",
		replay.EventResponse:   "response",
		replay.EventToolCall:   "tool_call",
		replay.EventToolResult: "tool_result",
		replay.EventError:      "error",
		replay.EventRetry:      "retry",
		replay.EventCancel:     "cancel",
		replay.EventTimeout:    "timeout",
		replay.EventCacheHit:   "cache_hit",
		replay.EventCacheMiss:  "cache_miss",
	}

	for et, expected := range types {
		if et.String() != expected {
			t.Errorf("expected %s, got %s", expected, et.String())
		}
	}
}

func TestPlayerRewind(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")
	rec.RecordPrompt("agent-1", "gpt-4.1", "Hello", 5)
	rec.RecordResponse("agent-1", "gpt-4.1", "Hi!", 10, 0.001, 100*time.Millisecond)

	player := replay.NewPlayer(rec.Session())
	player.Next()
	player.Next()
	player.Rewind()

	pos, _ := player.Position()
	if pos != -1 {
		t.Errorf("expected pos -1 after rewind, got %d", pos)
	}
}

func TestEventsBetween(t *testing.T) {
	rec := replay.NewRecorder("test", "agent-1", "")
	rec.RecordPrompt("agent-1", "gpt-4.1", "Step 1", 5)
	rec.RecordResponse("agent-1", "gpt-4.1", "OK", 10, 0.001, 100*time.Millisecond)
	rec.RecordPrompt("agent-1", "gpt-4.1", "Step 2", 5)
	rec.RecordResponse("agent-1", "gpt-4.1", "Done", 10, 0.001, 100*time.Millisecond)

	player := replay.NewPlayer(rec.Session())
	events := player.EventsBetween(2, 3)
	if len(events) != 2 {
		t.Errorf("expected 2 events between seq 2-3, got %d", len(events))
	}
}
