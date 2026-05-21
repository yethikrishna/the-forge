package transfer

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDiscovererGetPeersEmpty(t *testing.T) {
	d := NewDiscoverer(DefaultDiscoveryConfig())
	if len(d.GetPeers()) != 0 {
		t.Error("expected empty peers")
	}
}

func TestDiscovererAddPeer(t *testing.T) {
	d := NewDiscoverer(DefaultDiscoveryConfig())
	d.mu.Lock()
	d.peers["peer-1"] = &Peer{
		ID:        "peer-1",
		Name:      "agent-x",
		Addr:      "192.168.1.10",
		Port:      9000,
		SeenAt:    time.Now(),
		ExpiresAt: time.Now().Add(30 * time.Second),
	}
	d.mu.Unlock()

	peers := d.GetPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].Name != "agent-x" {
		t.Errorf("expected agent-x, got %s", peers[0].Name)
	}
}

func TestDiscovererExpiredPeers(t *testing.T) {
	d := NewDiscoverer(DefaultDiscoveryConfig())
	d.mu.Lock()
	d.peers["expired"] = &Peer{
		ID:        "expired",
		Name:      "old-peer",
		ExpiresAt: time.Now().Add(-10 * time.Second), // expired
	}
	d.peers["active"] = &Peer{
		ID:        "active",
		Name:      "live-peer",
		ExpiresAt: time.Now().Add(30 * time.Second),
	}
	d.mu.Unlock()

	peers := d.GetPeers()
	if len(peers) != 1 {
		t.Errorf("expected 1 active peer, got %d", len(peers))
	}
}

func TestDiscovererGetPeer(t *testing.T) {
	d := NewDiscoverer(DefaultDiscoveryConfig())
	d.mu.Lock()
	d.peers["peer-1"] = &Peer{
		ID:        "peer-1",
		Name:      "test",
		ExpiresAt: time.Now().Add(30 * time.Second),
	}
	d.mu.Unlock()

	p, ok := d.GetPeer("peer-1")
	if !ok || p.Name != "test" {
		t.Error("expected to find peer")
	}

	_, ok = d.GetPeer("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestDiscovererExpireStale(t *testing.T) {
	d := NewDiscoverer(DefaultDiscoveryConfig())
	d.mu.Lock()
	d.peers["stale"] = &Peer{ID: "stale", ExpiresAt: time.Now().Add(-1 * time.Second)}
	d.peers["fresh"] = &Peer{ID: "fresh", ExpiresAt: time.Now().Add(30 * time.Second)}
	d.mu.Unlock()

	expired := d.ExpireStale()
	if expired != 1 {
		t.Errorf("expected 1 expired, got %d", expired)
	}
	if len(d.GetPeers()) != 1 {
		t.Error("expected 1 peer after expiry")
	}
}

func TestDiscoveryConfigDefaults(t *testing.T) {
	cfg := DefaultDiscoveryConfig()
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.Interval != 5*time.Second {
		t.Errorf("expected 5s interval, got %v", cfg.Interval)
	}
	if cfg.TTL != 30*time.Second {
		t.Errorf("expected 30s TTL, got %v", cfg.TTL)
	}
}

func TestPeerSerialization(t *testing.T) {
	p := Peer{
		ID:        "peer-1",
		Name:      "test-agent",
		Addr:      "192.168.1.10",
		Port:      9000,
		SeenAt:    time.Now(),
		ExpiresAt: time.Now().Add(30 * time.Second),
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var p2 Peer
	json.Unmarshal(data, &p2)
	if p2.Name != "test-agent" {
		t.Errorf("expected test-agent, got %s", p2.Name)
	}
}
