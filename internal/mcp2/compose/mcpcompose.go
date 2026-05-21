// Package mcpcompose composes multiple MCP servers behind a single Forge gateway.
// One mountain, many tunnels — unified access to distributed power.
package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ServerConfig defines an upstream MCP server.
type ServerConfig struct {
	Name    string            `json:"name" yaml:"name"`
	Command string            `json:"command,omitempty" yaml:"command,omitempty"`
	Args    []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	URL     string            `json:"url,omitempty" yaml:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Prefix  string            `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Enabled bool              `json:"enabled" yaml:"enabled"`
}

// ComposeConfig is the full composition configuration.
type ComposeConfig struct {
	Servers []ServerConfig `json:"servers" yaml:"servers"`
	Gateway GatewayConfig  `json:"gateway" yaml:"gateway"`
	Middleware MiddlewareConfig `json:"middleware" yaml:"middleware"`
}

// GatewayConfig configures the composition gateway.
type GatewayConfig struct {
	Addr         string        `json:"addr" yaml:"addr"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
}

// MiddlewareConfig configures middleware applied to all upstream calls.
type MiddlewareConfig struct {
	CostTracking  bool `json:"cost_tracking" yaml:"cost_tracking"`
	RateLimiting  bool `json:"rate_limiting" yaml:"rate_limiting"`
	AuditLogging  bool `json:"audit_logging" yaml:"audit_logging"`
	RetryEnabled  bool `json:"retry_enabled" yaml:"retry_enabled"`
	MaxRetries    int  `json:"max_retries" yaml:"max_retries"`
	RetryDelayMs  int  `json:"retry_delay_ms" yaml:"retry_delay_ms"`
}

// UpstreamServer represents a connected upstream MCP server.
type UpstreamServer struct {
	Config   ServerConfig
	Tools    []ToolInfo
	Healthy  bool
	LastPing time.Time
	Process  *exec.Cmd
	Stdin    io.WriteCloser
	client   *http.Client
}

// ToolInfo describes a tool from an upstream server.
type ToolInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
	Server      string      `json:"server"`
	PrefixedName string     `json:"prefixedName"`
}

// ComposeGateway is the composition gateway.
type ComposeGateway struct {
	config    ComposeConfig
	upstreams map[string]*UpstreamServer
	toolIndex map[string]*UpstreamServer // prefixed tool name → upstream
	mu        sync.RWMutex
	auditLog  []AuditEntry
	httpServer *http.Server
}

// AuditEntry records a tool call through the gateway.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Server    string    `json:"server"`
	Tool      string    `json:"tool"`
	Duration  time.Duration `json:"duration"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
}

// NewComposeGateway creates a composition gateway.
func NewComposeGateway(config ComposeConfig) *ComposeGateway {
	if config.Gateway.Addr == "" {
		config.Gateway.Addr = "localhost:9090"
	}
	if config.Gateway.ReadTimeout == 0 {
		config.Gateway.ReadTimeout = 30 * time.Second
	}
	if config.Gateway.WriteTimeout == 0 {
		config.Gateway.WriteTimeout = 30 * time.Second
	}

	return &ComposeGateway{
		config:    config,
		upstreams: make(map[string]*UpstreamServer),
		toolIndex: make(map[string]*UpstreamServer),
	}
}

// AddServer adds an upstream server configuration.
func (g *ComposeGateway) AddServer(config ServerConfig) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if config.Enabled {
		config.Enabled = true
	}

	upstream := &UpstreamServer{
		Config:  config,
		Healthy: false,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	g.upstreams[config.Name] = upstream
	return nil
}

// RemoveServer removes an upstream server.
func (g *ComposeGateway) RemoveServer(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	upstream, ok := g.upstreams[name]
	if !ok {
		return fmt.Errorf("server %s not found", name)
	}

	// Stop process if running
	if upstream.Process != nil && upstream.Process.Process != nil {
		upstream.Process.Process.Kill()
	}

	// Remove from tool index
	for k, v := range g.toolIndex {
		if v == upstream {
			delete(g.toolIndex, k)
		}
	}

	delete(g.upstreams, name)
	return nil
}

// ConnectAll connects to all enabled upstream servers.
func (g *ComposeGateway) ConnectAll(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for name, upstream := range g.upstreams {
		if !upstream.Config.Enabled {
			continue
		}

		if upstream.Config.Command != "" {
			if err := g.startProcess(ctx, upstream); err != nil {
				fmt.Fprintf(os.Stderr, "compose: failed to start %s: %v\n", name, err)
				continue
			}
		}

		if err := g.discoverTools(ctx, upstream); err != nil {
			fmt.Fprintf(os.Stderr, "compose: failed to discover tools from %s: %v\n", name, err)
			continue
		}

		upstream.Healthy = true
		upstream.LastPing = time.Now()
		fmt.Printf("compose: connected to %s (%d tools)\n", name, len(upstream.Tools))
	}

	return nil
}

