// Package aibridge provides AI request routing and interception.
// Route every request through the forge's bridge — any model, any provider.
package aibridge

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProviderRoute maps a provider to its backend URL.
type ProviderRoute struct {
	Provider string
	BaseURL  string
	APIKey   string
	Models   []string // Models this route handles
}

// Router routes AI requests to the appropriate provider.
type Router struct {
	routes   map[string]*ProviderRoute
	mu       sync.RWMutex
	proxy    *httputil.ReverseProxy
	requests atomic.Int64
	errors   atomic.Int64
	latency  sync.Map // model -> *latencyTracker
}

type latencyTracker struct {
	total time.Duration
	count int64
	mu    sync.Mutex
}

// NewRouter creates a new AI request router.
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]*ProviderRoute),
	}
}

// AddRoute adds a provider route.
func (r *Router) AddRoute(route ProviderRoute) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.Provider] = &route
}

// RemoveRoute removes a provider route.
func (r *Router) RemoveRoute(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, provider)
}

// Route determines which provider should handle the request.
func (r *Router) Route(model string) (*ProviderRoute, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Parse provider/model format
	parts := strings.SplitN(model, "/", 2)
	if len(parts) == 2 {
		if route, ok := r.routes[parts[0]]; ok {
			return route, nil
		}
	}

	// Search routes for the model
	for _, route := range r.routes {
		for _, m := range route.Models {
			if m == model {
				return route, nil
			}
		}
	}

	return nil, fmt.Errorf("aibridge: no route for model %q", model)
}

// ServeHTTP implements http.Handler, proxying requests to the right provider.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	r.requests.Add(1)

	// Determine target from the request path or headers
	model := req.Header.Get("X-Model")
	if model == "" {
		model = "anthropic/claude-sonnet-4-20250514" // default
	}

	route, err := r.Route(model)
	if err != nil {
		r.errors.Add(1)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Create reverse proxy
	target, _ := url.Parse(route.BaseURL)
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Modify request to add API key
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("Authorization", "Bearer "+route.APIKey)
		if strings.Contains(route.BaseURL, "anthropic") {
			req.Header.Set("x-api-key", route.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	}

	proxy.ServeHTTP(w, req)

	// Track latency
	elapsed := time.Since(start)
	r.trackLatency(model, elapsed)
}

func (r *Router) trackLatency(model string, d time.Duration) {
	val, _ := r.latency.LoadOrStore(model, &latencyTracker{})
	tracker := val.(*latencyTracker)
	tracker.mu.Lock()
	tracker.total += d
	tracker.count++
	tracker.mu.Unlock()
}

// Stats returns router statistics.
type Stats struct {
	TotalRequests int64
	TotalErrors   int64
	ModelLatency  map[string]time.Duration
	ModelRequests map[string]int64
}

// GetStats returns current router statistics.
func (r *Router) GetStats() Stats {
	stats := Stats{
		TotalRequests: r.requests.Load(),
		TotalErrors:   r.errors.Load(),
		ModelLatency:  make(map[string]time.Duration),
		ModelRequests: make(map[string]int64),
	}

	r.latency.Range(func(key, value any) bool {
		model := key.(string)
		tracker := value.(*latencyTracker)
		tracker.mu.Lock()
		if tracker.count > 0 {
			stats.ModelLatency[model] = tracker.total / time.Duration(tracker.count)
			stats.ModelRequests[model] = tracker.count
		}
		tracker.mu.Unlock()
		return true
	})

	return stats
}

// Interceptor modifies requests before they're routed.
type Interceptor func(req *http.Request) (*http.Request, error)

// InterceptingRouter wraps a Router with request interceptors.
type InterceptingRouter struct {
	router        *Router
	interceptors  []Interceptor
}

// NewInterceptingRouter creates a router with interceptors.
func NewInterceptingRouter(router *Router) *InterceptingRouter {
	return &InterceptingRouter{router: router}
}

// AddInterceptor adds a request interceptor.
func (ir *InterceptingRouter) AddInterceptor(fn Interceptor) {
	ir.interceptors = append(ir.interceptors, fn)
}

// ServeHTTP implements http.Handler with interceptors.
func (ir *InterceptingRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Apply interceptors
	for _, interceptor := range ir.interceptors {
		modified, err := interceptor(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req = modified
	}

	ir.router.ServeHTTP(w, req)
}

// LoggingInterceptor logs all requests.
func LoggingInterceptor(logger func(format string, args ...any)) Interceptor {
	return func(req *http.Request) (*http.Request, error) {
		logger("aibridge: %s %s %s", req.Method, req.URL.Path, req.RemoteAddr)
		return req, nil
	}
}

// RateLimitInterceptor limits requests per time window.
func RateLimitInterceptor(maxRequests int, window time.Duration) Interceptor {
	var count atomic.Int64
	start := time.Now()

	return func(req *http.Request) (*http.Request, error) {
		if time.Since(start) > window {
			count.Store(0)
			start = time.Now()
		}

		if count.Add(1) > int64(maxRequests) {
			return nil, fmt.Errorf("aibridge: rate limit exceeded (%d req/%v)", maxRequests, window)
		}

		return req, nil
	}
}

// ModelAliasInterceptor maps model aliases to real model names.
func ModelAliasInterceptor(aliases map[string]string) Interceptor {
	return func(req *http.Request) (*http.Request, error) {
		model := req.Header.Get("X-Model")
		if alias, ok := aliases[model]; ok {
			req.Header.Set("X-Model", alias)
		}
		return req, nil
	}
}
