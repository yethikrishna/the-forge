// Package mcpgateway provides a governed MCP proxy gateway.
// It wraps an MCP server with authentication, rate limiting,
// audit logging, and schema validation for MCP v2.1 compatibility
// (Cursor/Copilot/Claude all on v2.1).
//
// The gateway guards the forge.
package mcpgateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProtocolVersion is the supported MCP protocol version.
const ProtocolVersion = "2025-03-26" // v2.1

// AuthMethod defines how clients authenticate.
type AuthMethod string

const (
	AuthNone    AuthMethod = "none"
	AuthToken   AuthMethod = "token"
	AuthAPIKey  AuthMethod = "api_key"
	AuthOAuth2  AuthMethod = "oauth2"
)

// AuthConfig configures gateway authentication.
type AuthConfig struct {
	Method    AuthMethod `json:"method"`
	// Valid tokens/keys for token and api_key auth.
	Tokens   []string   `json:"tokens,omitempty"`
	// OIDC discovery URL for oauth2.
	OIDCURL   string    `json:"oidc_url,omitempty"`
}

// RateLimitConfig configures per-client rate limiting.
type RateLimitConfig struct {
	// Max requests per minute.
	RequestsPerMinute int `json:"requests_per_minute"`
	// Max tokens per minute (for tool call payloads).
	TokensPerMinute int `json:"tokens_per_minute"`
	// Burst size (allow brief spikes).
	Burst int `json:"burst"`
}

// AuditEntry records a gateway request for auditing.
type AuditEntry struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	ClientID   string            `json:"client_id"`
	Method     string            `json:"method"`
	ToolName   string            `json:"tool_name,omitempty"`
	StatusCode string            `json:"status_code"` // "ok", "auth_failed", "rate_limited", "validation_failed", "error"
	Duration   time.Duration     `json:"duration"`
	Error      string            `json:"error,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// RateLimitEntry tracks rate limit state for a client.
type rateLimitEntry struct {
	count     int
	tokenCount int
	windowStart time.Time
}

// ValidationRule defines schema validation for a tool.
type ValidationRule struct {
	ToolName    string   `json:"tool_name"`
	Required    []string `json:"required,omitempty"`
	MaxPayload  int      `json:"max_payload,omitempty"` // max payload size in bytes
	AllowedArgs []string `json:"allowed_args,omitempty"` // if set, reject unknown args
}

// GatewayConfig is the full gateway configuration.
type GatewayConfig struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Auth        AuthConfig        `json:"auth"`
	RateLimit   RateLimitConfig   `json:"rate_limit"`
	Validation  []ValidationRule  `json:"validation,omitempty"`
	Enabled     bool              `json:"enabled"`
}

// Gateway is the governed MCP proxy.
type Gateway struct {
	mu         sync.RWMutex
	dir        string
	config     GatewayConfig
	rateLimits map[string]*rateLimitEntry
	audit      []AuditEntry
	maxAudit   int
}

// NewGateway creates a new governed MCP gateway.
func NewGateway(dir string, config GatewayConfig) (*Gateway, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create gateway dir: %w", err)
	}
	g := &Gateway{
		dir:        dir,
		config:     config,
		rateLimits: make(map[string]*rateLimitEntry),
		audit:      make([]AuditEntry, 0),
		maxAudit:   1000,
	}
	g.load()
	return g, nil
}

// GatewayRequest is an incoming request through the gateway.
type GatewayRequest struct {
	ClientID   string                 `json:"client_id"`
	RemoteAddr string                 `json:"remote_addr"`
	Token      string                 `json:"token,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
	Method     string                 `json:"method"`
	Args       map[string]interface{} `json:"args,omitempty"`
	Payload    json.RawMessage        `json:"payload,omitempty"`
}

// GatewayResponse is the gateway's response.
type GatewayResponse struct {
	Allowed   bool                   `json:"allowed"`
	Reason    string                 `json:"reason,omitempty"`
	RequestID string                 `json:"request_id"`
	Headers   map[string]string      `json:"headers,omitempty"`
}

