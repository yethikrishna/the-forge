// Package openclaw provides the bridge between Forge and the OpenClaw runtime.
// OpenClaw is the execution substrate for Forge — providing CLI, cron, sessions,
// browser control, channels, skills, node pairing, and memory.
//
// Forge agents live on the OpenClaw runtime. This bridge wraps the OpenClaw
// configuration, CLI invocation, and API surface so the rest of Forge can
// treat it as a Go-native dependency rather than shelling out.
package openclaw

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

// ConfigPath returns the default OpenClaw config location.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw", "openclaw.json")
}

// BridgeConfig holds configuration for connecting to the OpenClaw runtime.
type BridgeConfig struct {
	// GatewayURL is the OpenClaw gateway API endpoint.
	GatewayURL string `json:"gateway_url" yaml:"gateway_url"`
	// GatewayToken is the authentication token for the gateway.
	GatewayToken string `json:"gateway_token" yaml:"gateway_token"`
	// ConfigPath is the path to openclaw.json (auto-detected if empty).
	ConfigPath string `json:"config_path" yaml:"config_path"`
	// CLIBinPath is the path to the openclaw binary (auto-detected if empty).
	CLIBinPath string `json:"cli_bin_path" yaml:"cli_bin_path"`
	// WorkspaceDir is the OpenClaw workspace directory.
	WorkspaceDir string `json:"workspace_dir" yaml:"workspace_dir"`
}

// Bridge connects Forge to the OpenClaw runtime.
type Bridge struct {
	cfg    BridgeConfig
	mu     sync.RWMutex
	client *http.Client
	closed bool
}

// NewBridge creates a new OpenClaw bridge from config.
func NewBridge(cfg BridgeConfig) (*Bridge, error) {
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = ConfigPath()
	}
	if cfg.WorkspaceDir == "" {
		home, _ := os.UserHomeDir()
		cfg.WorkspaceDir = filepath.Join(home, "openclaw-workspace")
	}
	if cfg.GatewayURL == "" {
		cfg.GatewayURL = "http://localhost:3271"
	}
	if cfg.CLIBinPath == "" {
		// Try to find openclaw in PATH
		if p, err := exec.LookPath("openclaw"); err == nil {
			cfg.CLIBinPath = p
		}
	}
	b := &Bridge{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
	return b, nil
}

// LoadConfig reads the OpenClaw configuration from disk.
func (b *Bridge) LoadConfig() (map[string]interface{}, error) {
	data, err := os.ReadFile(b.cfg.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("read openclaw config: %w", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse openclaw config: %w", err)
	}
	return cfg, nil
}

// GatewayURL returns the configured gateway URL.
func (b *Bridge) GatewayURL() string {
	return b.cfg.GatewayURL
}

// WorkspaceDir returns the configured workspace directory.
func (b *Bridge) WorkspaceDir() string {
	return b.cfg.WorkspaceDir
}

// doRequest performs an HTTP request against the OpenClaw gateway with retry.
func (b *Bridge) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, fmt.Errorf("bridge closed")
	}
	b.mu.RUnlock()

	var lastErr error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*100) * time.Millisecond):
			}
		}

		var reqBody strings.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal request body: %w", err)
			}
			reqBody = *strings.NewReader(string(data))
		}

		req, err := http.NewRequestWithContext(ctx, method, b.cfg.GatewayURL+path, &reqBody)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if b.cfg.GatewayToken != "" {
			req.Header.Set("Authorization", "Bearer "+b.cfg.GatewayToken)
		}

		resp, err := b.client.Do(req)
		if err != nil {
			lastErr = err
			continue // retry on network errors
		}

		// Don't retry client errors (4xx), only server errors (5xx)
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("openclaw gateway %s: HTTP %d", path, resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("openclaw gateway %s: failed after %d retries: %w", path, maxRetries, lastErr)
}

// GetJSON performs a GET request and decodes the JSON response.
func (b *Bridge) GetJSON(ctx context.Context, path string, out interface{}) error {
	resp, err := b.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("openclaw gateway %s: HTTP %d", path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// PostJSON performs a POST request with a JSON body and decodes the response.
func (b *Bridge) PostJSON(ctx context.Context, path string, body, out interface{}) error {
	resp, err := b.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("openclaw gateway %s: HTTP %d", path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// PatchJSON performs a PATCH request with a JSON body.
func (b *Bridge) PatchJSON(ctx context.Context, path string, body interface{}) error {
	resp, err := b.doRequest(ctx, http.MethodPatch, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("openclaw gateway %s: HTTP %d", path, resp.StatusCode)
	}
	return nil
}

// Delete performs a DELETE request against the gateway.
func (b *Bridge) Delete(ctx context.Context, path string) error {
	resp, err := b.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("openclaw gateway %s: HTTP %d", path, resp.StatusCode)
	}
	return nil
}

// RunCLI executes an OpenClaw CLI command and returns its output.
func (b *Bridge) RunCLI(ctx context.Context, args ...string) (string, error) {
	if b.cfg.CLIBinPath == "" {
		return "", fmt.Errorf("openclaw CLI not found in PATH")
	}
	cmd := exec.CommandContext(ctx, b.cfg.CLIBinPath, args...)
	cmd.Dir = b.cfg.WorkspaceDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Health checks the OpenClaw gateway health.
func (b *Bridge) Health(ctx context.Context) error {
	return b.GetJSON(ctx, "/health", nil)
}

// Close shuts down the bridge.
func (b *Bridge) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.client.CloseIdleConnections()
}

// Config represents a subset of the OpenClaw configuration relevant to Forge.
type Config struct {
	Gateway struct {
		Port int    `json:"port"`
		Bind string `json:"bind"`
	} `json:"gateway"`
	Agents struct {
		DefaultModel string `json:"default_model"`
	} `json:"agents"`
	Env map[string]string `json:"env"`
}

// ReadConfig reads and parses the OpenClaw config.
func (b *Bridge) ReadConfig(ctx context.Context) (*Config, error) {
	var cfg Config
	err := b.GetJSON(ctx, "/api/config", &cfg)
	if err != nil {
		// Fall back to reading file directly
		data, ferr := os.ReadFile(b.cfg.ConfigPath)
		if ferr != nil {
			return nil, fmt.Errorf("gateway config read failed: %w, file read failed: %w", err, ferr)
		}
		if jerr := json.Unmarshal(data, &cfg); jerr != nil {
			return nil, fmt.Errorf("parse config: %w", jerr)
		}
	}
	return &cfg, nil
}
