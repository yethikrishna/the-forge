package integration

import (
	"encoding/json"
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

func TestOAuthManagerHandleCallback(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{
		Provider:    OAuthGitHub,
		ClientID:    "test-id",
		ClientSecret: "test-secret",
		RedirectURL: "http://localhost:8080/callback",
		Scopes:      []string{"repo"},
	})

	_, state, _ := mgr.StartFlow(OAuthGitHub)

	token, err := mgr.HandleCallback(state.State, "auth-code-123")
	if err != nil {
		t.Fatalf("HandleCallback failed: %v", err)
	}
	if token.AccessToken == "" {
		t.Error("expected access token")
	}
	if token.Expiry.IsZero() {
		t.Error("expected expiry")
	}
}

func TestOAuthManagerTokenPersistence(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{
		Provider: OAuthGoogle,
		ClientID: "test",
		Scopes:   []string{"email"},
	})

	_, state, _ := mgr.StartFlow(OAuthGoogle)
	mgr.HandleCallback(state.State, "code")

	// Reload
	mgr2 := NewOAuthManager(dir)
	if !mgr2.HasToken(OAuthGoogle) {
		t.Error("expected persisted token")
	}
}

func TestOAuthManagerGetToken(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{
		Provider: OAuthSlack,
		ClientID: "test",
		Scopes:   []string{"chat:write"},
	})

	_, state, _ := mgr.StartFlow(OAuthSlack)
	mgr.HandleCallback(state.State, "code")

	token, err := mgr.GetToken(OAuthSlack)
	if err != nil {
		t.Fatal(err)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", token.TokenType)
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
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{Provider: OAuthGoogle, ClientID: "test", Scopes: []string{"email"}})
	_, state, _ := mgr.StartFlow(OAuthGoogle)
	mgr.HandleCallback(state.State, "code")

	mgr.Revoke(OAuthGoogle)
	if mgr.HasToken(OAuthGoogle) {
		t.Error("expected token revoked")
	}
}

func TestOAuthManagerHandleHTTP(t *testing.T) {
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{Provider: OAuthGitHub, ClientID: "test", Scopes: []string{"repo"}})

	_, state, _ := mgr.StartFlow(OAuthGitHub)

	req := httptest.NewRequest("GET", "/callback?state="+state.State+"&code=abc123", nil)
	w := httptest.NewRecorder()
	mgr.HandleHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
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
	dir := t.TempDir()
	mgr := NewOAuthManager(dir)
	mgr.RegisterConfig(OAuthConfig{Provider: OAuthGitHub, ClientID: "test", Scopes: []string{"repo"}})

	_, state, _ := mgr.StartFlow(OAuthGitHub)
	mgr.HandleCallback(state.State, "code1")

	_, err := mgr.HandleCallback(state.State, "code2")
	if err == nil {
		t.Error("expected error for double callback")
	}
}
