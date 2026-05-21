// Package wgtunnel provides WireGuard tunnel management.
// Tunnel through any network — the forge's secret passages.
package wgtunnel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Key is a WireGuard key.
type Key [32]byte

// GenerateKey generates a new WireGuard private key.
func GenerateKey() (Key, error) {
	var key Key
	if _, err := rand.Read(key[:]); err != nil {
		return key, fmt.Errorf("wgtunnel: generate key: %w", err)
	}
	// Clamp private key per WireGuard spec
	key[0] &= 248
	key[31] = (key[31] & 127) | 64
	return key, nil
}

// String returns the base64-encoded key.
func (k Key) String() string {
	return base64.StdEncoding.EncodeToString(k[:])
}

// PublicKey computes the public key from a private key.
// Note: This is a stub — real Curve25519 requires the wireguard-go or
// golang.org/x/crypto/curve25519 package.
func (k Key) PublicKey() Key {
	var pub Key
	// Stub: in production, use curve25519.X25519
	copy(pub[:], k[:])
	return pub
}

// TunnelConfig configures a WireGuard tunnel.
type TunnelConfig struct {
	Name       string // Interface name (e.g., "wg0")
	Port       int    // Listening port
	PrivateKey Key
	Address    string // Tunnel IP address (e.g., "10.0.0.1/24")
	Peers      []PeerConfig
	DNS        []string
	MTU        int
}

// PeerConfig configures a WireGuard peer.
type PeerConfig struct {
	PublicKey  Key
	Endpoint   string   // e.g., "1.2.3.4:51820"
	AllowedIPs []string // e.g., ["10.0.0.2/32", "0.0.0.0/0"]
}

// Tunnel represents a WireGuard tunnel.
type Tunnel struct {
	ID        string
	Config    TunnelConfig
	Status    TunnelStatus
	CreatedAt time.Time
	mu        sync.Mutex
}

// TunnelStatus indicates the state of a tunnel.
type TunnelStatus string

const (
	TunnelCreated TunnelStatus = "created"
	TunnelActive  TunnelStatus = "active"
	TunnelDown    TunnelStatus = "down"
	TunnelError   TunnelStatus = "error"
)

// Manager manages WireGuard tunnels.
type Manager struct {
	tunnels map[string]*Tunnel
	mu      sync.RWMutex
	wgPath  string
}

// NewManager creates a new tunnel manager.
func NewManager() *Manager {
	wgPath, _ := exec.LookPath("wg")
	return &Manager{
		tunnels: make(map[string]*Tunnel),
		wgPath:  wgPath,
	}
}

// WireGuardAvailable returns whether the wg tool is available.
func (m *Manager) WireGuardAvailable() bool {
	return m.wgPath != ""
}

// CreateTunnel creates a new tunnel configuration.
func (m *Manager) CreateTunnel(config TunnelConfig) (*Tunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tunnels[config.Name]; exists {
		return nil, fmt.Errorf("wgtunnel: tunnel %q already exists", config.Name)
	}

	if config.Port == 0 {
		config.Port = 51820
	}

	if config.MTU == 0 {
		config.MTU = 1420
	}

	tunnel := &Tunnel{
		ID:        config.Name,
		Config:    config,
		Status:    TunnelCreated,
		CreatedAt: time.Now(),
	}

	m.tunnels[config.Name] = tunnel
	return tunnel, nil
}

// StartTunnel starts a WireGuard tunnel.
func (m *Manager) StartTunnel(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[name]
	if !ok {
		return fmt.Errorf("wgtunnel: tunnel %q not found", name)
	}

	if !m.WireGuardAvailable() {
		return fmt.Errorf("wgtunnel: wg tool not available")
	}

	// Generate WireGuard config file
	configPath := fmt.Sprintf("/tmp/%s.conf", name)
	config := m.generateConfig(tunnel.Config)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("wgtunnel: write config: %w", err)
	}

	// Bring up the interface
	cmd := exec.CommandContext(ctx, "wg-quick", "up", configPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		tunnel.Status = TunnelError
		return fmt.Errorf("wgtunnel: wg-quick up failed: %w\n%s", err, string(output))
	}

	tunnel.Status = TunnelActive

	// Clean up config file
	os.Remove(configPath)

	return nil
}

// StopTunnel stops a WireGuard tunnel.
func (m *Manager) StopTunnel(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[name]
	if !ok {
		return fmt.Errorf("wgtunnel: tunnel %q not found", name)
	}

	cmd := exec.CommandContext(ctx, "wg-quick", "down", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wgtunnel: wg-quick down failed: %w", err)
	}

	tunnel.Status = TunnelDown
	return nil
}

// DeleteTunnel removes a tunnel.
func (m *Manager) DeleteTunnel(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tunnels[name]; !ok {
		return fmt.Errorf("wgtunnel: tunnel %q not found", name)
	}

	delete(m.tunnels, name)
	return nil
}

// GetTunnel returns a tunnel by name.
func (m *Manager) GetTunnel(name string) (*Tunnel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, ok := m.tunnels[name]
	if !ok {
		return nil, fmt.Errorf("wgtunnel: tunnel %q not found", name)
	}
	return tunnel, nil
}

// ListTunnels returns all tunnels.
func (m *Manager) ListTunnels() []*Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		result = append(result, t)
	}
	return result
}

// GenerateTunnelConfig generates a complete WireGuard config for a tunnel.
func (m *Manager) GenerateTunnelConfig(name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, ok := m.tunnels[name]
	if !ok {
		return "", fmt.Errorf("wgtunnel: tunnel %q not found", name)
	}
	return m.generateConfig(tunnel.Config), nil
}

func (m *Manager) generateConfig(config TunnelConfig) string {
	var s strings.Builder

	fmt.Fprintf(&s, "[Interface]\n")
	fmt.Fprintf(&s, "PrivateKey = %s\n", config.PrivateKey.String())
	fmt.Fprintf(&s, "Address = %s\n", config.Address)
	fmt.Fprintf(&s, "ListenPort = %d\n", config.Port)
	if config.MTU > 0 {
		fmt.Fprintf(&s, "MTU = %d\n", config.MTU)
	}
	if len(config.DNS) > 0 {
		fmt.Fprintf(&s, "DNS = %s\n", strings.Join(config.DNS, ", "))
	}
	fmt.Fprintln(&s)

	for _, peer := range config.Peers {
		fmt.Fprintf(&s, "[Peer]\n")
		fmt.Fprintf(&s, "PublicKey = %s\n", peer.PublicKey.String())
		if peer.Endpoint != "" {
			fmt.Fprintf(&s, "Endpoint = %s\n", peer.Endpoint)
		}
		if len(peer.AllowedIPs) > 0 {
			fmt.Fprintf(&s, "AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ", "))
		}
		fmt.Fprintln(&s)
	}

	return s.String()
}

// FindFreePort finds an available UDP port.
func FindFreePort() (int, error) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.LocalAddr().(*net.UDPAddr).Port, nil
}
