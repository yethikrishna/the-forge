package identity

import (
	"testing"
	"time"
)

func TestGenerateIdentity(t *testing.T) {
	ks, err := NewKeyStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	id, err := ks.Generate("test-agent", map[string]string{"role": "worker"})
	if err != nil {
		t.Fatal(err)
	}

	if id.Name != "test-agent" {
		t.Errorf("expected name test-agent, got %s", id.Name)
	}
	if id.Algorithm != "ed25519" {
		t.Errorf("expected ed25519, got %s", id.Algorithm)
	}
	if id.Fingerprint == "" {
		t.Error("expected fingerprint")
	}
	if id.PublicKey == "" {
		t.Error("expected public key")
	}
}

func TestGetIdentity(t *testing.T) {
	ks, _ := NewKeyStore(t.TempDir())
	id, _ := ks.Generate("test-agent", nil)

	retrieved, ok := ks.Get(id.Fingerprint)
	if !ok {
		t.Fatal("expected to find identity")
	}
	if retrieved.Name != "test-agent" {
		t.Errorf("expected test-agent, got %s", retrieved.Name)
	}
}

func TestListIdentities(t *testing.T) {
	ks, _ := NewKeyStore(t.TempDir())
	ks.Generate("agent-1", nil)
	ks.Generate("agent-2", nil)

	ids := ks.List()
	if len(ids) != 2 {
		t.Errorf("expected 2 identities, got %d", len(ids))
	}
}

func TestSignAndVerify(t *testing.T) {
	ks, _ := NewKeyStore(t.TempDir())
	id, _ := ks.Generate("signer", nil)

	manifest := Manifest{
		AgentName:    "test-agent",
		Version:      "1.0.0",
		Description:  "Test agent",
		Capabilities: []string{"code.read", "code.write"},
		Protocols:    []string{"mcp", "acp"},
		Tools:        []string{"search", "build"},
	}

	signed, err := ks.Sign(id.Fingerprint, manifest)
	if err != nil {
		t.Fatal(err)
	}

	if signed.Signature == "" {
		t.Error("expected signature")
	}
	if signed.SignerID != id.Fingerprint {
		t.Error("signer ID mismatch")
	}

	// Verify
	if err := ks.Verify(signed); err != nil {
		t.Fatalf("verification failed: %v", err)
	}
}

func TestVerifyWrongKey(t *testing.T) {
	ks, _ := NewKeyStore(t.TempDir())
	id1, _ := ks.Generate("agent-1", nil)
	ks.Generate("agent-2", nil) // Different key

	manifest := Manifest{AgentName: "test", Version: "1.0"}
	signed, _ := ks.Sign(id1.Fingerprint, manifest)

	// Tamper with the manifest
	signed.Manifest.AgentName = "tampered"

	if err := ks.Verify(signed); err == nil {
		t.Error("expected verification to fail for tampered manifest")
	}
}

func TestDeleteIdentity(t *testing.T) {
	ks, _ := NewKeyStore(t.TempDir())
	id, _ := ks.Generate("to-delete", nil)

	if err := ks.Delete(id.Fingerprint); err != nil {
		t.Fatal(err)
	}

	if _, ok := ks.Get(id.Fingerprint); ok {
		t.Error("expected identity to be deleted")
	}
}

func TestTrustRegistryGrant(t *testing.T) {
	tr, err := NewTrustRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	err = tr.Grant("abc123def456", "test-agent", Trusted, "admin", "initial trust", nil)
	if err != nil {
		t.Fatal(err)
	}

	level := tr.Check("abc123def456")
	if level != Trusted {
		t.Errorf("expected trusted, got %s", level)
	}
}

func TestTrustRegistryRevoke(t *testing.T) {
	tr, _ := NewTrustRegistry(t.TempDir())
	tr.Grant("fp1", "agent", Trusted, "admin", "", nil)

	if err := tr.Revoke("fp1"); err != nil {
		t.Fatal(err)
	}

	level := tr.Check("fp1")
	if level != TrustUnknown {
		t.Errorf("expected unknown after revoke, got %s", level)
	}
}

func TestTrustRegistryExpiry(t *testing.T) {
	tr, _ := NewTrustRegistry(t.TempDir())
	past := time.Now().Add(-1 * time.Hour)
	tr.Grant("fp2", "agent", Trusted, "admin", "", &past)

	level := tr.Check("fp2")
	if level != TrustUntrusted {
		t.Errorf("expected untrusted for expired, got %s", level)
	}
}

func TestIsTrusted(t *testing.T) {
	tr, _ := NewTrustRegistry(t.TempDir())
	tr.Grant("fp-limited", "agent", TrustLimited, "admin", "", nil)
	tr.Grant("fp-unknown", "agent", TrustUnknown, "admin", "", nil)

	if !tr.IsTrusted("fp-limited") {
		t.Error("limited should be trusted")
	}
	if tr.IsTrusted("fp-unknown") {
		t.Error("unknown should not be trusted")
	}
	if tr.IsTrusted("nonexistent") {
		t.Error("nonexistent should not be trusted")
	}
}

func TestTrustRegistryList(t *testing.T) {
	tr, _ := NewTrustRegistry(t.TempDir())
	tr.Grant("fp1", "a1", Trusted, "admin", "", nil)
	tr.Grant("fp2", "a2", TrustVerified, "admin", "", nil)

	entries := tr.List()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	ks1, _ := NewKeyStore(dir)
	id, _ := ks1.Generate("persist-test", nil)

	ks2, _ := NewKeyStore(dir)
	retrieved, ok := ks2.Get(id.Fingerprint)
	if !ok {
		t.Fatal("expected identity to persist")
	}
	if retrieved.Name != "persist-test" {
		t.Error("identity name mismatch after reload")
	}
}

func TestTrustPersistence(t *testing.T) {
	dir := t.TempDir()
	tr1, _ := NewTrustRegistry(dir)
	tr1.Grant("fp1", "agent", Trusted, "admin", "test", nil)

	tr2, _ := NewTrustRegistry(dir)
	if tr2.Check("fp1") != Trusted {
		t.Error("expected trust to persist")
	}
}

func TestFormatIdentity(t *testing.T) {
	id := Identity{
		Name:        "test",
		Fingerprint: "abcdef1234567890",
		Algorithm:   "ed25519",
		CreatedAt:   time.Now(),
		Labels:      map[string]string{"env": "prod"},
	}
	output := FormatIdentity(id)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatTrustEntry(t *testing.T) {
	e := TrustEntry{
		IdentityFingerprint: "abcdef1234567890",
		Name:                "test",
		TrustLevel:          Trusted,
		GrantedBy:           "admin",
	}
	output := FormatTrustEntry(e)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
