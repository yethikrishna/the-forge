// Package identity provides cryptographic agent identities, signed manifests,
// and a trust registry for verifying agent authenticity and permissions.
//
// Trust, but verify.
package identity

import (
	"crypto/ed25519"
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

// Identity represents a cryptographic agent identity.
type Identity struct {
	Name        string            `json:"name"`
	PublicKey   string            `json:"public_key"`
	Algorithm   string            `json:"algorithm"`
	CreatedAt   time.Time         `json:"created_at"`
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// SignedManifest is a manifest signed by an agent's private key.
type SignedManifest struct {
	Manifest   Manifest `json:"manifest"`
	Signature  string   `json:"signature"`
	SignerID   string   `json:"signer_id"`
	SignedAt   time.Time `json:"signed_at"`
}

// Manifest describes an agent's capabilities and configuration.
type Manifest struct {
	AgentName    string   `json:"agent_name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Protocols    []string `json:"protocols"`
	Tools        []string `json:"tools"`
	Permissions  []string `json:"permissions"`
	Checksum     string   `json:"checksum"`
}

// TrustLevel represents how much an agent is trusted.
type TrustLevel string

const (
	TrustUnknown  TrustLevel = "unknown"
	TrustUntrusted TrustLevel = "untrusted"
	TrustLimited  TrustLevel = "limited"
	Trusted       TrustLevel = "trusted"
	TrustVerified TrustLevel = "verified"
)

// TrustEntry records the trust level for an agent identity.
type TrustEntry struct {
	IdentityFingerprint string    `json:"identity_fingerprint"`
	Name                string    `json:"name"`
	TrustLevel          TrustLevel `json:"trust_level"`
	GrantedBy           string    `json:"granted_by"`
	GrantedAt           time.Time `json:"granted_at"`
	Reason              string    `json:"reason,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
}

// KeyStore manages agent identity keys.
type KeyStore struct {
	mu   sync.RWMutex
	dir  string
	keys map[string]*keyEntry // fingerprint → entry
}

type keyEntry struct {
	Identity   Identity
	PrivateKey ed25519.PrivateKey
}

// NewKeyStore creates or loads a key store from the given directory.
func NewKeyStore(dir string) (*KeyStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("identity: create dir: %w", err)
	}
	ks := &KeyStore{
		dir:  dir,
		keys: make(map[string]*keyEntry),
	}
	ks.load()
	return ks, nil
}

// Generate creates a new agent identity with an Ed25519 key pair.
func (ks *KeyStore) Generate(name string, labels map[string]string) (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("identity: generate key: %w", err)
	}

	fp := fingerprint(pub)
	id := &Identity{
		Name:        name,
		PublicKey:   hex.EncodeToString(pub),
		Algorithm:   "ed25519",
		CreatedAt:   time.Now(),
		Fingerprint: fp,
		Labels:      labels,
	}

	if id.Labels == nil {
		id.Labels = make(map[string]string)
	}

	ks.mu.Lock()
	ks.keys[fp] = &keyEntry{Identity: *id, PrivateKey: priv}
	ks.mu.Unlock()

	if err := ks.save(fp); err != nil {
		return nil, err
	}

	return id, nil
}

// Get retrieves an identity by fingerprint.
func (ks *KeyStore) Get(fingerprint string) (*Identity, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	entry, ok := ks.keys[fingerprint]
	if !ok {
		return nil, false
	}
	id := entry.Identity
	return &id, true
}

// List returns all identities.
func (ks *KeyStore) List() []Identity {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	ids := make([]Identity, 0, len(ks.keys))
	for _, entry := range ks.keys {
		ids = append(ids, entry.Identity)
	}
	return ids
}

// Sign creates a signed manifest for an agent.
func (ks *KeyStore) Sign(fingerprint string, manifest Manifest) (*SignedManifest, error) {
	ks.mu.RLock()
	entry, ok := ks.keys[fingerprint]
	if !ok {
		ks.mu.RUnlock()
		return nil, fmt.Errorf("identity: unknown fingerprint: %s", fingerprint)
	}
	ks.mu.RUnlock()

	// Compute manifest checksum
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("identity: marshal manifest: %w", err)
	}
	manifest.Checksum = hex.EncodeToString(sha256.New().Sum(data))

	// Re-marshal with checksum
	data, _ = json.Marshal(manifest)

	sig := ed25519.Sign(entry.PrivateKey, data)

	return &SignedManifest{
		Manifest:  manifest,
		Signature: hex.EncodeToString(sig),
		SignerID:  fingerprint,
		SignedAt:  time.Now(),
	}, nil
}

// Verify checks a signed manifest against a known identity.
func (ks *KeyStore) Verify(sm *SignedManifest) error {
	ks.mu.RLock()
	entry, ok := ks.keys[sm.SignerID]
	ks.mu.RUnlock()

	if !ok {
		return fmt.Errorf("identity: unknown signer: %s", sm.SignerID)
	}

	data, err := json.Marshal(sm.Manifest)
	if err != nil {
		return fmt.Errorf("identity: marshal for verify: %w", err)
	}

	sig, err := hex.DecodeString(sm.Signature)
	if err != nil {
		return fmt.Errorf("identity: decode signature: %w", err)
	}

	pub, err := hex.DecodeString(entry.Identity.PublicKey)
	if err != nil {
		return fmt.Errorf("identity: decode public key: %w", err)
	}

	if !ed25519.Verify(pub, data, sig) {
		return fmt.Errorf("identity: signature verification failed")
	}

	return nil
}

