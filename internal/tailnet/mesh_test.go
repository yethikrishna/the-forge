package tailnet

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMeshInit(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})

	node, err := mesh.Init("test-node")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if node.Name != "test-node" {
		t.Errorf("expected test-node, got %s", node.Name)
	}
	if node.IP == "" {
		t.Error("expected non-empty IP")
	}
	if node.PublicKey == "" {
		t.Error("expected non-empty public key")
	}
	if node.State != NodeOnline {
		t.Errorf("expected online, got %s", node.State)
	}
}

func TestMeshAddPeer(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})
	mesh.Init("local")

	err := mesh.AddPeer(Node{
		ID:        "peer-1",
		Name:      "remote-agent",
		IP:        "100.65.1.1",
		PublicKey: "abc123",
	})
	if err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}

	peers := mesh.GetPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].Name != "remote-agent" {
		t.Errorf("expected remote-agent, got %s", peers[0].Name)
	}
}

func TestMeshAddDuplicatePeer(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})
	mesh.Init("local")

	mesh.AddPeer(Node{ID: "peer-1", Name: "a", IP: "100.65.1.1", PublicKey: "abc"})
	err := mesh.AddPeer(Node{ID: "peer-1", Name: "a", IP: "100.65.1.1", PublicKey: "abc"})
	if err == nil {
		t.Error("expected error for duplicate peer")
	}
}

func TestMeshGetPeer(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})
	mesh.Init("local")

	mesh.AddPeer(Node{ID: "peer-1", Name: "agent-x", IP: "100.65.1.1", PublicKey: "abc"})

	// By ID
	p, ok := mesh.GetPeer("peer-1")
	if !ok || p.Name != "agent-x" {
		t.Error("expected to find peer by ID")
	}

	// By name
	p, ok = mesh.GetPeer("agent-x")
	if !ok || p.ID != "peer-1" {
		t.Error("expected to find peer by name")
	}

	// Not found
	_, ok = mesh.GetPeer("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestMeshRemovePeer(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})
	mesh.Init("local")

	mesh.AddPeer(Node{ID: "peer-1", Name: "a", IP: "100.65.1.1", PublicKey: "abc"})
	mesh.RemovePeer("peer-1")

	if len(mesh.GetPeers()) != 0 {
		t.Error("expected 0 peers after removal")
	}
}

func TestMeshLocalNode(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})

	if mesh.LocalNode() != nil {
		t.Error("expected nil before init")
	}

	mesh.Init("local")
	local := mesh.LocalNode()
	if local == nil || local.Name != "local" {
		t.Error("expected local node after init")
	}
}

func TestMeshHeartbeat(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})
	mesh.Init("local")

	mesh.AddPeer(Node{ID: "peer-1", Name: "a", IP: "100.65.1.1", PublicKey: "abc"})
	mesh.Heartbeat("peer-1")

	p, ok := mesh.GetPeer("peer-1")
	if !ok {
		t.Fatal("peer not found")
	}
	if time.Since(p.LastHeartbeat) > time.Second {
		t.Error("expected recent heartbeat")
	}
}

func TestMeshStalePeers(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})
	mesh.Init("local")

	mesh.AddPeer(Node{ID: "peer-1", Name: "stale", IP: "100.65.1.1", PublicKey: "abc"})
	// Make peer stale
	mesh.mu.Lock()
	mesh.peers["peer-1"].LastHeartbeat = time.Now().Add(-2 * time.Hour)
	mesh.mu.Unlock()

	stale := mesh.StalePeers(time.Hour)
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale peer, got %d", len(stale))
	}
	if stale[0].State != NodeOffline {
		t.Errorf("expected offline, got %s", stale[0].State)
	}
}

func TestMeshGenerateWGConfig(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir, Port: 51820})
	mesh.Init("local")

	mesh.AddPeer(Node{
		ID:        "peer-1",
		Name:      "remote",
		IP:        "100.65.1.1",
		PublicKey: "abc123",
		Endpoint:  "1.2.3.4:51820",
	})

	config := mesh.GenerateWGConfig()
	if !strings.Contains(config, "[Interface]") {
		t.Error("expected [Interface] section")
	}
	if !strings.Contains(config, "ListenPort = 51820") {
		t.Error("expected ListenPort in config")
	}
	if !strings.Contains(config, "PublicKey = abc123") {
		t.Error("expected peer public key in config")
	}
	if !strings.Contains(config, "Endpoint = 1.2.3.4:51820") {
		t.Error("expected peer endpoint in config")
	}
	if !strings.Contains(config, "AllowedIPs = 100.65.1.1/32") {
		t.Error("expected AllowedIPs in config")
	}
}

func TestMeshGenerateWGConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})

	config := mesh.GenerateWGConfig()
	if config != "" {
		t.Error("expected empty config before init")
	}
}

func TestMeshAssignIP(t *testing.T) {
	dir := t.TempDir()
	mesh := NewMesh(MeshConfig{StoreDir: dir})

	ip1, _ := mesh.assignIP("node-a")
	ip2, _ := mesh.assignIP("node-b")

	if ip1 == "" || ip2 == "" {
		t.Error("expected non-empty IPs")
	}
	if ip1 == ip2 {
		t.Error("expected different IPs for different names")
	}
	if !strings.HasPrefix(ip1, "100.") {
		t.Errorf("expected IP in 100.x range, got %s", ip1)
	}
}

func TestGenerateKeyPair(t *testing.T) {
	priv, pub, err := generateKeyPair()
	if err != nil {
		t.Fatalf("generateKeyPair failed: %v", err)
	}
	if len(priv) != 64 { // 32 bytes hex
		t.Errorf("expected 64-char private key, got %d", len(priv))
	}
	if len(pub) != 64 {
		t.Errorf("expected 64-char public key, got %d", len(pub))
	}
	if priv == pub {
		t.Error("private and public should differ")
	}
}

func TestNodeSerialization(t *testing.T) {
	node := Node{
		ID:            "node-test",
		Name:          "test",
		IP:            "100.64.1.1",
		PublicKey:     "abc123",
		Endpoint:      "1.2.3.4:51820",
		State:         NodeOnline,
		LastHeartbeat: time.Now(),
		Labels:        map[string]string{"role": "agent"},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}

	var node2 Node
	if err := json.Unmarshal(data, &node2); err != nil {
		t.Fatal(err)
	}
	if node2.Name != "test" {
		t.Errorf("expected test, got %s", node2.Name)
	}
	if node2.Labels["role"] != "agent" {
		t.Error("expected role label")
	}
}
