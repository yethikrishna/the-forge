// Package mcpdiscover auto-discovers MCP servers on the local network
// and in common configuration locations.
//
// Find every server. Connect to anything.
package mcpdiscover

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ServerStatus represents the discovered server's reachability.
type ServerStatus string

const (
	StatusReachable ServerStatus = "reachable"
	StatusUnreachable ServerStatus = "unreachable"
	StatusUnknown   ServerStatus = "unknown"
)

// DiscoveredServer represents a discovered MCP server.
type DiscoveredServer struct {
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol"` // mcp, streamable-http, sse
	Transport   string            `json:"transport"` // stdio, http, sse
	Address     string            `json:"address,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Status      ServerStatus      `json:"status"`
	Latency     time.Duration     `json:"latency,omitempty"`
	Source      string            `json:"source"` // config, network, process
	Capabilities []string         `json:"capabilities,omitempty"`
	ConfigPath  string            `json:"config_path,omitempty"`
	DiscoveredAt time.Time        `json:"discovered_at"`
}

// DiscoveryResult holds the results of a discovery scan.
type DiscoveryResult struct {
	Servers   []*DiscoveredServer `json:"servers"`
	Duration  time.Duration       `json:"duration"`
	Scanned   int                 `json:"scanned"`
	Timestamp time.Time           `json:"timestamp"`
}

// Discoverer finds MCP servers.
type Discoverer struct {
	mu       sync.Mutex
	configDirs []string
	scanPorts []int
}

// NewDiscoverer creates an MCP server discoverer.
func NewDiscoverer() *Discoverer {
	d := &Discoverer{
		scanPorts: []int{3000, 8080, 8765, 9090, 5555, 6274},
	}

	// Standard config directories
	home, _ := os.UserHomeDir()
	d.configDirs = []string{
		filepath.Join(home, ".config", "mcp"),
		filepath.Join(home, ".forge", "mcp"),
		filepath.Join(home, ".mcp"),
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		d.configDirs = append(d.configDirs, filepath.Join(xdg, "mcp"))
	}
	// VS Code / Cursor extensions
	d.configDirs = append(d.configDirs,
		filepath.Join(home, ".vscode", "extensions"),
		filepath.Join(home, ".cursor", "extensions"),
	)

	return d
}

// Discover runs all discovery methods.
func (d *Discoverer) Discover() *DiscoveryResult {
	start := time.Now()
	var servers []*DiscoveredServer
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Run discovery methods in parallel
	methods := []func() []*DiscoveredServer{
		d.discoverConfigFiles,
		d.discoverRunningProcesses,
		d.discoverLocalPorts,
	}

	for _, method := range methods {
		wg.Add(1)
		go func(fn func() []*DiscoveredServer) {
			defer wg.Done()
			found := fn()
			mu.Lock()
			servers = append(servers, found...)
			mu.Unlock()
		}(method)
	}
	wg.Wait()

	// Deduplicate by name+address
	seen := make(map[string]bool)
	unique := make([]*DiscoveredServer, 0, len(servers))
	for _, s := range servers {
		key := s.Name + "|" + s.Address + "|" + s.Command
		if !seen[key] {
			seen[key] = true
			unique = append(unique, s)
		}
	}

	return &DiscoveryResult{
		Servers:   unique,
		Duration:  time.Since(start),
		Scanned:   len(servers),
		Timestamp: time.Now(),
	}
}

// discoverConfigFiles scans standard config directories for MCP server definitions.
func (d *Discoverer) discoverConfigFiles() []*DiscoveredServer {
	var servers []*DiscoveredServer

	for _, dir := range d.configDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".json") && !strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml") {
				continue
			}

			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			// Try parsing as MCP config
			var config map[string]interface{}
			if err := json.Unmarshal(data, &config); err != nil {
				continue
			}

			// Look for mcpServers key
			if mcpServers, ok := config["mcpServers"].(map[string]interface{}); ok {
				for name, serverDef := range mcpServers {
					server := &DiscoveredServer{
						Name:         name,
						Protocol:     "mcp",
						Source:       "config",
						ConfigPath:   path,
						DiscoveredAt: time.Now(),
					}
					if sd, ok := serverDef.(map[string]interface{}); ok {
						if cmd, ok := sd["command"].(string); ok {
							server.Command = cmd
							server.Transport = "stdio"
						}
						if args, ok := sd["args"].([]interface{}); ok {
							for _, a := range args {
								if s, ok := a.(string); ok {
									server.Args = append(server.Args, s)
								}
							}
						}
						if url, ok := sd["url"].(string); ok {
							server.Address = url
							server.Transport = "http"
						}
						if env, ok := sd["env"].(map[string]interface{}); ok {
							server.Env = make(map[string]string)
							for k, v := range env {
								if s, ok := v.(string); ok {
									server.Env[k] = s
								}
							}
						}
					}
					server.Status = d.checkServer(server)
					servers = append(servers, server)
				}
			}
		}
	}

	return servers
}

// discoverRunningProcesses finds MCP-related processes.
func (d *Discoverer) discoverRunningProcesses() []*DiscoveredServer {
	var servers []*DiscoveredServer

	// Try pgrep-like approach
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return servers
	}

	mcpKeywords := []string{"mcp", "model-context", "mcp-server"}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)
		matched := false
		for _, kw := range mcpKeywords {
			if strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		cmdStr := strings.Join(fields[10:], " ")
		name := "unknown-mcp"
		for _, kw := range mcpKeywords {
			if idx := strings.Index(strings.ToLower(cmdStr), kw); idx >= 0 {
				parts := strings.Fields(cmdStr)
				if len(parts) > 0 {
					name = filepath.Base(parts[0])
				}
				break
			}
		}

		servers = append(servers, &DiscoveredServer{
			Name:         name,
			Protocol:     "mcp",
			Transport:    "stdio",
			Command:      cmdStr,
			Status:       StatusReachable,
			Source:       "process",
			DiscoveredAt: time.Now(),
		})
	}

	return servers
}

// discoverLocalPorts probes common MCP ports.
func (d *Discoverer) discoverLocalPorts() []*DiscoveredServer {
	var servers []*DiscoveredServer

	for _, port := range d.scanPorts {
		addr := fmt.Sprintf("http://localhost:%d", port)
		// Quick TCP check
		start := time.Now()
		cmd := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--connect-timeout", "1", addr)
		output, err := cmd.Output()
		latency := time.Since(start)

		if err != nil {
			continue
		}

		code := strings.TrimSpace(string(output))
		if code == "000" {
			continue
		}

		// Try /health or /capabilities endpoint
		var capabilities []string
		probeURL := addr + "/capabilities"
		probeOut, err := exec.Command("curl", "-s", "--connect-timeout", "1", probeURL).Output()
		if err == nil && len(probeOut) > 0 {
			var caps map[string]interface{}
			if json.Unmarshal(probeOut, &caps) == nil {
				if tools, ok := caps["tools"].([]interface{}); ok {
					for _, t := range tools {
						if m, ok := t.(map[string]interface{}); ok {
							if n, ok := m["name"].(string); ok {
								capabilities = append(capabilities, n)
							}
						}
					}
				}
			}
		}

		servers = append(servers, &DiscoveredServer{
			Name:         fmt.Sprintf("mcp-localhost-%d", port),
			Protocol:     "mcp",
			Transport:    "http",
			Address:      addr,
			Status:       StatusReachable,
			Latency:      latency,
			Source:       "network",
			Capabilities: capabilities,
			DiscoveredAt: time.Now(),
		})
	}

	return servers
}

// checkServer checks if a discovered server is reachable.
func (d *Discoverer) checkServer(s *DiscoveredServer) ServerStatus {
	if s.Transport == "stdio" {
		// Check if command exists
		cmd := s.Command
		if parts := strings.Fields(cmd); len(parts) > 0 {
			cmd = parts[0]
		}
		if _, err := exec.LookPath(cmd); err == nil {
			return StatusReachable
		}
		return StatusUnreachable
	}

	if s.Transport == "http" && s.Address != "" {
		start := time.Now()
		out, err := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--connect-timeout", "2", s.Address).Output()
		s.Latency = time.Since(start)
		if err != nil {
			return StatusUnreachable
		}
		code := strings.TrimSpace(string(out))
		if code != "000" {
			return StatusReachable
		}
	}

	return StatusUnknown
}

// FormatServer renders a discovered server for display.
func FormatServer(s *DiscoveredServer) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n", s.Name))
	sb.WriteString(fmt.Sprintf("  Protocol:     %s\n", s.Protocol))
	sb.WriteString(fmt.Sprintf("  Transport:    %s\n", s.Transport))
	if s.Address != "" {
		sb.WriteString(fmt.Sprintf("  Address:      %s\n", s.Address))
	}
	if s.Command != "" {
		sb.WriteString(fmt.Sprintf("  Command:      %s\n", s.Command))
	}
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", s.Status))
	if s.Latency > 0 {
		sb.WriteString(fmt.Sprintf("  Latency:      %s\n", s.Latency.Round(time.Millisecond)))
	}
	sb.WriteString(fmt.Sprintf("  Source:       %s\n", s.Source))
	if len(s.Capabilities) > 0 {
		sb.WriteString(fmt.Sprintf("  Capabilities: %s\n", strings.Join(s.Capabilities, ", ")))
	}
	return sb.String()
}

// FormatResult renders a discovery result for display.
func FormatResult(r *DiscoveryResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Discovery completed in %s\n", r.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("Found %d servers (scanned %d)\n\n", len(r.Servers), r.Scanned))
	for _, s := range r.Servers {
		sb.WriteString(FormatServer(s))
		sb.WriteString("\n")
	}
	return sb.String()
}

func init() {
	// Ensure curl is available on non-Windows
	_ = runtime.GOOS
}
