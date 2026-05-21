// Package crosstool provides cross-tool bridge adapters for interoperability
// between Forge and other AI agent tools (Cursor, GitHub Copilot, Claude Code).
//
// Be the glue, not the replacement.
package crosstool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolType identifies the external tool.
type ToolType string

const (
	ToolCursor  ToolType = "cursor"
	ToolCopilot ToolType = "copilot"
	ToolClaude  ToolType = "claude"
	ToolCodex   ToolType = "codex"
	ToolOpenAI  ToolType = "openai"
)

// ToolInfo describes an external tool's capabilities.
type ToolInfo struct {
	Type         ToolType  `json:"type"`
	Name         string    `json:"name"`
	Version      string    `json:"version,omitempty"`
	Endpoint     string    `json:"endpoint,omitempty"`
	Connected    bool      `json:"connected"`
	Capabilities []string  `json:"capabilities"`
	LastSeen     time.Time `json:"last_seen,omitempty"`
}

// BridgeMessage is a message exchanged between tools.
type BridgeMessage struct {
	ID        string                 `json:"id"`
	From      ToolType               `json:"from"`
	To        ToolType               `json:"to"`
	Method    string                 `json:"method"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// CursorConfig is configuration for the Cursor bridge.
type CursorConfig struct {
	// Extension API endpoint (Cursor exposes a local API).
	Endpoint string `json:"endpoint,omitempty"`
	// API key if Cursor requires authentication.
	APIKey string `json:"api_key,omitempty"`
	// Workspace path.
	Workspace string `json:"workspace,omitempty"`
}

// CopilotConfig is configuration for the Copilot bridge.
type CopilotConfig struct {
	// GitHub token for Copilot API.
	Token string `json:"token,omitempty"`
	// Repository for context.
	Repo string `json:"repo,omitempty"`
	// GitHub API base URL.
	BaseURL string `json:"base_url,omitempty"`
}

// ClaudeConfig is configuration for the Claude Code bridge.
type ClaudeConfig struct {
	// Anthropic API key.
	APIKey string `json:"api_key,omitempty"`
	// Model to use.
	Model string `json:"model,omitempty"`
}

// CrossBridge manages connections to external agent tools.
type CrossBridge struct {
	mu      sync.RWMutex
	dir     string
	tools   map[ToolType]*ToolInfo
	history []BridgeMessage
	configs map[string]interface{}
	client  *http.Client
	maxHist int
}

// NewCrossBridge creates a new cross-tool bridge.
func NewCrossBridge(dir string) (*CrossBridge, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create crosstool dir: %w", err)
	}
	cb := &CrossBridge{
		dir:     dir,
		tools:   make(map[ToolType]*ToolInfo),
		history: make([]BridgeMessage, 0),
		configs: make(map[string]interface{}),
		client:  &http.Client{Timeout: 30 * time.Second},
		maxHist: 500,
	}
	cb.load()
	return cb, nil
}

// Register registers an external tool connection.
func (cb *CrossBridge) Register(toolType ToolType, config interface{}) (*ToolInfo, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	info := &ToolInfo{
		Type:      toolType,
		Connected: true,
		LastSeen:  time.Now().UTC(),
	}

	switch toolType {
	case ToolCursor:
		cfg, _ := config.(CursorConfig)
		info.Name = "Cursor"
		info.Endpoint = cfg.Endpoint
		info.Capabilities = []string{"code_edit", "file_search", "terminal", "lsp", "agent"}
		cb.configs["cursor"] = cfg

	case ToolCopilot:
		cfg, _ := config.(CopilotConfig)
		info.Name = "GitHub Copilot"
		info.Endpoint = cfg.BaseURL
		info.Capabilities = []string{"code_complete", "chat", "agent", "pr_review"}
		cb.configs["copilot"] = cfg

	case ToolClaude:
		cfg, _ := config.(ClaudeConfig)
		info.Name = "Claude Code"
		info.Capabilities = []string{"code_edit", "terminal", "agent", "search"}
		cb.configs["claude"] = cfg

	case ToolCodex:
		info.Name = "OpenAI Codex"
		info.Capabilities = []string{"code_generate", "sandbox", "agent"}
		cb.configs["codex"] = config

	case ToolOpenAI:
		info.Name = "OpenAI Agents SDK"
		info.Capabilities = []string{"agent", "tools", "handoff"}
		cb.configs["openai"] = config

	default:
		info.Name = string(toolType)
		cb.configs[string(toolType)] = config
	}

	cb.tools[toolType] = info
	cb.save()
	return info, nil
}

// Unregister removes a tool connection.
func (cb *CrossBridge) Unregister(toolType ToolType) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if _, ok := cb.tools[toolType]; !ok {
		return fmt.Errorf("tool %q not registered", toolType)
	}
	delete(cb.tools, toolType)
	delete(cb.configs, string(toolType))
	cb.save()
	return nil
}

// List returns all registered tools.
func (cb *CrossBridge) List() []ToolInfo {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var result []ToolInfo
	for _, info := range cb.tools {
		result = append(result, *info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Get returns info about a specific tool.
func (cb *CrossBridge) Get(toolType ToolType) (*ToolInfo, bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	info, ok := cb.tools[toolType]
	if !ok {
		return nil, false
	}
	cp := *info
	return &cp, true
}

// SendTo sends a message to an external tool.
func (cb *CrossBridge) SendTo(ctx context.Context, target ToolType, method string, params map[string]interface{}) (*BridgeMessage, error) {
	cb.mu.RLock()
	_, ok := cb.tools[target]
	cb.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %q not registered", target)
	}

	msg := BridgeMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		From:      ToolType("forge"),
		To:        target,
		Method:    method,
		Params:    params,
		Timestamp: time.Now().UTC(),
	}

	// For Cursor: translate to Cursor's API format
	// For Copilot: translate to GitHub API format
	// For Claude: translate to Anthropic format
	// This is the bridge layer — it handles protocol translation

	var result interface{}
	var err error

	switch target {
	case ToolCursor:
		result, err = cb.sendToCursor(ctx, method, params)
	case ToolCopilot:
		result, err = cb.sendToCopilot(ctx, method, params)
	case ToolClaude:
		result, err = cb.sendToClaude(ctx, method, params)
	default:
		result = map[string]string{"status": "forwarded", "method": method}
	}

	if err != nil {
		msg.Error = err.Error()
	} else {
		msg.Result = result
	}

	cb.mu.Lock()
	cb.history = append(cb.history, msg)
	if len(cb.history) > cb.maxHist {
		cb.history = cb.history[len(cb.history)-cb.maxHist:]
	}
	cb.mu.Unlock()
	cb.save()

	return &msg, err
}

func (cb *CrossBridge) sendToCursor(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	cb.mu.RLock()
	cfg, _ := cb.configs["cursor"].(CursorConfig)
	cb.mu.RUnlock()

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:9999"
	}

	// Translate Forge method to Cursor's protocol
	cursorReq := map[string]interface{}{
		"method": method,
		"params": params,
	}

	return cb.doHTTPPost(ctx, endpoint+"/api/bridge", cursorReq)
}

func (cb *CrossBridge) sendToCopilot(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	cb.mu.RLock()
	cfg, _ := cb.configs["copilot"].(CopilotConfig)
	cb.mu.RUnlock()

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	copilotReq := map[string]interface{}{
		"method": method,
		"params": params,
	}
	if cfg.Repo != "" {
		copilotReq["repo"] = cfg.Repo
	}

	return cb.doHTTPPost(ctx, baseURL+"/copilot/bridge", copilotReq)
}

func (cb *CrossBridge) sendToClaude(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	// Claude Code bridge — would use Anthropic API
	return map[string]interface{}{
		"status": "sent",
		"method": method,
	}, nil
}

func (cb *CrossBridge) doHTTPPost(ctx context.Context, url string, body interface{}) (interface{}, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := cb.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respData))
	}

	var result interface{}
	json.Unmarshal(respData, &result)
	return result, nil
}

// History returns bridge message history.
func (cb *CrossBridge) History(target ToolType, limit int) []BridgeMessage {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var result []BridgeMessage
	for i := len(cb.history) - 1; i >= 0; i-- {
		msg := cb.history[i]
		if target != "" && (msg.To != target && msg.From != target) {
			continue
		}
		result = append(result, msg)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// Stats returns bridge statistics.
type BridgeStats struct {
	RegisteredTools int              `json:"registered_tools"`
	TotalMessages   int              `json:"total_messages"`
	ByTool          map[ToolType]int `json:"by_tool"`
	LastActivity    *time.Time       `json:"last_activity,omitempty"`
}

// Stats returns aggregate statistics.
func (cb *CrossBridge) Stats() BridgeStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	s := BridgeStats{
		RegisteredTools: len(cb.tools),
		TotalMessages:   len(cb.history),
		ByTool:          make(map[ToolType]int),
	}

	for _, msg := range cb.history {
		s.ByTool[msg.To]++
	}

	if len(cb.history) > 0 {
		last := cb.history[len(cb.history)-1].Timestamp
		s.LastActivity = &last
	}

	return s
}

// TranslateCapability maps a capability from one tool to another.
// e.g., Cursor's "code_edit" maps to Forge's "run" or "patch".
func TranslateCapability(from, to ToolType, capability string) string {
	// Standard capability mapping
	mappings := map[string]map[string]map[string]string{
		"cursor": {
			"forge": {
				"code_edit":   "patch",
				"file_search": "search",
				"terminal":    "exec",
				"agent":       "run",
			},
			"copilot": {
				"code_edit":   "code_complete",
				"file_search": "search",
				"agent":       "agent",
			},
		},
		"copilot": {
			"forge": {
				"code_complete": "run",
				"chat":          "chat",
				"agent":         "run",
				"pr_review":     "review",
			},
			"cursor": {
				"code_complete": "code_edit",
				"agent":         "agent",
			},
		},
		"forge": {
			"cursor": {
				"run":    "agent",
				"exec":   "terminal",
				"search": "file_search",
				"patch":  "code_edit",
				"chat":   "chat",
				"review": "code_edit",
			},
			"copilot": {
				"run":    "agent",
				"exec":   "terminal",
				"chat":   "chat",
				"review": "pr_review",
			},
			"claude": {
				"run":    "agent",
				"exec":   "terminal",
				"search": "search",
				"chat":   "chat",
			},
		},
	}

	if fromMap, ok := mappings[string(from)]; ok {
		if toMap, ok := fromMap[string(to)]; ok {
			if translated, ok := toMap[capability]; ok {
				return translated
			}
		}
	}

	return capability // no translation found, pass through
}

func (cb *CrossBridge) load() {
	data, err := os.ReadFile(filepath.Join(cb.dir, "tools.json"))
	if err == nil {
		json.Unmarshal(data, &cb.tools)
	}
	hdata, err := os.ReadFile(filepath.Join(cb.dir, "history.json"))
	if err == nil {
		json.Unmarshal(hdata, &cb.history)
	}
	cdata, err := os.ReadFile(filepath.Join(cb.dir, "configs.json"))
	if err == nil {
		json.Unmarshal(cdata, &cb.configs)
	}
}

func (cb *CrossBridge) save() {
	data, _ := json.MarshalIndent(cb.tools, "", "  ")
	os.WriteFile(filepath.Join(cb.dir, "tools.json"), data, 0644)

	hdata, _ := json.MarshalIndent(cb.history, "", "  ")
	os.WriteFile(filepath.Join(cb.dir, "history.json"), hdata, 0644)

	cdata, _ := json.MarshalIndent(cb.configs, "", "  ")
	os.WriteFile(filepath.Join(cb.dir, "configs.json"), cdata, 0644)
}

// FormatToolInfo formats tool info for display.
func FormatToolInfo(info ToolInfo) string {
	status := "disconnected"
	if info.Connected {
		status = "connected"
	}
	line := fmt.Sprintf("  %-15s %-20s [%s] %s", info.Type, info.Name, status,
		strings.Join(info.Capabilities, ", "))
	if info.Endpoint != "" {
		line += fmt.Sprintf("\n    Endpoint: %s", info.Endpoint)
	}
	return line
}

// FormatBridgeMessage formats a bridge message for display.
func FormatBridgeMessage(msg BridgeMessage) string {
	line := fmt.Sprintf("  %s → %s [%s] %s", msg.From, msg.To, msg.Method,
		msg.Timestamp.Format("15:04:05"))
	if msg.Error != "" {
		line += fmt.Sprintf(" ERROR: %s", msg.Error)
	}
	return line
}
