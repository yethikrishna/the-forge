package mcpgateway

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestGatewayNoAuth(t *testing.T) {
	dir := t.TempDir()
	gw, err := NewGateway(dir, GatewayConfig{
		Name:    "test",
		Version: "1.0",
		Auth:    AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}

	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "client-1",
		Method:   "tools/list",
	})
	if !resp.Allowed {
		t.Errorf("Allowed = false, reason: %s", resp.Reason)
	}
}

func TestGatewayTokenAuth(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{
			Method: AuthToken,
			Tokens: []string{"secret-123", "secret-456"},
		},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
	})

	// Valid token
	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Token: "secret-123", Method: "tools/list",
	})
	if !resp.Allowed {
		t.Error("valid token should be allowed")
	}

	// Invalid token
	resp2 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Token: "wrong", Method: "tools/list",
	})
	if resp2.Allowed {
		t.Error("invalid token should be denied")
	}
	if resp2.Reason == "" {
		t.Error("denied response should have reason")
	}

	// No token
	resp3 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/list",
	})
	if resp3.Allowed {
		t.Error("no token should be denied")
	}
}

func TestGatewayAPIKeyAuth(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{
			Method: AuthAPIKey,
			Tokens: []string{"ak-test-key"},
		},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
	})

	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Token: "ak-test-key", Method: "ping",
	})
	if !resp.Allowed {
		t.Error("valid API key should be allowed")
	}
}

func TestGatewayOAuth2Auth(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{
			Method: AuthOAuth2,
			OIDCURL: "https://auth.example.com",
		},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
	})

	// Valid format
	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Token: "Bearer eyJhbGciOiJSUzI1NiIs", Method: "ping",
	})
	if !resp.Allowed {
		t.Error("valid bearer token format should be allowed")
	}

	// Invalid format
	resp2 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Token: "Basic abc123", Method: "ping",
	})
	if resp2.Allowed {
		t.Error("non-bearer auth should be denied")
	}

	// No token
	resp3 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "ping",
	})
	if resp3.Allowed {
		t.Error("missing token should be denied")
	}
}

func TestGatewayRateLimit(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth:      AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 5, Burst: 2},
		Enabled:   true,
	})

	// Should allow up to 5 + 2 = 7 requests
	allowed := 0
	for i := 0; i < 10; i++ {
		resp := gw.ProcessRequest(GatewayRequest{
			ClientID: "c1", Method: "tools/list",
		})
		if resp.Allowed {
			allowed++
		}
	}

	if allowed != 7 {
		t.Errorf("allowed %d, want 7 (5 limit + 2 burst)", allowed)
	}

	// Different client should not be affected
	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "c2", Method: "tools/list",
	})
	if !resp.Allowed {
		t.Error("different client should not be rate limited")
	}
}

func TestGatewayNoRateLimit(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth:      AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 0},
		Enabled:   true,
	})

	for i := 0; i < 100; i++ {
		resp := gw.ProcessRequest(GatewayRequest{
			ClientID: "c1", Method: "tools/list",
		})
		if !resp.Allowed {
			t.Fatalf("request %d should be allowed (no rate limit)", i)
		}
	}
}

func TestGatewayValidation(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth:      AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
		Validation: []ValidationRule{
			{
				ToolName:   "forge_run",
				Required:   []string{"prompt"},
				AllowedArgs: []string{"prompt", "model", "agent"},
			},
		},
	})

	// Valid request
	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/call", ToolName: "forge_run",
		Args: map[string]interface{}{"prompt": "hello", "model": "gpt-4"},
	})
	if !resp.Allowed {
		t.Errorf("valid request denied: %s", resp.Reason)
	}

	// Missing required arg
	resp2 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/call", ToolName: "forge_run",
		Args: map[string]interface{}{"model": "gpt-4"},
	})
	if resp2.Allowed {
		t.Error("missing required arg should be denied")
	}

	// Unknown arg
	resp3 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/call", ToolName: "forge_run",
		Args: map[string]interface{}{"prompt": "hi", "dangerous": true},
	})
	if resp3.Allowed {
		t.Error("unknown arg should be denied")
	}

	// Tool without validation rules should pass
	resp4 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/call", ToolName: "other_tool",
		Args: map[string]interface{}{"anything": true},
	})
	if !resp4.Allowed {
		t.Error("unvalidated tool should be allowed")
	}
}

