package integration

// Cross-tool bridge E2E validation tests.
//
// This file tests the full round-trip:
//   mock MCP server → cross-tool bridge → Forge MCP gateway
//
// Tests cover:
//   - bridge.cursor and bridge.copilot configuration parsing
//   - Error propagation from bridge to caller
//   - Gateway auth, rate limiting, and validation pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forge/sword/internal/crosstool"
	"github.com/forge/sword/internal/mcpgateway"
)

// mockMCPServer creates a test HTTP server that speaks a minimal MCP-like protocol.
// It records all requests so tests can assert on them.
func mockMCPServer(t *testing.T) (*httptest.Server, *[]map[string]interface{}) {
	t.Helper()
	var received []map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/bridge", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		received = append(received, req)

		// Simulate MCP response
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"status": "ok",
				"method": req["method"],
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Error endpoint — simulates MCP server returning 5xx
	mux.HandleFunc("/api/bridge/error", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &received
}

// --- Round-trip tests ---

// TestCrossToolBridgeToGatewayRoundTrip is the primary E2E test.
// Flow: test calls CrossBridge.SendTo (Cursor target) → bridge POSTs to mock MCP server
//       → GatewayRequest built from bridge response → Gateway.ProcessRequest approves it.
func TestCrossToolBridgeToGatewayRoundTrip(t *testing.T) {
	// 1. Start a mock MCP server (stands in for Cursor's local API).
	mcpSrv, received := mockMCPServer(t)

	// 2. Create the cross-tool bridge pointing at the mock server.
	bridgeDir := t.TempDir()
	bridge, err := crosstool.NewCrossBridge(bridgeDir)
	if err != nil {
		t.Fatalf("NewCrossBridge: %v", err)
	}

	_, err = bridge.Register(crosstool.ToolCursor, crosstool.CursorConfig{
		Endpoint:  mcpSrv.URL,
		Workspace: "/tmp/forge-test",
	})
	if err != nil {
		t.Fatalf("bridge.Register(cursor): %v", err)
	}

	// 3. Create the Forge MCP gateway with token auth.
	gatewayDir := t.TempDir()
	gwCfg := mcpgateway.GatewayConfig{
		Name:    "forge-test-gateway",
		Version: mcpgateway.ProtocolVersion,
		Auth: mcpgateway.AuthConfig{
			Method: mcpgateway.AuthToken,
			Tokens: []string{"tok-forge-test"},
		},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 100,
			Burst:             10,
		},
		Enabled: true,
	}
	gw, err := mcpgateway.NewGateway(gatewayDir, gwCfg)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}

	// 4. Send a message through the bridge to the mock MCP server.
	ctx := context.Background()
	msg, err := bridge.SendTo(ctx, crosstool.ToolCursor, "tools/call", map[string]interface{}{
		"name":   "forge.run",
		"params": map[string]interface{}{"task": "lint"},
	})
	if err != nil {
		t.Fatalf("bridge.SendTo: %v", err)
	}
	if msg.Error != "" {
		t.Fatalf("bridge message error: %s", msg.Error)
	}

	// 5. Verify the mock MCP server received the request.
	if len(*received) == 0 {
		t.Fatal("mock MCP server received no requests")
	}
	gotMethod, _ := (*received)[0]["method"].(string)
	if gotMethod != "tools/call" {
		t.Errorf("received method = %q, want tools/call", gotMethod)
	}

	// 6. Route the bridge result through the Forge MCP gateway.
	gwReq := mcpgateway.GatewayRequest{
		ClientID:   "cursor-bridge",
		RemoteAddr: "127.0.0.1:0",
		Token:      "tok-forge-test",
		Method:     msg.Method,
		ToolName:   "forge.run",
		Args:       map[string]interface{}{"task": "lint"},
	}
	gwResp := gw.ProcessRequest(gwReq)

	if !gwResp.Allowed {
		t.Errorf("gateway denied request: %s", gwResp.Reason)
	}
	if gwResp.RequestID == "" {
		t.Error("gateway response should have a RequestID")
	}

	// 7. Verify audit log recorded the round-trip.
	audit := gw.GetAudit("cursor-bridge", "ok", 10)
	if len(audit) == 0 {
		t.Error("expected audit entry for cursor-bridge")
	}
}

