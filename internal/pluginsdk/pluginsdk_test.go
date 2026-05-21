package pluginsdk_test

import (
	"context"
	"testing"

	"github.com/forge/sword/internal/pluginsdk"
)

// testPlugin is a minimal plugin implementation for testing.
type testPlugin struct {
	name    string
	version string
	initErr error
}

func (p *testPlugin) Name() string                           { return p.name }
func (p *testPlugin) Version() string                        { return p.version }
func (p *testPlugin) Init(ctx *pluginsdk.Context) error      { return p.initErr }
func (p *testPlugin) Hooks() []*pluginsdk.Hook               { return nil }
func (p *testPlugin) Tools() []*pluginsdk.Tool               { return nil }
func (p *testPlugin) Middleware() []pluginsdk.MiddlewareFunc { return nil }
func (p *testPlugin) Close() error                           { return nil }

func TestRegisterPlugin(t *testing.T) {
	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config:  make(map[string]interface{}),
		Store:   pluginsdk.NewSimpleStore(),
		Logger:  &pluginsdk.SimpleLogger{},
		Metrics: pluginsdk.NewSimpleMetrics(),
	}

	plugin := &testPlugin{name: "test-plugin", version: "1.0.0"}
	err := registry.Register(plugin, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infos := registry.ListPlugins()
	if len(infos) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(infos))
	}
	if infos[0].Name != "test-plugin" {
		t.Errorf("expected test-plugin, got %s", infos[0].Name)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	plugin := &testPlugin{name: "dup", version: "1.0.0"}
	registry.Register(plugin, ctx)

	err := registry.Register(plugin, ctx)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestUnregisterPlugin(t *testing.T) {
	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	plugin := &testPlugin{name: "removable", version: "1.0.0"}
	registry.Register(plugin, ctx)

	err := registry.Unregister("removable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infos := registry.ListPlugins()
	if len(infos) != 0 {
		t.Errorf("expected 0 plugins after unregister, got %d", len(infos))
	}
}

func TestUnregisterNotFound(t *testing.T) {
	registry := pluginsdk.NewRegistry()
	err := registry.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestPluginWithHooks(t *testing.T) {
	hookCalled := false

	plugin := &hookPlugin{
		name:    "hook-plugin",
		version: "1.0.0",
		hooks: []*pluginsdk.Hook{
			{
				Event:    "agent.before_call",
				Priority: 10,
				Handler: func(event *pluginsdk.Event) (*pluginsdk.Result, error) {
					hookCalled = true
					return &pluginsdk.Result{}, nil
				},
			},
		},
	}

	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	registry.Register(plugin, ctx)

	event := &pluginsdk.Event{
		Type: "agent.before_call",
		Data: make(map[string]interface{}),
	}
	registry.ExecuteHook("agent.before_call", event)

	if !hookCalled {
		t.Error("expected hook to be called")
	}
}

func TestHookPriority(t *testing.T) {
	var order []string

	plugin := &hookPlugin{
		name:    "priority-plugin",
		version: "1.0.0",
		hooks: []*pluginsdk.Hook{
			{
				Event:    "test",
				Priority: 20,
				Handler: func(event *pluginsdk.Event) (*pluginsdk.Result, error) {
					order = append(order, "second")
					return &pluginsdk.Result{}, nil
				},
			},
			{
				Event:    "test",
				Priority: 10,
				Handler: func(event *pluginsdk.Event) (*pluginsdk.Result, error) {
					order = append(order, "first")
					return &pluginsdk.Result{}, nil
				},
			},
		},
	}

	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	registry.Register(plugin, ctx)

	event := &pluginsdk.Event{Type: "test", Data: make(map[string]interface{})}
	registry.ExecuteHook("test", event)

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("expected [first, second], got %v", order)
	}
}

func TestPluginWithTool(t *testing.T) {
	plugin := &toolPlugin{
		name:    "tool-plugin",
		version: "1.0.0",
		tools: []*pluginsdk.Tool{
			{
				Name:        "greet",
				Description: "Say hello",
				Category:    "custom",
				Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
					name, _ := params["name"].(string)
					return "Hello, " + name, nil
				},
				Parameters: []pluginsdk.ToolParameter{
					{Name: "name", Type: "string", Required: true},
				},
			},
		},
	}

	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	registry.Register(plugin, ctx)

	// List tools
	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Key != "tool-plugin.greet" {
		t.Errorf("expected tool-plugin.greet, got %s", tools[0].Key)
	}

	// Execute tool
	result, err := registry.ExecuteTool(context.Background(), "tool-plugin.greet", map[string]interface{}{
		"name": "World",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello, World" {
		t.Errorf("expected 'Hello, World', got %v", result)
	}
}

func TestExecuteToolNotFound(t *testing.T) {
	registry := pluginsdk.NewRegistry()
	_, err := registry.ExecuteTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestSimpleStore(t *testing.T) {
	store := pluginsdk.NewSimpleStore()

	store.Set("key1", "value1")
	v, ok := store.Get("key1")
	if !ok || v != "value1" {
		t.Errorf("expected value1, got %v (ok=%v)", v, ok)
	}

	keys := store.Keys()
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}

	store.Delete("key1")
	_, ok = store.Get("key1")
	if ok {
		t.Error("expected key to be deleted")
	}
}

func TestSimpleMetrics(t *testing.T) {
	metrics := pluginsdk.NewSimpleMetrics()

	metrics.Counter("requests", 1)
	metrics.Counter("requests", 2)
	metrics.Gauge("active", 5.0)
	metrics.Timing("duration", 100)

	// Just verify no panics
}

func TestGetPlugin(t *testing.T) {
	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	plugin := &testPlugin{name: "gettable", version: "2.0.0"}
	registry.Register(plugin, ctx)

	got, err := registry.GetPlugin("gettable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version() != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", got.Version())
	}

	_, err = registry.GetPlugin("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestCloseAll(t *testing.T) {
	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	registry.Register(&testPlugin{name: "p1", version: "1.0"}, ctx)
	registry.Register(&testPlugin{name: "p2", version: "1.0"}, ctx)

	err := registry.CloseAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHookModifyEvent(t *testing.T) {
	plugin := &hookPlugin{
		name:    "modifier",
		version: "1.0.0",
		hooks: []*pluginsdk.Hook{
			{
				Event:    "test",
				Priority: 10,
				Handler: func(event *pluginsdk.Event) (*pluginsdk.Result, error) {
					return &pluginsdk.Result{
						Modified: true,
						Data:     map[string]interface{}{"injected": true},
					}, nil
				},
			},
		},
	}

	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	registry.Register(plugin, ctx)

	event := &pluginsdk.Event{Type: "test", Data: make(map[string]interface{})}
	registry.ExecuteHook("test", event)

	if event.Data["injected"] != true {
		t.Error("expected data to be modified by hook")
	}
}

func TestHookCancelEvent(t *testing.T) {
	plugin := &hookPlugin{
		name:    "canceller",
		version: "1.0.0",
		hooks: []*pluginsdk.Hook{
			{
				Event:    "test",
				Priority: 10,
				Handler: func(event *pluginsdk.Event) (*pluginsdk.Result, error) {
					event.Cancel = true
					return &pluginsdk.Result{}, nil
				},
			},
		},
	}

	registry := pluginsdk.NewRegistry()
	ctx := &pluginsdk.Context{
		Config: make(map[string]interface{}),
		Store:  pluginsdk.NewSimpleStore(),
	}

	registry.Register(plugin, ctx)

	event := &pluginsdk.Event{Type: "test", Data: make(map[string]interface{})}
	registry.ExecuteHook("test", event)

	if !event.Cancel {
		t.Error("expected event to be cancelled")
	}
}

// Helper plugin implementations

type hookPlugin struct {
	name    string
	version string
	hooks   []*pluginsdk.Hook
}

func (p *hookPlugin) Name() string                           { return p.name }
func (p *hookPlugin) Version() string                        { return p.version }
func (p *hookPlugin) Init(ctx *pluginsdk.Context) error      { return nil }
func (p *hookPlugin) Hooks() []*pluginsdk.Hook               { return p.hooks }
func (p *hookPlugin) Tools() []*pluginsdk.Tool               { return nil }
func (p *hookPlugin) Middleware() []pluginsdk.MiddlewareFunc { return nil }
func (p *hookPlugin) Close() error                           { return nil }

type toolPlugin struct {
	name    string
	version string
	tools   []*pluginsdk.Tool
}

func (p *toolPlugin) Name() string                           { return p.name }
func (p *toolPlugin) Version() string                        { return p.version }
func (p *toolPlugin) Init(ctx *pluginsdk.Context) error      { return nil }
func (p *toolPlugin) Hooks() []*pluginsdk.Hook               { return nil }
func (p *toolPlugin) Tools() []*pluginsdk.Tool               { return p.tools }
func (p *toolPlugin) Middleware() []pluginsdk.MiddlewareFunc { return nil }
func (p *toolPlugin) Close() error                           { return nil }
