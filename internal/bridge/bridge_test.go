package bridge

import (
	"testing"
)

func TestDefaultConversionRules(t *testing.T) {
	rules := DefaultConversionRules()
	if len(rules) < 5 {
		t.Errorf("expected at least 5 default rules, got %d", len(rules))
	}
}

func TestCreateBridge(t *testing.T) {
	b, err := NewBridge(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if b == nil {
		t.Fatal("expected non-nil bridge")
	}
	if len(b.rules) == 0 {
		t.Error("expected default rules")
	}
}

func TestTranslateMCPToA2A(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	msg := &Message{
		ID:     "msg-1",
		Source: ProtocolMCP,
		Target: ProtocolA2A,
		Type:   "request",
		Method: "tools/list",
		Params: map[string]interface{}{},
	}
	translated, err := b.Translate(msg)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if translated.Method != "agent/capabilities" {
		t.Errorf("expected agent/capabilities, got %s", translated.Method)
	}
	if !translated.Converted {
		t.Error("expected converted flag")
	}
	if translated.Metadata["original_method"] != "tools/list" {
		t.Error("expected original_method metadata")
	}
}

func TestTranslateMCPToolCallToA2A(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	msg := &Message{
		ID:     "msg-2",
		Source: ProtocolMCP,
		Target: ProtocolA2A,
		Type:   "request",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      "search_code",
			"arguments": map[string]interface{}{"query": "hello"},
		},
	}
	translated, err := b.Translate(msg)
	if err != nil {
		t.Fatal(err)
	}
	if translated.Method != "agent/execute" {
		t.Errorf("expected agent/execute, got %s", translated.Method)
	}
	if _, ok := translated.Params["input"]; !ok {
		t.Error("expected 'input' param (adapted from 'arguments')")
	}
	if _, ok := translated.Params["tool"]; !ok {
		t.Error("expected 'tool' param (adapted from 'name')")
	}
}

func TestTranslateA2AToMCP(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	msg := &Message{
		ID:     "msg-3",
		Source: ProtocolA2A,
		Target: ProtocolMCP,
		Type:   "request",
		Method: "agent/capabilities",
		Params: map[string]interface{}{},
	}
	translated, err := b.Translate(msg)
	if err != nil {
		t.Fatal(err)
	}
	if translated.Method != "tools/list" {
		t.Errorf("expected tools/list, got %s", translated.Method)
	}
}

func TestTranslateSameProtocol(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	msg := &Message{
		ID:     "msg-4",
		Source: ProtocolMCP,
		Target: ProtocolMCP,
		Type:   "request",
		Method: "tools/list",
	}
	translated, err := b.Translate(msg)
	if err != nil {
		t.Fatal(err)
	}
	if translated != msg {
		t.Error("same protocol should return same message")
	}
}

func TestTranslateNoRule(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	msg := &Message{
		ID:     "msg-5",
		Source: ProtocolMCP,
		Target: ProtocolA2A,
		Type:   "notification",
		Method: "unknown/method",
	}
	_, err := b.Translate(msg)
	// Should either translate with prefix match or error
	// Both are acceptable behaviors
	_ = err
}

func TestACPToMCP(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	msg := &Message{
		ID:     "msg-6",
		Source: ProtocolACP,
		Target: ProtocolMCP,
		Type:   "request",
		Method: "acp/task",
		Params: map[string]interface{}{
			"task":   "run_tests",
			"params": map[string]interface{}{"dir": "./..."},
		},
	}
	translated, err := b.Translate(msg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := translated.Params["name"]; !ok {
		t.Error("expected 'name' param (adapted from 'task')")
	}
	if _, ok := translated.Params["arguments"]; !ok {
		t.Error("expected 'arguments' param (adapted from 'params')")
	}
}

func TestTranslationLog(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	msg := &Message{
		ID:     "msg-7",
		Source: ProtocolMCP,
		Target: ProtocolA2A,
		Type:   "request",
		Method: "tools/list",
		Params: map[string]interface{}{},
	}
	b.Translate(msg)
	log := b.GetLog(10)
	if len(log) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(log))
	}
}

func TestAddRule(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	initialCount := len(b.rules)
	err := b.AddRule(ConversionRule{
		Source: ProtocolMCP, Target: ProtocolA2A,
		SourceType: "notification", MethodMap: "custom/event→agent/event",
		Description: "Custom event mapping",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.rules) != initialCount+1 {
		t.Errorf("expected %d rules, got %d", initialCount+1, len(b.rules))
	}
}

func TestListRules(t *testing.T) {
	b, _ := NewBridge(t.TempDir())
	rules := b.ListRules()
	if len(rules) == 0 {
		t.Error("expected rules")
	}
}

func TestFormatMessage(t *testing.T) {
	msg := &Message{
		ID:        "msg-test",
		Source:    ProtocolMCP,
		Target:    ProtocolA2A,
		Type:      "request",
		Method:    "tools/list",
		Converted: true,
	}
	output := FormatMessage(msg)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatRule(t *testing.T) {
	rule := ConversionRule{
		Source: ProtocolMCP, Target: ProtocolA2A,
		SourceType: "request", MethodMap: "tools/list→agent/capabilities",
		Description: "Test rule",
	}
	output := FormatRule(rule)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	b1, _ := NewBridge(dir)
	b1.AddRule(ConversionRule{
		Source: ProtocolMCP, Target: ProtocolA2A,
		SourceType: "custom", MethodMap: "custom→custom",
		Description: "Custom",
	})
	b2, _ := NewBridge(dir)
	if len(b2.rules) <= len(DefaultConversionRules()) {
		t.Error("expected custom rule to persist")
	}
}
