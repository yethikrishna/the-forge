package bridge

import (
	"context"
	"testing"
	"time"
)

func TestMCPAdapter(t *testing.T) {
	a := NewMCPAdapter("test-mcp", "http://localhost:8080")
	if a.Protocol() != ProtocolMCP {
		t.Errorf("expected mcp, got %s", a.Protocol())
	}
	if a.Name() != "test-mcp" {
		t.Errorf("expected test-mcp, got %s", a.Name())
	}

	ctx := context.Background()
	msg := &Message{ID: "1", Source: ProtocolA2A, Target: ProtocolMCP, Method: "tools/list", Type: "request"}
	if err := a.Send(ctx, msg); err != nil {
		t.Fatal(err)
	}

	status := a.Status()
	if status.Sent != 1 {
		t.Errorf("expected 1 sent, got %d", status.Sent)
	}
}

func TestA2AAdapter(t *testing.T) {
	a := NewA2AAdapter("test-a2a", "http://localhost:9090")
	if a.Protocol() != ProtocolA2A {
		t.Errorf("expected a2a, got %s", a.Protocol())
	}
}

func TestACPAdapter(t *testing.T) {
	a := NewACPAdapter("test-acp", "http://localhost:3000")
	if a.Protocol() != ProtocolACP {
		t.Errorf("expected acp, got %s", a.Protocol())
	}
}

func TestAdapterInject(t *testing.T) {
	a := NewMCPAdapter("inject-test", "http://localhost:8080")
	msg := &Message{ID: "1", Source: ProtocolMCP, Method: "tools/list", Type: "request"}
	a.Inject(msg)

	status := a.Status()
	if status.Received != 1 {
		t.Errorf("expected 1 received, got %d", status.Received)
	}
	if !status.Connected {
		t.Error("expected connected after inject")
	}
}

func TestAdapterClose(t *testing.T) {
	a := NewMCPAdapter("close-test", "http://localhost:8080")
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewRouter(t *testing.T) {
	r, err := NewRouter(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestRouterRegisterAdapter(t *testing.T) {
	r, _ := NewRouter(t.TempDir())
	a := NewMCPAdapter("test-mcp", "http://localhost:8080")
	r.RegisterAdapter(a)

	adapters := r.ListAdapters()
	if len(adapters) != 1 {
		t.Errorf("expected 1 adapter, got %d", len(adapters))
	}
}

func TestRouterRemoveAdapter(t *testing.T) {
	r, _ := NewRouter(t.TempDir())
	a := NewMCPAdapter("remove-test", "http://localhost:8080")
	r.RegisterAdapter(a)
	r.RemoveAdapter("remove-test")

	adapters := r.ListAdapters()
	if len(adapters) != 0 {
		t.Errorf("expected 0 adapters after removal, got %d", len(adapters))
	}
}

func TestRouterAddRoute(t *testing.T) {
	r, _ := NewRouter(t.TempDir())
	initialCount := len(r.ListRoutes())
	r.AddRoute(Route{Name: "custom", Source: ProtocolMCP, Target: ProtocolA2A, Adapter: "test", Enabled: true})
	if len(r.ListRoutes()) != initialCount+1 {
		t.Error("expected route to be added")
	}
}

func TestRouterRouteMessage(t *testing.T) {
	r, _ := NewRouter(t.TempDir())
	a := NewA2AAdapter("a2a-default", "http://localhost:9090")
	r.RegisterAdapter(a)

	msg := &Message{
		ID:     "1",
		Source: ProtocolMCP,
		Target: ProtocolA2A,
		Type:   "request",
		Method: "tools/list",
		Params: map[string]interface{}{},
	}

	ctx := context.Background()
	if err := r.RouteMessage(ctx, msg); err != nil {
		t.Fatalf("RouteMessage: %v", err)
	}

	stats := r.Stats()
	if stats.TotalRouted != 1 {
		t.Errorf("expected 1 routed, got %d", stats.TotalRouted)
	}
}

func TestRouterRouteNoAdapter(t *testing.T) {
	r, _ := NewRouter(t.TempDir())
	msg := &Message{
		ID:     "1",
		Source: ProtocolMCP,
		Target: ProtocolA2A,
		Type:   "request",
		Method: "tools/list",
		Params: map[string]interface{}{},
	}

	ctx := context.Background()
	// No adapter registered for "a2a-default" route
	// Router silently skips routes with missing adapters, no error
	r.RouteMessage(ctx, msg)
	// Just verify it doesn't panic
}

func TestDefaultRoutes(t *testing.T) {
	routes := DefaultRoutes()
	if len(routes) < 6 {
		t.Errorf("expected at least 6 default routes, got %d", len(routes))
	}
}

func TestRouterStats(t *testing.T) {
	r, _ := NewRouter(t.TempDir())
	stats := r.Stats()
	// StartedAt is set when Start() is called, not on creation
	if stats.TotalRouted != 0 {
		t.Error("expected zero initial routed count")
	}
}

func TestFormatAdapterStatus(t *testing.T) {
	s := AdapterStatus{
		Name:      "test",
		Protocol:  ProtocolMCP,
		Connected: true,
		Sent:      10,
		Received:  5,
		Errors:    1,
		LastMsgAt: time.Now(),
	}
	output := FormatAdapterStatus(s)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestDiscoveryConfig(t *testing.T) {
	cfg := DefaultDiscoveryConfig()
	if !cfg.ScanLocalhost {
		t.Error("expected ScanLocalhost to be true by default")
	}
	if cfg.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
	if len(cfg.KnownPorts) == 0 {
		t.Error("expected known ports")
	}
}

func TestDiscovererScan(t *testing.T) {
	cfg := DefaultDiscoveryConfig()
	cfg.ScanLocalhost = false // Don't actually scan localhost in tests
	cfg.ConfigDir = t.TempDir()
	d := NewDiscoverer(cfg)

	ctx := context.Background()
	endpoints, err := d.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = endpoints // May be empty in test env, that's fine
}

func TestDiscovererCache(t *testing.T) {
	cfg := DefaultDiscoveryConfig()
	cfg.ScanLocalhost = false
	cfg.ConfigDir = t.TempDir()
	d := NewDiscoverer(cfg)

	ctx := context.Background()
	d.Scan(ctx)
	cache := d.Cache()
	// Cache should match scan results
	_ = cache
}

func TestFormatEndpoint(t *testing.T) {
	ep := Endpoint{
		Name:         "test-endpoint",
		Protocol:     ProtocolMCP,
		Address:      "http://localhost:8080",
		Healthy:      true,
		Source:       "scan",
		DiscoveredAt: time.Now(),
	}
	output := FormatEndpoint(ep)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
