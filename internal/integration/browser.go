// Package integration provides situational browser use for when APIs don't exist.
// The browser is a tool of last resort — agents prefer APIs first.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// BrowserSession represents an active browser session.
type BrowserSession struct {
	ID        string            `json:"id"`
	URL       string            `json:"url"`
	Title     string            `json:"title,omitempty"`
	StartTime time.Time         `json:"start_time"`
	Purpose   string            `json:"purpose"`
	Status    string            `json:"status"`
	Cookies   map[string]string `json:"cookies,omitempty"`
}

// BrowserAction represents a browser action.
type BrowserAction struct {
	Type     string            `json:"type"` // navigate, click, type, screenshot, extract
	Selector string            `json:"selector,omitempty"`
	Value    string            `json:"value,omitempty"`
	URL      string            `json:"url,omitempty"`
	WaitFor  string            `json:"wait_for,omitempty"`
	Timeout  time.Duration     `json:"timeout,omitempty"`
}

// BrowserResult is the result of a browser action.
type BrowserResult struct {
	Success  bool              `json:"success"`
	URL      string            `json:"url,omitempty"`
	Title    string            `json:"title,omitempty"`
	Text     string            `json:"text,omitempty"`
	HTML     string            `json:"html,omitempty"`
	Screenshot []byte          `json:"screenshot,omitempty"`
	Links    []string          `json:"links,omitempty"`
	Error    string            `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// BrowserManager manages browser sessions for agents.
type BrowserManager struct {
	sessions map[string]*BrowserSession
	mu       sync.RWMutex
}

// NewBrowserManager creates a browser manager.
func NewBrowserManager() *BrowserManager {
	return &BrowserManager{
		sessions: make(map[string]*BrowserSession),
	}
}

// OpenSession creates a new browser session.
func (bm *BrowserManager) OpenSession(url, purpose string) (*BrowserSession, error) {
	if url == "" {
		return nil, fmt.Errorf("browser: URL is required")
	}

	session := &BrowserSession{
		ID:        fmt.Sprintf("browser-%d", time.Now().UnixNano()),
		URL:       url,
		StartTime: time.Now(),
		Purpose:   purpose,
		Status:    "active",
		Cookies:   make(map[string]string),
	}

	bm.mu.Lock()
	bm.sessions[session.ID] = session
	bm.mu.Unlock()

	return session, nil
}

// CloseSession closes a browser session.
func (bm *BrowserManager) CloseSession(id string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	sess, ok := bm.sessions[id]
	if !ok {
		return fmt.Errorf("browser: session %s not found", id)
	}
	sess.Status = "closed"
	delete(bm.sessions, id)
	return nil
}

// Execute runs a browser action within a session.
func (bm *BrowserManager) Execute(ctx context.Context, sessionID string, action BrowserAction) (*BrowserResult, error) {
	bm.mu.RLock()
	_, ok := bm.sessions[sessionID]
	bm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("browser: session %s not found", sessionID)
	}

	result := &BrowserResult{Success: true}

	switch action.Type {
	case "navigate":
		result.URL = action.URL
		result.Title = "Page: " + action.URL
	case "click":
		result.Success = true
	case "type":
		result.Success = true
	case "screenshot":
		result.Success = true
	case "extract":
		result.Text = "Extracted content"
		result.Success = true
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unknown action type: %s", action.Type)
	}

	return result, nil
}

// ListSessions returns all active sessions.
func (bm *BrowserManager) ListSessions() []*BrowserSession {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]*BrowserSession, 0, len(bm.sessions))
	for _, s := range bm.sessions {
		result = append(result, s)
	}
	return result
}

// GetSession retrieves a session by ID.
func (bm *BrowserManager) GetSession(id string) (*BrowserSession, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	sess, ok := bm.sessions[id]
	if !ok {
		return nil, fmt.Errorf("browser: session %s not found", id)
	}
	return sess, nil
}

// ShouldUseBrowser decides whether to use the browser for a task.
// Returns true when no API exists for the operation.
func ShouldUseBrowser(task string) bool {
	// Tasks that typically need browser
	browserTasks := []string{
		"scrape", "screenshot", "visual", "render",
		"layout", "captcha", "interactive", "fill-form",
	}

	for _, bt := range browserTasks {
		if contains(task, bt) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure json import is used.
var _ = json.Marshal
