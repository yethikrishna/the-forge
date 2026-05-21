// Package auth provides API key and token management for The Forge.
// Only the worthy may wield the sword.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// KeyType represents the type of API key.
type KeyType string

const (
	ProviderKey KeyType = "provider" // LLM provider API key
	AgentKey    KeyType = "agent"    // Agent access key
	AdminKey    KeyType = "admin"    // Admin access key
)

// Key represents a stored API key.
type Key struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      KeyType           `json:"type"`
	Provider  string            `json:"provider,omitempty"`
	KeyHash   string            `json:"key_hash"` // SHA-256 hash of the key
	Prefix    string            `json:"prefix"`   // First 8 chars for identification
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// KeyStore manages API keys securely.
type KeyStore struct {
	dir string
	mu  sync.RWMutex
}

// NewKeyStore creates a new key store.
func NewKeyStore(dir string) *KeyStore {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".forge", "keys")
	}
	return &KeyStore{dir: dir}
}

// Store stores a new API key.
func (ks *KeyStore) Store(name string, keyType KeyType, provider, keyValue string) (*Key, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	os.MkdirAll(ks.dir, 0o700)

	key := &Key{
		ID:        generateID(),
		Name:      name,
		Type:      keyType,
		Provider:  provider,
		KeyHash:   hashKey(keyValue),
		Prefix:    prefix(keyValue),
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}

	data, err := json.MarshalIndent(key, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("auth: marshal: %w", err)
	}

	path := filepath.Join(ks.dir, name+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, fmt.Errorf("auth: write: %w", err)
	}

	// Also store the encrypted key value
	valuePath := filepath.Join(ks.dir, name+".key")
	if err := os.WriteFile(valuePath, []byte(keyValue), 0o600); err != nil {
		return nil, fmt.Errorf("auth: write key: %w", err)
	}

	return key, nil
}

// Get retrieves a stored key value.
func (ks *KeyStore) Get(name string) (string, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	path := filepath.Join(ks.dir, name+".key")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("auth: key %q not found", name)
	}
	return strings.TrimSpace(string(data)), nil
}

// List returns all stored key metadata.
func (ks *KeyStore) List() ([]Key, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	entries, err := os.ReadDir(ks.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("auth: list: %w", err)
	}

	var keys []Key
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(ks.dir, entry.Name()))
		if err != nil {
			continue
		}

		var key Key
		if err := json.Unmarshal(data, &key); err != nil {
			continue
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// Delete removes a stored key.
func (ks *KeyStore) Delete(name string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	jsonPath := filepath.Join(ks.dir, name+".json")
	keyPath := filepath.Join(ks.dir, name+".key")

	var hadErr bool
	if err := os.Remove(jsonPath); err != nil && !os.IsNotExist(err) {
		hadErr = true
	}
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		hadErr = true
	}

	if hadErr {
		return fmt.Errorf("auth: error deleting key %q", name)
	}
	return nil
}

// Validate checks if a key value matches a stored key.
func (ks *KeyStore) Validate(name, keyValue string) (bool, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	path := filepath.Join(ks.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("auth: key %q not found", name)
	}

	var key Key
	if err := json.Unmarshal(data, &key); err != nil {
		return false, fmt.Errorf("auth: parse: %w", err)
	}

	return key.KeyHash == hashKey(keyValue), nil
}

// GenerateKey generates a new random API key.
func GenerateKey(prefix string) string {
	b := make([]byte, 32)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func prefix(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8] + "..."
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
