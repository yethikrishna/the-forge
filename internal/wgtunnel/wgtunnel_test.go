package wgtunnel_test

import (
	"testing"

	"github.com/forge/sword/internal/wgtunnel"
)

func TestGenerateKey(t *testing.T) {
	key, err := wgtunnel.GenerateKey()
	if err != nil {
		t.Fatalf("generate key error: %v", err)
	}
	if key.String() == "" {
		t.Error("key string should not be empty")
	}

	// Generate a second key and verify they're different
	key2, _ := wgtunnel.GenerateKey()
	if key.String() == key2.String() {
		t.Error("two generated keys should be different")
	}
}

func TestNewManager(t *testing.T) {
	mgr := wgtunnel.NewManager()
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestCreateTunnel(t *testing.T) {
	key, _ := wgtunnel.GenerateKey()
	mgr := wgtunnel.NewManager()

	tunnel, err := mgr.CreateTunnel(wgtunnel.TunnelConfig{
		Name:       "wg0",
		Port:       51820,
		PrivateKey: key,
		Address:    "10.0.0.1/24",
	})
	if err != nil {
		t.Fatalf("create tunnel error: %v", err)
	}
	if tunnel.Status != wgtunnel.TunnelCreated {
		t.Errorf("expected created status, got %s", tunnel.Status)
	}
}

func TestCreateTunnelDuplicate(t *testing.T) {
	key, _ := wgtunnel.GenerateKey()
	mgr := wgtunnel.NewManager()

	mgr.CreateTunnel(wgtunnel.TunnelConfig{Name: "wg0", PrivateKey: key})
	_, err := mgr.CreateTunnel(wgtunnel.TunnelConfig{Name: "wg0", PrivateKey: key})
	if err == nil {
		t.Error("should error for duplicate tunnel")
	}
}

func TestGetTunnel(t *testing.T) {
	key, _ := wgtunnel.GenerateKey()
	mgr := wgtunnel.NewManager()
	mgr.CreateTunnel(wgtunnel.TunnelConfig{Name: "wg0", PrivateKey: key})

	tunnel, err := mgr.GetTunnel("wg0")
	if err != nil {
		t.Fatalf("get tunnel error: %v", err)
	}
	if tunnel.ID != "wg0" {
		t.Errorf("expected wg0, got %s", tunnel.ID)
	}
}

func TestDeleteTunnel(t *testing.T) {
	key, _ := wgtunnel.GenerateKey()
	mgr := wgtunnel.NewManager()
	mgr.CreateTunnel(wgtunnel.TunnelConfig{Name: "wg0", PrivateKey: key})

	if err := mgr.DeleteTunnel("wg0"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	if _, err := mgr.GetTunnel("wg0"); err == nil {
		t.Error("should error after delete")
	}
}

func TestGenerateConfig(t *testing.T) {
	key, _ := wgtunnel.GenerateKey()
	peerKey, _ := wgtunnel.GenerateKey()
	mgr := wgtunnel.NewManager()

	mgr.CreateTunnel(wgtunnel.TunnelConfig{
		Name:       "wg0",
		Port:       51820,
		PrivateKey: key,
		Address:    "10.0.0.1/24",
		Peers: []wgtunnel.PeerConfig{
			{
				PublicKey:  peerKey.PublicKey(),
				Endpoint:   "1.2.3.4:51820",
				AllowedIPs: []string{"10.0.0.2/32"},
			},
		},
	})

	config, err := mgr.GenerateTunnelConfig("wg0")
	if err != nil {
		t.Fatalf("generate config error: %v", err)
	}
	if config == "" {
		t.Error("config should not be empty")
	}
}

func TestFindFreePort(t *testing.T) {
	port, err := wgtunnel.FindFreePort()
	if err != nil {
		t.Fatalf("find free port error: %v", err)
	}
	if port <= 0 {
		t.Errorf("expected positive port, got %d", port)
	}
}
