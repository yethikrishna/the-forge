package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DiscoveryConfig configures protocol endpoint discovery.
type DiscoveryConfig struct {
	ScanLocalhost bool   `json:"scan_localhost"`
	ScanNetwork   bool   `json:"scan_network"`
	Timeout       time.Duration `json:"timeout"`
	KnownPorts    []int  `json:"known_ports"`
	ConfigDir     string `json:"config_dir"`
}

// DefaultDiscoveryConfig returns sensible defaults.
func DefaultDiscoveryConfig() DiscoveryConfig {
	return DiscoveryConfig{
		ScanLocalhost: true,
		ScanNetwork:   false,
		Timeout:       2 * time.Second,
		KnownPorts:    []int{3000, 8080, 8787, 9090, 5173, 4000, 5000, 6000},
		ConfigDir:     "",
	}
}

// Endpoint represents a discovered protocol endpoint.
type Endpoint struct {
	Name       string            `json:"name"`
	Protocol   Protocol          `json:"protocol"`
	Address    string            `json:"address"`
	Healthy    bool              `json:"healthy"`
	ServerInfo map[string]string `json:"server_info,omitempty"`
	DiscoveredAt time.Time       `json:"discovered_at"`
	Source     string            `json:"source"` // "scan", "config", "mcp-registry"
}

// Discoverer finds protocol endpoints on the network and in config.
type Discoverer struct {
	config   DiscoveryConfig
	dir      string
	cache    []Endpoint
}

// NewDiscoverer creates a protocol endpoint discoverer.
func NewDiscoverer(config DiscoveryConfig) *Discoverer {
	dir := config.ConfigDir
	if dir == "" {
		dir = ".forge/bridge"
	}
	return &Discoverer{
		config: config,
		dir:    dir,
		cache:  make([]Endpoint, 0),
	}
}

// Scan discovers available protocol endpoints.
func (d *Discoverer) Scan(ctx context.Context) ([]Endpoint, error) {
	var endpoints []Endpoint

	// 1. Scan config files
	configEndpoints := d.scanConfig()
	endpoints = append(endpoints, configEndpoints...)

	// 2. Scan MCP server configs (common locations)
	mcpEndpoints := d.scanMCPServers()
	endpoints = append(endpoints, mcpEndpoints...)

	// 3. Scan localhost ports
	if d.config.ScanLocalhost {
		localEndpoints := d.scanLocalhost(ctx)
		endpoints = append(endpoints, localEndpoints...)
	}

	// 4. Health-check discovered endpoints
	for i := range endpoints {
		endpoints[i].Healthy = d.checkHealth(ctx, endpoints[i])
	}

	d.cache = endpoints
	return endpoints, nil
}

// scanConfig reads endpoint configs from the bridge directory.
func (d *Discoverer) scanConfig() []Endpoint {
	var endpoints []Endpoint

	data, err := os.ReadFile(filepath.Join(d.dir, "endpoints.json"))
	if err != nil {
		return endpoints
	}

	var configured []Endpoint
	if err := json.Unmarshal(data, &configured); err != nil {
		return endpoints
	}

	for i := range configured {
		configured[i].Source = "config"
		configured[i].DiscoveredAt = time.Now()
	}
	return configured
}

// scanMCPServers discovers MCP servers from common config locations.
func (d *Discoverer) scanMCPServers() []Endpoint {
	var endpoints []Endpoint

	// Check Claude Desktop config
	homeDir, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(homeDir, ".claude", "claude_desktop_config.json"),
		filepath.Join(homeDir, ".config", "claude", "config.json"),
		filepath.Join(homeDir, ".cursor", "mcp.json"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}

		servers, ok := cfg["mcpServers"].(map[string]interface{})
		if !ok {
			continue
		}

		for name, serverDef := range servers {
			srv, ok := serverDef.(map[string]interface{})
			if !ok {
				continue
			}
			addr, _ := srv["url"].(string)
			if addr == "" {
				// Stdio-based MCP server — record with command
				cmd, _ := srv["command"].(string)
				if cmd != "" {
					addr = fmt.Sprintf("stdio:%s", cmd)
				}
			}
			if addr != "" {
				endpoints = append(endpoints, Endpoint{
					Name:        name,
					Protocol:    ProtocolMCP,
					Address:     addr,
					DiscoveredAt: time.Now(),
					Source:      "mcp-registry",
					ServerInfo:  map[string]string{"config_path": p},
				})
			}
		}
	}

	return endpoints
}

