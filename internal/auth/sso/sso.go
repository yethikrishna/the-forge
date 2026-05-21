// Package sso provides Single Sign-On integration for Forge.
// The gate recognizes its own — OIDC, SAML, and API key authentication.
package sso

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Provider represents an SSO provider type.
type Provider string

const (
	ProviderOIDC   Provider = "oidc"
	ProviderSAML   Provider = "saml"
	ProviderAPIKey Provider = "api_key"
	ProviderLocal  Provider = "local"
)

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	Issuer       string        `json:"issuer"`
	ClientID     string        `json:"client_id"`
	ClientSecret string        `json:"client_secret"`
	RedirectURL  string        `json:"redirect_url"`
	Scopes       []string      `json:"scopes"`
	Endpoints    OIDCEndpoints `json:"endpoints,omitempty"`
}

// OIDCEndpoints holds OIDC endpoint URLs.
type OIDCEndpoints struct {
	Authorization string `json:"authorization"`
	Token         string `json:"token"`
	UserInfo      string `json:"userinfo"`
	JWKS          string `json:"jwks"`
}

// SAMLConfig holds SAML provider configuration.
type SAMLConfig struct {
	EntityID         string            `json:"entity_id"`
	SSOURL           string            `json:"sso_url"`
	SLOURL           string            `json:"slo_url"`
	Certificate      string            `json:"certificate"`
	AttributeMapping map[string]string `json:"attribute_mapping,omitempty"`
}

// APIKeyConfig holds API key configuration.
type APIKeyConfig struct {
	Prefix     string        `json:"prefix"`     // e.g., "forge_"
	KeyLength  int           `json:"key_length"` // bytes of randomness
	Expiration time.Duration `json:"expiration"` // 0 = never expires
	RateLimit  float64       `json:"rate_limit"` // requests per second
}

// Session represents an authenticated session.
type Session struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	Provider     Provider          `json:"provider"`
	Token        string            `json:"token"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at"`
	CreatedAt    time.Time         `json:"created_at"`
	LastUsed     time.Time         `json:"last_used"`
	Claims       map[string]string `json:"claims,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
}

// APIKey represents an API key.
type APIKey struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Key          string     `json:"key"`      // only shown at creation
	KeyHash      string     `json:"key_hash"` // stored hash
	Prefix       string     `json:"prefix"`   // first 8 chars for identification
	UserID       string     `json:"user_id"`
	Scopes       []string   `json:"scopes"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsed     *time.Time `json:"last_used,omitempty"`
	RequestCount int64      `json:"request_count"`
	Active       bool       `json:"active"`
}

// SSOManager manages SSO authentication.
type SSOManager struct {
	oidcConfigs  map[string]OIDCConfig
	samlConfigs  map[string]SAMLConfig
	apiKeyConfig APIKeyConfig
	sessions     map[string]*Session
	apiKeys      map[string]*APIKey
	secretKey    []byte
	storeDir     string
	mu           sync.RWMutex
}

// NewSSOManager creates a new SSO manager.
func NewSSOManager(storeDir string) *SSOManager {
	secret := make([]byte, 32)
	rand.Read(secret)

	m := &SSOManager{
		oidcConfigs: make(map[string]OIDCConfig),
		samlConfigs: make(map[string]SAMLConfig),
		sessions:    make(map[string]*Session),
		apiKeys:     make(map[string]*APIKey),
		secretKey:   secret,
		storeDir:    storeDir,
		apiKeyConfig: APIKeyConfig{
			Prefix:    "forge_",
			KeyLength: 32,
		},
	}

	os.MkdirAll(storeDir, 0o755)
	m.load()
	return m
}

// RegisterOIDC registers an OIDC provider.
func (m *SSOManager) RegisterOIDC(name string, config OIDCConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Scopes == nil {
		config.Scopes = []string{"openid", "profile", "email"}
	}

	m.oidcConfigs[name] = config
	m.save()
	return nil
}

// RegisterSAML registers a SAML provider.
func (m *SSOManager) RegisterSAML(name string, config SAMLConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.AttributeMapping == nil {
		config.AttributeMapping = map[string]string{
			"email": "email",
			"name":  "displayName",
		}
	}

	m.samlConfigs[name] = config
	m.save()
	return nil
}

