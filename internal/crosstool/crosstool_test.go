package crosstool

import (
	"context"
	"testing"
	"time"
)

func TestRegisterCursor(t *testing.T) {
	dir := t.TempDir()
	cb, err := NewCrossBridge(dir)
	if err != nil {
		t.Fatalf("NewCrossBridge: %v", err)
	}

	info, err := cb.Register(ToolCursor, CursorConfig{
		Endpoint:  "http://localhost:9999",
		Workspace: "/tmp/project",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if info.Name != "Cursor" {
		t.Errorf("Name = %q, want Cursor", info.Name)
	}
	if !info.Connected {
		t.Error("should be connected")
	}
	if len(info.Capabilities) == 0 {
		t.Error("should have capabilities")
	}
}

func TestRegisterCopilot(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	info, err := cb.Register(ToolCopilot, CopilotConfig{
		Token: "ghp_test",
		Repo:  "org/repo",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if info.Name != "GitHub Copilot" {
		t.Errorf("Name = %q, want GitHub Copilot", info.Name)
	}
}

func TestRegisterClaude(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	info, _ := cb.Register(ToolClaude, ClaudeConfig{
		APIKey: "sk-ant-test",
		Model:  "claude-sonnet-4",
	})
	if info.Name != "Claude Code" {
		t.Errorf("Name = %q, want Claude Code", info.Name)
	}
}

func TestUnregister(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	cb.Register(ToolCursor, CursorConfig{})

	if err := cb.Unregister(ToolCursor); err != nil {
		t.Fatalf("Unregister: %v", err)
	}

	_, ok := cb.Get(ToolCursor)
	if ok {
		t.Error("should not find unregistered tool")
	}

	if err := cb.Unregister(ToolCursor); err == nil {
		t.Error("unregistering non-existent should error")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	cb.Register(ToolCursor, CursorConfig{})
	cb.Register(ToolCopilot, CopilotConfig{})
	cb.Register(ToolClaude, ClaudeConfig{})

	list := cb.List()
	if len(list) != 3 {
		t.Errorf("List len = %d, want 3", len(list))
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	cb.Register(ToolCursor, CursorConfig{})

	info, ok := cb.Get(ToolCursor)
	if !ok {
		t.Fatal("Get should find registered tool")
	}
	if info.Type != ToolCursor {
		t.Errorf("Type = %q, want cursor", info.Type)
	}

	_, ok = cb.Get(ToolCopilot)
	if ok {
		t.Error("Get should not find unregistered tool")
	}
}

func TestSendToUnregistered(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	_, err := cb.SendTo(context.Background(), ToolCursor, "test", nil)
	if err == nil {
		t.Error("sending to unregistered tool should error")
	}
}

func TestSendToClaude(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	cb.Register(ToolClaude, ClaudeConfig{APIKey: "test"})

	msg, err := cb.SendTo(context.Background(), ToolClaude, "agent", map[string]interface{}{
		"prompt": "hello",
	})
	if err != nil {
		t.Fatalf("SendTo: %v", err)
	}
	if msg.Error != "" {
		t.Errorf("Error = %q, want empty", msg.Error)
	}
	if msg.Result == nil {
		t.Error("Result should not be nil")
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	cb.Register(ToolClaude, ClaudeConfig{})
	cb.Register(ToolCursor, CursorConfig{})

	cb.SendTo(context.Background(), ToolClaude, "test1", nil)
	cb.SendTo(context.Background(), ToolCursor, "test2", nil)
	cb.SendTo(context.Background(), ToolClaude, "test3", nil)

	// All history
	all := cb.History("", 0)
	if len(all) != 3 {
		t.Errorf("all history = %d, want 3", len(all))
	}

	// Filtered to Claude
	claudeHist := cb.History(ToolClaude, 0)
	if len(claudeHist) != 2 {
		t.Errorf("claude history = %d, want 2", len(claudeHist))
	}

	// Limited
	limited := cb.History("", 1)
	if len(limited) != 1 {
		t.Errorf("limited = %d, want 1", len(limited))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)

	cb.Register(ToolClaude, ClaudeConfig{})
	cb.SendTo(context.Background(), ToolClaude, "test", nil)
	cb.SendTo(context.Background(), ToolClaude, "test2", nil)

	stats := cb.Stats()
	if stats.RegisteredTools != 1 {
		t.Errorf("RegisteredTools = %d, want 1", stats.RegisteredTools)
	}
	if stats.TotalMessages != 2 {
		t.Errorf("TotalMessages = %d, want 2", stats.TotalMessages)
	}
	if stats.LastActivity == nil {
		t.Error("LastActivity should not be nil")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	cb1, _ := NewCrossBridge(dir)
	cb1.Register(ToolCursor, CursorConfig{Endpoint: "http://test:9999"})
	cb1.SendTo(context.Background(), ToolCursor, "test", nil)

	cb2, _ := NewCrossBridge(dir)
	list := cb2.List()
	if len(list) != 1 {
		t.Fatalf("after reload: len = %d, want 1", len(list))
	}
}

func TestTranslateCapability(t *testing.T) {
	tests := []struct {
		from, to  ToolType
		cap, want string
	}{
		{"cursor", "forge", "code_edit", "patch"},
		{"cursor", "forge", "file_search", "search"},
		{"cursor", "forge", "agent", "run"},
		{"copilot", "forge", "code_complete", "run"},
		{"copilot", "forge", "pr_review", "review"},
		{"forge", "cursor", "run", "agent"},
		{"forge", "cursor", "exec", "terminal"},
		{"forge", "copilot", "run", "agent"},
		{"forge", "copilot", "review", "pr_review"},
		{"forge", "claude", "run", "agent"},
		{"forge", "claude", "search", "search"},
		// Unknown capability passes through
		{"forge", "cursor", "unknown_cap", "unknown_cap"},
	}

	for _, tt := range tests {
		got := TranslateCapability(tt.from, tt.to, tt.cap)
		if got != tt.want {
			t.Errorf("TranslateCapability(%s, %s, %q) = %q, want %q",
				tt.from, tt.to, tt.cap, got, tt.want)
		}
	}
}

func TestFormatToolInfo(t *testing.T) {
	info := ToolInfo{
		Type:         ToolCursor,
		Name:         "Cursor",
		Connected:    true,
		Capabilities: []string{"code_edit", "agent"},
		Endpoint:     "http://localhost:9999",
	}
	output := FormatToolInfo(info)
	if len(output) == 0 {
		t.Error("FormatToolInfo returned empty")
	}
}

func TestFormatBridgeMessage(t *testing.T) {
	msg := BridgeMessage{
		From:      "forge",
		To:        ToolCursor,
		Method:    "agent",
		Timestamp: time.Now(),
	}
	output := FormatBridgeMessage(msg)
	if len(output) == 0 {
		t.Error("FormatBridgeMessage returned empty")
	}

	// Error message
	msg.Error = "connection refused"
	output2 := FormatBridgeMessage(msg)
	if len(output2) == 0 {
		t.Error("FormatBridgeMessage with error returned empty")
	}
}

func TestConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	cb, _ := NewCrossBridge(dir)
	cb.Register(ToolClaude, ClaudeConfig{})

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			cb.SendTo(context.Background(), ToolClaude, "test", nil)
			cb.List()
			cb.Stats()
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := cb.Stats()
	if stats.TotalMessages != 10 {
		t.Errorf("TotalMessages = %d, want 10", stats.TotalMessages)
	}
}
