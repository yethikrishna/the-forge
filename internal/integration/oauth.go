// Package integration provides OAuth flow handling for autonomous agent access.
// Agents complete OAuth flows without requiring user interaction for each one.
package integration

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OAuthProvider identifies the OAuth provider.
type OAuthProvider string

const (
	OAuthGoogle   OAuthProvider = "google"
	OAuthGitHub   OAuthProvider = "github"
	OAuthSlack    OAuthProvider = "slack"
	OAuthDiscord  OAuthProvider = "discord"
	OAuthMicrosoft OAuthProvider = "microsoft"
)

// OAuthConfig stores OAuth app credentials.
type OAuthConfig struct {
	Provider     OAuthProvider `json:"provider"`
	ClientID     string        `json:"client_id"`
	ClientSecret string        `json:"client_secret"`
	RedirectURL  string        `json:"redirect_url"`
	Scopes       []string      `json:"scopes"`
	AuthURL      string        `json:"auth_url"`
	TokenURL     string        `json:"token_url"`
}

// Token represents an OAuth access token.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Scope        string    `json:"scope,omitempty"`
}

// IsExpired returns whether the token has expired.
func (t *Token) IsExpired() bool {
	return time.Now().After(t.Expiry)
}

// OAuthState tracks an in-progress OAuth flow.
type OAuthState struct {
	ID        string        `json:"id"`
	Provider  OAuthProvider `json:"provider"`
	State     string        `json:"state"`
	Code      string        `json:"code,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	Completed bool          `json:"completed"`
	Token     *Token        `json:"token,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// OAuthManager handles OAuth flows.
type OAuthManager struct {
	storeDir string
	configs  map[OAuthProvider]OAuthConfig
	states   map[string]*OAuthState
	tokens   map[OAuthProvider]*Token
	mu       sync.RWMutex
}

// NewOAuthManager creates an OAuth manager.
func NewOAuthManager(storeDir string) *OAuthManager {
	os.MkdirAll(storeDir, 0o755)
	m := &OAuthManager{
		storeDir: storeDir,
		configs:  make(map[OAuthProvider]OAuthConfig),
		states:   make(map[string]*OAuthState),
		tokens:   make(map[OAuthProvider]*Token),
	}
	m.loadTokens()
	m.registerBuiltinConfigs()
	return m
}

// RegisterConfig registers OAuth credentials for a provider.
func (om *OAuthManager) RegisterConfig(config OAuthConfig) error {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.configs[config.Provider] = config
	return nil
}

// GetConfig retrieves the OAuth config for a provider.
func (om *OAuthManager) GetConfig(provider OAuthProvider) (OAuthConfig, bool) {
	om.mu.RLock()
	defer om.mu.RUnlock()
	cfg, ok := om.configs[provider]
	return cfg, ok
}

// StartFlow initiates an OAuth flow, returning the authorization URL.
func (om *OAuthManager) StartFlow(provider OAuthProvider) (string, *OAuthState, error) {
	cfg, ok := om.configs[provider]
	if !ok {
		return "", nil, fmt.Errorf("oauth: no config for %s", provider)
	}

	state := generateState()
	oauthState := &OAuthState{
		ID:        fmt.Sprintf("oauth-%d", time.Now().UnixNano()),
		Provider:  provider,
		State:     state,
		CreatedAt: time.Now(),
	}

	om.mu.Lock()
	om.states[state] = oauthState
	om.mu.Unlock()

	// Build authorization URL
	authURL := cfg.AuthURL
	if authURL == "" {
		authURL = providerAuthURL(provider)
	}

	params := []string{
		"client_id=" + cfg.ClientID,
		"redirect_uri=" + cfg.RedirectURL,
		"response_type=code",
		"state=" + state,
	}
	if len(cfg.Scopes) > 0 {
		params = append(params, "scope="+strings.Join(cfg.Scopes, "+"))
	}

	url := authURL + "?" + strings.Join(params, "&")
	return url, oauthState, nil
}

// HandleCallback processes an OAuth callback.
func (om *OAuthManager) HandleCallback(state, code string) (*Token, error) {
	om.mu.Lock()
	oauthState, ok := om.states[state]
	if !ok {
		om.mu.Unlock()
		return nil, fmt.Errorf("oauth: unknown state")
	}
	om.mu.Unlock()

	if oauthState.Completed {
		return nil, fmt.Errorf("oauth: flow already completed")
	}

	// Exchange code for token
	cfg, ok := om.configs[oauthState.Provider]
	if !ok {
		return nil, fmt.Errorf("oauth: no config for %s", oauthState.Provider)
	}

	token, err := exchangeCode(cfg, code)
	if err != nil {
		oauthState.Error = err.Error()
		return nil, err
	}

	oauthState.Code = code
	oauthState.Completed = true
	oauthState.Token = token

	om.mu.Lock()
	om.tokens[oauthState.Provider] = token
	om.mu.Unlock()
	om.persistTokens()

	return token, nil
}

// GetToken retrieves a stored token for a provider.
func (om *OAuthManager) GetToken(provider OAuthProvider) (*Token, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	token, ok := om.tokens[provider]
	if !ok {
		return nil, fmt.Errorf("oauth: no token for %s", provider)
	}
	return token, nil
}

// HasToken checks if a token exists for a provider.
func (om *OAuthManager) HasToken(provider OAuthProvider) bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	_, ok := om.tokens[provider]
	return ok
}

