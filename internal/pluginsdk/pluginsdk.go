// Package pluginsdk provides the SDK for building Forge plugins.
// Plugins extend Forge's capabilities by hooking into the agent
// lifecycle, adding custom commands, and providing middleware.
//
// A plugin implements the Plugin interface and is loaded at runtime.
// Plugins can:
//   - Subscribe to agent lifecycle events (before/after agent calls)
//   - Add custom Cobra commands to the CLI
//   - Provide middleware for agent request/response processing
//   - Register custom tools that agents can use
//   - Extend the dashboard with custom widgets
//
// Example:
//
//	package main
//
//	import (
//	    "github.com/forge/sword/internal/pluginsdk"
//	)
//
//	type MyPlugin struct{}
//
//	func (p *MyPlugin) Name() string { return "my-plugin" }
//	func (p *MyPlugin) Version() string { return "1.0.0" }
//	func (p *MyPlugin) Init(ctx *pluginsdk.Context) error { return nil }
//	func (p *MyPlugin) Hooks() []*pluginsdk.Hook { return nil }
//	func (p *MyPlugin) Tools() []*pluginsdk.Tool { return nil }
//	func (p *MyPlugin) Middleware() []pluginsdk.MiddlewareFunc { return nil }
//	func (p *MyPlugin) Close() error { return nil }
package pluginsdk

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Plugin is the interface that all Forge plugins must implement.
type Plugin interface {
	// Name returns the plugin's unique identifier.
	Name() string

	// Version returns the plugin's semantic version.
	Version() string

	// Init initializes the plugin with the Forge context.
	Init(ctx *Context) error

	// Hooks returns lifecycle hooks the plugin wants to subscribe to.
	Hooks() []*Hook

	// Tools returns custom tools the plugin provides to agents.
	Tools() []*Tool

	// Middleware returns middleware functions for agent request/response processing.
	Middleware() []MiddlewareFunc

	// Close cleans up plugin resources.
	Close() error
}

// Context provides the plugin with access to Forge internals.
type Context struct {
	Config    map[string]interface{}
	EventBus  EventBus
	Store     Store
	Logger    Logger
	Metrics   Metrics
}

// EventBus allows plugins to publish and subscribe to events.
type EventBus interface {
	Publish(topic string, data interface{})
	Subscribe(topic string, handler func(data interface{})) string
	Unsubscribe(subscriptionID string)
}

// Store provides key-value storage for plugin state.
type Store interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	Keys() []string
}

// Logger provides structured logging.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

// Field is a structured logging field.
type Field struct {
	Key   string
	Value interface{}
}

// Metrics provides metric recording.
type Metrics interface {
	Counter(name string, delta float64)
	Gauge(name string, value float64)
	Timing(name string, duration time.Duration)
}

// Hook represents a lifecycle hook subscription.
type Hook struct {
	Event    string    // "agent.before_call", "agent.after_call", "tool.before", "tool.after", "session.start", "session.end"
	Priority int       // lower = runs first (default: 100)
	Handler  HookHandler
}

// HookHandler processes a lifecycle event.
type HookHandler func(event *Event) (*Result, error)

// Event represents a lifecycle event.
type Event struct {
	ID        string
	Type      string
	Timestamp time.Time
	AgentID   string
	SessionID string
	Data      map[string]interface{}
	Metadata  map[string]string
	Cancel    bool // set to true to cancel the operation
}

// Result represents the outcome of a hook handler.
type Result struct {
	Modified bool                // whether the event data was modified
	Data     map[string]interface{} // modified event data
	Error    string              // error message if the hook failed
	Skip     bool                // skip remaining hooks
}

// Tool represents a custom tool that agents can use.
type Tool struct {
	Name        string
	Description string
	Parameters  []ToolParameter
	Handler     ToolHandler
	Category    string // "file", "exec", "network", "custom"
	Dangerous   bool   // requires approval before execution
}

// ToolParameter defines a tool input parameter.
type ToolParameter struct {
	Name        string
	Type        string // "string", "int", "bool", "file", "json"
	Description string
	Required    bool
	Default     interface{}
}

// ToolHandler executes the tool.
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// MiddlewareFunc processes agent requests/responses.
type MiddlewareFunc func(ctx context.Context, req *AgentRequest, next MiddlewareNext) (*AgentResponse, error)

// MiddlewareNext calls the next middleware in the chain.
type MiddlewareNext func(ctx context.Context, req *AgentRequest) (*AgentResponse, error)

// AgentRequest represents a request to an AI agent.
type AgentRequest struct {
	AgentID    string
	Model      string
	Prompt     string
	Tools      []string
	Context    map[string]interface{}
	MaxTokens  int
	Temperature float64
}

// AgentResponse represents a response from an AI agent.
type AgentResponse struct {
	AgentID    string
	Output     string
	TokensUsed int
	CostUSD    float64
	Model      string
	Duration   time.Duration
	ToolCalls  []ToolCall
	Metadata   map[string]string
}

// ToolCall represents a tool invocation in a response.
type ToolCall struct {
	ToolName string
	Params   map[string]interface{}
	Result   interface{}
}

// Registry manages loaded plugins.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	hooks   map[string][]*hookEntry
	tools   map[string]*Tool
}