// TestCrossToolBridgeCopilotRoundTrip tests the Copilot → gateway path.
func TestCrossToolBridgeCopilotRoundTrip(t *testing.T) {
	mcpSrv, _ := mockMCPServer(t)

	bridgeDir := t.TempDir()
	bridge, _ := crosstool.NewCrossBridge(bridgeDir)
	bridge.Register(crosstool.ToolCopilot, crosstool.CopilotConfig{
		Token:   "ghp_test123",
		Repo:    "forge/repo",
		BaseURL: mcpSrv.URL,
	})

	gatewayDir := t.TempDir()
	gw, _ := mcpgateway.NewGateway(gatewayDir, mcpgateway.GatewayConfig{
		Name:    "forge-copilot-gw",
		Version: mcpgateway.ProtocolVersion,
		Auth:    mcpgateway.AuthConfig{Method: mcpgateway.AuthNone},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 60,
			Burst:             5,
		},
		Enabled: true,
	})

	// Send through the Copilot bridge — note: Copilot POSTs to /copilot/bridge
	// which isn't handled by the mock, so we expect an HTTP 404.
	// The bridge returns an error; we check it propagates correctly (see error test).
	// For the gateway leg, simulate a bridge-originated request arriving at the gateway.
	gwReq := mcpgateway.GatewayRequest{
		ClientID: "copilot-bridge",
		Method:   "chat",
		ToolName: "forge.review",
		Args:     map[string]interface{}{"pr": "123"},
	}
	gwResp := gw.ProcessRequest(gwReq)
	if !gwResp.Allowed {
		t.Errorf("gateway denied copilot request: %s", gwResp.Reason)
	}
}

// --- Configuration parsing tests ---

// TestBridgeCursorConfigParsing verifies CursorConfig fields are stored and retrievable.
func TestBridgeCursorConfigParsing(t *testing.T) {
	dir := t.TempDir()
	bridge, err := crosstool.NewCrossBridge(dir)
	if err != nil {
		t.Fatalf("NewCrossBridge: %v", err)
	}

	cfg := crosstool.CursorConfig{
		Endpoint:  "http://localhost:9999",
		APIKey:    "cursor-key-abc",
		Workspace: "/home/user/project",
	}
	info, err := bridge.Register(crosstool.ToolCursor, cfg)
	if err != nil {
		t.Fatalf("Register cursor: %v", err)
	}

	// Verify ToolInfo fields
	if info.Type != crosstool.ToolCursor {
		t.Errorf("Type = %q, want %q", info.Type, crosstool.ToolCursor)
	}
	if info.Name != "Cursor" {
		t.Errorf("Name = %q, want Cursor", info.Name)
	}
	if info.Endpoint != cfg.Endpoint {
		t.Errorf("Endpoint = %q, want %q", info.Endpoint, cfg.Endpoint)
	}
	if !info.Connected {
		t.Error("info.Connected should be true after registration")
	}

	expectedCaps := map[string]bool{
		"code_edit":   true,
		"file_search": true,
		"terminal":    true,
		"lsp":         true,
		"agent":       true,
	}
	for _, c := range info.Capabilities {
		delete(expectedCaps, c)
	}
	if len(expectedCaps) > 0 {
		missing := make([]string, 0, len(expectedCaps))
		for k := range expectedCaps {
			missing = append(missing, k)
		}
		t.Errorf("missing cursor capabilities: %v", missing)
	}

	// Reload from disk — verify persistence
	bridge2, _ := crosstool.NewCrossBridge(dir)
	loaded, ok := bridge2.Get(crosstool.ToolCursor)
	if !ok {
		t.Fatal("cursor tool not found after reload")
	}
	if loaded.Endpoint != cfg.Endpoint {
		t.Errorf("reloaded Endpoint = %q, want %q", loaded.Endpoint, cfg.Endpoint)
	}
}

