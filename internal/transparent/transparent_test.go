package transparent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewTracker(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	if tr == nil {
		t.Fatal("expected tracker")
	}
	if !tr.Enabled() {
		t.Error("should be enabled")
	}
}

func TestDisabledTracker(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", false)
	if tr.Enabled() {
		t.Error("should be disabled")
	}
	tr.Record(EventModelSelect, WithModel("gpt-4"))
	// Should not panic or record
	if len(tr.Events()) != 0 {
		t.Error("disabled tracker should not record events")
	}
}

func TestRecordModelSelect(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventModelSelect, WithModel("gpt-4"))

	events := tr.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", events[0].Model)
	}
	if events[0].Type != EventModelSelect {
		t.Errorf("wrong type: %s", events[0].Type)
	}
}

func TestRecordTokenCount(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventTokenCount, WithTokens(100, 50))

	events := tr.Events()
	if events[0].Tokens.Prompt != 100 {
		t.Errorf("expected 100, got %d", events[0].Tokens.Prompt)
	}
	if events[0].Tokens.Total != 150 {
		t.Errorf("expected 150 total, got %d", events[0].Tokens.Total)
	}
}

func TestRecordCost(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventCost, WithCost(0.03, 0.06, "$"))

	events := tr.Events()
	if events[0].Cost.TotalCost != 0.09 {
		t.Errorf("expected 0.09, got %.4f", events[0].Cost.TotalCost)
	}
}

func TestRecordToolCall(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventToolCall, WithTool("read_file", 50*time.Millisecond, true))

	events := tr.Events()
	if events[0].Tool.Name != "read_file" {
		t.Errorf("expected read_file, got %s", events[0].Tool.Name)
	}
	if !events[0].Tool.Success {
		t.Error("tool should be successful")
	}
}

func TestRecordFileAccess(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventFileAccess, WithFile("main.go", "read", 1024))

	events := tr.Events()
	if events[0].File.Path != "main.go" {
		t.Errorf("expected main.go, got %s", events[0].File.Path)
	}
	if events[0].File.Action != "read" {
		t.Errorf("expected read, got %s", events[0].File.Action)
	}
}

func TestRecordNetwork(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventNetwork, WithNetwork("https://api.openai.com/v1/chat", "POST", 200, 500*time.Millisecond))

	events := tr.Events()
	if events[0].Network.StatusCode != 200 {
		t.Errorf("expected 200, got %d", events[0].Network.StatusCode)
	}
}

func TestRecordDecision(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventDecision, WithDecision("Chose gpt-4 over gpt-3.5 for code generation"))

	events := tr.Events()
	if events[0].Decision != "Chose gpt-4 over gpt-3.5 for code generation" {
		t.Error("decision mismatch")
	}
}

func TestRecordError(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventError, WithError("API rate limit exceeded"))

	events := tr.Events()
	if events[0].Error != "API rate limit exceeded" {
		t.Error("error mismatch")
	}
}

func TestRecordWithMetadata(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventModelSelect, WithModel("gpt-4"), WithMetadata("reason", "user requested"))

	events := tr.Events()
	if events[0].Metadata["reason"] != "user requested" {
		t.Error("metadata mismatch")
	}
}

func TestStats(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventModelSelect, WithModel("gpt-4"))
	tr.Record(EventTokenCount, WithTokens(100, 50))
	tr.Record(EventTokenCount, WithTokens(200, 100))
	tr.Record(EventCost, WithCost(0.03, 0.06, "$"))
	tr.Record(EventToolCall, WithTool("read", 10*time.Millisecond, true))
	tr.Record(EventToolCall, WithTool("write", 20*time.Millisecond, false))
	tr.Record(EventFileAccess, WithFile("a.go", "read", 0))
	tr.Record(EventNetwork, WithNetwork("https://api.example.com", "GET", 200, 100*time.Millisecond))
	tr.Record(EventError, WithError("oops"))

	stats := tr.Stats()
	if stats.Model != "gpt-4" {
		t.Errorf("model: %s", stats.Model)
	}
	if stats.TotalTokens.Total != 450 {
		t.Errorf("tokens: %d", stats.TotalTokens.Total)
	}
	if stats.TotalCost.TotalCost != 0.09 {
		t.Errorf("cost: %.4f", stats.TotalCost.TotalCost)
	}
	if stats.ToolCalls != 2 {
		t.Errorf("tools: %d", stats.ToolCalls)
	}
	if stats.FileAccesses != 1 {
		t.Errorf("files: %d", stats.FileAccesses)
	}
	if stats.NetworkReqs != 1 {
		t.Errorf("network: %d", stats.NetworkReqs)
	}
	if stats.Errors != 1 {
		t.Errorf("errors: %d", stats.Errors)
	}
}