// Delete removes an identity.
func (ks *KeyStore) Delete(fingerprint string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if _, ok := ks.keys[fingerprint]; !ok {
		return fmt.Errorf("identity: not found: %s", fingerprint)
	}

	delete(ks.keys, fingerprint)
	os.Remove(filepath.Join(ks.dir, fingerprint+".json"))
	return nil
}

// load reads all identities from disk.
func (ks *KeyStore) load() {
	entries, err := os.ReadDir(ks.dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(ks.dir, e.Name()))
		if err != nil {
			continue
		}

		var stored struct {
			Identity   Identity `json:"identity"`
			PrivateKey string   `json:"private_key"`
		}

		if err := json.Unmarshal(data, &stored); err != nil {
			continue
		}

		privBytes, err := hex.DecodeString(stored.PrivateKey)
		if err != nil {
			continue
		}

		ks.keys[stored.Identity.Fingerprint] = &keyEntry{
			Identity:   stored.Identity,
			PrivateKey: ed25519.PrivateKey(privBytes),
		}
	}
}

// save persists an identity to disk.
func (ks *KeyStore) save(fingerprint string) error {
	ks.mu.RLock()
	entry, ok := ks.keys[fingerprint]
	ks.mu.RUnlock()

	if !ok {
		return fmt.Errorf("identity: not found: %s", fingerprint)
	}

	stored := struct {
		Identity   Identity `json:"identity"`
		PrivateKey string   `json:"private_key"`
	}{
		Identity:   entry.Identity,
		PrivateKey: hex.EncodeToString(entry.PrivateKey),
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("identity: marshal: %w", err)
	}

	return os.WriteFile(filepath.Join(ks.dir, fingerprint+".json"), data, 0o600)
}

// fingerprint computes a short fingerprint for a public key.
func fingerprint(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:])[:16]
}

// TrustRegistry manages trust levels for agent identities.
type TrustRegistry struct {
	mu      sync.RWMutex
	dir     string
	entries map[string]TrustEntry // fingerprint → entry
}

// NewTrustRegistry creates or loads a trust registry.
func NewTrustRegistry(dir string) (*TrustRegistry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tr := &TrustRegistry{
		dir:     dir,
		entries: make(map[string]TrustEntry),
	}
	tr.load()
	return tr, nil
}

// Grant sets the trust level for an identity.
func (tr *TrustRegistry) Grant(fp string, name string, level TrustLevel, grantedBy, reason string, expiresAt *time.Time) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	entry := TrustEntry{
		IdentityFingerprint: fp,
		Name:                name,
		TrustLevel:          level,
		GrantedBy:           grantedBy,
		GrantedAt:           time.Now(),
		Reason:              reason,
		ExpiresAt:           expiresAt,
	}

	tr.entries[fp] = entry
	return tr.save()
}

// Revoke removes trust for an identity.
func (tr *TrustRegistry) Revoke(fp string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	delete(tr.entries, fp)
	return tr.save()
}

// Check returns the trust level for an identity.
func (tr *TrustRegistry) Check(fp string) TrustLevel {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	entry, ok := tr.entries[fp]
	if !ok {
		return TrustUnknown
	}

	// Check expiration
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		return TrustUntrusted
	}

	return entry.TrustLevel
}

// GetEntry returns the full trust entry.
func (tr *TrustRegistry) GetEntry(fp string) (TrustEntry, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	e, ok := tr.entries[fp]
	return e, ok
}

// List returns all trust entries.
func (tr *TrustRegistry) List() []TrustEntry {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	out := make([]TrustEntry, 0, len(tr.entries))
	for _, e := range tr.entries {
		out = append(out, e)
	}
	return out
}

// IsTrusted returns true if the identity has at least "limited" trust.
func (tr *TrustRegistry) IsTrusted(fp string) bool {
	level := tr.Check(fp)
	return level == TrustLimited || level == Trusted || level == TrustVerified
}

func (tr *TrustRegistry) load() {
	data, err := os.ReadFile(filepath.Join(tr.dir, "trust.json"))
	if err != nil {
		return
	}
	var entries []TrustEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	for _, e := range entries {
		tr.entries[e.IdentityFingerprint] = e
	}
}

func (tr *TrustRegistry) save() error {
	entries := make([]TrustEntry, 0, len(tr.entries))
	for _, e := range tr.entries {
		entries = append(entries, e)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(tr.dir, "trust.json"), data, 0o644)
}

// FormatIdentity renders an identity for display.
func FormatIdentity(id Identity) string {
	labels := ""
	if len(id.Labels) > 0 {
		parts := make([]string, 0, len(id.Labels))
		for k, v := range id.Labels {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		labels = " [" + strings.Join(parts, ",") + "]"
	}
	return fmt.Sprintf("%-20s %s…%s  ed25519  %s%s",
		id.Name, id.Fingerprint[:8], id.Fingerprint[len(id.Fingerprint)-4:],
		id.CreatedAt.Format("2006-01-02"), labels)
}

// FormatTrustEntry renders a trust entry for display.
func FormatTrustEntry(e TrustEntry) string {
	expires := "never"
	if e.ExpiresAt != nil {
		expires = e.ExpiresAt.Format("2006-01-02")
	}
	return fmt.Sprintf("%s…%s  %-20s  %-10s  by:%s  expires:%s",
		e.IdentityFingerprint[:8], e.IdentityFingerprint[len(e.IdentityFingerprint)-4:],
		e.Name, e.TrustLevel, e.GrantedBy, expires)
}
