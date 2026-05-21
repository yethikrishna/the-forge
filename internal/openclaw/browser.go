// Package openclaw provides browser control via the OpenClaw browser server.
// Forge agents use the browser situationally — when APIs don't exist,
// when real-time data is needed, or when manual intervention is required.
//
// The browser is a tool of last resort. APIs first, browser only when necessary.
package openclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// BrowserAction represents the type of browser action to perform.
type BrowserAction string

const (
	BrowserNavigate BrowserAction = "navigate"
	BrowserClick    BrowserAction = "click"
	BrowserType     BrowserAction = "type"
	BrowserSnap    BrowserAction = "snapshot"
	BrowserScreen  BrowserAction = "screenshot"
	BrowserScroll   BrowserAction = "scroll"
	BrowserFill     BrowserAction = "fill"
	BrowserWait     BrowserAction = "wait"
	BrowserEvaluate BrowserAction = "evaluate"
	BrowserOpen     BrowserAction = "open"
	BrowserClose    BrowserAction = "close"
	BrowserTabs     BrowserAction = "tabs"
)

// BrowserTarget specifies where the browser runs.
type BrowserTarget string

const (
	BrowserTargetHost   BrowserTarget = "host"
	BrowserTargetSandbox BrowserTarget = "sandbox"
	BrowserTargetNode   BrowserTarget = "node"
)

// BrowserRequest is a request to control the browser.
type BrowserRequest struct {
	Action    BrowserAction `json:"action"`
	Target    BrowserTarget `json:"target,omitempty"`
	Node      string        `json:"node,omitempty"`      // node ID for target=node
	Profile   string        `json:"profile,omitempty"`   // browser profile
	TargetID  string        `json:"targetId,omitempty"`  // tab ID
	URL       string        `json:"url,omitempty"`
	Ref       string        `json:"ref,omitempty"`       // element reference from snapshot
	Text      string        `json:"text,omitempty"`
	Selector  string        `json:"selector,omitempty"`
	Key       string        `json:"key,omitempty"`
	TimeoutMs int           `json:"timeoutMs,omitempty"`
}

// BrowserSnapshot represents a DOM snapshot returned by the browser.
type BrowserSnapshot struct {
	TargetID string `json:"targetId"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Content  string `json:"content"` // accessibility tree or markdown
}

// BrowserScreenshot represents a screenshot result.
type BrowserScreenshot struct {
	TargetID string `json:"targetId"`
	Data     string `json:"data"` // base64-encoded PNG
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// BrowserTab represents a browser tab.
type BrowserTab struct {
	TargetID string `json:"targetId"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Active   bool   `json:"active"`
}

// BrowserController provides browser automation via the OpenClaw runtime.
type BrowserController struct {
	bridge *Bridge
}

// NewBrowserController creates a new browser controller.
func NewBrowserController(bridge *Bridge) *BrowserController {
	return &BrowserController{bridge: bridge}
}

// Navigate opens a URL in the browser.
func (bc *BrowserController) Navigate(ctx context.Context, req BrowserRequest) error {
	req.Action = BrowserNavigate
	if req.URL == "" {
		return fmt.Errorf("URL is required for navigate")
	}
	return bc.bridge.PostJSON(ctx, "/api/browser", req, nil)
}

// Snapshot takes an accessibility snapshot of the current page.
func (bc *BrowserController) Snapshot(ctx context.Context, req BrowserRequest) (*BrowserSnapshot, error) {
	req.Action = BrowserSnap
	var snap BrowserSnapshot
	if err := bc.bridge.PostJSON(ctx, "/api/browser", req, &snap); err != nil {
		return nil, fmt.Errorf("browser snapshot: %w", err)
	}
	return &snap, nil
}

// Screenshot captures a screenshot of the current page.
func (bc *BrowserController) Screenshot(ctx context.Context, req BrowserRequest) (*BrowserScreenshot, error) {
	req.Action = BrowserScreen
	var shot BrowserScreenshot
	if err := bc.bridge.PostJSON(ctx, "/api/browser", req, &shot); err != nil {
		return nil, fmt.Errorf("browser screenshot: %w", err)
	}
	return &shot, nil
}

// Click clicks on an element identified by ref or selector.
func (bc *BrowserController) Click(ctx context.Context, req BrowserRequest) error {
	req.Action = BrowserClick
	if req.Ref == "" && req.Selector == "" {
		return fmt.Errorf("ref or selector is required for click")
	}
	return bc.bridge.PostJSON(ctx, "/api/browser", req, nil)
}

