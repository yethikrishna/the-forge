package lsp

import (
	"encoding/json"
	"testing"
)

func TestServerInitialize(t *testing.T) {
	s := NewServer("0.6.0")

	params, _ := json.Marshal(map[string]interface{}{
		"processId": 1234,
		"rootUri":   "file:///tmp/test",
	})

	result, err := s.handleInitialize(params)
	if err != nil {
		t.Fatalf("handleInitialize failed: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	caps, ok := m["capabilities"]
	if !ok {
		t.Error("expected capabilities in result")
	}
	_ = caps
}

func TestServerHandle(t *testing.T) {
	s := NewServer("0.6.0")

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"processId": 1234,
		},
	}

	data, _ := json.Marshal(req)
	resp := s.Handle(data)

	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}
	if resp.ID != float64(1) {
		t.Errorf("expected ID 1, got %v", resp.ID)
	}
}

func TestServerMethodNotFound(t *testing.T) {
	s := NewServer("0.6.0")

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "nonexistent/method",
	}

	data, _ := json.Marshal(req)
	resp := s.Handle(data)

	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected -32601, got %d", resp.Error.Code)
	}
}

func TestServerDidOpen(t *testing.T) {
	s := NewServer("0.6.0")

	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":  "file:///test.go",
			"text": "package main\n\nfunc main() {}",
		},
	})

	_, err := s.handleDidOpen(params)
	if err != nil {
		t.Fatalf("handleDidOpen failed: %v", err)
	}

	s.mu.RLock()
	content, exists := s.docs["file:///test.go"]
	s.mu.RUnlock()

	if !exists {
		t.Error("expected document to be stored")
	}
	if content != "package main\n\nfunc main() {}" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestServerDidChange(t *testing.T) {
	s := NewServer("0.6.0")

	// Open first
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":  "file:///test.go",
			"text": "original",
		},
	})
	s.handleDidOpen(openParams)

	// Change
	changeParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
		"contentChanges": []map[string]interface{}{
			{"text": "updated"},
		},
	})
	s.handleChange(changeParams)

	s.mu.RLock()
	content := s.docs["file:///test.go"]
	s.mu.RUnlock()

	if content != "updated" {
		t.Errorf("expected updated content, got %s", content)
	}
}

func TestServerDidClose(t *testing.T) {
	s := NewServer("0.6.0")

	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":  "file:///test.go",
			"text": "content",
		},
	})
	s.handleDidOpen(openParams)

	closeParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
	})
	s.handleDidClose(closeParams)

	s.mu.RLock()
	_, exists := s.docs["file:///test.go"]
	s.mu.RUnlock()

	if exists {
		t.Error("expected document to be removed after close")
	}
}

func TestServerHover(t *testing.T) {
	s := NewServer("0.6.0")

	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":  "file:///test.go",
			"text": "package main\n\nfunc hello() {}",
		},
	})
	s.handleDidOpen(openParams)

	hoverParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
		"position": map[string]interface{}{
			"line":      2,
			"character": 5,
		},
	})

	result, err := s.handleHover(hoverParams)
	if err != nil {
		t.Fatalf("handleHover failed: %v", err)
	}

	hover, ok := result.(*Hover)
	if !ok {
		t.Fatal("expected Hover result")
	}
	if hover.Contents.Value == "" {
		t.Error("expected non-empty hover content")
	}
}

func TestServerCompletion(t *testing.T) {
	s := NewServer("0.6.0")

	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
		"position": map[string]interface{}{
			"line":      0,
			"character": 0,
		},
	})

	result, err := s.handleCompletion(params)
	if err != nil {
		t.Fatalf("handleCompletion failed: %v", err)
	}

	list, ok := result.(*CompletionList)
	if !ok {
		t.Fatal("expected CompletionList result")
	}
	if len(list.Items) == 0 {
		t.Error("expected at least one completion item")
	}
}

