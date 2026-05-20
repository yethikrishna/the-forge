package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Router connects multiple protocol adapters and routes messages between them.
// It is the core of the bridge server — every message that enters from one
// protocol gets translated and dispatched to the appropriate target adapter.
type Router struct {
	mu       sync.RWMutex
	bridge   *Bridge
	adapters map[string]Adapter // name → adapter
	routes   []Route            // routing rules
	server   *http.Server
	dir      string
	stats    RouterStats
}

// RouterStats tracks aggregate routing metrics.
type RouterStats struct {
	TotalRouted  int64     `json:"total_routed"`
	TotalErrors  int64     `json:"total_errors"`
	StartedAt    time.Time `json:"started_at"`
	LastRoutedAt time.Time `json:"last_routed_at,omitempty"`
}

// Route defines a routing rule: messages matching the filter get forwarded
// to all target adapters after protocol translation.
type Route struct {
	Name      string   `json:"name"`
	Source    Protocol `json:"source"`    // source protocol filter (empty = any)
	Target    Protocol `json:"target"`    // target protocol (required)
	Method    string   `json:"method"`    // method prefix filter (empty = any)
	Adapter   string   `json:"adapter"`   // target adapter name (required)
	Enabled   bool     `json:"enabled"`
}

// NewRouter creates a message router.
func NewRouter(bridgeDir string) (*Router, error) {
	b, err := NewBridge(bridgeDir)
	if err != nil {
		return nil, err
	}
	return &Router{
		bridge:   b,
		adapters: make(map[string]Adapter),
		routes:   DefaultRoutes(),
		dir:      bridgeDir,
	}, nil
}

// DefaultRoutes returns built-in routing rules.
func DefaultRoutes() []Route {
	return []Route{
		{Name: "mcp-to-a2a", Source: ProtocolMCP, Target: ProtocolA2A, Adapter: "a2a-default", Enabled: true},
		{Name: "a2a-to-mcp", Source: ProtocolA2A, Target: ProtocolMCP, Adapter: "mcp-default", Enabled: true},
		{Name: "acp-to-mcp", Source: ProtocolACP, Target: ProtocolMCP, Adapter: "mcp-default", Enabled: true},
		{Name: "mcp-to-acp", Source: ProtocolMCP, Target: ProtocolACP, Adapter: "acp-default", Enabled: true},
		{Name: "a2a-to-acp", Source: ProtocolA2A, Target: ProtocolACP, Adapter: "acp-default", Enabled: true},
		{Name: "acp-to-a2a", Source: ProtocolACP, Target: ProtocolA2A, Adapter: "a2a-default", Enabled: true},
	}
}

// RegisterAdapter adds an adapter to the router.
func (r *Router) RegisterAdapter(adapter Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Name()] = adapter
}

// RemoveAdapter removes an adapter by name.
func (r *Router) RemoveAdapter(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.adapters[name]; ok {
		a.Close()
		delete(r.adapters, name)
	}
}

// AddRoute adds a routing rule.
func (r *Router) AddRoute(route Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, route)
}

// ListRoutes returns all routing rules.
func (r *Router) ListRoutes() []Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Route, len(r.routes))
	copy(out, r.routes)
	return out
}

// ListAdapters returns status of all adapters.
func (r *Router) ListAdapters() []AdapterStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	statuses := make([]AdapterStatus, 0, len(r.adapters))
	for _, a := range r.adapters {
		statuses = append(statuses, a.Status())
	}
	return statuses
}

// RouteMessage translates and forwards a message according to routing rules.
func (r *Router) RouteMessage(ctx context.Context, msg *Message) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find matching routes
	var matched []Route
	for _, route := range r.routes {
		if !route.Enabled {
			continue
		}
		if route.Source != "" && route.Source != msg.Source {
			continue
		}
		if route.Method != "" && msg.Method != route.Method {
			continue
		}
		matched = append(matched, route)
	}

	if len(matched) == 0 {
		return fmt.Errorf("no route for %s/%s → %s", msg.Source, msg.Method, msg.Target)
	}

	var firstErr error
	for _, route := range matched {
		adapter, ok := r.adapters[route.Adapter]
		if !ok {
			continue
		}

		// Set target protocol from route
		targetMsg := *msg
		targetMsg.Target = route.Target

		// Translate
		translated, err := r.bridge.Translate(&targetMsg)
		if err != nil {
			r.stats.TotalErrors++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// Send via adapter
		if err := adapter.Send(ctx, translated); err != nil {
			r.stats.TotalErrors++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		r.stats.TotalRouted++
		r.stats.LastRoutedAt = time.Now()
	}

	return firstErr
}

// Start begins the routing loop: reads from all adapters and routes messages.
func (r *Router) Start(ctx context.Context) error {
	r.stats.StartedAt = time.Now()

	// Start a goroutine per adapter to read incoming messages
	for _, adapter := range r.adapters {
		go func(a Adapter) {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-a.Receive():
					if !ok {
						return
					}
					if err := r.RouteMessage(ctx, msg); err != nil {
						log.Printf("bridge: route error from %s: %v", a.Name(), err)
					}
				}
			}
		}(adapter)
	}

	<-ctx.Done()
	return nil
}

// ServeHTTP starts an HTTP API for the bridge router.
func (r *Router) ServeHTTP(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/bridge/status", func(w http.ResponseWriter, req *http.Request) {
		r.mu.RLock()
		defer r.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stats":    r.stats,
			"adapters": r.ListAdapters(),
			"routes":   r.ListRoutes(),
		})
	})

	mux.HandleFunc("/bridge/route", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var msg Message
		if err := json.NewDecoder(req.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := r.RouteMessage(req.Context(), &msg); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "routed"})
	})

	mux.HandleFunc("/bridge/adapters", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(r.ListAdapters())
	})

	mux.HandleFunc("/bridge/routes", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(r.ListRoutes())
	})

	r.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("bridge: listen: %w", err)
	}

	log.Printf("bridge: HTTP server listening on %s", addr)
	return r.server.Serve(ln)
}

// Shutdown gracefully stops the HTTP server.
func (r *Router) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.adapters {
		a.Close()
	}
	if r.server != nil {
		return r.server.Shutdown(ctx)
	}
	return nil
}

// Stats returns aggregate routing metrics.
func (r *Router) Stats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// SaveConfig persists the router configuration.
func (r *Router) SaveConfig() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg := struct {
		Routes   []Route         `json:"routes"`
		Adapters []AdapterConfig `json:"adapters"`
	}{
		Routes: r.routes,
	}

	for _, a := range r.adapters {
		cfg.Adapters = append(cfg.Adapters, AdapterConfig{
			Name:     a.Name(),
			Protocol: string(a.Protocol()),
		})
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.dir, "router.json"), data, 0o644)
}

// AdapterConfig is the serialized form of an adapter's configuration.
type AdapterConfig struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Address  string `json:"address,omitempty"`
}