func TestGatewayPayloadSize(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth:      AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
		Validation: []ValidationRule{
			{
				ToolName:   "upload",
				MaxPayload: 100,
			},
		},
	})

	// Small payload
	smallPayload, _ := json.Marshal(map[string]string{"data": "hello"})
	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/call", ToolName: "upload",
		Args: map[string]interface{}{"data": "hello"},
		Payload: smallPayload,
	})
	if !resp.Allowed {
		t.Errorf("small payload denied: %s", resp.Reason)
	}

	// Large payload
	largePayload := make([]byte, 200)
	for i := range largePayload {
		largePayload[i] = 'a'
	}
	resp2 := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/call", ToolName: "upload",
		Args: map[string]interface{}{"data": "big"},
		Payload: largePayload,
	})
	if resp2.Allowed {
		t.Error("large payload should be denied")
	}
}

func TestGatewayDisabled(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Enabled: false,
	})

	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "c1", Method: "tools/list",
	})
	if !resp.Allowed {
		t.Error("disabled gateway should allow all requests")
	}
}

func TestGatewayAuditLog(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth:      AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled:   true,
	})

	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/call", ToolName: "forge_run"})
	gw.ProcessRequest(GatewayRequest{ClientID: "c2", Method: "ping"})

	// All audit
	all := gw.GetAudit("", "", 0)
	if len(all) != 3 {
		t.Errorf("audit entries = %d, want 3", len(all))
	}

	// Filter by client
	c1 := gw.GetAudit("c1", "", 0)
	if len(c1) != 2 {
		t.Errorf("c1 audit = %d, want 2", len(c1))
	}

	// Filter by status
	okEntries := gw.GetAudit("", "ok", 0)
	if len(okEntries) != 3 {
		t.Errorf("ok audit = %d, want 3", len(okEntries))
	}

	// Limited
	limited := gw.GetAudit("", "", 2)
	if len(limited) != 2 {
		t.Errorf("limited audit = %d, want 2", len(limited))
	}
}

func TestGatewayDeniedAuditLog(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthToken, Tokens: []string{"valid"}},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled: true,
	})

	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list", Token: "invalid"})

	denied := gw.GetAudit("", "auth_failed", 0)
	if len(denied) != 1 {
		t.Fatalf("denied entries = %d, want 1", len(denied))
	}
	if denied[0].ClientID != "c1" {
		t.Errorf("denied client = %q, want c1", denied[0].ClientID)
	}
}

func TestGatewayStats(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthToken, Tokens: []string{"valid"}},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled: true,
	})

	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list", Token: "valid"})
	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/call", ToolName: "run", Token: "valid"})
	gw.ProcessRequest(GatewayRequest{ClientID: "c2", Method: "ping", Token: "invalid"})

	stats := gw.Stats()
	if stats.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", stats.TotalRequests)
	}
	if stats.AllowedRequests != 2 {
		t.Errorf("AllowedRequests = %d, want 2", stats.AllowedRequests)
	}
	if stats.DeniedRequests != 1 {
		t.Errorf("DeniedRequests = %d, want 1", stats.DeniedRequests)
	}
	if len(stats.TopClients) != 2 {
		t.Errorf("TopClients = %d, want 2", len(stats.TopClients))
	}
}

func TestGatewayResetRateLimits(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 2},
		Enabled: true,
	})

	// Hit the limit
	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	resp := gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	if resp.Allowed {
		t.Error("should be rate limited")
	}

	// Reset
	gw.ResetRateLimits()
	resp2 := gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	if !resp2.Allowed {
		t.Error("should be allowed after reset")
	}
}

func TestGatewayPurgeAudit(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled: true,
	})

	gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	gw.PurgeAudit()

	all := gw.GetAudit("", "", 0)
	if len(all) != 0 {
		t.Errorf("audit after purge = %d, want 0", len(all))
	}
}

func TestGatewayUpdateConfig(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthNone},
		Enabled: true,
	})

	// Initially no auth required
	resp := gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	if !resp.Allowed {
		t.Error("should be allowed with no auth")
	}

	// Switch to token auth
	gw.UpdateConfig(GatewayConfig{
		Auth: AuthConfig{Method: AuthToken, Tokens: []string{"secret"}},
		Enabled: true,
	})

	resp2 := gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})
	if resp2.Allowed {
		t.Error("should be denied after switching to token auth")
	}

	resp3 := gw.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list", Token: "secret"})
	if !resp3.Allowed {
		t.Error("should be allowed with valid token")
	}
}

func TestGatewayGetConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := GatewayConfig{
		Name: "test-gw",
		Auth: AuthConfig{Method: AuthToken, Tokens: []string{"abc"}},
		Enabled: true,
	}
	gw, _ := NewGateway(dir, cfg)

	got := gw.GetConfig()
	if got.Name != "test-gw" {
		t.Errorf("Name = %q, want test-gw", got.Name)
	}
}

func TestGatewayPersistence(t *testing.T) {
	dir := t.TempDir()
	gw1, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Enabled: true,
	})
	gw1.ProcessRequest(GatewayRequest{ClientID: "c1", Method: "tools/list"})

	// Reload
	gw2, _ := NewGateway(dir, GatewayConfig{})
	audit := gw2.GetAudit("", "", 0)
	if len(audit) != 1 {
		t.Errorf("audit after reload = %d, want 1", len(audit))
	}
}

func TestFormatAuditEntry(t *testing.T) {
	entry := AuditEntry{
		Timestamp: time.Now(),
		ClientID:  "client-1",
		Method:    "tools/call",
		ToolName:  "forge_run",
		StatusCode: "ok",
		Duration:  50 * time.Millisecond,
	}
	output := FormatAuditEntry(entry)
	if len(output) == 0 {
		t.Error("FormatAuditEntry returned empty")
	}
}

func TestFormatStats(t *testing.T) {
	stats := GatewayStats{
		TotalRequests:   100,
		AllowedRequests: 90,
		DeniedRequests:  10,
		AvgLatency:      5 * time.Millisecond,
		ActiveClients:   5,
	}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Error("FormatStats returned empty")
	}
}

func TestGatewayConcurrency(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthNone},
		RateLimit: RateLimitConfig{RequestsPerMinute: 1000},
		Enabled: true,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			gw.ProcessRequest(GatewayRequest{
				ClientID: fmt.Sprintf("client-%d", n%5),
				Method:   "tools/list",
			})
		}(i)
	}
	wg.Wait()

	stats := gw.Stats()
	if stats.TotalRequests != 50 {
		t.Errorf("TotalRequests = %d, want 50", stats.TotalRequests)
	}
}

func TestGatewayFullPipeline(t *testing.T) {
	dir := t.TempDir()
	gw, _ := NewGateway(dir, GatewayConfig{
		Auth: AuthConfig{Method: AuthToken, Tokens: []string{"secret"}},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100, Burst: 10},
		Enabled: true,
		Validation: []ValidationRule{
			{
				ToolName:   "forge_run",
				Required:   []string{"prompt"},
				AllowedArgs: []string{"prompt", "model"},
			},
		},
	})

	// Full valid request
	resp := gw.ProcessRequest(GatewayRequest{
		ClientID: "cursor",
		Token:    "secret",
		Method:   "tools/call",
		ToolName: "forge_run",
		Args:     map[string]interface{}{"prompt": "fix bug", "model": "gpt-4"},
	})
	if !resp.Allowed {
		t.Errorf("full valid request denied: %s", resp.Reason)
	}
	if resp.Headers["X-Request-ID"] == "" {
		t.Error("should have request ID header")
	}

	// Auth failure
	resp2 := gw.ProcessRequest(GatewayRequest{
		ClientID: "cursor", Method: "tools/call", ToolName: "forge_run",
		Args: map[string]interface{}{"prompt": "test"},
	})
	if resp2.Allowed {
		t.Error("missing token should be denied")
	}

	// Validation failure
	resp3 := gw.ProcessRequest(GatewayRequest{
		ClientID: "cursor", Token: "secret", Method: "tools/call", ToolName: "forge_run",
		Args: map[string]interface{}{"model": "gpt-4"}, // missing prompt
	})
	if resp3.Allowed {
		t.Error("missing required arg should be denied")
	}

	// Check audit
	all := gw.GetAudit("", "", 0)
	if len(all) != 3 {
		t.Errorf("audit = %d, want 3", len(all))
	}

	stats := gw.Stats()
	if stats.TotalRequests != 3 {
		t.Errorf("stats TotalRequests = %d, want 3", stats.TotalRequests)
	}
	if stats.DeniedRequests != 2 {
		t.Errorf("stats DeniedRequests = %d, want 2", stats.DeniedRequests)
	}
}
