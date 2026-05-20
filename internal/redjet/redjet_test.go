package redjet_test

import (
	"context"
	"testing"
	"time"

	"github.com/forge/sword/internal/redjet"
)

func TestNewClient(t *testing.T) {
	cfg := redjet.DefaultConfig()
	client := redjet.New(cfg)
	if client == nil {
		t.Fatal("client should not be nil")
	}
	client.Close()
}

func TestNewClientDefaults(t *testing.T) {
	client := redjet.New(redjet.Config{})
	if client == nil {
		t.Fatal("client should not be nil")
	}
	client.Close()
}

func TestPingWithoutServer(t *testing.T) {
	cfg := redjet.Config{
		Addr:        "localhost:16379",
		DialTimeout: 100 * time.Millisecond,
	}
	client := redjet.New(cfg)
	defer client.Close()

	_, err := client.Ping(context.Background())
	if err == nil {
		t.Error("expected error when Redis is not running")
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := redjet.DefaultConfig()
	if cfg.Addr != "localhost:6379" {
		t.Errorf("expected localhost:6379, got %s", cfg.Addr)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("expected pool size 10, got %d", cfg.PoolSize)
	}
}
