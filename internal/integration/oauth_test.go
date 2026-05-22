package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOAuthManagerStartFlow(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{
		Provider:    OAuthGoogle,
		ClientID:    "test-id",
		RedirectURL: "http://localhost:8080/callback",
		Scopes:      []string{"email", "profile"},
	})

	url, state, err := mgr.StartFlow(OAuthGoogle)
	if err != nil {
		t.Fatalf("StartFlow failed: %v", err)
	}
	if !strings.Contains(url, "accounts.google.com") {
		t.Errorf("expected google auth URL, got %s", url)
	}
	if state == nil {
		t.Error("expected non-nil state")
	}
	if state.Completed {
		t.Error("expected incomplete state")
	}
}

func TestOAuthManagerNoConfig(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	_, _, err := mgr.StartFlow(OAuthProvider("nonexistent"))
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

// setupOAuthWithMock creates an OAuthManager with a mock token server.
func setupOAuthWithMock(t *testing.T, provider OAuthProvider) (*OAuthManager, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("expected form content-type, got %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Errorf("expected grant_type=authorization_code, got %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("code") == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing code"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "mock-access-token-" + string(provider),
			"refresh_token": "mock-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"scope":         "email profile",
		})
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{
		Provider:      provider,
		ClientID:      "test-id",
		ClientSecret:  "test-secret",
		RedirectURL:   "http://localhost:8080/callback",
		TokenURL:      srv.URL, // point to mock server
		Scopes:        []string{"email"},
	})

	return mgr, srv
}

func TestOAuthManagerHandleCallback(t *testing.T) {
	mgr, _ := setupOAuthWithMock(t, OAuthGitHub)

	_, state, err := mgr.StartFlow(OAuthGitHub)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}

	token, err := mgr.HandleCallback(state.State, "auth-code-123")
	if err != nil {
		t.Fatalf("HandleCallback failed: %v", err)
	}
	if token.AccessToken != "mock-access-token-github" {
		t.Errorf("expected mock token, got %s", token.AccessToken)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", token.TokenType)
	}
	if token.Expiry.IsZero() {
		t.Error("expected non-zero expiry")
	}
	if token.Expiry.Before(time.Now()) {
		t.Error("token should not be expired immediately after issuance")
	}
}

func TestOAuthManagerTokenPersistence(t *testing.T) {
	mgr, _ := setupOAuthWithMock(t, OAuthGoogle)

	_, state, _ := mgr.StartFlow(OAuthGoogle)
	mgr.HandleCallback(state.State, "auth-code")

	dir := t.TempDir()
	// Read the persisted tokens file from the original dir
	mgr2 := NewOAuthManager(mgr.storeDir)
	if !mgr2.HasToken(OAuthGoogle) {
		t.Error("expected persisted token to be loadable")
	}
	_ = dir
}

func TestOAuthManagerGetToken(t *testing.T) {
	mgr, _ := setupOAuthWithMock(t, OAuthSlack)

	_, state, _ := mgr.StartFlow(OAuthSlack)
	mgr.HandleCallback(state.State, "code")

	token, err := mgr.GetToken(OAuthSlack)
	if err != nil {
		t.Fatal(err)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", token.TokenType)
	}
	if token.AccessToken != "mock-access-token-slack" {
		t.Errorf("expected mock token for slack, got %s", token.AccessToken)
	}
}

func TestOAuthManagerGetTokenNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	_, err := mgr.GetToken(OAuthDiscord)
	if err == nil {
		t.Error("expected error for missing token")
	}
}

func TestOAuthManagerRevoke(t *testing.T) {
	mgr, _ := setupOAuthWithMock(t, OAuthGoogle)

	_, state, _ := mgr.StartFlow(OAuthGoogle)
	mgr.HandleCallback(state.State, "code")

	mgr.Revoke(OAuthGoogle)
	if mgr.HasToken(OAuthGoogle) {
		t.Error("expected token revoked")
	}
}

