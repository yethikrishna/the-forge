// Package codersdk provides a Go SDK for programmatic workspace and agent management.
// Client for the Forge agent API — mirrors coder/codersdk patterns for
// workspace lifecycle, file ops, and command execution.
package codersdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the Forge API client.
type Client struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

// NewClient creates an API client.
func NewClient(baseURL, authToken string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		AuthToken:  authToken,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Status ---

// StatusResponse is the server status.
type StatusResponse struct {
	AgentID   string `json:"agent_id"`
	Workspace string `json:"workspace"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	Version   string `json:"version"`
	Hostname  string `json:"hostname"`
}

// Status fetches the agent status.
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	var resp StatusResponse
	if err := c.get(ctx, "/api/status", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Health checks if the agent is reachable.
func (c *Client) Health(ctx context.Context) error {
	var result map[string]string
	return c.get(ctx, "/api/health", &result)
}

// --- Exec ---

// ExecRequest is a command execution request.
type ExecRequest struct {
	Command string            `json:"command"`
	Dir     string            `json:"dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
}

// ExecResponse is the result of command execution.
type ExecResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error,omitempty"`
}

// Exec runs a command on the remote agent.
func (c *Client) Exec(ctx context.Context, req ExecRequest) (*ExecResponse, error) {
	var resp ExecResponse
	if err := c.post(ctx, "/api/exec", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Files ---

// FileReadRequest is a file read request.
type FileReadRequest struct {
	Path string `json:"path"`
}

// FileWriteRequest is a file write request.
type FileWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FileResponse is a file operation response.
type FileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	Error   string `json:"error,omitempty"`
}

// ReadFile reads a file from the remote agent.
func (c *Client) ReadFile(ctx context.Context, path string) (*FileResponse, error) {
	var resp FileResponse
	if err := c.post(ctx, "/api/file/read", FileReadRequest{Path: path}, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("read file: %s", resp.Error)
	}
	return &resp, nil
}

// WriteFile writes a file on the remote agent.
func (c *Client) WriteFile(ctx context.Context, path, content string) error {
	var resp FileResponse
	if err := c.post(ctx, "/api/file/write", FileWriteRequest{Path: path, Content: content}, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("write file: %s", resp.Error)
	}
	return nil
}

// --- Workspace ---

// WorkspaceInfo is workspace metadata.
type WorkspaceInfo struct {
	Workspace string `json:"workspace"`
	CWD       string `json:"cwd"`
	AgentID   string `json:"agent_id"`
}

// Workspace returns workspace info from the remote agent.
func (c *Client) Workspace(ctx context.Context) (*WorkspaceInfo, error) {
	var resp WorkspaceInfo
	if err := c.get(ctx, "/api/workspace", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- HTTP helpers ---

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	c.setAuth(req)
	return c.do(req, result)
}

func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	return c.do(req, result)
}

func (c *Client) setAuth(req *http.Request) {
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}
}

func (c *Client) do(req *http.Request, result interface{}) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", req.URL.Path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
