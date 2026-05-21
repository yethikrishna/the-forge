package wgtunnel

import (
	"testing"
	"time"
)

func TestNewMesh(t *testing.T) {
	mesh, err := NewMesh("node-1")
	if err != nil {
		t.Fatal(err)
	}
	if mesh.LocalID() != "node-1" {
		t.Error("wrong local ID")
	}
}

func TestKeyGeneration(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}
	if km.PublicKey() == "" {
		t.Error("public key should not be empty")
	}
}

func TestKeyRotation(t *testing.T) {
	km, _ := NewKeyManager()
	old := km.PublicKey()
	km.Rotate()
	new := km.PublicKey()
	if old == new {
		t.Error("rotation should generate new key")
	}
}

func TestPairAndPeers(t *testing.T) {
	mesh, _ := NewMesh("node-1")
	token := mesh.GeneratePairToken()

	peer, err := mesh.Pair(token)
	if err != nil {
		t.Fatal(err)
	}
	if peer.ID == "" {
		t.Error("peer should have an ID")
	}

	peers := mesh.Peers()
	if len(peers) != 1 {
		t.Errorf("expected 1 peer, got %d", len(peers))
	}
}

func TestHealthCheck(t *testing.T) {
	mesh, _ := NewMesh("node-1")
	token := mesh.GeneratePairToken()
	mesh.Pair(token)

	health := mesh.HealthCheck()
	if len(health) != 1 {
		t.Errorf("expected 1 health report, got %d", len(health))
	}
	if len(health[0].Issues) == 0 {
		t.Error("unconnected peer should have issues")
	}
}

func TestTrafficAndConnectivity(t *testing.T) {
	mesh, _ := NewMesh("node-1")
	token := mesh.GeneratePairToken()
	peer, _ := mesh.Pair(token)

	mesh.MarkConnected(peer.ID, 12.5)
	mesh.RecordTraffic(peer.ID, 1024, 2048)

	stats, ok := mesh.Stats(peer.ID)
	if !ok {
		t.Fatal("expected stats for peer")
	}
	if stats.BytesSent != 1024 {
		t.Errorf("expected 1024 bytes sent, got %d", stats.BytesSent)
	}
}

func TestKeyRotationNeeded(t *testing.T) {
	mesh, _ := NewMesh("node-1")
	if mesh.KeyRotationNeeded(24 * time.Hour) {
		t.Error("fresh keys should not need rotation")
	}
	if !mesh.KeyRotationNeeded(0) {
		t.Error("zero max age should always need rotation")
	}
}
