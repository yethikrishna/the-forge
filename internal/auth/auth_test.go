package auth_test

import (
	"os"
	"testing"

	"github.com/forge/sword/internal/auth"
)

func TestStoreAndGet(t *testing.T) {
	dir := t.TempDir()
	ks := auth.NewKeyStore(dir)

	key, err := ks.Store("openai", auth.ProviderKey, "openai", "sk-test-key-12345678")
	if err != nil {
		t.Fatalf("store error: %v", err)
	}

	if key.Name != "openai" {
		t.Errorf("expected openai, got %s", key.Name)
	}
	if key.Type != auth.ProviderKey {
		t.Errorf("expected provider, got %s", key.Type)
	}
	if key.Prefix != "sk-test-..." {
		t.Errorf("expected prefix, got %s", key.Prefix)
	}

	// Retrieve key value
	value, err := ks.Get("openai")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if value != "sk-test-key-12345678" {
		t.Errorf("expected key value, got %s", value)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	ks := auth.NewKeyStore(dir)

	ks.Store("openai", auth.ProviderKey, "openai", "sk-openai-key")
	ks.Store("anthropic", auth.ProviderKey, "anthropic", "sk-ant-key")

	keys, err := ks.List()
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	ks := auth.NewKeyStore(dir)

	ks.Store("test-key", auth.AgentKey, "", "test-value")
	ks.Delete("test-key")

	_, err := ks.Get("test-key")
	if err == nil {
		t.Error("key should be deleted")
	}
}

func TestValidate(t *testing.T) {
	dir := t.TempDir()
	ks := auth.NewKeyStore(dir)

	ks.Store("test", auth.ProviderKey, "openai", "sk-secret-key")

	valid, err := ks.Validate("test", "sk-secret-key")
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if !valid {
		t.Error("key should validate")
	}

	valid, _ = ks.Validate("test", "wrong-key")
	if valid {
		t.Error("wrong key should not validate")
	}
}

func TestGenerateKey(t *testing.T) {
	key := auth.GenerateKey("forge_")
	if len(key) < 10 {
		t.Errorf("generated key too short: %s", key)
	}
	if key[:6] != "forge_" {
		t.Errorf("key should start with forge_, got %s", key[:6])
	}
}

func TestGetNonExistent(t *testing.T) {
	dir := t.TempDir()
	ks := auth.NewKeyStore(dir)

	_, err := ks.Get("nonexistent")
	if err == nil {
		t.Error("should error for non-existent key")
	}
}

func TestDefaultDir(t *testing.T) {
	ks := auth.NewKeyStore("")
	if ks == nil {
		t.Error("keystore should not be nil")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
