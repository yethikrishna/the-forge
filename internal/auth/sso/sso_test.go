package sso

import (
	"strings"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	session, err := m.CreateSession("user-1", ProviderOIDC, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.UserID != "user-1" {
		t.Fatal("wrong user ID")
	}
	if session.Provider != ProviderOIDC {
		t.Fatal("wrong provider")
	}
	if session.Token == "" {
		t.Fatal("empty token")
	}
}

func TestValidateSession(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	session, _ := m.CreateSession("user-1", ProviderOIDC, time.Hour)

	validated, err := m.ValidateSession(session.Token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if validated.UserID != "user-1" {
		t.Fatal("wrong user in validated session")
	}
}

func TestValidateExpiredSession(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	session, _ := m.CreateSession("user-1", ProviderOIDC, -time.Hour) // already expired

	_, err := m.ValidateSession(session.Token)
	if err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestValidateInvalidSession(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	_, err := m.ValidateSession("invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestRevokeSession(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	session, _ := m.CreateSession("user-1", ProviderOIDC, time.Hour)

	if err := m.RevokeSession(session.ID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	_, err := m.ValidateSession(session.Token)
	if err == nil {
		t.Fatal("expected error after revocation")
	}
}

func TestCreateAPIKey(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	key, err := m.CreateAPIKey("test-key", "user-1", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if key.Key == "" {
		t.Fatal("empty key")
	}
	if !strings.HasPrefix(key.Key, "forge_") {
		t.Fatal("key should start with forge_ prefix")
	}
	if len(key.Scopes) != 2 {
		t.Fatal("wrong number of scopes")
	}
}

func TestValidateAPIKey(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	key, _ := m.CreateAPIKey("test-key", "user-1", []string{"read"}, nil)

	validated, err := m.ValidateAPIKey(key.Key)
	if err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
	if validated.UserID != "user-1" {
		t.Fatal("wrong user")
	}
	if validated.RequestCount != 1 {
		t.Fatal("request count should be incremented")
	}
}

func TestValidateInvalidAPIKey(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	_, err := m.ValidateAPIKey("forge_invalidkey")
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}
}

func TestRevokeAPIKey(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	key, _ := m.CreateAPIKey("test-key", "user-1", []string{"read"}, nil)

	if err := m.RevokeAPIKey(key.ID); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}

	_, err := m.ValidateAPIKey(key.Key)
	if err == nil {
		t.Fatal("expected error for revoked key")
	}
}

func TestExpiredAPIKey(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	past := time.Now().UTC().Add(-time.Hour)
	key, _ := m.CreateAPIKey("test-key", "user-1", []string{"read"}, &past)

	_, err := m.ValidateAPIKey(key.Key)
	if err == nil {
		t.Fatal("expected error for expired key")
	}
}

func TestListAPIKeys(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	m.CreateAPIKey("key-1", "user-1", []string{"read"}, nil)
	m.CreateAPIKey("key-2", "user-1", []string{"write"}, nil)
	m.CreateAPIKey("key-3", "user-2", []string{"read"}, nil)

	allKeys := m.ListAPIKeys("")
	if len(allKeys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(allKeys))
	}

	user1Keys := m.ListAPIKeys("user-1")
	if len(user1Keys) != 2 {
		t.Fatalf("expected 2 keys for user-1, got %d", len(user1Keys))
	}
}

func TestListSessions(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	m.CreateSession("user-1", ProviderOIDC, time.Hour)
	m.CreateSession("user-2", ProviderSAML, time.Hour)

	sessions := m.ListSessions("")
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	user1Sessions := m.ListSessions("user-1")
	if len(user1Sessions) != 1 {
		t.Fatalf("expected 1 session for user-1, got %d", len(user1Sessions))
	}
}

func TestCleanupExpired(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	m.CreateSession("user-1", ProviderOIDC, -time.Hour) // expired
	m.CreateSession("user-2", ProviderOIDC, time.Hour)  // valid

	sessionsRemoved, _ := m.CleanupExpired()
	if sessionsRemoved != 1 {
		t.Fatalf("expected 1 session removed, got %d", sessionsRemoved)
	}
}

func TestRegisterOIDC(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	config := OIDCConfig{
		Issuer:       "https://accounts.google.com",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/callback",
	}

	if err := m.RegisterOIDC("google", config); err != nil {
		t.Fatalf("RegisterOIDC: %v", err)
	}

	cfg, ok := m.GetOIDCConfig("google")
	if !ok {
		t.Fatal("OIDC config not found")
	}
	if cfg.Issuer != "https://accounts.google.com" {
		t.Fatal("wrong issuer")
	}
	if len(cfg.Scopes) == 0 {
		t.Fatal("default scopes not set")
	}
}

func TestRegisterSAML(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	config := SAMLConfig{
		EntityID:    "forge",
		SSOURL:      "https://idp.example.com/sso",
		Certificate: "MIIC...",
	}

	if err := m.RegisterSAML("okta", config); err != nil {
		t.Fatalf("RegisterSAML: %v", err)
	}

	cfg, ok := m.GetSAMLConfig("okta")
	if !ok {
		t.Fatal("SAML config not found")
	}
	if cfg.SSOURL != "https://idp.example.com/sso" {
		t.Fatal("wrong SSO URL")
	}
}

func TestListProviders(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	m.RegisterOIDC("google", OIDCConfig{Issuer: "https://accounts.google.com"})
	m.RegisterSAML("okta", SAMLConfig{EntityID: "forge"})

	providers := m.ListProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
}

func TestStats(t *testing.T) {
	m := NewSSOManager(t.TempDir())

	m.RegisterOIDC("google", OIDCConfig{Issuer: "https://accounts.google.com"})
	m.CreateSession("user-1", ProviderOIDC, time.Hour)
	m.CreateAPIKey("test", "user-1", []string{"read"}, nil)

	stats := m.Stats()
	if stats.OIDCProviders != 1 {
		t.Fatalf("expected 1 OIDC provider, got %d", stats.OIDCProviders)
	}
	if stats.ActiveSessions != 1 {
		t.Fatalf("expected 1 active session, got %d", stats.ActiveSessions)
	}
	if stats.ActiveAPIKeys != 1 {
		t.Fatalf("expected 1 active API key, got %d", stats.ActiveAPIKeys)
	}
}

func TestFormatSession(t *testing.T) {
	session := &Session{
		ID:        "sess-1",
		UserID:    "user-1",
		Provider:  ProviderOIDC,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	output := FormatSession(session)
	if len(output) == 0 {
		t.Fatal("empty session format")
	}
}

func TestFormatAPIKey(t *testing.T) {
	key := &APIKey{
		ID:        "key-1",
		Name:      "Test Key",
		UserID:    "user-1",
		Prefix:    "forge_abc",
		Scopes:    []string{"read"},
		Active:    true,
		CreatedAt: time.Now(),
	}
	output := FormatAPIKey(key)
	if len(output) == 0 {
		t.Fatal("empty API key format")
	}
}

func TestFormatStats(t *testing.T) {
	stats := SSOStats{
		OIDCProviders:  2,
		ActiveSessions: 5,
		ActiveAPIKeys:  3,
	}
	output := FormatStats(stats)
	if len(output) == 0 {
		t.Fatal("empty stats format")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	m1 := NewSSOManager(dir)

	m1.CreateSession("user-1", ProviderOIDC, time.Hour)
	m1.CreateAPIKey("test", "user-1", []string{"read"}, nil)

	m2 := NewSSOManager(dir)
	stats := m2.Stats()
	if stats.TotalAPIKeys < 1 {
		t.Fatalf("expected persisted API keys, got %d", stats.TotalAPIKeys)
	}
}

// strings used above