func TestOAuthManagerHandleHTTP(t *testing.T) {
	mgr, _ := setupOAuthWithMock(t, OAuthGitHub)

	_, state, _ := mgr.StartFlow(OAuthGitHub)

	req := httptest.NewRequest("GET", "/callback?state="+state.State+"&code=abc123", nil)
	w := httptest.NewRecorder()
	mgr.HandleHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOAuthManagerHandleHTTPMissing(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)

	req := httptest.NewRequest("GET", "/callback", nil)
	w := httptest.NewRecorder()
	mgr.HandleHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTokenIsExpired(t *testing.T) {
	expired := &Token{Expiry: time.Now().Add(-time.Hour)}
	if !expired.IsExpired() {
		t.Error("expected expired")
	}

	valid := &Token{Expiry: time.Now().Add(time.Hour)}
	if valid.IsExpired() {
		t.Error("expected not expired")
	}
}

func TestTokenSerialization(t *testing.T) {
	token := &Token{
		AccessToken:  "abc123",
		RefreshToken: "refresh-xyz",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
		Scope:        "email profile",
	}

	data, _ := json.Marshal(token)
	var t2 Token
	json.Unmarshal(data, &t2)
	if t2.AccessToken != "abc123" {
		t.Errorf("expected abc123, got %s", t2.AccessToken)
	}
}

func TestOAuthStateSerialization(t *testing.T) {
	s := &OAuthState{
		ID:        "oauth-1",
		Provider:  OAuthGoogle,
		State:     "random-state",
		CreatedAt: time.Now(),
		Completed: true,
	}

	data, _ := json.Marshal(s)
	var s2 OAuthState
	json.Unmarshal(data, &s2)
	if s2.Provider != OAuthGoogle {
		t.Errorf("expected google, got %s", s2.Provider)
	}
}

func TestProviderAuthURL(t *testing.T) {
	tests := []struct {
		provider OAuthProvider
		contains string
	}{
		{OAuthGoogle, "accounts.google.com"},
		{OAuthGitHub, "github.com"},
		{OAuthSlack, "slack.com"},
		{OAuthDiscord, "discord.com"},
	}

	for _, tt := range tests {
		url := providerAuthURL(tt.provider)
		if !strings.Contains(url, tt.contains) {
			t.Errorf("expected %s in URL for %s, got %s", tt.contains, tt.provider, url)
		}
	}
}

func TestOAuthManagerDoubleCallback(t *testing.T) {
	mgr, _ := setupOAuthWithMock(t, OAuthGitHub)

	_, state, _ := mgr.StartFlow(OAuthGitHub)
	mgr.HandleCallback(state.State, "code1")

	_, err := mgr.HandleCallback(state.State, "code2")
	if err == nil {
		t.Error("expected error for double callback")
	}
}

func TestOAuthExchangeError(t *testing.T) {
	// Server that returns an error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "The authorization code is invalid",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{
		Provider:     OAuthGitHub,
		ClientID:     "test",
		ClientSecret: "test",
		RedirectURL:  "http://localhost:8080/callback",
		TokenURL:     srv.URL,
		Scopes:       []string{"repo"},
	})

	_, state, _ := mgr.StartFlow(OAuthGitHub)
	_, err := mgr.HandleCallback(state.State, "bad-code")
	if err == nil {
		t.Error("expected error for invalid code")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got %v", err)
	}
}

func TestProviderTokenURL(t *testing.T) {
	tests := []struct {
		provider OAuthProvider
		contains string
	}{
		{OAuthGoogle, "oauth2.googleapis.com"},
		{OAuthGitHub, "github.com"},
		{OAuthSlack, "slack.com"},
		{OAuthDiscord, "discord.com"},
		{OAuthMicrosoft, "microsoftonline.com"},
	}

	for _, tt := range tests {
		url := providerTokenURL(tt.provider)
		if url == "" {
			t.Errorf("expected non-empty token URL for %s", tt.provider)
			continue
		}
		if !strings.Contains(url, tt.contains) {
			t.Errorf("expected %s in token URL for %s, got %s", tt.contains, tt.provider, url)
		}
	}
}