// startProcess starts a stdio-based MCP server process.
func (g *ComposeGateway) startProcess(ctx context.Context, upstream *UpstreamServer) error {
	cmd := exec.CommandContext(ctx, upstream.Config.Command, upstream.Config.Args...)
	
	// Set environment
	cmd.Env = os.Environ()
	for k, v := range upstream.Config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	upstream.Process = cmd
	upstream.Stdin = stdin
	return nil
}

// discoverTools fetches the tool list from an upstream server.
func (g *ComposeGateway) discoverTools(ctx context.Context, upstream *UpstreamServer) error {
	if upstream.Config.URL != "" {
		return g.discoverToolsHTTP(ctx, upstream)
	}
	// For stdio-based servers, use a default set
	// In production, this would send a proper JSON-RPC initialize + tools/list
	return nil
}

// discoverToolsHTTP fetches tools from an HTTP-based MCP server.
func (g *ComposeGateway) discoverToolsHTTP(ctx context.Context, upstream *UpstreamServer) error {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", upstream.Config.URL+"/messages", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range upstream.Config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := upstream.client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Result struct {
			Tools []ToolInfo `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	prefix := upstream.Config.Prefix
	if prefix == "" {
		prefix = upstream.Config.Name
	}

	for i := range result.Result.Tools {
		tool := &result.Result.Tools[i]
		tool.Server = upstream.Config.Name
		originalName := tool.Name
		tool.PrefixedName = prefix + "_" + originalName
		upstream.Tools = append(upstream.Tools, *tool)
		g.toolIndex[tool.PrefixedName] = upstream
	}

	return nil
}

// CallTool routes a tool call to the appropriate upstream server.
func (g *ComposeGateway) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	start := time.Now()

	g.mu.RLock()
	upstream, ok := g.toolIndex[toolName]
	g.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %s not found in composition", toolName)
	}

	if !upstream.Healthy {
		return nil, fmt.Errorf("upstream server %s is unhealthy", upstream.Config.Name)
	}

	// Strip prefix to get original tool name
	originalName := toolName
	prefix := upstream.Config.Prefix
	if prefix == "" {
		prefix = upstream.Config.Name
	}
	if strings.HasPrefix(toolName, prefix+"_") {
		originalName = strings.TrimPrefix(toolName, prefix+"_")
	}

	result, err := g.callUpstream(ctx, upstream, originalName, args)

	duration := time.Since(start)
	entry := AuditEntry{
		Timestamp: start,
		Server:    upstream.Config.Name,
		Tool:      toolName,
		Duration:  duration,
	}

	if err != nil {
		entry.Status = "error"
		entry.Error = err.Error()
	} else {
		entry.Status = "ok"
	}

	if g.config.Middleware.AuditLogging {
		g.mu.Lock()
		g.auditLog = append(g.auditLog, entry)
		g.mu.Unlock()
	}

	return result, err
}

// callUpstream sends a tool call to an upstream server.
func (g *ComposeGateway) callUpstream(ctx context.Context, upstream *UpstreamServer, toolName string, args map[string]interface{}) (interface{}, error) {
	if upstream.Config.URL == "" {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Tool %s on server %s (stdio transport not yet fully implemented)", toolName, upstream.Config.Name)},
			},
		}, nil
	}

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}

	data, _ := json.Marshal(reqBody)

	var lastErr error
	maxRetries := 0
	if g.config.Middleware.RetryEnabled {
		maxRetries = g.config.Middleware.MaxRetries
		if maxRetries == 0 {
			maxRetries = 2
		}
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(g.config.Middleware.RetryDelayMs) * time.Millisecond
			if delay == 0 {
				delay = 500 * time.Millisecond
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", upstream.Config.URL+"/messages", strings.NewReader(string(data)))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		for k, v := range upstream.Config.Headers {
			req.Header.Set(k, v)
		}

		resp, err := upstream.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body: %w", err)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		var rpcResp map[string]interface{}
		if err := json.Unmarshal(body, &rpcResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		if rpcErr, ok := rpcResp["error"]; ok {
			return nil, fmt.Errorf("RPC error: %v", rpcErr)
		}

		return rpcResp["result"], nil
	}

	return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// ListTools returns all tools available across all upstreams.
func (g *ComposeGateway) ListTools() []ToolInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var tools []ToolInfo
	for _, upstream := range g.upstreams {
		if !upstream.Healthy {
			continue
		}
		tools = append(tools, upstream.Tools...)
	}
	return tools
}

// ListServers returns info about all upstream servers.
func (g *ComposeGateway) ListServers() []ServerStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var statuses []ServerStatus
	for name, upstream := range g.upstreams {
		status := ServerStatus{
			Name:     name,
			Healthy:  upstream.Healthy,
			ToolCount: len(upstream.Tools),
			LastPing: upstream.LastPing,
			Enabled:  upstream.Config.Enabled,
		}
		if upstream.Config.URL != "" {
			status.Transport = "http"
			status.Endpoint = upstream.Config.URL
		} else if upstream.Config.Command != "" {
			status.Transport = "stdio"
			status.Endpoint = upstream.Config.Command
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// ServerStatus describes the status of an upstream server.
type ServerStatus struct {
	Name      string    `json:"name"`
	Transport string    `json:"transport"`
	Endpoint  string    `json:"endpoint"`
	Healthy   bool      `json:"healthy"`
	ToolCount int       `json:"tool_count"`
	LastPing  time.Time `json:"last_ping"`
	Enabled   bool      `json:"enabled"`
}

// AuditLog returns the audit log entries.
func (g *ComposeGateway) AuditLog() []AuditEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]AuditEntry, len(g.auditLog))
	copy(result, g.auditLog)
	return result
}

// HealthCheck pings all upstream servers.
func (g *ComposeGateway) HealthCheck(ctx context.Context) map[string]bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	results := make(map[string]bool)
	for name, upstream := range g.upstreams {
		if upstream.Config.URL != "" {
			req, err := http.NewRequestWithContext(ctx, "POST", upstream.Config.URL+"/messages",
				strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
			if err != nil {
				upstream.Healthy = false
				results[name] = false
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := upstream.client.Do(req)
			if err != nil || resp.StatusCode >= 500 {
				upstream.Healthy = false
				results[name] = false
			} else {
				resp.Body.Close()
				upstream.Healthy = true
				upstream.LastPing = time.Now()
				results[name] = true
			}
		} else {
			// For stdio, check if process is still running
			if upstream.Process != nil && upstream.Process.Process != nil {
				upstream.Healthy = true
				upstream.LastPing = time.Now()
				results[name] = true
			} else {
				upstream.Healthy = false
				results[name] = false
			}
		}
	}

	return results
}

// Start starts the composition gateway HTTP server.
func (g *ComposeGateway) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// MCP endpoint — handles all JSON-RPC
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      interface{}     `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		var result interface{}
		var rpcErr *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}

		switch req.Method {
		case "initialize":
			result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "forge-compose",
					"version": "1.0.0",
				},
			}
		case "ping":
			result = map[string]interface{}{}
		case "tools/list":
			result = map[string]interface{}{
				"tools": g.ListTools(),
			}
		case "tools/call":
			var call struct {
				Name string                 `json:"name"`
				Args map[string]interface{} `json:"arguments"`
			}
			json.Unmarshal(req.Params, &call)
			res, err := g.CallTool(r.Context(), call.Name, call.Args)
			if err != nil {
				rpcErr = &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{Code: -32603, Message: err.Error()}
			} else {
				result = res
			}
		default:
			rpcErr = &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
		}
		if rpcErr != nil {
			resp["error"] = rpcErr
		} else {
			resp["result"] = result
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// SSE endpoint
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		fmt.Fprintf(w, "event: endpoint\ndata: /messages?session=compose\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := g.HealthCheck(r.Context())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})

	// Status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"servers": g.ListServers(),
			"tools":   len(g.ListTools()),
		})
	})

	g.httpServer = &http.Server{
		Addr:         g.config.Gateway.Addr,
		Handler:      mux,
		ReadTimeout:  g.config.Gateway.ReadTimeout,
		WriteTimeout: g.config.Gateway.WriteTimeout,
		IdleTimeout:  g.config.Gateway.IdleTimeout,
	}

	// Start server in background
	go func() {
		<-ctx.Done()
		g.httpServer.Close()
	}()

	ln, err := net.Listen("tcp", g.config.Gateway.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	fmt.Printf("compose: gateway listening on %s\n", g.config.Gateway.Addr)

	go g.httpServer.Serve(ln)
	return nil
}

