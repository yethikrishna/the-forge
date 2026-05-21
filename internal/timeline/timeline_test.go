package timeline_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/timeline"
)

func TestRecord(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	e := tl.Record("agent-1", timeline.EventAction, "file_write", "main.go")

	if e.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", e.AgentID)
	}
	if e.Type != timeline.EventAction {
		t.Errorf("expected action, got %s", e.Type)
	}
}

func TestRecordMultiple(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	tl.Record("agent-1", timeline.EventStart, "session", "")
	tl.Record("agent-1", timeline.EventAction, "file_write", "main.go")
	tl.Record("agent-1", timeline.EventEnd, "session", "")

	events := tl.Query("", time.Time{}, time.Time{}, "", 0)
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestStartEndSpan(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	s := tl.StartSpan("agent-1", "build")

	if !s.Active {
		t.Error("expected span to be active")
	}

	err := tl.EndSpan(s.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := tl.Spans("", false)
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Active {
		t.Error("expected span to be inactive after end")
	}
}

func TestEndNonExistentSpan(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	err := tl.EndSpan("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent span")
	}
}

func TestQueryByAgent(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	tl.Record("agent-1", timeline.EventAction, "write", "a.go")
	tl.Record("agent-2", timeline.EventAction, "read", "b.go")
	tl.Record("agent-1", timeline.EventAction, "write", "c.go")

	events := tl.Query("agent-1", time.Time{}, time.Time{}, "", 0)
	if len(events) != 2 {
		t.Errorf("expected 2 events for agent-1, got %d", len(events))
	}
}

func TestQueryByType(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	tl.Record("agent-1", timeline.EventAction, "write", "")
	tl.Record("agent-1", timeline.EventError, "fail", "")
	tl.Record("agent-1", timeline.EventAction, "read", "")

	events := tl.Query("", time.Time{}, time.Time{}, timeline.EventError, 0)
	if len(events) != 1 {
		t.Errorf("expected 1 error event, got %d", len(events))
	}
}

func TestQueryWithLimit(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	for i := 0; i < 10; i++ {
		tl.Record("agent-1", timeline.EventAction, "action", "")
	}

	events := tl.Query("", time.Time{}, time.Time{}, "", 5)
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestSpansActiveOnly(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	s1 := tl.StartSpan("agent-1", "build")
	tl.StartSpan("agent-2", "test")
	tl.EndSpan(s1.ID)

	active := tl.Spans("", true)
	if len(active) != 1 {
		t.Errorf("expected 1 active span, got %d", len(active))
	}
}

func TestSummary(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	tl.Record("agent-1", timeline.EventAction, "write", "")
	tl.Record("agent-1", timeline.EventAction, "read", "")

	summary := tl.Summary("hour")
	if len(summary) == 0 {
		t.Error("expected non-empty summary")
	}
}

func TestStats(t *testing.T) {
	tl := timeline.NewTimeline(t.TempDir())
	tl.Record("agent-1", timeline.EventAction, "write", "")
	tl.Record("agent-2", timeline.EventError, "fail", "")

	stats := tl.Stats()
	if stats["events"].(int) != 2 {
		t.Errorf("expected 2 events, got %v", stats["events"])
	}
}

func TestRenderASCII(t *testing.T) {
	events := []timeline.Event{
		{AgentID: "a1", Type: timeline.EventStart, Name: "start", Timestamp: time.Now()},
		{AgentID: "a1", Type: timeline.EventAction, Name: "build", Timestamp: time.Now().Add(time.Second)},
		{AgentID: "a1", Type: timeline.EventEnd, Name: "end", Timestamp: time.Now().Add(2 * time.Second)},
	}

	text := timeline.RenderASCII(events, 40)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestRenderSpan(t *testing.T) {
	s := &timeline.Span{
		Name:    "build",
		AgentID: "agent-1",
		Start:   time.Now(),
		Active:  true,
	}
	text := timeline.RenderSpan(s)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestRenderASCIIEmpty(t *testing.T) {
	text := timeline.RenderASCII(nil, 40)
	if text == "" {
		t.Error("expected non-empty render for empty events")
	}
}