type hookEntry struct {
	plugin  string
	hook    *Hook
}

// NewRegistry creates a new plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		hooks:   make(map[string][]*hookEntry),
		tools:   make(map[string]*Tool),
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(plugin Plugin, ctx *Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := plugin.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	// Initialize the plugin
	if err := plugin.Init(ctx); err != nil {
		return fmt.Errorf("plugin %s init failed: %w", name, err)
	}

	r.plugins[name] = plugin

	// Register hooks
	for _, hook := range plugin.Hooks() {
		entry := &hookEntry{plugin: name, hook: hook}
		r.hooks[hook.Event] = append(r.hooks[hook.Event], entry)
		// Sort by priority
		sort.Slice(r.hooks[hook.Event], func(i, j int) bool {
			return r.hooks[hook.Event][i].hook.Priority < r.hooks[hook.Event][j].hook.Priority
		})
	}

	// Register tools
	for _, tool := range plugin.Tools() {
		toolKey := name + "." + tool.Name
		r.tools[toolKey] = tool
	}

	return nil
}

// Unregister removes a plugin from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Close the plugin
	if err := plugin.Close(); err != nil {
		return fmt.Errorf("plugin %s close failed: %w", name, err)
	}

	delete(r.plugins, name)

	// Remove hooks
	for event, entries := range r.hooks {
		var filtered []*hookEntry
		for _, e := range entries {
			if e.plugin != name {
				filtered = append(filtered, e)
			}
		}
		r.hooks[event] = filtered
	}

	// Remove tools
	for key, tool := range r.tools {
		prefix := name + "."
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(r.tools, key)
		}
		_ = tool
	}

	return nil
}

// GetPlugin returns a plugin by name.
func (r *Registry) GetPlugin(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return p, nil
}

// ListPlugins returns all registered plugins.
func (r *Registry) ListPlugins() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []PluginInfo
	for name, p := range r.plugins {
		infos = append(infos, PluginInfo{
			Name:    name,
			Version: p.Version(),
			Hooks:   len(p.Hooks()),
			Tools:   len(p.Tools()),
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// PluginInfo holds metadata about a registered plugin.
type PluginInfo struct {
	Name    string
	Version string
	Hooks   int
	Tools   int
}

// ExecuteHook runs all hooks for a given event.
func (r *Registry) ExecuteHook(eventType string, event *Event) (*Event, error) {
	r.mu.RLock()
	entries, ok := r.hooks[eventType]
	r.mu.RUnlock()

	if !ok {
		return event, nil
	}

	for _, entry := range entries {
		result, err := entry.hook.Handler(event)
		if err != nil {
			return event, fmt.Errorf("hook %s.%s failed: %w", entry.plugin, eventType, err)
		}

		if result != nil {
			if result.Modified {
				for k, v := range result.Data {
					event.Data[k] = v
				}
			}
			if result.Skip {
				break
			}
			if event.Cancel {
				break
			}
		}
	}

	return event, nil
}

// ExecuteTool runs a registered tool.
func (r *Registry) ExecuteTool(ctx context.Context, toolKey string, params map[string]interface{}) (interface{}, error) {
	r.mu.RLock()
	tool, ok := r.tools[toolKey]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %s not found", toolKey)
	}

	return tool.Handler(ctx, params)
}

// ListTools returns all registered tools.
func (r *Registry) ListTools() []ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []ToolInfo
	for key, tool := range r.tools {
		infos = append(infos, ToolInfo{
			Key:         key,
			Name:        tool.Name,
			Description: tool.Description,
			Category:    tool.Category,
			Dangerous:   tool.Dangerous,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Key < infos[j].Key
	})

	return infos
}

// ToolInfo holds metadata about a registered tool.
type ToolInfo struct {
	Key         string
	Name        string
	Description string
	Category    string
	Dangerous   bool
}

// CloseAll closes all registered plugins.
func (r *Registry) CloseAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, p := range r.plugins {
		if err := p.Close(); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing plugins: %v", errs)
	}

	return nil
}

// SimpleStore is an in-memory implementation of Store.
type SimpleStore struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewSimpleStore creates a new in-memory store.
func NewSimpleStore() *SimpleStore {
	return &SimpleStore{data: make(map[string]interface{})}
}

func (s *SimpleStore) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *SimpleStore) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *SimpleStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *SimpleStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// SimpleLogger is a basic logger implementation.
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, fields ...Field) {}
func (l *SimpleLogger) Info(msg string, fields ...Field)  {}
func (l *SimpleLogger) Warn(msg string, fields ...Field)  {}
func (l *SimpleLogger) Error(msg string, fields ...Field) {}

// SimpleMetrics is a basic metrics implementation.
type SimpleMetrics struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
}

// NewSimpleMetrics creates a new simple metrics collector.
func NewSimpleMetrics() *SimpleMetrics {
	return &SimpleMetrics{
		counters: make(map[string]float64),
		gauges:   make(map[string]float64),
	}
}

func (m *SimpleMetrics) Counter(name string, delta float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += delta
}

func (m *SimpleMetrics) Gauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

func (m *SimpleMetrics) Timing(name string, duration time.Duration) {
	m.Counter(name+"_total_ms", float64(duration.Milliseconds()))
}