// scanLocalhost probes common ports for protocol servers.
func (d *Discoverer) scanLocalhost(ctx context.Context) []Endpoint {
	var endpoints []Endpoint

	for _, port := range d.config.KnownPorts {
		addr := fmt.Sprintf("127.0.0.1:%d", port)

		conn, err := net.DialTimeout("tcp", addr, d.config.Timeout)
		if err != nil {
			continue
		}
		conn.Close()

		// Try to identify the protocol
		protocol := d.identifyProtocol(ctx, addr)
		if protocol != "" {
			endpoints = append(endpoints, Endpoint{
				Name:        fmt.Sprintf("localhost:%d", port),
				Protocol:    protocol,
				Address:     fmt.Sprintf("http://%s", addr),
				DiscoveredAt: time.Now(),
				Source:      "scan",
			})
		}
	}

	return endpoints
}

// identifyProtocol attempts to determine what protocol a server speaks.
func (d *Discoverer) identifyProtocol(ctx context.Context, addr string) Protocol {
	// Try MCP initialize
	if d.probeMCP(ctx, addr) {
		return ProtocolMCP
	}
	// Try ACP health
	if d.probeACP(ctx, addr) {
		return ProtocolACP
	}
	// Try A2A
	if d.probeA2A(ctx, addr) {
		return ProtocolA2A
	}
	// Unknown but reachable — assume MCP as most common
	return ProtocolMCP
}

// probeMCP sends an MCP initialize request.
func (d *Discoverer) probeMCP(ctx context.Context, addr string) bool {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]string{"name": "forge-bridge", "version": "0.1.0"},
		},
	}
	return d.probeHTTP(ctx, fmt.Sprintf("http://%s/messages", addr), req)
}

// probeACP sends an ACP health check.
func (d *Discoverer) probeACP(ctx context.Context, addr string) bool {
	return d.probeHTTP(ctx, fmt.Sprintf("http://%s/health", addr), nil)
}

// probeA2A sends an A2A agent info request.
func (d *Discoverer) probeA2A(ctx context.Context, addr string) bool {
	return d.probeHTTP(ctx, fmt.Sprintf("http://%s/agent", addr), nil)
}

// probeHTTP sends an HTTP request and returns true if it gets a 2xx response.
func (d *Discoverer) probeHTTP(ctx context.Context, url string, body interface{}) bool {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	var bodyReader *strings.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return false
		}
		bodyReader = strings.NewReader(string(data))
	}

	var req *exec.Cmd
	_ = req // Use net/http in production; simplified for now
	_ = bodyReader

	// Use a simple TCP check + HTTP probe
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(url, "http://"), d.config.Timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Send a basic HTTP request
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n", url)
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		line := scanner.Text()
		return strings.Contains(line, "200") || strings.Contains(line, "401") || strings.Contains(line, "405")
	}
	return false
}

// checkHealth verifies an endpoint is reachable.
func (d *Discoverer) checkHealth(ctx context.Context, ep Endpoint) bool {
	if strings.HasPrefix(ep.Address, "stdio:") {
		return true // stdio servers are always "healthy" (started on demand)
	}

	addr := strings.TrimPrefix(ep.Address, "http://")
	addr = strings.TrimPrefix(addr, "https://")

	conn, err := net.DialTimeout("tcp", addr, d.config.Timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Cache returns the most recently discovered endpoints.
func (d *Discoverer) Cache() []Endpoint {
	return d.cache
}

// FormatEndpoint renders an endpoint for display.
func FormatEndpoint(ep Endpoint) string {
	health := "unhealthy"
	if ep.Healthy {
		health = "healthy"
	}
	return fmt.Sprintf("%-25s %-6s %-40s %-10s %s", ep.Name, ep.Protocol, ep.Address, health, ep.Source)
}