// CreateSession creates a new authenticated session.
func (m *SSOManager) CreateSession(userID string, provider Provider, ttl time.Duration) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token := m.generateToken()
	session := &Session{
		ID:        fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		UserID:    userID,
		Provider:  provider,
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(ttl),
		CreatedAt: time.Now().UTC(),
		LastUsed:  time.Now().UTC(),
	}

	m.sessions[session.ID] = session
	m.save()
	return session, nil
}

// ValidateSession validates a session token.
func (m *SSOManager) ValidateSession(token string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, session := range m.sessions {
		if session.Token == token {
			if time.Now().UTC().After(session.ExpiresAt) {
				return nil, fmt.Errorf("session expired")
			}
			return session, nil
		}
	}
	return nil, fmt.Errorf("invalid session token")
}

// RevokeSession revokes a session.
func (m *SSOManager) RevokeSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	m.save()
	return nil
}

// CreateAPIKey creates a new API key.
func (m *SSOManager) CreateAPIKey(name, userID string, scopes []string, expiresAt *time.Time) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyBytes := make([]byte, m.apiKeyConfig.KeyLength)
	rand.Read(keyBytes)
	key := m.apiKeyConfig.Prefix + base64.RawURLEncoding.EncodeToString(keyBytes)

	hash := m.hashKey(key)

	apiKey := &APIKey{
		ID:        fmt.Sprintf("key-%d", time.Now().UnixNano()),
		Name:      name,
		Key:       key, // only available at creation
		KeyHash:   hash,
		Prefix:    key[:8],
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
		Active:    true,
	}

	m.apiKeys[apiKey.ID] = apiKey
	m.save()
	return apiKey, nil
}

// ValidateAPIKey validates an API key.
func (m *SSOManager) ValidateAPIKey(key string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hash := m.hashKey(key)
	for _, apiKey := range m.apiKeys {
		if apiKey.KeyHash == hash {
			if !apiKey.Active {
				return nil, fmt.Errorf("API key revoked")
			}
			if apiKey.ExpiresAt != nil && time.Now().UTC().After(*apiKey.ExpiresAt) {
				return nil, fmt.Errorf("API key expired")
			}
			// Update usage stats
			now := time.Now().UTC()
			apiKey.LastUsed = &now
			apiKey.RequestCount++
			return apiKey, nil
		}
	}
	return nil, fmt.Errorf("invalid API key")
}

// RevokeAPIKey revokes an API key.
func (m *SSOManager) RevokeAPIKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.apiKeys[keyID]
	if !ok {
		return fmt.Errorf("API key %s not found", keyID)
	}
	key.Active = false
	m.save()
	return nil
}

// ListAPIKeys lists API keys for a user.
func (m *SSOManager) ListAPIKeys(userID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []*APIKey
	for _, k := range m.apiKeys {
		if userID == "" || k.UserID == userID {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt.After(keys[j].CreatedAt)
	})
	return keys
}

// ListSessions lists active sessions.
func (m *SSOManager) ListSessions(userID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, s := range m.sessions {
		if userID == "" || s.UserID == userID {
			if time.Now().UTC().Before(s.ExpiresAt) {
				sessions = append(sessions, s)
			}
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})
	return sessions
}

// CleanupExpired removes expired sessions and keys.
func (m *SSOManager) CleanupExpired() (sessionsRemoved, keysExpired int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	for id, s := range m.sessions {
		if now.After(s.ExpiresAt) {
			delete(m.sessions, id)
			sessionsRemoved++
		}
	}

	for _, k := range m.apiKeys {
		if k.ExpiresAt != nil && now.After(*k.ExpiresAt) && k.Active {
			k.Active = false
			keysExpired++
		}
	}

	m.save()
	return
}

// Stats returns SSO statistics.
func (m *SSOManager) Stats() SSOStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := SSOStats{
		OIDCProviders:  len(m.oidcConfigs),
		SAMLProviders:  len(m.samlConfigs),
		ActiveSessions: 0,
		TotalAPIKeys:   len(m.apiKeys),
		ActiveAPIKeys:  0,
	}

	now := time.Now().UTC()
	for _, s := range m.sessions {
		if now.Before(s.ExpiresAt) {
			stats.ActiveSessions++
		}
	}
	for _, k := range m.apiKeys {
		if k.Active {
			stats.ActiveAPIKeys++
		}
	}

	return stats
}