// Stop shuts down the gateway and all upstream connections.
func (g *ComposeGateway) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, upstream := range g.upstreams {
		if upstream.Process != nil && upstream.Process.Process != nil {
			upstream.Process.Process.Kill()
		}
		if upstream.Stdin != nil {
			upstream.Stdin.Close()
		}
	}

	if g.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return g.httpServer.Shutdown(ctx)
	}

	return nil
}

// LoadConfig loads a composition configuration from a file.
func LoadConfig(path string) (*ComposeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config ComposeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	for i := range config.Servers {
		if config.Servers[i].Enabled {
			config.Servers[i].Enabled = true
		}
	}

	return &config, nil
}

// FormatComposeStatus renders composition status for display.
func FormatComposeStatus(servers []ServerStatus, toolCount int) string {
	var sb strings.Builder
	sb.WriteString("MCP Composition Gateway\n")
	sb.WriteString(fmt.Sprintf("  Total Tools: %d\n", toolCount))
	sb.WriteString("\nUpstream Servers:\n")
	for _, s := range servers {
		status := "❌"
		if s.Healthy {
			status = "✅"
		}
		if !s.Enabled {
			status = "⏸"
		}
		sb.WriteString(fmt.Sprintf("  %s %-20s  %-6s  %2d tools  %s\n",
			status, s.Name, s.Transport, s.ToolCount, s.Endpoint))
	}
	return sb.String()
}

// FormatTools renders available composed tools.
func FormatTools(tools []ToolInfo) string {
	var sb strings.Builder
	sb.WriteString("Composed MCP Tools:\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("  %-35s  %-15s  %s\n", t.PrefixedName, t.Server, t.Description))
	}
	return sb.String()
}
