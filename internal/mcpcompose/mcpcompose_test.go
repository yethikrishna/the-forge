package mcpcompose

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComposeGateway(t *testing.T) {
	gateway := NewComposeGateway(ComposeConfig{
		Gateway: GatewayConfig{Addr: "localhost:0"},
		Middleware: MiddlewareConfig{
			AuditLogging: true,
			RetryEnabled: true,
			MaxRetries:   1,
		},
	})

	err := gateway.AddServer(ServerConfig{
		Name:    "test-server",
		URL:     "http://localhost:9999",
		Enabled: true,
		Prefix:  "test",
	})
	if err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	servers := gateway.ListServers()
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "test-server" {
		t.Fatalf("expected test-server, got %s", servers[0].Name)
	}
}

func TestComposeGatewayRemoveServer(t *testing.T) {
	gateway := NewComposeGateway(ComposeConfig{})

	gateway.AddServer(ServerConfig{Name: "s1", Enabled: true})
	gateway.AddServer(ServerConfig{Name: "s2", Enabled: true})

	if err := gateway.RemoveServer("s1"); err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}

	servers := gateway.ListServers()
	if len(servers) != 1 || servers[0].Name != "s2" {
		t.Fatalf("expected only s2, got %v", servers)
	}
}

func TestComposeConfigLoad(t *testing.T) {
	dir := t.TempDir()
	config := ComposeConfig{
		Servers: []ServerConfig{
			{Name: "github", Command: "mcp-server-github", Enabled: true},
			{Name: "filesystem", URL: "http://localhost:8080", Enabled: true, Prefix: "fs"},
		},
		Gateway: GatewayConfig{Addr: "localhost:9090"},
		Middleware: MiddlewareConfig{
			AuditLogging: true,
			CostTracking: true,
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	path := filepath.Join(dir, "compose.json")
	os.WriteFile(path, data, 0o644)

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(loaded.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(loaded.Servers))
	}
	if loaded.Servers[0].Name != "github" {
		t.Fatalf("expected github, got %s", loaded.Servers[0].Name)
	}
	if loaded.Servers[1].Prefix != "fs" {
		t.Fatalf("expected fs prefix, got %s", loaded.Servers[1].Prefix)
	}
}

func TestToolIndexBuilding(t *testing.T) {
	gateway := NewComposeGateway(ComposeConfig{})

	upstream := &UpstreamServer{
		Config: ServerConfig{Name: "myserver", Prefix: "my"},
		Healthy: true,
		Tools: []ToolInfo{
			{Name: "read", Description: "Read a file", Server: "myserver", PrefixedName: "my_read"},
			{Name: "write", Description: "Write a file", Server: "myserver", PrefixedName: "my_write"},
		},
		client: &http.Client{Timeout: 5 * time.Second},
	}

	gateway.mu.Lock()
	gateway.upstreams["myserver"] = upstream
	for _, tool := range upstream.Tools {
		gateway.toolIndex[tool.PrefixedName] = upstream
	}
	gateway.mu.Unlock()

	tools := gateway.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	// Verify tool index
	if _, ok := gateway.toolIndex["my_read"]; !ok {
		t.Fatal("my_read not in tool index")
	}
}

func TestCallToolMissing(t *testing.T) {
	gateway := NewComposeGateway(ComposeConfig{})

	_, err := gateway.CallTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestFormatComposeStatus(t *testing.T) {
	servers := []ServerStatus{
		{Name: "github", Transport: "stdio", Endpoint: "mcp-server-github", Healthy: true, ToolCount: 5, Enabled: true},
		{Name: "fs", Transport: "http", Endpoint: "http://localhost:8080", Healthy: false, ToolCount: 3, Enabled: true},
		{Name: "disabled", Transport: "http", Endpoint: "http://localhost:9090", Healthy: false, ToolCount: 0, Enabled: false},
	}

	output := FormatComposeStatus(servers, 8)
	if len(output) == 0 {
		t.Fatal("empty status output")
	}
}

func TestFormatTools(t *testing.T) {
	tools := []ToolInfo{
		{Name: "read", Description: "Read files", Server: "fs", PrefixedName: "fs_read"},
		{Name: "search", Description: "Search code", Server: "github", PrefixedName: "github_search"},
	}

	output := FormatTools(tools)
	if len(output) == 0 {
		t.Fatal("empty tools output")
	}
}

func TestAuditLog(t *testing.T) {
	gateway := NewComposeGateway(ComposeConfig{
		Middleware: MiddlewareConfig{AuditLogging: true},
	})

	// Manually add audit entries
	gateway.mu.Lock()
	gateway.auditLog = append(gateway.auditLog, AuditEntry{
		Timestamp: time.Now(),
		Server:    "test",
		Tool:      "read",
		Duration:  50 * time.Millisecond,
		Status:    "ok",
	})
	gateway.mu.Unlock()

	log := gateway.AuditLog()
	if len(log) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(log))
	}
	if log[0].Tool != "read" {
		t.Fatalf("expected read tool, got %s", log[0].Tool)
	}
}

func TestMiddlewareDefaults(t *testing.T) {
	gateway := NewComposeGateway(ComposeConfig{})
	if gateway.config.Gateway.Addr != "localhost:9090" {
		t.Fatalf("expected localhost:9090, got %s", gateway.config.Gateway.Addr)
	}
	if gateway.config.Gateway.ReadTimeout != 30*time.Second {
		t.Fatalf("expected 30s read timeout, got %v", gateway.config.Gateway.ReadTimeout)
	}
}