// ProcessRequest evaluates a request through the gateway pipeline:
// 1. Check if gateway is enabled
// 2. Authenticate
// 3. Rate limit check
// 4. Validate schema
// 5. Audit log
func (g *Gateway) ProcessRequest(req GatewayRequest) GatewayResponse {
	start := time.Now()
	reqID := fmt.Sprintf("gw-%d", time.Now().UnixNano())

	resp := GatewayResponse{
		RequestID: reqID,
		Headers:   make(map[string]string),
	}

	// Step 1: Check enabled
	if !g.config.Enabled {
		g.auditLog(AuditEntry{
			ID: reqID, Timestamp: start, ClientID: req.ClientID,
			Method: req.Method, StatusCode: "ok", Duration: time.Since(start),
			RemoteAddr: req.RemoteAddr,
		})
		resp.Allowed = true
		return resp
	}

	// Step 2: Authenticate
	if err := g.authenticate(req); err != nil {
		resp.Allowed = false
		resp.Reason = fmt.Sprintf("authentication failed: %v", err)
		g.auditLog(AuditEntry{
			ID: reqID, Timestamp: start, ClientID: req.ClientID,
			Method: req.Method, StatusCode: "auth_failed", Duration: time.Since(start),
			Error: resp.Reason, RemoteAddr: req.RemoteAddr,
		})
		return resp
	}

	// Step 3: Rate limit
	if err := g.checkRateLimit(req.ClientID); err != nil {
		resp.Allowed = false
		resp.Reason = fmt.Sprintf("rate limited: %v", err)
		resp.Headers["Retry-After"] = "60"
		g.auditLog(AuditEntry{
			ID: reqID, Timestamp: start, ClientID: req.ClientID,
			Method: req.Method, StatusCode: "rate_limited", Duration: time.Since(start),
			Error: resp.Reason, RemoteAddr: req.RemoteAddr,
		})
		return resp
	}

	// Step 4: Validate
	if req.ToolName != "" {
		if err := g.validate(req); err != nil {
			resp.Allowed = false
			resp.Reason = fmt.Sprintf("validation failed: %v", err)
			g.auditLog(AuditEntry{
				ID: reqID, Timestamp: start, ClientID: req.ClientID,
				Method: req.Method, ToolName: req.ToolName,
				StatusCode: "validation_failed", Duration: time.Since(start),
				Error: resp.Reason, RemoteAddr: req.RemoteAddr,
			})
			return resp
		}
	}

	// All checks passed
	g.auditLog(AuditEntry{
		ID: reqID, Timestamp: start, ClientID: req.ClientID,
		Method: req.Method, ToolName: req.ToolName,
		StatusCode: "ok", Duration: time.Since(start),
		RemoteAddr: req.RemoteAddr,
	})

	resp.Allowed = true
	resp.Headers["X-Request-ID"] = reqID
	resp.Headers["X-RateLimit-Limit"] = fmt.Sprintf("%d", g.config.RateLimit.RequestsPerMinute)
	return resp
}

// authenticate checks client credentials.
func (g *Gateway) authenticate(req GatewayRequest) error {
	switch g.config.Auth.Method {
	case AuthNone:
		return nil

	case AuthToken, AuthAPIKey:
		if req.Token == "" {
			return fmt.Errorf("missing authentication token")
		}
		for _, valid := range g.config.Auth.Tokens {
			if req.Token == valid {
				return nil
			}
		}
		return fmt.Errorf("invalid token")

	case AuthOAuth2:
		// OAuth2 validation would check the bearer token against OIDC
		if req.Token == "" {
			return fmt.Errorf("missing bearer token")
		}
		if !strings.HasPrefix(req.Token, "Bearer ") {
			return fmt.Errorf("invalid bearer token format")
		}
		return nil

	default:
		return fmt.Errorf("unknown auth method: %s", g.config.Auth.Method)
	}
}

// checkRateLimit enforces per-client rate limits.
func (g *Gateway) checkRateLimit(clientID string) error {
	cfg := g.config.RateLimit
	if cfg.RequestsPerMinute <= 0 {
		return nil // no rate limit configured
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	entry, ok := g.rateLimits[clientID]
	if !ok {
		g.rateLimits[clientID] = &rateLimitEntry{
			count:       1,
			windowStart: time.Now(),
		}
		return nil
	}

	now := time.Now()
	if now.Sub(entry.windowStart) >= time.Minute {
		// Reset window
		entry.count = 1
		entry.windowStart = now
		return nil
	}

	entry.count++
	if entry.count > cfg.RequestsPerMinute+cfg.Burst {
		return fmt.Errorf("exceeded %d requests/minute (burst: %d)", cfg.RequestsPerMinute, cfg.Burst)
	}

	return nil
}

// validate checks request against validation rules.
func (g *Gateway) validate(req GatewayRequest) error {
	for _, rule := range g.config.Validation {
		if rule.ToolName != req.ToolName {
			continue
		}

		// Check required args
		for _, reqArg := range rule.Required {
			val, ok := req.Args[reqArg]
			if !ok || val == nil {
				return fmt.Errorf("missing required argument: %s", reqArg)
			}
		}

		// Check allowed args (reject unknown)
		if len(rule.AllowedArgs) > 0 {
			allowedSet := make(map[string]bool)
			for _, a := range rule.AllowedArgs {
				allowedSet[a] = true
			}
			for key := range req.Args {
				if !allowedSet[key] {
					return fmt.Errorf("unknown argument: %s", key)
				}
			}
		}

		// Check payload size
		if rule.MaxPayload > 0 && len(req.Payload) > rule.MaxPayload {
			return fmt.Errorf("payload too large: %d bytes (max: %d)", len(req.Payload), rule.MaxPayload)
		}
	}

	return nil
}

// auditLog records an audit entry.
func (g *Gateway) auditLog(entry AuditEntry) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.audit = append(g.audit, entry)
	if len(g.audit) > g.maxAudit {
		g.audit = g.audit[len(g.audit)-g.maxAudit:]
	}
	g.save()
}

