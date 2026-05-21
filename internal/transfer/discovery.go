// Package transfer provides zero-config peer discovery via mDNS and local broadcast.
package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Peer is a discovered transfer peer.
type Peer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Addr      string    `json:"addr"`
	Port      int       `json:"port"`
	SeenAt    time.Time `json:"seen_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DiscoveryConfig configures peer discovery.
type DiscoveryConfig struct {
	Port       int           `json:"port"`
	BroadcastAddr string     `json:"broadcast_addr"`
	Interval   time.Duration `json:"interval"`
	TTL        time.Duration `json:"ttl"`
	NodeID     string        `json:"node_id"`
	NodeName   string        `json:"node_name"`
}

// DefaultDiscoveryConfig returns sensible defaults.
func DefaultDiscoveryConfig() DiscoveryConfig {
	return DiscoveryConfig{
		Port:          9090,
		BroadcastAddr: "239.255.255.250:9090",
		Interval:      5 * time.Second,
		TTL:           30 * time.Second,
	}
}

// Discoverer finds peers on the local network.
type Discoverer struct {
	config DiscoveryConfig
	peers  map[string]*Peer
	mu     sync.RWMutex
}

// NewDiscoverer creates a peer discoverer.
func NewDiscoverer(config DiscoveryConfig) *Discoverer {
	return &Discoverer{
		config: config,
		peers:  make(map[string]*Peer),
	}
}

// Announce broadcasts this node's presence.
func (d *Discoverer) Announce(ctx context.Context, listenPort int) error {
	addr, err := net.ResolveUDPAddr("udp", d.config.BroadcastAddr)
	if err != nil {
		return fmt.Errorf("resolve broadcast addr: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("dial broadcast: %w", err)
	}
	defer conn.Close()

	announcement := map[string]interface{}{
		"type":     "forge-peer",
		"id":       d.config.NodeID,
		"name":     d.config.NodeName,
		"port":     listenPort,
		"ts":       time.Now().Unix(),
	}

	_, _ = json.Marshal(announcement)

	ticker := time.NewTicker(d.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			announcement["ts"] = time.Now().Unix()
			data, _ := json.Marshal(announcement)
			conn.Write(data)
		case <-ctx.Done():
			return nil
		}
	}
}

// Listen discovers peers on the local network.
func (d *Discoverer) Listen(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", d.config.BroadcastAddr)
	if err != nil {
		return err
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("listen multicast: %w", err)
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return err
		}

		var msg map[string]interface{}
		if json.Unmarshal(buf[:n], &msg) != nil {
			continue
		}

		if msg["type"] != "forge-peer" {
			continue
		}

		// Skip self
		if msg["id"] == d.config.NodeID {
			continue
		}

		peerID, _ := msg["id"].(string)
		peerName, _ := msg["name"].(string)
		peerPort := 0
		if p, ok := msg["port"].(float64); ok {
			peerPort = int(p)
		}

		d.mu.Lock()
		d.peers[peerID] = &Peer{
			ID:        peerID,
			Name:      peerName,
			Addr:      src.IP.String(),
			Port:      peerPort,
			SeenAt:    time.Now(),
			ExpiresAt: time.Now().Add(d.config.TTL),
		}
		d.mu.Unlock()
	}
}

// GetPeers returns all discovered peers.
func (d *Discoverer) GetPeers() []Peer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Expire stale peers
	now := time.Now()
	var result []Peer
	for _, p := range d.peers {
		if now.Before(p.ExpiresAt) {
			result = append(result, *p)
		}
	}
	return result
}

// GetPeer finds a peer by ID.
func (d *Discoverer) GetPeer(id string) (*Peer, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	p, ok := d.peers[id]
	if !ok {
		return nil, false
	}
	if time.Now().After(p.ExpiresAt) {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// ExpireStale removes expired peers.
func (d *Discoverer) ExpireStale() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	expired := 0
	for id, p := range d.peers {
		if now.After(p.ExpiresAt) {
			delete(d.peers, id)
			expired++
		}
	}
	return expired
}