// Revoke removes a token.
func (om *OAuthManager) Revoke(provider OAuthProvider) {
	om.mu.Lock()
	defer om.mu.Unlock()
	delete(om.tokens, provider)
	om.persistTokens()
}

// HandleHTTP is an HTTP handler for OAuth callbacks.
func (om *OAuthManager) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}

	token, err := om.HandleCallback(state, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "connected",
		"token_type": token.TokenType,
		"expiry":  token.Expiry.Format(time.RFC3339),
	})
}

func (om *OAuthManager) registerBuiltinConfigs() {
	om.configs[OAuthGoogle] = OAuthConfig{
		Provider:    OAuthGoogle,
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		Scopes:      []string{"openid", "email", "profile"},
	}
	om.configs[OAuthGitHub] = OAuthConfig{
		Provider:    OAuthGitHub,
		AuthURL:     "https://github.com/login/oauth/authorize",
		TokenURL:    "https://github.com/login/oauth/access_token",
		Scopes:      []string{"repo", "user:email"},
	}
	om.configs[OAuthSlack] = OAuthConfig{
		Provider:    OAuthSlack,
		AuthURL:     "https://slack.com/oauth/v2/authorize",
		TokenURL:    "https://slack.com/api/oauth.v2.access",
		Scopes:      []string{"chat:write", "channels:read"},
	}
}

func (om *OAuthManager) persistTokens() {
	data, _ := json.MarshalIndent(om.tokens, "", "  ")
	os.WriteFile(filepath.Join(om.storeDir, "tokens.json"), data, 0o600)
}

func (om *OAuthManager) loadTokens() {
	data, err := os.ReadFile(filepath.Join(om.storeDir, "tokens.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &om.tokens)
}

func providerAuthURL(p OAuthProvider) string {
	switch p {
	case OAuthGoogle:
		return "https://accounts.google.com/o/oauth2/v2/auth"
	case OAuthGitHub:
		return "https://github.com/login/oauth/authorize"
	case OAuthSlack:
		return "https://slack.com/oauth/v2/authorize"
	case OAuthDiscord:
		return "https://discord.com/api/oauth2/authorize"
	case OAuthMicrosoft:
		return "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	default:
		return ""
	}
}

func exchangeCode(cfg OAuthConfig, code string) (*Token, error) {
	// In production, this makes an HTTP POST to the token endpoint
	// For the runtime layer, we return a simulated token
	return &Token{
		AccessToken:  "simulated-access-token",
		RefreshToken: "simulated-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
		Scope:        strings.Join(cfg.Scopes, " "),
	}, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
