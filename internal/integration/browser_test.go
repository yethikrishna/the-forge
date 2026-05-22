package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBrowserOpenSession(t *testing.T) {
	bm := NewBrowserManager()

	sess, err := bm.OpenSession("https://example.com", "testing")
	if err != nil {
		t.Fatal(err)
	}
	if sess.URL != "https://example.com" {
		t.Errorf("expected example.com URL, got %s", sess.URL)
	}
	if sess.Status != "active" {
		t.Errorf("expected active, got %s", sess.Status)
	}
	if sess.ID == "" {
		t.Error("expected session ID")
	}
}

func TestBrowserOpenSessionNoURL(t *testing.T) {
	bm := NewBrowserManager()
	_, err := bm.OpenSession("", "test")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestBrowserCloseSession(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")

	if err := bm.CloseSession(sess.ID); err != nil {
		t.Fatal(err)
	}

	// Session should be gone
	if _, err := bm.GetSession(sess.ID); err == nil {
		t.Error("expected error for closed session")
	}
}

func TestBrowserCloseNonexistent(t *testing.T) {
	bm := NewBrowserManager()
	err := bm.CloseSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestBrowserExecuteNavigate(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")

	result, err := bm.Execute(context.Background(), sess.ID, BrowserAction{
		Type: "navigate",
		URL:  "https://google.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.URL != "https://google.com" {
		t.Errorf("expected google.com, got %s", result.URL)
	}

	// Session should be updated
	updated, _ := bm.GetSession(sess.ID)
	if updated.URL != "https://google.com" {
		t.Errorf("session URL not updated: %s", updated.URL)
	}
}

func TestBrowserExecuteExtract(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")

	result, err := bm.Execute(context.Background(), sess.ID, BrowserAction{
		Type: "extract",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Text == "" {
		t.Error("expected extracted text")
	}
}

func TestBrowserExecuteSnapshot(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")

	result, err := bm.Execute(context.Background(), sess.ID, BrowserAction{
		Type: "snapshot",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestBrowserExecuteUnknownAction(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")

	result, err := bm.Execute(context.Background(), sess.ID, BrowserAction{
		Type: "unknown-action",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Error("expected failure for unknown action")
	}
}

func TestBrowserExecuteInvalidSession(t *testing.T) {
	bm := NewBrowserManager()
	_, err := bm.Execute(context.Background(), "nonexistent", BrowserAction{Type: "navigate"})
	if err == nil {
		t.Error("expected error for invalid session")
	}
}

func TestBrowserListSessions(t *testing.T) {
	bm := NewBrowserManager()
	bm.OpenSession("https://a.com", "a")
	bm.OpenSession("https://b.com", "b")

	sessions := bm.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestBrowserShouldUseBrowser(t *testing.T) {
	tests := []struct {
		task     string
		expected bool
	}{
		{"scrape product page", true},
		{"take screenshot of dashboard", true},
		{"visual regression test", true},
		{"fill-form signup", true},
		{"interactive map widget", true},
		{"captcha solving", true},
		{"render chart as image", true},
		{"fetch API data", false},
		{"send email", false},
		{"process payment", false},
	}

	for _, tt := range tests {
		result := ShouldUseBrowser(tt.task)
		if result != tt.expected {
			t.Errorf("ShouldUseBrowser(%q) = %v, want %v", tt.task, result, tt.expected)
		}
	}
}

func TestBrowserWithGateway(t *testing.T) {
	// Mock OpenClaw gateway
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/browser/open":
			var payload map[string]string
			json.NewDecoder(r.Body).Decode(&payload)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"targetId": "tab-abc123",
				"url":      payload["url"],
				"title":    "Example Page",
			})

		case "/api/browser/act":
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			w.Header().Set("Content-Type", "application/json")

			action, _ := payload["action"].(string)
			switch action {
			case "snapshot":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"snapshot": "button 'Click Me' [ref=e1]\ntextbox 'Search' [ref=e2]",
				})
			case "navigate":
				url, _ := payload["url"].(string)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"url":   url,
					"title": "Navigated Page",
				})
			case "screenshot":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"imageData": []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header bytes
				})
			default:
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
				})
			}

		case "/api/browser/close":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	bm := NewBrowserManagerWithConfig(BrowserConfig{
		GatewayURL: srv.URL,
		Token:      "test-token",
		Timeout:    5 * time.Second,
	})

	// Open session via gateway
	sess, err := bm.OpenSession("https://example.com", "gateway-test")
	if err != nil {
		t.Fatal(err)
	}
	if sess.TargetID != "tab-abc123" {
		t.Errorf("expected tab-abc123, got %s", sess.TargetID)
	}
	if sess.Title != "Example Page" {
		t.Errorf("expected 'Example Page', got %s", sess.Title)
	}

	// Snapshot via gateway
	snapshot, err := bm.Snapshot(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot == "" {
		t.Error("expected non-empty snapshot")
	}

	// Navigate via gateway
	err = bm.Navigate(context.Background(), sess.ID, "https://google.com")
	if err != nil {
		t.Fatal(err)
	}

	// Screenshot via gateway
	img, err := bm.Screenshot(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(img) == 0 {
		t.Error("expected screenshot data")
	}

	// Close session via gateway
	if err := bm.CloseSession(sess.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserGatewayAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer correct-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"targetId": "tab-1",
		})
	}))
	defer srv.Close()

	bm := NewBrowserManagerWithConfig(BrowserConfig{
		GatewayURL: srv.URL,
		Token:      "wrong-token",
	})

	sess, _ := bm.OpenSession("https://example.com", "auth-test")
	// OpenSession should handle the auth error gracefully
	// It stores the session even if the gateway returns an error
	if sess == nil {
		t.Error("session should still be created even with auth failure")
	}
}

func TestBrowserHelperMethods(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")

	// Navigate helper
	if err := bm.Navigate(context.Background(), sess.ID, "https://new.com"); err != nil {
		t.Fatal(err)
	}

	// ExtractText helper
	text, err := bm.ExtractText(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Error("expected text")
	}

	// Snapshot helper
	snap, err := bm.Snapshot(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap == "" {
		t.Error("expected snapshot")
	}

	// Screenshot helper
	img, err := bm.Screenshot(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	// In local mode, no actual image data
	_ = img
}