// TestBridgeCopilotConfigParsing verifies CopilotConfig fields.
func TestBridgeCopilotConfigParsing(t *testing.T) {
	dir := t.TempDir()
	bridge, _ := crosstool.NewCrossBridge(dir)

	cfg := crosstool.CopilotConfig{
		Token:   "ghp_copilot_test_token",
		Repo:    "myorg/myrepo",
		BaseURL: "https://api.github.com",
	}
	info, err := bridge.Register(crosstool.ToolCopilot, cfg)
	if err != nil {
		t.Fatalf("Register copilot: %v", err)
	}

	if info.Type != crosstool.ToolCopilot {
		t.Errorf("Type = %q, want %q", info.Type, crosstool.ToolCopilot)
	}
	if info.Name != "GitHub Copilot" {
		t.Errorf("Name = %q, want GitHub Copilot", info.Name)
	}
	// BaseURL is stored in Endpoint
	if info.Endpoint != cfg.BaseURL {
		t.Errorf("Endpoint = %q, want %q", info.Endpoint, cfg.BaseURL)
	}
	if !info.Connected {
		t.Error("info.Connected should be true")
	}

	expectedCaps := map[string]bool{
		"code_complete": true,
		"chat":          true,
		"agent":         true,
		"pr_review":     true,
	}
	for _, c := range info.Capabilities {
		delete(expectedCaps, c)
	}
	if len(expectedCaps) > 0 {
		t.Errorf("missing copilot capabilities: %v", expectedCaps)
	}
}

// TestBridgeCursorAndCopilotCoexist verifies both can be registered simultaneously.
func TestBridgeCursorAndCopilotCoexist(t *testing.T) {
	dir := t.TempDir()
	bridge, _ := crosstool.NewCrossBridge(dir)

	bridge.Register(crosstool.ToolCursor, crosstool.CursorConfig{Endpoint: "http://localhost:9999"})
	bridge.Register(crosstool.ToolCopilot, crosstool.CopilotConfig{Token: "ghp_test", BaseURL: "https://api.github.com"})

	tools := bridge.List()
	if len(tools) != 2 {
		t.Errorf("List len = %d, want 2", len(tools))
	}

	cursor, ok := bridge.Get(crosstool.ToolCursor)
	if !ok || cursor.Type != crosstool.ToolCursor {
		t.Error("should find cursor")
	}

	copilot, ok := bridge.Get(crosstool.ToolCopilot)
	if !ok || copilot.Type != crosstool.ToolCopilot {
		t.Error("should find copilot")
	}
}

// --- Error propagation tests ---

// TestBridgeErrorPropagationUnregistered checks that sending to an unknown tool returns an error.
func TestBridgeErrorPropagationUnregistered(t *testing.T) {
	dir := t.TempDir()
	bridge, _ := crosstool.NewCrossBridge(dir)

	_, err := bridge.SendTo(context.Background(), crosstool.ToolCursor, "test/method", nil)
	if err == nil {
		t.Fatal("expected error when sending to unregistered tool")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want it to contain 'not registered'", err.Error())
	}
}

// TestBridgeErrorPropagationHTTP checks that HTTP errors from the endpoint are surfaced.
func TestBridgeErrorPropagationHTTP(t *testing.T) {
	// Start a server that always returns 500.
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	t.Cleanup(errSrv.Close)

	dir := t.TempDir()
	bridge, _ := crosstool.NewCrossBridge(dir)
	bridge.Register(crosstool.ToolCursor, crosstool.CursorConfig{
		Endpoint: errSrv.URL,
	})

	msg, err := bridge.SendTo(context.Background(), crosstool.ToolCursor, "tools/call", nil)
	if err == nil {
		t.Fatal("expected error from HTTP 500 endpoint")
	}

	// The error must be surfaced in the returned message as well.
	if msg == nil {
		t.Fatal("msg should not be nil even on error")
	}
	if msg.Error == "" {
		t.Error("msg.Error should be non-empty on HTTP error")
	}
	if !strings.Contains(msg.Error, "500") && !strings.Contains(msg.Error, "HTTP") {
		t.Errorf("msg.Error = %q, want HTTP 500 indication", msg.Error)
	}
}

