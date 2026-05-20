package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/forge/sword/internal/mcp"
)

func TestInitialize(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := server.HandleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}
	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %v", resp.ID)
	}
}

func TestPing(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "ping",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("ping error: %s", resp.Error.Message)
	}
}

func TestRegisterTool(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")

	server.RegisterTool("test/tool", "A test tool",
		map[string]interface{}{"type": "object"},
		func(_ context.Context, _ json.RawMessage) (string, error) {
			return "tool result", nil
		},
	)

	// List tools
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/list",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("tools/list error: %s", resp.Error.Message)
	}

	// Call tool
	callParams, _ := json.Marshal(map[string]interface{}{
		"name":      "test/tool",
		"arguments": map[string]interface{}{},
	})
	req = mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  callParams,
	}

	resp = server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("tools/call error: %s", resp.Error.Message)
	}
}

func TestToolNotFound(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")

	callParams, _ := json.Marshal(map[string]interface{}{
		"name":      "nonexistent",
		"arguments": map[string]interface{}{},
	})
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  callParams,
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error == nil {
		t.Error("should error for nonexistent tool")
	}
}

func TestRegisterResource(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")

	server.RegisterResource("forge://test", "Test Resource", "A test resource", "text/plain",
		func(_ context.Context, _ string) (string, string, error) {
			return "resource content", "text/plain", nil
		},
	)

	// List resources
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "resources/list",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("resources/list error: %s", resp.Error.Message)
	}

	// Read resource
	readParams, _ := json.Marshal(map[string]interface{}{
		"uri": "forge://test",
	})
	req = mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "resources/read",
		Params:  readParams,
	}

	resp = server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("resources/read error: %s", resp.Error.Message)
	}
}

func TestRegisterPrompt(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")

	server.RegisterPrompt("test/prompt", "A test prompt", []mcp.PromptArg{
		{Name: "query", Description: "Search query", Required: true},
	}, func(_ context.Context, args map[string]string) ([]mcp.PromptMessage, error) {
		return []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: "Query: " + args["query"]}},
		}, nil
	})

	// List prompts
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "prompts/list",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("prompts/list error: %s", resp.Error.Message)
	}

	// Get prompt
	getParams, _ := json.Marshal(map[string]interface{}{
		"name":      "test/prompt",
		"arguments": map[string]string{"query": "hello"},
	})
	req = mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "prompts/get",
		Params:  getParams,
	}

	resp = server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("prompts/get error: %s", resp.Error.Message)
	}
}

func TestMethodNotFound(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "nonexistent/method",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error == nil {
		t.Error("should error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected -32601, got %d", resp.Error.Code)
	}
}

func TestForgeTools(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")
	server.RegisterForgeTools()

	// List should have tools
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/list",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("tools/list error: %s", resp.Error.Message)
	}
}

func TestForgeResources(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")
	server.RegisterForgeResources()

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "resources/list",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("resources/list error: %s", resp.Error.Message)
	}
}

func TestForgePrompts(t *testing.T) {
	server := mcp.NewServer("test-forge", "0.4.0")
	server.RegisterForgePrompts()

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "prompts/list",
	}

	resp := server.HandleRequest(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("prompts/list error: %s", resp.Error.Message)
	}
}

func TestIsMCPRequest(t *testing.T) {
	tests := []struct {
		data     string
		expected bool
	}{
		{`{"jsonrpc":"2.0","method":"initialize","id":1}`, true},
		{`{"jsonrpc":"2.0","method":"ping","id":2}`, true},
		{`{"jsonrpc":"2.0","method":"tools/list","id":3}`, true},
		{`{"method":"random"}`, false},
		{`not json`, false},
	}

	for _, tt := range tests {
		result := mcp.IsMCPRequest([]byte(tt.data))
		if result != tt.expected {
			t.Errorf("IsMCPRequest(%s): expected %v, got %v", tt.data[:20], tt.expected, result)
		}
	}
}