// Type types text into an element identified by ref or selector.
func (bc *BrowserController) Type(ctx context.Context, req BrowserRequest) error {
	req.Action = BrowserType
	if req.Text == "" {
		return fmt.Errorf("text is required for type")
	}
	if req.Ref == "" && req.Selector == "" {
		return fmt.Errorf("ref or selector is required for type")
	}
	return bc.bridge.PostJSON(ctx, "/api/browser", req, nil)
}

// Fill fills a form field with text and optionally submits.
func (bc *BrowserController) Fill(ctx context.Context, req BrowserRequest) error {
	req.Action = BrowserFill
	if req.Text == "" {
		return fmt.Errorf("text is required for fill")
	}
	return bc.bridge.PostJSON(ctx, "/api/browser", req, nil)
}

// Evaluate runs JavaScript in the browser context.
func (bc *BrowserController) Evaluate(ctx context.Context, fn string, targetID string) (string, error) {
	payload := map[string]interface{}{
		"action":   BrowserEvaluate,
		"targetId": targetID,
		"fn":       fn,
	}
	var result struct {
		Result string `json:"result"`
	}
	if err := bc.bridge.PostJSON(ctx, "/api/browser", payload, &result); err != nil {
		return "", fmt.Errorf("browser evaluate: %w", err)
	}
	return result.Result, nil
}

// ListTabs returns all open browser tabs.
func (bc *BrowserController) ListTabs(ctx context.Context) ([]BrowserTab, error) {
	var tabs []BrowserTab
	if err := bc.bridge.PostJSON(ctx, "/api/browser", map[string]interface{}{
		"action": BrowserTabs,
	}, &tabs); err != nil {
		return nil, fmt.Errorf("list tabs: %w", err)
	}
	return tabs, nil
}

// OpenTab opens a new browser tab with the given URL.
func (bc *BrowserController) OpenTab(ctx context.Context, url string) (string, error) {
	var result struct {
		TargetID string `json:"targetId"`
	}
	if err := bc.bridge.PostJSON(ctx, "/api/browser", map[string]interface{}{
		"action": BrowserOpen,
		"url":    url,
	}, &result); err != nil {
		return "", fmt.Errorf("open tab: %w", err)
	}
	return result.TargetID, nil
}

// CloseTab closes a browser tab.
func (bc *BrowserController) CloseTab(ctx context.Context, targetID string) error {
	return bc.bridge.PostJSON(ctx, "/api/browser", map[string]interface{}{
		"action":   BrowserClose,
		"targetId": targetID,
	}, nil)
}

// WaitForElement waits for an element to appear on the page.
func (bc *BrowserController) WaitForElement(ctx context.Context, selector string, timeout time.Duration) error {
	payload := map[string]interface{}{
		"action":    BrowserWait,
		"selector":  selector,
		"timeoutMs": int(timeout.Milliseconds()),
	}
	return bc.bridge.PostJSON(ctx, "/api/browser", payload, nil)
}

// IsLoggedIn checks if a service is already logged in by navigating to its URL
// and checking for login indicators.
func (bc *BrowserController) IsLoggedIn(ctx context.Context, url, loginIndicator string) (bool, error) {
	snap, err := bc.Snapshot(ctx, BrowserRequest{URL: url})
	if err != nil {
		return false, err
	}
	// If login indicator text is NOT found, user is logged in
	return !contains(snap.Content, loginIndicator), nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BrowserDownload represents a downloaded file from the browser.
type BrowserDownload struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`     // workspace-relative path
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// ListDownloads lists files in the browser downloads directory.
func (bc *BrowserController) ListDownloads(ctx context.Context) ([]BrowserDownload, error) {
	var downloads []BrowserDownload
	if err := bc.bridge.GetJSON(ctx, "/api/browser/downloads", &downloads); err != nil {
		return nil, fmt.Errorf("list downloads: %w", err)
	}
	return downloads, nil
}

// ClickAndDownload clicks a download button and waits for the file.
func (bc *BrowserController) ClickAndDownload(ctx context.Context, req BrowserRequest, timeout time.Duration) (*BrowserDownload, error) {
	// Click the element
	if err := bc.Click(ctx, req); err != nil {
		return nil, fmt.Errorf("click for download: %w", err)
	}

	// Poll downloads until a new file appears
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		downloads, err := bc.ListDownloads(ctx)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if len(downloads) > 0 {
			// Return the most recent download
			return &downloads[0], nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("download timed out after %v", timeout)
}

// Ensure the interface satisfaction check
var _ json.Marshaler = (*BrowserRequest)(nil)

// MarshalJSON implements json.Marshaler for BrowserRequest.
func (r BrowserRequest) MarshalJSON() ([]byte, error) {
	type Alias BrowserRequest
	return json.Marshal(&struct{ *Alias }{Alias: (*Alias)(&r)})
}
