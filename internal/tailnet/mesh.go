// Package tailnet provides WireGuard-based mesh networking between all agents
// and machines. Every node gets a unique IP in the Forge tailnet and can
// communicate securely with every other node.
package tailnet

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/curve25519"
	"sync"
	"time"
)

// NodeState is the connectivity state of a tailnet node.
type NodeState string

const (
	NodeOnline  NodeState = "online"
	NodeOffline NodeState = "offline"
	NodeUnknown NodeState = "unknown"
)

// Node represents a machine or agent in the tailnet.
type Node struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	IP            string            `json:"ip"`
	PublicKey     string            `json:"public_key"`
	Endpoint      string            `json:"endpoint,omitempty"`
	State         NodeState         `json:"state"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// MeshConfig configures the tailnet mesh.
type MeshConfig struct {
	NetworkCIDR string `json:"network_cidr"`
	Port        int    `json:"port"`
	MTU         int    `json:"mtu"`
	StoreDir    string `json:"store_dir"`
}

// DefaultMeshConfig returns sensible defaults.
func DefaultMeshConfig() MeshConfig {
	home, _ := os.UserHomeDir()
	return MeshConfig{
		NetworkCIDR: "100.64.0.0/10",
		Port:        51820,
		MTU:         1280,
		StoreDir:    filepath.Join(home, ".forge", "tailnet"),
	}
}

// Mesh manages the WireGuard mesh network.
type Mesh struct {
	config MeshConfig
	local  *Node
	peers  map[string]*Node
	mu     sync.RWMutex
}

// NewMesh creates a mesh with the given config.
func NewMesh(config MeshConfig) *Mesh {
	os.MkdirAll(config.StoreDir, 0o755)
	return &Mesh{
		config: config,
		peers:  make(map[string]*Node),
	}
}

// Init sets up the local node: generates keys, assigns IP, creates interface.
func (m *Mesh) Init(name string) (*Node, error) {
	privateKey, publicKey, err := generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate keys: %w", err)
	}

	ip, err := m.assignIP(name)
	if err != nil {
		return nil, fmt.Errorf("assign IP: %w", err)
	}

	node := &Node{
		ID:            generateNodeID(name),
		Name:          name,
		IP:            ip,
		PublicKey:     publicKey,
		State:         NodeOnline,
		LastHeartbeat: time.Now(),
		Labels:        make(map[string]string),
	}

	m.local = node

	keyPath := filepath.Join(m.config.StoreDir, "private.key")
	if err := os.WriteFile(keyPath, []byte(privateKey), 0o600); err != nil {
		return nil, fmt.Errorf("save private key: %w", err)
	}

	m.persistNode(node)
	m.persistPrivateKey(privateKey)

	return node, nil
}

// AddPeer adds a remote node to the mesh.
func (m *Mesh) AddPeer(node Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.peers[node.ID]; exists {
		return fmt.Errorf("peer %s already exists", node.ID)
	}

	node.LastHeartbeat = time.Now()
	m.peers[node.ID] = &node
	m.persistNode(&node)
	return nil
}

// RemovePeer removes a remote node.
func (m *Mesh) RemovePeer(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.peers, nodeID)
	metaPath := filepath.Join(m.config.StoreDir, "peers", nodeID+".json")
	os.Remove(metaPath)
	return nil
}

// GetPeers returns all known peers.
func (m *Mesh) GetPeers() []Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Node, 0, len(m.peers))
	for _, p := range m.peers {
		result = append(result, *p)
	}
	return result
}

// GetPeer finds a peer by ID or name.
func (m *Mesh) GetPeer(idOrName string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.peers {
		if p.ID == idOrName || p.Name == idOrName {
			cp := *p
			return &cp, true
		}
	}
	return nil, false
}

// LocalNode returns the local node info.
func (m *Mesh) LocalNode() *Node {
	if m.local == nil {
		return nil
	}
	cp := *m.local
	return &cp
}

// Ping checks connectivity to a peer via ICMP.
func (m *Mesh) Ping(peerID string) (time.Duration, error) {
	m.mu.RLock()
	peer, ok := m.peers[peerID]
	m.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("peer %s not found", peerID)
	}

	start := time.Now()
	cmd := exec.Command("ping", "-c", "1", "-W", "2", peer.IP)
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ping %s: %w", peer.IP, err)
	}
	return time.Since(start), nil
}

// GenerateWGConfig produces a WireGuard config for the local node.
func (m *Mesh) GenerateWGConfig() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.local == nil {
		return ""
	}

	lines := []string{
		"[Interface]",
		fmt.Sprintf("Address = %s/10", m.local.IP),
		fmt.Sprintf("PrivateKey = %s", m.loadPrivateKey()),
		fmt.Sprintf("ListenPort = %d", m.config.Port),
		"",
	}

	for _, peer := range m.peers {
		lines = append(lines,
			"[Peer]",
			fmt.Sprintf("PublicKey = %s", peer.PublicKey),
		)
		if peer.Endpoint != "" {
			lines = append(lines, fmt.Sprintf("Endpoint = %s", peer.Endpoint))
		}
		lines = append(lines,
			fmt.Sprintf("AllowedIPs = %s/32", peer.IP),
			"PersistentKeepalive = 25",
			"",
		)
	}

	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

// Heartbeat updates the last-seen timestamp for a peer.
func (m *Mesh) Heartbeat(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if peer, ok := m.peers[nodeID]; ok {
		peer.LastHeartbeat = time.Now()
		peer.State = NodeOnline
	}
}

// StalePeers returns peers not seen within the given duration.
func (m *Mesh) StalePeers(maxAge time.Duration) []Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stale []Node
	cutoff := time.Now().Add(-maxAge)
	for _, p := range m.peers {
		if p.LastHeartbeat.Before(cutoff) {
			p.State = NodeOffline
			stale = append(stale, *p)
		}
	}
	return stale
}

// assignIP deterministically assigns an IP from the mesh CIDR.
func (m *Mesh) assignIP(name string) (string, error) {
	sum := 0
	for _, c := range name {
		sum = sum*31 + int(c)
	}
	octet3 := 64 + (sum % 64)
	octet4 := 1 + ((sum / 64) % 254)
	return fmt.Sprintf("100.%d.%d.1", octet3, octet4), nil
}

func (m *Mesh) persistNode(node *Node) {
	dir := filepath.Join(m.config.StoreDir, "peers")
	os.MkdirAll(dir, 0o755)
	data, _ := json.MarshalIndent(node, "", "  ")
	path := filepath.Join(dir, node.ID+".json")
	os.WriteFile(path, data, 0o644)
}

func (m *Mesh) persistPrivateKey(key string) {
	path := filepath.Join(m.config.StoreDir, "private.key")
	os.WriteFile(path, []byte(key), 0o600)
}

func (m *Mesh) loadPrivateKey() string {
	path := filepath.Join(m.config.StoreDir, "private.key")
	data, _ := os.ReadFile(path)
	return strings.TrimSpace(string(data))
}

func generateKeyPair() (private, public string, err error) {
	// Generate a Curve25519 private key using the standard approach:
	// 32 random bytes clamped per RFC 7748 §6.1
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return "", "", err
	}

	// Clamp the private key (same as WireGuard does)
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Compute the public key using curve25519 scalar multiplication
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privateKey)

	private = hex.EncodeToString(privateKey[:])
	public = hex.EncodeToString(pubKey[:])
	return private, public, nil
}

func generateNodeID(name string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("node-%s-%s", name, hex.EncodeToString(b))
}

// Ensure net package is used.
var _ = net.IPv4len