func TestServerCodeAction(t *testing.T) {
	s := NewServer("0.6.0")

	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
		"range": map[string]interface{}{
			"start": map[string]interface{}{"line": 0, "character": 0},
			"end":   map[string]interface{}{"line": 0, "character": 10},
		},
	})

	result, err := s.handleCodeAction(params)
	if err != nil {
		t.Fatalf("handleCodeAction failed: %v", err)
	}

	actions, ok := result.([]CodeAction)
	if !ok {
		t.Fatal("expected []CodeAction result")
	}
	if len(actions) < 3 {
		t.Errorf("expected at least 3 code actions, got %d", len(actions))
	}
}

func TestServerDiagnostic(t *testing.T) {
	s := NewServer("0.6.0")

	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":  "file:///test.go",
			"text": "package main\n// TODO: fix this\nvar password = \"secret\"",
		},
	})
	s.handleDidOpen(openParams)

	diagParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
	})

	result, err := s.handleDiagnostic(diagParams)
	if err != nil {
		t.Fatalf("handleDiagnostic failed: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	items, ok := m["items"].([]Diagnostic)
	if !ok {
		t.Fatal("expected diagnostics items")
	}
	if len(items) < 2 {
		t.Errorf("expected at least 2 diagnostics (TODO + secret), got %d", len(items))
	}
}

func TestAnalyzeDocument(t *testing.T) {
	s := NewServer("0.6.0")

	content := `package main
// TODO: implement this
// FIXME: broken
var api_key = "sk-xxx"
func main() {}`

	diagnostics := s.analyzeDocument("file:///test.go", content)

	if len(diagnostics) < 2 {
		t.Errorf("expected at least 2 diagnostics, got %d", len(diagnostics))
	}

	// Check for TODO
	foundTODO := false
	foundSecret := false
	for _, d := range diagnostics {
		if d.Code == "forge-todo" {
			foundTODO = true
		}
		if d.Code == "forge-secret" {
			foundSecret = true
		}
	}
	if !foundTODO {
		t.Error("expected TODO diagnostic")
	}
	if !foundSecret {
		t.Error("expected secret diagnostic")
	}
}

func TestServerExecuteCommand(t *testing.T) {
	s := NewServer("0.6.0")

	params, _ := json.Marshal(map[string]interface{}{
		"command": "forge.explain",
		"arguments": []interface{}{
			"file:///test.go",
			map[string]interface{}{
				"start": map[string]interface{}{"line": 0, "character": 0},
				"end":   map[string]interface{}{"line": 0, "character": 10},
			},
		},
	})

	result, err := s.handleExecuteCommand(params)
	if err != nil {
		t.Fatalf("handleExecuteCommand failed: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}
	if m["message"] == nil {
		t.Error("expected message in result")
	}
}

func TestServerExecuteUnknownCommand(t *testing.T) {
	s := NewServer("0.6.0")

	params, _ := json.Marshal(map[string]interface{}{
		"command": "forge.nonexistent",
	})

	_, err := s.handleExecuteCommand(params)
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRegisterAction(t *testing.T) {
	s := NewServer("0.6.0")

	customAction := &ForgeAction{
		ID:          "forge.custom",
		Title:       "Custom Action",
		Kind:        QuickFixCodeAction,
		Description: "A custom Forge action",
	}
	s.RegisterAction(customAction)

	if _, exists := s.actions["forge.custom"]; !exists {
		t.Error("expected custom action to be registered")
	}
}

func TestGetLine(t *testing.T) {
	content := "line 0\nline 1\nline 2"

	if getLine(content, 0) != "line 0" {
		t.Errorf("expected 'line 0', got %s", getLine(content, 0))
	}
	if getLine(content, 2) != "line 2" {
		t.Errorf("expected 'line 2', got %s", getLine(content, 2))
	}
	if getLine(content, 10) != "" {
		t.Errorf("expected empty for out of range, got %s", getLine(content, 10))
	}
}

func TestParseError(t *testing.T) {
	s := NewServer("0.6.0")

	resp := s.Handle([]byte("not valid json"))

	if resp.Error == nil {
		t.Error("expected error for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected parse error code -32700, got %d", resp.Error.Code)
	}
}