// SSOStats holds SSO statistics.
type SSOStats struct {
	OIDCProviders  int `json:"oidc_providers"`
	SAMLProviders  int `json:"saml_providers"`
	ActiveSessions int `json:"active_sessions"`
	TotalAPIKeys   int `json:"total_api_keys"`
	ActiveAPIKeys  int `json:"active_api_keys"`
}

// GetOIDCConfig returns OIDC config for a provider.
func (m *SSOManager) GetOIDCConfig(name string) (OIDCConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.oidcConfigs[name]
	return cfg, ok
}

// GetSAMLConfig returns SAML config for a provider.
func (m *SSOManager) GetSAMLConfig(name string) (SAMLConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.samlConfigs[name]
	return cfg, ok
}

// ListProviders lists all configured providers.
func (m *SSOManager) ListProviders() []ProviderInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var providers []ProviderInfo
	for name := range m.oidcConfigs {
		providers = append(providers, ProviderInfo{Name: name, Type: "oidc"})
	}
	for name := range m.samlConfigs {
		providers = append(providers, ProviderInfo{Name: name, Type: "saml"})
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})
	return providers
}

// ProviderInfo describes an SSO provider.
type ProviderInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FormatSession renders a session.
func FormatSession(s *Session) string {
	return fmt.Sprintf("Session: %s\n  User:     %s\n  Provider: %s\n  Expires:  %s\n  Created:  %s\n",
		s.ID, s.UserID, s.Provider, s.ExpiresAt.Format(time.RFC3339), s.CreatedAt.Format(time.RFC3339))
}

// FormatAPIKey renders an API key (safe display).
func FormatAPIKey(k *APIKey) string {
	status := "active"
	if !k.Active {
		status = "revoked"
	}
	expiry := "never"
	if k.ExpiresAt != nil {
		expiry = k.ExpiresAt.Format(time.RFC3339)
	}
	return fmt.Sprintf("API Key: %s (%s)\n  Name:   %s\n  User:   %s\n  Prefix: %s...\n  Scopes: %s\n  Status: %s\n  Expires: %s\n  Requests: %d\n",
		k.ID, k.Prefix, k.Name, k.UserID, k.Prefix, strings.Join(k.Scopes, ","), status, expiry, k.RequestCount)
}

// FormatStats renders SSO stats.
func FormatStats(stats SSOStats) string {
	return fmt.Sprintf("SSO Stats:\n  OIDC Providers:  %d\n  SAML Providers:  %d\n  Active Sessions: %d\n  Total API Keys:  %d\n  Active API Keys: %d\n",
		stats.OIDCProviders, stats.SAMLProviders, stats.ActiveSessions, stats.TotalAPIKeys, stats.ActiveAPIKeys)
}

func (m *SSOManager) generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (m *SSOManager) hashKey(key string) string {
	mac := hmac.New(sha256.New, m.secretKey)
	mac.Write([]byte(key))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *SSOManager) save() {
	sessionsData, _ := json.MarshalIndent(m.sessions, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "sessions.json"), sessionsData, 0o644)

	keysData, _ := json.MarshalIndent(m.apiKeys, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "api_keys.json"), keysData, 0o644)

	oidcData, _ := json.MarshalIndent(m.oidcConfigs, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "oidc_configs.json"), oidcData, 0o644)

	samlData, _ := json.MarshalIndent(m.samlConfigs, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "saml_configs.json"), samlData, 0o644)
}

func (m *SSOManager) load() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "sessions.json"))
	if err == nil {
		json.Unmarshal(data, &m.sessions)
	}

	data, err = os.ReadFile(filepath.Join(m.storeDir, "api_keys.json"))
	if err == nil {
		json.Unmarshal(data, &m.apiKeys)
	}

	data, err = os.ReadFile(filepath.Join(m.storeDir, "oidc_configs.json"))
	if err == nil {
		json.Unmarshal(data, &m.oidcConfigs)
	}

	data, err = os.ReadFile(filepath.Join(m.storeDir, "saml_configs.json"))
	if err == nil {
		json.Unmarshal(data, &m.samlConfigs)
	}
}