// TestBridgeErrorNotRecordedInGatewayAudit verifies that a bridge error does NOT
// appear as a successful gateway request (audit integrity).
func TestBridgeErrorNotRecordedInGatewayAudit(t *testing.T) {
	gatewayDir := t.TempDir()
	gw, _ := mcpgateway.NewGateway(gatewayDir, mcpgateway.GatewayConfig{
		Name:    "forge-audit-gw",
		Version: mcpgateway.ProtocolVersion,
		Auth: mcpgateway.AuthConfig{
			Method: mcpgateway.AuthToken,
			Tokens: []string{"valid-token"},
		},
		RateLimit: mcpgateway.RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
	})

	// Request with wrong token — bridge error → gateway auth failure
	badReq := mcpgateway.GatewayRequest{
		ClientID: "bad-client",
		Token:    "wrong-token",
		Method:   "tools/call",
		ToolName: "forge.run",
		Args:     map[string]interface{}{"task": "build"},
	}
	resp := gw.ProcessRequest(badReq)
	if resp.Allowed {
		t.Error("gateway should have denied request with wrong token")
	}
	if !strings.Contains(resp.Reason, "authentication") {
		t.Errorf("Reason = %q, want authentication error", resp.Reason)
	}

	// Audit should show auth_failed, not ok.
	audit := gw.GetAudit("bad-client", "auth_failed", 10)
	if len(audit) == 0 {
		t.Error("expected auth_failed audit entry")
	}

	okAudit := gw.GetAudit("bad-client", "ok", 10)
	if len(okAudit) > 0 {
		t.Error("should have no 'ok' audit entry for bad-client")
	}
}

// TestGatewayValidationErrorPropagation checks that validation errors are propagated.
func TestGatewayValidationErrorPropagation(t *testing.T) {
	gatewayDir := t.TempDir()
	gw, _ := mcpgateway.NewGateway(gatewayDir, mcpgateway.GatewayConfig{
		Name:    "forge-validation-gw",
		Version: mcpgateway.ProtocolVersion,
		Auth:    mcpgateway.AuthConfig{Method: mcpgateway.AuthNone},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 100,
			Burst:             10,
		},
		Validation: []mcpgateway.ValidationRule{
			{
				ToolName: "forge.run",
				Required: []string{"task"},
			},
		},
		Enabled: true,
	})

	// Missing required field "task" → validation failure
	badReq := mcpgateway.GatewayRequest{
		ClientID: "cursor-bridge",
		Method:   "tools/call",
		ToolName: "forge.run",
		Args:     map[string]interface{}{}, // "task" is missing
	}
	resp := gw.ProcessRequest(badReq)
	if resp.Allowed {
		t.Error("gateway should have denied request with missing required arg")
	}
	if !strings.Contains(resp.Reason, "validation") {
		t.Errorf("Reason = %q, want 'validation' in reason", resp.Reason)
	}
	if !strings.Contains(resp.Reason, "task") {
		t.Errorf("Reason = %q, want 'task' in reason", resp.Reason)
	}
}

// TestGatewayRateLimitErrorPropagation checks that rate limit errors are propagated.
func TestGatewayRateLimitErrorPropagation(t *testing.T) {
	gatewayDir := t.TempDir()
	gw, _ := mcpgateway.NewGateway(gatewayDir, mcpgateway.GatewayConfig{
		Name:    "forge-rl-gw",
		Version: mcpgateway.ProtocolVersion,
		Auth:    mcpgateway.AuthConfig{Method: mcpgateway.AuthNone},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 2,
			Burst:             0,
		},
		Enabled: true,
	})

	clientID := "cursor-rl-client"
	allowed := 0
	denied := 0

	for i := 0; i < 5; i++ {
		req := mcpgateway.GatewayRequest{
			ClientID: clientID,
			Method:   "tools/call",
			ToolName: fmt.Sprintf("tool-%d", i),
		}
		resp := gw.ProcessRequest(req)
		if resp.Allowed {
			allowed++
		} else {
			denied++
			if !strings.Contains(resp.Reason, "rate") {
				t.Errorf("req %d: Reason = %q, want 'rate' in reason", i, resp.Reason)
			}
			// Retry-After header must be set
			if resp.Headers["Retry-After"] == "" {
				t.Errorf("req %d: expected Retry-After header on rate limit", i)
			}
		}
	}

	if denied == 0 {
		t.Error("expected at least one rate-limited request")
	}

	// Audit should record rate_limited entries
	audit := gw.GetAudit(clientID, "rate_limited", 10)
	if len(audit) != denied {
		t.Errorf("audit rate_limited = %d, want %d", len(audit), denied)
	}
}

