package mcp2_test

import (
	"testing"

	"github.com/forge/sword/internal/mcp2/compose"
	"github.com/forge/sword/internal/mcp2/discover"
	"github.com/forge/sword/internal/mcp2/server"
)

func TestMCPServerCreation(t *testing.T) {
	srv := server.NewServer("test-server", "1.0.0")
	if srv == nil {
		t.Fatal("NewServer should return a server")
	}
}

func TestMCPServerRegisterTool(t *testing.T) {
	srv := server.NewServer("test", "1.0.0")
	srv.RegisterTool(server.Tool{
		Name:        "test-tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}, func(args map[string]interface{}) (server.ToolResult, error) {
		return server.ToolResult{
			Content: []server.ContentBlock{{Type: "text", Text: "ok"}},
		}, nil
	})

	resp := srv.Handle(server.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      1,
	})
	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error)
	}
}

func TestMCPServerInitialize(t *testing.T) {
	srv := server.NewServer("test", "1.0.0")
	resp := srv.Handle(server.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
	})
	if resp.Error != nil {
		t.Fatalf("initialize error: %v", resp.Error)
	}
}

func TestMCPServerPing(t *testing.T) {
	srv := server.NewServer("test", "1.0.0")
	resp := srv.Handle(server.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "ping",
		ID:      1,
	})
	if resp.Error != nil {
		t.Fatalf("ping error: %v", resp.Error)
	}
}

func TestMCPServerBuiltins(t *testing.T) {
	tools := server.ForgeBuiltins()
	if len(tools) == 0 {
		t.Error("ForgeBuiltins should return some tools")
	}
}

func TestMCPComposeGateway(t *testing.T) {
	gw := compose.NewComposeGateway(compose.ComposeConfig{
		Servers: []compose.ServerConfig{
			{Command: "echo", Args: []string{"hello"}},
		},
	})
	if gw == nil {
		t.Fatal("NewComposeGateway should return a gateway")
	}

	tools := gw.ListTools()
	_ = tools
}

func TestMCPComposeAddServer(t *testing.T) {
	gw := compose.NewComposeGateway(compose.ComposeConfig{})
	err := gw.AddServer(compose.ServerConfig{
		Name:    "added-srv",
		Command: "test-cmd",
	})
	if err != nil {
		t.Fatalf("AddServer error: %v", err)
	}
}

func TestMCPComposeRemoveServer(t *testing.T) {
	gw := compose.NewComposeGateway(compose.ComposeConfig{})
	gw.AddServer(compose.ServerConfig{Name: "rm-srv", Command: "test"})
	err := gw.RemoveServer("rm-srv")
	if err != nil {
		t.Fatalf("RemoveServer error: %v", err)
	}
}

func TestMCPDiscover(t *testing.T) {
	disc := discover.NewDiscoverer()
	if disc == nil {
		t.Fatal("NewDiscoverer should return a discoverer")
	}

	result := disc.Discover()
	if result == nil {
		t.Fatal("Discover should return a result")
	}
}

func TestMCPDiscoverFormatServer(t *testing.T) {
	s := &discover.DiscoveredServer{
		Name:    "test-server",
		Address: "localhost:3000",
		Source:  "config",
	}
	formatted := discover.FormatServer(s)
	if formatted == "" {
		t.Error("FormatServer should not be empty")
	}
}

func TestMCPDiscoverFormatResult(t *testing.T) {
	r := &discover.DiscoveryResult{
		Servers: []*discover.DiscoveredServer{
			{Name: "srv1", Source: "config"},
		},
	}
	formatted := discover.FormatResult(r)
	if formatted == "" {
		t.Error("FormatResult should not be empty")
	}
}
