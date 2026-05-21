// Package suna provides the bridge between Forge and the Suna frontend/sandbox.
// Suna is the experience layer for Forge — providing web UI, mobile app,
// sandbox runtime, skills marketplace, and 3000+ integrations.
//
// This bridge connects the Forge org layer to Suna's infrastructure so agents
// get the best UX without the Forge team reimplementing frontend and sandbox.
package suna

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BridgeConfig holds configuration for connecting to Suna.
type BridgeConfig struct {
	// FrontendURL is the Suna web UI URL (rebranded as Forge UI).
	FrontendURL string `json:"frontend_url" yaml:"frontend_url"`
	// APIURL is the Suna backend API URL.
	APIURL string `json:"api_url" yaml:"api_url"`
	// APIKey is the authentication key for the Suna API.
	APIKey string `json:"api_key" yaml:"api_key"`
	// SandboxDockerImage is the Docker image for agent sandboxes.
	SandboxDockerImage string `json:"sandbox_docker_image" yaml:"sandbox_docker_image"`
	// DataDir is where Suna data is stored locally.
	DataDir string `json:"data_dir" yaml:"data_dir"`
}

// Bridge connects Forge to the Suna runtime.
type Bridge struct {
	cfg    BridgeConfig
	mu     sync.RWMutex
	client *http.Client
	closed bool
}

// NewBridge creates a new Suna bridge from config.
func NewBridge(cfg BridgeConfig) (*Bridge, error) {
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:3000"
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:8000"
	}
	if cfg.SandboxDockerImage == "" {
		cfg.SandboxDockerImage = "suna/sandbox:latest"
	}
	if cfg.DataDir == "" {
		home, _ := os.UserHomeDir()
		cfg.DataDir = filepath.Join(home, ".forge", "suna")
	}
	return &Bridge{
		cfg:    cfg,
		client: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// FrontendURL returns the configured Suna frontend URL.
func (b *Bridge) FrontendURL() string {
	return b.cfg.FrontendURL
}

// APIURL returns the configured Suna API URL.
func (b *Bridge) APIURL() string {
	return b.cfg.APIURL
}

// DataDir returns the Suna data directory.
func (b *Bridge) DataDir() string {
	return b.cfg.DataDir
}

// doRequest performs an HTTP request against the Suna API.
func (b *Bridge) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, fmt.Errorf("suna bridge closed")
	}
	b.mu.RUnlock()

	var reqBody strings.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = *strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, b.cfg.APIURL+path, &reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if b.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", b.cfg.APIKey)
	}
	return b.client.Do(req)
}

// GetJSON performs a GET and decodes JSON.
func (b *Bridge) GetJSON(ctx context.Context, path string, out interface{}) error {
	resp, err := b.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("suna API %s: HTTP %d", path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// PostJSON performs a POST with JSON body and decodes response.
func (b *Bridge) PostJSON(ctx context.Context, path string, body, out interface{}) error {
	resp, err := b.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("suna API %s: HTTP %d", path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// DeleteJSON performs a DELETE.
func (b *Bridge) DeleteJSON(ctx context.Context, path string) error {
	resp, err := b.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("suna API %s: HTTP %d", path, resp.StatusCode)
	}
	return nil
}

// Health checks Suna API availability.
func (b *Bridge) Health(ctx context.Context) error {
	return b.GetJSON(ctx, "/api/health", nil)
}

// Close shuts down the bridge.
func (b *Bridge) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.client.CloseIdleConnections()
}

// DockerAvailable checks if Docker is available for sandbox creation.
func (b *Bridge) DockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// PullSandboxImage pulls the sandbox Docker image if not already present.
func (b *Bridge) PullSandboxImage(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", b.cfg.SandboxDockerImage)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull sandbox image %s: %w: %s", b.cfg.SandboxDockerImage, err, string(out))
	}
	return nil
}
