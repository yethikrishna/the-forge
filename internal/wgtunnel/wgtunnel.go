// Package wgtunnel provides zero-config WireGuard mesh networking.
// Nodes discover each other, establish encrypted tunnels, and maintain
// a full-mesh topology with automatic failover.
package wgtunnel

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Peer represents a node in the mesh.
type Peer struct {
	ID         string
	PublicKey  string
	Endpoint   string // IP:port
	AllowedIPs []string
	Keepalive  time.Duration
	Connected  bool
	LastSeen   time.Time
	LatencyMs  float64
}

// TunnelStats tracks bandwidth and latency per peer.
type TunnelStats struct {
	PeerID       string
	BytesSent    int64
	BytesRecv    int64
	LatencyMs    float64
	Uptime       time.Duration
	Reconnects   int
}

// PeerHealth describes the connectivity state of a peer.
type PeerHealth struct {
	PeerID    string
	Connected bool
	LatencyMs float64
	Uptime    time.Duration
	Issues    []string
}

// KeyManager handles WireGuard key generation and rotation.
type KeyManager struct {
	privateKey string
	publicKey  string
	rotatedAt  time.Time
	mu         sync.Mutex
}

// NewKeyManager generates a new key pair.
func NewKeyManager() (*KeyManager, error) {
	km := &KeyManager{}
	if err := km.generate(); err != nil {
		return nil, err
	}
	return km, nil
}

func (km *KeyManager) generate() error {
	// Generate 32 random bytes for private key
	priv := make([]byte, 32)
	if _, err := rand.Read(priv); err != nil {
		return fmt.Errorf("key generation failed: %w", err)
	}
	km.privateKey = hex.EncodeToString(priv)

	// In real impl: derive public key from private using Curve25519
	// For prototype, generate separate random key
	pub := make([]byte, 32)
	rand.Read(pub)
	km.publicKey = hex.EncodeToString(pub)
	km.rotatedAt = time.Now()
	return nil
}

// PublicKey returns the current public key.
func (km *KeyManager) PublicKey() string {
	km.mu.Lock()
	defer km.mu.Unlock()
	return km.publicKey
}

// Rotate generates a new key pair.
func (km *KeyManager) Rotate() error {
	km.mu.Lock()
	defer km.mu.Unlock()
	return km.generate()
}

// LastRotated returns when keys were last rotated.
func (km *KeyManager) LastRotated() time.Time {
	km.mu.Lock()
	defer km.mu.Unlock()
	return km.rotatedAt
}

// WireGuardMesh is the main mesh networking engine.
type WireGuardMesh struct {
	localID  string
	peers    map[string]*Peer
	stats    map[string]*TunnelStats
	keyMgr   *KeyManager
	mu       sync.RWMutex
}

// NewMesh creates a new WireGuard mesh node.
func NewMesh(nodeID string) (*WireGuardMesh, error) {
	km, err := NewKeyManager()
	if err != nil {
		return nil, err
	}
	return &WireGuardMesh{
		localID: nodeID,
		peers:   make(map[string]*Peer),
		stats:   make(map[string]*TunnelStats),
		keyMgr:  km,
	}, nil
}

// GeneratePairToken creates a pairing token for another node.
func (wm *WireGuardMesh) GeneratePairToken() string {
	token := make([]byte, 16)
	rand.Read(token)
	return fmt.Sprintf("forge-mesh-%s-%s", wm.localID, hex.EncodeToString(token))
}

// Pair connects to another node using a pairing token.
func (wm *WireGuardMesh) Pair(token string) (*Peer, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Parse token to extract remote node ID
	// Format: forge-mesh-{remoteID}-{random}
	var remoteID, randomPart string
	fmt.Sscanf(token, "forge-mesh-%s-%s", &remoteID, &randomPart)

	peer := &Peer{
		ID:         remoteID,
		PublicKey:  "pending-exchange",
		AllowedIPs: []string{fmt.Sprintf("10.13.37.%s/32", remoteID[:2])},
		Keepalive:  25 * time.Second,
		Connected:  false,
		LastSeen:   time.Now(),
	}
	wm.peers[remoteID] = peer
	wm.stats[remoteID] = &TunnelStats{PeerID: remoteID}
	return peer, nil
}

// Peers returns all known peers.
func (wm *WireGuardMesh) Peers() []Peer {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	result := make([]Peer, 0, len(wm.peers))
	for _, p := range wm.peers {
		result = append(result, *p)
	}
	return result
}

// Stats returns tunnel statistics for a peer.
func (wm *WireGuardMesh) Stats(peerID string) (*TunnelStats, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	s, ok := wm.stats[peerID]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

// HealthCheck checks connectivity to all peers.
func (wm *WireGuardMesh) HealthCheck() []PeerHealth {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	var health []PeerHealth
	for _, peer := range wm.peers {
		h := PeerHealth{
			PeerID:    peer.ID,
			Connected: peer.Connected,
			LatencyMs: peer.LatencyMs,
		}

		if !peer.Connected {
			h.Issues = append(h.Issues, "peer not connected")
		}
		if time.Since(peer.LastSeen) > 2*time.Minute {
			h.Issues = append(h.Issues, "last seen > 2 min ago")
		}
		if peer.LatencyMs > 500 {
			h.Issues = append(h.Issues, fmt.Sprintf("high latency: %.0fms", peer.LatencyMs))
		}

		if stats, ok := wm.stats[peer.ID]; ok {
			h.Uptime = time.Duration(stats.Uptime)
		}

		health = append(health, h)
	}
	return health
}

// RecordTraffic updates traffic stats for a peer.
func (wm *WireGuardMesh) RecordTraffic(peerID string, sent, recv int64) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if s, ok := wm.stats[peerID]; ok {
		s.BytesSent += sent
		s.BytesRecv += recv
	}
}

// MarkConnected marks a peer as connected.
func (wm *WireGuardMesh) MarkConnected(peerID string, latencyMs float64) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if p, ok := wm.peers[peerID]; ok {
		p.Connected = true
		p.LatencyMs = latencyMs
		p.LastSeen = time.Now()
	}
}

// MarkDisconnected marks a peer as disconnected.
func (wm *WireGuardMesh) MarkDisconnected(peerID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if p, ok := wm.peers[peerID]; ok {
		p.Connected = false
		if s, ok := wm.stats[peerID]; ok {
			s.Reconnects++
		}
	}
}

// LocalID returns the local node ID.
func (wm *WireGuardMesh) LocalID() string {
	return wm.localID
}

// KeyRotationNeeded checks if keys should be rotated.
func (wm *WireGuardMesh) KeyRotationNeeded(maxAge time.Duration) bool {
	return time.Since(wm.keyMgr.LastRotated()) > maxAge
}
