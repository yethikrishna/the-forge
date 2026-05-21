package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleInitialize(t *testing.T) {
	s := NewServer("forge", "1.0.0")
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	resp := s.Handle(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if result["protocolVersion"] != Version {
		t.Errorf("expected version %s, got %v", Version, result["protocolVersion"])
	}

	info, ok := result["serverInfo"].(ServerInfo)
	if !ok {
		t.Fatal("expected serverInfo")
	}
	if info.Name != "forge" {
		t.Errorf("expected name forge, got %s", info.Name)
	}
}

func TestHandlePing(t *testing.T) {
	s := NewServer("forge", "1.0.0")
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "ping"}
	resp := s.Handle(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleMethodNotFound(t *testing.T) {
	s := NewServer("forge", "1.0.0")
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 3, Method: "nonexistent"}
	resp := s.Handle(req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != ErrorMethodNotFound {
		t.Errorf("expected method not found error, got %d", resp.Error.Code)
	}
}

func TestHandleRaw(t *testing.T) {
	s := NewServer("forge", "1.0.0")

	data := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	resp := s.HandleRaw(data)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRawInvalid(t *testing.T) {
	s := NewServer("forge", "1.0.0")

	data := []byte(`{invalid json}`)
	resp := s.HandleRaw(data)

	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if resp.Error.Code != ErrorParseError {
		t.Errorf("expected parse error, got %d", resp.Error.Code)
	}
}

func TestRegisterTool(t *testing.T) {
	s := NewServer("forge", "1.0.0")

	called := false
	s.RegisterTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{"type": "object"},
	}, func(args map[string]interface{}) (ToolResult, error) {
		called = true
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "done"}},
		}, nil
	})

	// List tools
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 4, Method: "tools/list"}
	resp := s.Handle(req)

	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]Tool)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "test_tool" {
		t.Errorf("expected test_tool, got %s", tools[0].Name)
	}

	// Call tool
	params, _ := json.Marshal(map[string]interface{}{
		"name": "test_tool",
		"arguments": map[string]interface{}{"key": "value"},
	})
	req2 := JSONRPCRequest{JSONRPC: "2.0", ID: 5, Method: "tools/call", Params: params}
	resp2 := s.Handle(req2)

	if resp2.Error != nil {
		t.Fatalf("tool call failed: %v", resp2.Error)
	}
	if !called {
		t.Error("tool handler should have been called")
	}
}

func TestCallUnknownTool(t *testing.T) {
	s := NewServer("forge", "1.0.0")

	params, _ := json.Marshal(map[string]interface{}{
		"name": "unknown_tool",
	})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 6, Method: "tools/call", Params: params}
	resp := s.Handle(req)

	result, ok := resp.Result.(ToolResult)
	if !ok {
		t.Fatal("expected ToolResult")
	}
	if !result.IsError {
		t.Error("expected error result for unknown tool")
	}
}

func TestRegisterResource(t *testing.T) {
	s := NewServer("forge", "1.0.0")

	s.RegisterResource(Resource{
		URI:         "forge://config",
		Name:        "Forge Config",
		Description: "Current configuration",
		MimeType:    "application/json",
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 7, Method: "resources/list"}
	resp := s.Handle(req)

	result := resp.Result.(map[string]interface{})
	resources := result["resources"].([]Resource)
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].URI != "forge://config" {
		t.Errorf("expected forge://config, got %s", resources[0].URI)
	}
}

func TestRegisterPrompt(t *testing.T) {
	s := NewServer("forge", "1.0.0")

	s.RegisterPrompt(Prompt{
		Name:        "code_review",
		Description: "Review code for issues",
		Arguments: []PromptArg{
			{Name: "code", Description: "Code to review", Required: true},
		},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 8, Method: "prompts/list"}
	resp := s.Handle(req)

	result := resp.Result.(map[string]interface{})
	prompts := result["prompts"].([]Prompt)
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Name != "code_review" {
		t.Errorf("expected code_review, got %s", prompts[0].Name)
	}
}

func TestForgeBuiltins(t *testing.T) {
	tools := ForgeBuiltins()
	if len(tools) == 0 {
		t.Error("expected built-in tools")
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %s should have a description", tool.Name)
		}
	}

	if !names["forge_run"] {
		t.Error("expected forge_run tool")
	}
	if !names["forge_build"] {
		t.Error("expected forge_build tool")
	}
	if !names["forge_search"] {
		t.Error("expected forge_search tool")
	}
}

func TestFormatServerInfo(t *testing.T) {
	s := NewServer("forge", "1.0.0")
	s.RegisterTool(Tool{Name: "test", Description: "Test"}, nil)

	output := FormatServerInfo(s)
	if !strings.Contains(output, "forge") {
		t.Error("expected server name in output")
	}
	if !strings.Contains(output, "Tools:       1") {
		t.Error("expected tool count in output")
	}
}

func TestFormatTools(t *testing.T) {
	tools := []Tool{
		{Name: "forge_run", Description: "Run an agent"},
		{Name: "forge_build", Description: "Build project"},
	}

	output := FormatTools(tools)
	if !strings.Contains(output, "forge_run") {
		t.Error("expected tool name in output")
	}
}

func TestJSONRPCRequestSerialization(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      42,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"test"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var req2 JSONRPCRequest
	if err := json.Unmarshal(data, &req2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if req2.Method != "tools/call" {
		t.Errorf("expected tools/call, got %s", req2.Method)
	}
}

func TestToolResultSerialization(t *testing.T) {
	result := ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "Hello from tool"},
		},
		IsError: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result2 ToolResult
	if err := json.Unmarshal(data, &result2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result2.Content[0].Text != "Hello from tool" {
		t.Errorf("unexpected content: %s", result2.Content[0].Text)
	}
}

func TestRPCErrorCodes(t *testing.T) {
	codes := map[string]int{
		"parse":     ErrorParseError,
		"invalid":   ErrorInvalidRequest,
		"notFound":  ErrorMethodNotFound,
		"params":    ErrorInvalidParams,
		"internal":  ErrorInternal,
	}

	for name, code := range codes {
		if code >= 0 {
			t.Errorf("%s error code should be negative, got %d", name, code)
		}
	}
}
