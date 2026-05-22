// Package integration provides situational browser use for when APIs don't exist.
// The browser is a tool of last resort — agents prefer APIs first.
// This implementation connects to the OpenClaw browser control API for real
// browser automation: navigation, screenshots, DOM interaction, and extraction.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	TargetID  string            `json:"target_id,omitempty"` // OpenClaw tab target ID
}

// BrowserAction represents a browser action.
type BrowserAction struct {
	Type     string        `json:"type"` // navigate, click, type, screenshot, extract, snapshot, fill, press
	Selector string        `json:"selector,omitempty"`
	Value    string        `json:"value,omitempty"`
	URL      string        `json:"url,omitempty"`
	WaitFor  string        `json:"wait_for,omitempty"`
	Timeout  time.Duration `json:"timeout,omitempty"`
	Ref      string        `json:"ref,omitempty"`     // ARIA/role ref for element targeting
	TargetID string        `json:"target_id,omitempty"` // Tab target ID
}

// BrowserResult is the result of a browser action.
type BrowserResult struct {
	Success    bool              `json:"success"`
	URL        string            `json:"url,omitempty"`
	Title      string            `json:"title,omitempty"`
	Text       string            `json:"text,omitempty"`
	HTML       string            `json:"html,omitempty"`
	Screenshot []byte            `json:"screenshot,omitempty"`
	Links      []string          `json:"links,omitempty"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Snapshot   string            `json:"snapshot,omitempty"` // Accessibility tree snapshot
}

// BrowserConfig configures the browser bridge to OpenClaw.
type BrowserConfig struct {
	// GatewayURL is the OpenClaw gateway base URL (e.g. "http://localhost:4080")
	GatewayURL string `json:"gateway_url"`
	// Token is the authentication token for the gateway API.
	Token string `json:"token"`
	// Timeout is the default request timeout.
	Timeout time.Duration `json:"timeout"`
}

// BrowserManager manages browser sessions for agents via the OpenClaw API.
type BrowserManager struct {
	sessions map[string]*BrowserSession
	config   BrowserConfig
	client   *http.Client
	mu       sync.RWMutex
}

// NewBrowserManager creates a browser manager.
func NewBrowserManager() *BrowserManager {
	return NewBrowserManagerWithConfig(BrowserConfig{})
}

// NewBrowserManagerWithConfig creates a browser manager with OpenClaw gateway config.
func NewBrowserManagerWithConfig(config BrowserConfig) *BrowserManager {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &BrowserManager{
		sessions: make(map[string]*BrowserSession),
		config:   config,
		client:   &http.Client{Timeout: config.Timeout},
	}
}

// gatewayURL returns the full API URL for a browser action.
func (bm *BrowserManager) gatewayURL(action string) string {
	base := strings.TrimRight(bm.config.GatewayURL, "/")
	if base == "" {
		return ""
	}
	return base + "/api/browser/" + action
}

// hasGateway returns true if OpenClaw gateway is configured.
func (bm *BrowserManager) hasGateway() bool {
	return bm.config.GatewayURL != ""
}

// apiRequest makes an authenticated request to the OpenClaw browser API.
func (bm *BrowserManager) apiRequest(ctx context.Context, method, endpoint string, payload interface{}) ([]byte, int, error) {
	url := bm.gatewayURL(endpoint)
	if url == "" {
		return nil, 0, fmt.Errorf("browser: gateway not configured")
	}

	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("browser: marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("browser: create request: %w", err)
	}
	if bm.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+bm.config.Token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := bm.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("browser: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("browser: read response: %w", err)
	}

	return body, resp.StatusCode, nil
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

	// If gateway is configured, open a real browser tab
	if bm.hasGateway() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		respBody, status, err := bm.apiRequest(ctx, "POST", "open", map[string]string{
			"url": url,
		})
		if err != nil {
			session.Status = "error"
			session.URL = url // still store intended URL
		} else if status == http.StatusOK {
			var result struct {
				TargetID string `json:"targetId"`
				URL      string `json:"url"`
				Title    string `json:"title"`
			}
			if json.Unmarshal(respBody, &result) == nil {
				session.TargetID = result.TargetID
				if result.URL != "" {
					session.URL = result.URL
				}
				session.Title = result.Title
			}
		}
	}

	bm.mu.Lock()
	bm.sessions[session.ID] = session
	bm.mu.Unlock()

	return session, nil
}

// CloseSession closes a browser session.
func (bm *BrowserManager) CloseSession(id string) error {
	bm.mu.Lock()
	sess, ok := bm.sessions[id]
	if !ok {
		bm.mu.Unlock()
		return fmt.Errorf("browser: session %s not found", id)
	}
	sess.Status = "closed"
	delete(bm.sessions, id)
	bm.mu.Unlock()

	// Close the real tab if gateway is configured
	if bm.hasGateway() && sess.TargetID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		bm.apiRequest(ctx, "POST", "close", map[string]string{
			"targetId": sess.TargetID,
		})
	}

	return nil
}

// Execute runs a browser action within a session.
func (bm *BrowserManager) Execute(ctx context.Context, sessionID string, action BrowserAction) (*BrowserResult, error) {
	bm.mu.RLock()
	sess, ok := bm.sessions[sessionID]
	bm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("browser: session %s not found", sessionID)
	}

	// If gateway is configured, execute via the real API
	if bm.hasGateway() {
		return bm.executeViaGateway(ctx, sess, action)
	}

	// Fallback: local-only simulation for testing without gateway
	return bm.executeLocal(sess, action)
}

// executeViaGateway sends the action to the OpenClaw browser control API.
func (bm *BrowserManager) executeViaGateway(ctx context.Context, sess *BrowserSession, action BrowserAction) (*BrowserResult, error) {
	payload := map[string]interface{}{
		"action":   action.Type,
		"targetId": action.TargetID,
	}
	if action.TargetID == "" && sess.TargetID != "" {
		payload["targetId"] = sess.TargetID
	}

	switch action.Type {
	case "navigate":
		payload["url"] = action.URL
	case "click":
		payload["ref"] = action.Ref
		payload["selector"] = action.Selector
	case "type":
		payload["ref"] = action.Ref
		payload["selector"] = action.Selector
		payload["text"] = action.Value
	case "fill":
		payload["ref"] = action.Ref
		payload["selector"] = action.Selector
		payload["value"] = action.Value
	case "press":
		payload["key"] = action.Value
	case "screenshot":
		// No extra fields needed
	case "snapshot":
		payload["refs"] = "aria"
		payload["compact"] = true
	case "extract":
		payload["selector"] = action.Selector
	default:
		return nil, fmt.Errorf("browser: unknown action type: %s", action.Type)
	}

	if action.Timeout > 0 {
		payload["timeoutMs"] = action.Timeout.Milliseconds()
	}

	respBody, status, err := bm.apiRequest(ctx, "POST", "act", payload)
	if err != nil {
		return &BrowserResult{Success: false, Error: err.Error()}, err
	}

	result := &BrowserResult{Success: status == http.StatusOK}

	var apiResp struct {
		URL       string `json:"url"`
		Title     string `json:"title"`
		Text      string `json:"text"`
		HTML      string `json:"html"`
		Snapshot  string `json:"snapshot"`
		Error     string `json:"error"`
		ImageData []byte `json:"imageData"`
	}
	if json.Unmarshal(respBody, &apiResp) == nil {
		result.URL = apiResp.URL
		result.Title = apiResp.Title
		result.Text = apiResp.Text
		result.HTML = apiResp.HTML
		result.Snapshot = apiResp.Snapshot
		result.Screenshot = apiResp.ImageData
		if apiResp.Error != "" {
			result.Error = apiResp.Error
			result.Success = false
		}
	}

	// Update session URL on navigate
	if action.Type == "navigate" && result.URL != "" {
		bm.mu.Lock()
		if s, ok := bm.sessions[sess.ID]; ok {
			s.URL = result.URL
			s.Title = result.Title
		}
		bm.mu.Unlock()
	}

	return result, nil
}

// executeLocal handles actions locally when no gateway is configured.
func (bm *BrowserManager) executeLocal(sess *BrowserSession, action BrowserAction) (*BrowserResult, error) {
	result := &BrowserResult{Success: true}

	switch action.Type {
	case "navigate":
		result.URL = action.URL
		result.Title = "Page: " + action.URL
		bm.mu.Lock()
		if s, ok := bm.sessions[sess.ID]; ok {
			s.URL = action.URL
			s.Title = result.Title
		}
		bm.mu.Unlock()
	case "click":
		result.Success = true
	case "type":
		result.Success = true
	case "fill":
		result.Success = true
	case "press":
		result.Success = true
	case "screenshot":
		result.Success = true
	case "snapshot":
		result.Snapshot = "[local mode - no snapshot available]"
		result.Success = true
	case "extract":
		result.Text = "Extracted content from " + sess.URL
		result.Success = true
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unknown action type: %s", action.Type)
	}

	return result, nil
}

// Snapshot takes an accessibility snapshot of the current page.
func (bm *BrowserManager) Snapshot(ctx context.Context, sessionID string) (string, error) {
	result, err := bm.Execute(ctx, sessionID, BrowserAction{Type: "snapshot"})
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("snapshot failed: %s", result.Error)
	}
	return result.Snapshot, nil
}

// Screenshot takes a screenshot of the current page.
func (bm *BrowserManager) Screenshot(ctx context.Context, sessionID string) ([]byte, error) {
	result, err := bm.Execute(ctx, sessionID, BrowserAction{Type: "screenshot"})
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("screenshot failed: %s", result.Error)
	}
	return result.Screenshot, nil
}

// Navigate navigates a session to a new URL.
func (bm *BrowserManager) Navigate(ctx context.Context, sessionID, url string) error {
	_, err := bm.Execute(ctx, sessionID, BrowserAction{Type: "navigate", URL: url})
	return err
}

// ExtractText extracts visible text from the current page.
func (bm *BrowserManager) ExtractText(ctx context.Context, sessionID string) (string, error) {
	result, err := bm.Execute(ctx, sessionID, BrowserAction{Type: "extract"})
	if err != nil {
		return "", err
	}
	return result.Text, nil
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
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Ensure json import is used.
var _ = json.Marshal