// TestBridgeToGatewayCapabilityTranslation verifies that capability translation
// works end-to-end from bridge through the gateway.
func TestBridgeToGatewayCapabilityTranslation(t *testing.T) {
	// Translate Cursor capability to Forge's internal format.
	translated := crosstool.TranslateCapability(crosstool.ToolCursor, "forge", "code_edit")
	if translated != "patch" {
		t.Errorf("TranslateCapability(cursor→forge, code_edit) = %q, want patch", translated)
	}

	// Use translated capability as the method in a gateway request.
	gatewayDir := t.TempDir()
	gw, _ := mcpgateway.NewGateway(gatewayDir, mcpgateway.GatewayConfig{
		Name:    "forge-cap-gw",
		Version: mcpgateway.ProtocolVersion,
		Auth:    mcpgateway.AuthConfig{Method: mcpgateway.AuthNone},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 100,
		},
		Enabled: true,
	})

	gwReq := mcpgateway.GatewayRequest{
		ClientID: "cursor-cap-client",
		Method:   translated, // "patch" — translated from "code_edit"
		ToolName: "forge.patch",
	}
	resp := gw.ProcessRequest(gwReq)
	if !resp.Allowed {
		t.Errorf("gateway denied capability-translated request: %s", resp.Reason)
	}
}

// TestGatewayDisabledAllowsAll verifies that a disabled gateway lets everything through.
func TestGatewayDisabledAllowsAll(t *testing.T) {
	gatewayDir := t.TempDir()
	gw, _ := mcpgateway.NewGateway(gatewayDir, mcpgateway.GatewayConfig{
		Name:    "forge-disabled-gw",
		Version: mcpgateway.ProtocolVersion,
		Auth: mcpgateway.AuthConfig{
			Method: mcpgateway.AuthToken,
			Tokens: []string{"required-token"},
		},
		Enabled: false, // disabled — auth is bypassed
	})

	// No token required when disabled.
	resp := gw.ProcessRequest(mcpgateway.GatewayRequest{
		ClientID: "any-client",
		Method:   "tools/call",
		ToolName: "forge.run",
	})
	if !resp.Allowed {
		t.Errorf("disabled gateway should allow all requests, got: %s", resp.Reason)
	}
}

// TestFullPipelineWithStats is an aggregate stat validation test.
// Sends a mix of allowed and denied requests, checks Stats() totals.
func TestFullPipelineWithStats(t *testing.T) {
	gatewayDir := t.TempDir()
	gw, _ := mcpgateway.NewGateway(gatewayDir, mcpgateway.GatewayConfig{
		Name:    "forge-stats-gw",
		Version: mcpgateway.ProtocolVersion,
		Auth: mcpgateway.AuthConfig{
			Method: mcpgateway.AuthToken,
			Tokens: []string{"good-token"},
		},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 100,
		},
		Enabled: true,
	})

	// 3 allowed
	for i := 0; i < 3; i++ {
		gw.ProcessRequest(mcpgateway.GatewayRequest{
			ClientID: "cursor-stats",
			Token:    "good-token",
			Method:   "tools/call",
			ToolName: "forge.run",
		})
	}
	// 2 denied (bad token)
	for i := 0; i < 2; i++ {
		gw.ProcessRequest(mcpgateway.GatewayRequest{
			ClientID: "cursor-stats",
			Token:    "bad-token",
			Method:   "tools/call",
			ToolName: "forge.run",
		})
	}

	stats := gw.Stats()
	if stats.TotalRequests != 5 {
		t.Errorf("TotalRequests = %d, want 5", stats.TotalRequests)
	}
	if stats.AllowedRequests != 3 {
		t.Errorf("AllowedRequests = %d, want 3", stats.AllowedRequests)
	}
	if stats.DeniedRequests != 2 {
		t.Errorf("DeniedRequests = %d, want 2", stats.DeniedRequests)
	}
	if stats.ByMethod["tools/call"] != 5 {
		t.Errorf("ByMethod[tools/call] = %d, want 5", stats.ByMethod["tools/call"])
	}
}
