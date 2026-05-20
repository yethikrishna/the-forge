package aibridge_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forge/sword/internal/aibridge"
)

func TestNewRouter(t *testing.T) {
	r := aibridge.NewRouter()
	if r == nil {
		t.Fatal("router should not be nil")
	}
}

func TestAddAndRoute(t *testing.T) {
	r := aibridge.NewRouter()
	r.AddRoute(aibridge.ProviderRoute{
		Provider: "anthropic",
		BaseURL:  "https://api.anthropic.com/v1",
		APIKey:   "test-key",
		Models:   []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514"},
	})

	route, err := r.Route("anthropic/claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("route error: %v", err)
	}
	if route.Provider != "anthropic" {
		t.Errorf("expected anthropic, got %s", route.Provider)
	}
}

func TestRouteByModel(t *testing.T) {
	r := aibridge.NewRouter()
	r.AddRoute(aibridge.ProviderRoute{
		Provider: "openai",
		BaseURL:  "https://api.openai.com/v1",
		APIKey:   "test-key",
		Models:   []string{"gpt-5-mini", "o3"},
	})

	route, err := r.Route("gpt-5-mini")
	if err != nil {
		t.Fatalf("route error: %v", err)
	}
	if route.Provider != "openai" {
		t.Errorf("expected openai, got %s", route.Provider)
	}
}

func TestRouteNotFound(t *testing.T) {
	r := aibridge.NewRouter()
	_, err := r.Route("nonexistent/model")
	if err == nil {
		t.Error("should error for unknown model")
	}
}

func TestRemoveRoute(t *testing.T) {
	r := aibridge.NewRouter()
	r.AddRoute(aibridge.ProviderRoute{Provider: "test"})
	r.RemoveRoute("test")
	_, err := r.Route("test/model")
	if err == nil {
		t.Error("should error after route removal")
	}
}

func TestRouterStats(t *testing.T) {
	r := aibridge.NewRouter()
	stats := r.GetStats()
	if stats.TotalRequests != 0 {
		t.Errorf("expected 0 requests, got %d", stats.TotalRequests)
	}
}

func TestInterceptingRouter(t *testing.T) {
	r := aibridge.NewRouter()
	ir := aibridge.NewInterceptingRouter(r)

	called := false
	ir.AddInterceptor(func(req *http.Request) (*http.Request, error) {
		called = true
		return req, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Model", "test")
	ir.ServeHTTP(w, req)

	if !called {
		t.Error("interceptor should have been called")
	}
}

func TestLoggingInterceptor(t *testing.T) {
	logged := false
	li := aibridge.LoggingInterceptor(func(format string, args ...any) {
		logged = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := li(req)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}
	if !logged {
		t.Error("should have logged")
	}
}

func TestModelAliasInterceptor(t *testing.T) {
	aliases := map[string]string{
		"sonnet": "claude-sonnet-4-20250514",
		"opus":   "claude-opus-4-20250514",
	}

	alias := aibridge.ModelAliasInterceptor(aliases)
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Model", "sonnet")

	modified, err := alias(req)
	if err != nil {
		t.Fatalf("alias error: %v", err)
	}
	if modified.Header.Get("X-Model") != "claude-sonnet-4-20250514" {
		t.Errorf("expected alias resolution, got %s", modified.Header.Get("X-Model"))
	}
}
