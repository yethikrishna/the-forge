package mcp2_test

import (
	"testing"

	"github.com/forge/sword/internal/mcpgateway"
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

// TestMCPGovernanceWiring verifies the full integration: mcp2/server ↔ mcpgateway.
// This is the AD-1 architecture decision: governance as middleware chain in mcp2.
func TestMCPGovernanceWiring(t *testing.T) {
	dir := t.TempDir()

	// Create a governed MCP gateway.
	gw, err := mcpgateway.NewGateway(dir, mcpgateway.GatewayConfig{
		Auth:      mcpgateway.AuthConfig{Method: mcpgateway.AuthToken, Tokens: []string{"valid-token"}},
		RateLimit: mcpgateway.RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	defer gw.Close()

	// Create an MCP server and wire governance middleware.
	srv := server.NewServer("governed-forge", "1.0.0")
	srv.SetGovernance(func(req server.JSONRPCRequest, clientID, token string) server.GovernanceDecision {
		gwResp := gw.ProcessRequest(mcpgateway.GatewayRequest{
			ClientID: clientID,
			Token:    token,
			Method:   req.Method,
		})
		return server.GovernanceDecision{
			Allowed:   gwResp.Allowed,
			Reason:    gwResp.Reason,
			RequestID: gwResp.RequestID,
		}
	})

	// Valid token — should be allowed.
	resp := srv.HandleWithClient(server.JSONRPCRequest{
		JSONRPC: "2.0", Method: "tools/list", ID: 1,
	}, "client-1", "valid-token")
	if resp.Error != nil {
		t.Errorf("valid token should be allowed: %v", resp.Error)
	}

	// Invalid token — should be denied by governance chain.
	resp2 := srv.HandleWithClient(server.JSONRPCRequest{
		JSONRPC: "2.0", Method: "tools/call", ID: 2,
	}, "attacker", "wrong-token")
	if resp2.Error == nil {
		t.Fatal("invalid token should be denied")
	}

	// Verify audit trail captured both requests.
	audit := gw.GetAudit("", "", 0)
	if len(audit) != 2 {
		t.Errorf("expected 2 audit entries, got %d", len(audit))
	}

	// Stats should reflect 1 allowed, 1 denied.
	stats := gw.Stats()
	if stats.AllowedRequests != 1 {
		t.Errorf("expected 1 allowed, got %d", stats.AllowedRequests)
	}
	if stats.DeniedRequests != 1 {
		t.Errorf("expected 1 denied, got %d", stats.DeniedRequests)
	}
}