func TestEventsByType(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventModelSelect, WithModel("gpt-4"))
	tr.Record(EventTokenCount, WithTokens(100, 50))
	tr.Record(EventTokenCount, WithTokens(200, 100))

	tokenEvents := tr.EventsByType(EventTokenCount)
	if len(tokenEvents) != 2 {
		t.Errorf("expected 2 token events, got %d", len(tokenEvents))
	}

	modelEvents := tr.EventsByType(EventModelSelect)
	if len(modelEvents) != 1 {
		t.Errorf("expected 1 model event, got %d", len(modelEvents))
	}
}

func TestFormatEventModel(t *testing.T) {
	e := &Event{
		Type:      EventModelSelect,
		Timestamp: time.Now(),
		AgentID:   "agent-1",
		Model:     "gpt-4",
	}
	s := FormatEvent(e)
	if !strings.Contains(s, "MODEL") || !strings.Contains(s, "gpt-4") {
		t.Errorf("bad format: %s", s)
	}
}

func TestFormatEventTokens(t *testing.T) {
	e := &Event{
		Type:      EventTokenCount,
		Timestamp: time.Now(),
		Tokens:    TokenUsage{Prompt: 100, Completion: 50, Total: 150},
	}
	s := FormatEvent(e)
	if !strings.Contains(s, "150") {
		t.Errorf("should contain total: %s", s)
	}
}

func TestFormatEventCost(t *testing.T) {
	e := &Event{
		Type:      EventCost,
		Timestamp: time.Now(),
		Cost:      CostInfo{TotalCost: 0.09, InputCost: 0.03, OutputCost: 0.06, Currency: "$"},
	}
	s := FormatEvent(e)
	if !strings.Contains(s, "$0.0900") {
		t.Errorf("should contain cost: %s", s)
	}
}

func TestFormatEventTool(t *testing.T) {
	e := &Event{
		Type:      EventToolCall,
		Timestamp: time.Now(),
		Tool:      ToolInfo{Name: "read_file", Duration: 50 * time.Millisecond, Success: true},
	}
	s := FormatEvent(e)
	if !strings.Contains(s, "OK") {
		t.Errorf("should show OK: %s", s)
	}
}

func TestFormatStats(t *testing.T) {
	s := SessionStats{
		SessionID: "sess-1",
		AgentID:   "agent-1",
		Model:     "gpt-4",
		TotalTokens: TokenUsage{Prompt: 100, Completion: 50, Total: 150},
		TotalCost:   CostInfo{TotalCost: 0.09, Currency: "$"},
		ToolCalls:   5,
		FileAccesses: 3,
		NetworkReqs: 2,
	}
	out := FormatStats(&s)
	if !strings.Contains(out, "gpt-4") {
		t.Error("should show model")
	}
	if !strings.Contains(out, "150") {
		t.Error("should show total tokens")
	}
}

func TestLiveOutput(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTracker("sess-1", "agent-1", true)
	tr.SetWriter(&buf)

	tr.Record(EventModelSelect, WithModel("gpt-4"))

	if buf.Len() == 0 {
		t.Error("should write to writer")
	}
	if !strings.Contains(buf.String(), "gpt-4") {
		t.Errorf("output should contain model: %s", buf.String())
	}
}

func TestExportJSON(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventModelSelect, WithModel("gpt-4"))

	data, err := tr.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}

	var events []Event
	json.Unmarshal(data, &events)
	if len(events) != 1 {
		t.Errorf("expected 1 event in JSON, got %d", len(events))
	}
}

func TestExportStatsJSON(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)
	tr.Record(EventModelSelect, WithModel("gpt-4"))
	tr.Record(EventTokenCount, WithTokens(100, 50))

	data, err := tr.ExportStatsJSON()
	if err != nil {
		t.Fatal(err)
	}

	var stats SessionStats
	json.Unmarshal(data, &stats)
	if stats.Model != "gpt-4" {
		t.Errorf("model: %s", stats.Model)
	}
	if stats.TotalTokens.Total != 150 {
		t.Errorf("tokens: %d", stats.TotalTokens.Total)
	}
}

func TestMultipleEventsConcurrency(t *testing.T) {
	tr := NewTracker("sess-1", "agent-1", true)

	for i := 0; i < 100; i++ {
		tr.Record(EventTokenCount, WithTokens(i, i))
	}

	if len(tr.Events()) != 100 {
		t.Errorf("expected 100 events, got %d", len(tr.Events()))
	}

	stats := tr.Stats()
	if stats.TotalTokens.Total != 9900 {
		t.Errorf("expected 9900 total tokens, got %d", stats.TotalTokens.Total)
	}
}