// GetAudit returns audit log, optionally filtered.
func (g *Gateway) GetAudit(clientID, statusCode string, limit int) []AuditEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []AuditEntry
	for i := len(g.audit) - 1; i >= 0; i-- {
		e := g.audit[i]
		if clientID != "" && e.ClientID != clientID {
			continue
		}
		if statusCode != "" && e.StatusCode != statusCode {
			continue
		}
		result = append(result, e)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// Stats returns gateway statistics.
type GatewayStats struct {
	TotalRequests   int                `json:"total_requests"`
	AllowedRequests int                `json:"allowed_requests"`
	DeniedRequests  int                `json:"denied_requests"`
	ByStatus        map[string]int     `json:"by_status"`
	ByMethod        map[string]int     `json:"by_method"`
	TopClients      []ClientStat       `json:"top_clients"`
	AvgLatency      time.Duration      `json:"avg_latency"`
	ActiveClients   int                `json:"active_clients"`
}

// ClientStat is a per-client statistics entry.
type ClientStat struct {
	ClientID string `json:"client_id"`
	Requests int    `json:"requests"`
	Denied   int    `json:"denied"`
}

// Stats returns aggregate gateway statistics.
func (g *Gateway) Stats() GatewayStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	s := GatewayStats{
		ByStatus: make(map[string]int),
		ByMethod: make(map[string]int),
	}

	clientStats := make(map[string]*ClientStat)
	var totalDuration time.Duration

	for _, e := range g.audit {
		s.TotalRequests++
		s.ByStatus[e.StatusCode]++
		s.ByMethod[e.Method]++
		totalDuration += e.Duration

		cs, ok := clientStats[e.ClientID]
		if !ok {
			cs = &ClientStat{ClientID: e.ClientID}
			clientStats[e.ClientID] = cs
		}
		cs.Requests++
		if e.StatusCode != "ok" {
			s.DeniedRequests++
			cs.Denied++
		} else {
			s.AllowedRequests++
		}
	}

	if s.TotalRequests > 0 {
		s.AvgLatency = totalDuration / time.Duration(s.TotalRequests)
	}
	s.ActiveClients = len(g.rateLimits)

	var clients []ClientStat
	for _, cs := range clientStats {
		clients = append(clients, *cs)
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Requests > clients[j].Requests
	})
	if len(clients) > 10 {
		clients = clients[:10]
	}
	s.TopClients = clients

	return s
}

// UpdateConfig updates the gateway configuration.
func (g *Gateway) UpdateConfig(config GatewayConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.config = config
	g.save()
}

// GetConfig returns the current configuration.
func (g *Gateway) GetConfig() GatewayConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config
}

// ResetRateLimits clears all rate limit counters.
func (g *Gateway) ResetRateLimits() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rateLimits = make(map[string]*rateLimitEntry)
}

// PurgeAudit removes all audit entries.
func (g *Gateway) PurgeAudit() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.audit = make([]AuditEntry, 0)
	g.save()
}

func (g *Gateway) load() {
	data, err := os.ReadFile(filepath.Join(g.dir, "audit.json"))
	if err == nil {
		json.Unmarshal(data, &g.audit)
	}
	cdata, err := os.ReadFile(filepath.Join(g.dir, "config.json"))
	if err == nil {
		json.Unmarshal(cdata, &g.config)
	}
}

func (g *Gateway) save() {
	data, _ := json.MarshalIndent(g.audit, "", "  ")
	os.WriteFile(filepath.Join(g.dir, "audit.json"), data, 0644)

	cdata, _ := json.MarshalIndent(g.config, "", "  ")
	os.WriteFile(filepath.Join(g.dir, "config.json"), cdata, 0644)
}

// FormatAuditEntry formats an audit entry for display.
func FormatAuditEntry(e AuditEntry) string {
	status := "✓"
	if e.StatusCode != "ok" {
		status = "✗"
	}
	line := fmt.Sprintf("  %s [%s] %s → %s (%s) %s",
		status, e.Timestamp.Format("15:04:05"), e.ClientID, e.Method, e.StatusCode,
		e.Duration.Round(time.Microsecond))
	if e.Error != "" {
		line += fmt.Sprintf("\n    %s", e.Error)
	}
	if e.ToolName != "" {
		line = fmt.Sprintf("  %s [%s] %s → %s/%s (%s) %s",
			status, e.Timestamp.Format("15:04:05"), e.ClientID, e.Method, e.ToolName,
			e.StatusCode, e.Duration.Round(time.Microsecond))
	}
	return line
}

// FormatStats formats gateway stats for display.
func FormatStats(s GatewayStats) string {
	out := fmt.Sprintf("  Requests: %d (allowed: %d, denied: %d)\n", s.TotalRequests, s.AllowedRequests, s.DeniedRequests)
	out += fmt.Sprintf("  Avg latency: %s\n", s.AvgLatency.Round(time.Microsecond))
	out += fmt.Sprintf("  Active clients: %d\n", s.ActiveClients)

	if len(s.TopClients) > 0 {
		out += "\n  Top Clients:\n"
		for _, c := range s.TopClients {
			out += fmt.Sprintf("    %-20s %d requests (%d denied)\n", c.ClientID, c.Requests, c.Denied)
		}
	}
	return out
}
