package mcpdiscover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDiscoverer(t *testing.T) {
	d := NewDiscoverer()
	if d == nil {
		t.Fatal("expected non-nil discoverer")
	}
	if len(d.configDirs) == 0 {
		t.Error("expected config dirs")
	}
}

func TestDiscoverConfigFiles(t *testing.T) {
	// Create a temp config with mcpServers
	dir := t.TempDir()
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"test-server": map[string]interface{}{
				"command": "echo",
				"args":    []string{"hello"},
			},
			"http-server": map[string]interface{}{
				"url": "http://localhost:3000",
			},
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644)

	d := NewDiscoverer()
	d.configDirs = []string{dir}

	servers := d.discoverConfigFiles()
	if len(servers) < 2 {
		t.Errorf("expected at least 2 servers, got %d", len(servers))
	}

	// Check server properties
	found := map[string]bool{}
	for _, s := range servers {
		found[s.Name] = true
		if s.Source != "config" {
			t.Errorf("expected source=config, got %s", s.Source)
		}
	}
	if !found["test-server"] || !found["http-server"] {
		t.Errorf("expected test-server and http-server, got %v", found)
	}
}

func TestDiscoverRunningProcesses(t *testing.T) {
	d := NewDiscoverer()
	// This test just ensures no panic
	servers := d.discoverRunningProcesses()
	// May or may not find processes, just shouldn't error
	_ = servers
}

func TestFormatServer(t *testing.T) {
	s := &DiscoveredServer{
		Name:      "test-server",
		Protocol:  "mcp",
		Transport: "stdio",
		Command:   "mcp-server-filesystem",
		Status:    StatusReachable,
		Source:    "config",
	}
	output := FormatServer(s)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatResult(t *testing.T) {
	r := &DiscoveryResult{
		Servers: []*DiscoveredServer{
			{Name: "s1", Protocol: "mcp", Transport: "http", Address: "http://localhost:3000", Status: StatusReachable, Source: "network"},
		},
		Duration: 100000000,
		Scanned:  5,
	}
	output := FormatResult(r)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestDiscoverIntegration(t *testing.T) {
	d := NewDiscoverer()
	// Use temp dir so we don't scan real config
	d.configDirs = []string{t.TempDir()}
	result := d.Discover()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Timestamp.IsZero() {
		t.Error("expected timestamp")
	}
}

func TestDeduplication(t *testing.T) {
	dir := t.TempDir()
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"dup-server": map[string]interface{}{
				"command": "test-cmd",
			},
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644)

	d := NewDiscoverer()
	d.configDirs = []string{dir}
	result := d.Discover()

	// Count servers named dup-server
	count := 0
	for _, s := range result.Servers {
		if s.Name == "dup-server" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected at most 1 dup-server, got %d", count)
	}
}

func TestCheckServerStdio(t *testing.T) {
	d := NewDiscoverer()
	s := &DiscoveredServer{
		Name:      "echo-test",
		Transport: "stdio",
		Command:   "echo",
	}
	status := d.checkServer(s)
	if status != StatusReachable {
		t.Errorf("echo should be reachable, got %s", status)
	}
}

func TestCheckServerStdioMissing(t *testing.T) {
	d := NewDiscoverer()
	s := &DiscoveredServer{
		Name:      "missing-cmd",
		Transport: "stdio",
		Command:   "this-command-does-not-exist-xyz123",
	}
	status := d.checkServer(s)
	if status != StatusUnreachable {
		t.Errorf("expected unreachable, got %s", status)
	}
}

func TestConfigWithEnv(t *testing.T) {
	dir := t.TempDir()
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"env-server": map[string]interface{}{
				"command": "node",
				"args":    []string{"server.js"},
				"env": map[string]interface{}{
					"API_KEY": "test-key",
					"DEBUG":   "true",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644)

	d := NewDiscoverer()
	d.configDirs = []string{dir}
	servers := d.discoverConfigFiles()

	for _, s := range servers {
		if s.Name == "env-server" {
			if len(s.Env) != 2 {
				t.Errorf("expected 2 env vars, got %d", len(s.Env))
			}
			if s.Env["API_KEY"] != "test-key" {
				t.Errorf("expected API_KEY=test-key, got %s", s.Env["API_KEY"])
			}
			return
		}
	}
	t.Error("env-server not found")
}
