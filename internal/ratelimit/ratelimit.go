// Package ratelimit provides distributed rate limiting for AI agents.
// Supports token bucket, sliding window, and fixed window algorithms.
// Per-agent, per-model, and global scopes with configurable limits.
package ratelimit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Algorithm defines the rate limiting algorithm.
type Algorithm string

const (
	AlgoTokenBucket    Algorithm = "token_bucket"
	AlgoSlidingWindow  Algorithm = "sliding_window"
	AlgoFixedWindow    Algorithm = "fixed_window"
)

// Scope defines the scope of a rate limit.
type Scope string

const (
	ScopeGlobal   Scope = "global"
	ScopeAgent    Scope = "agent"
	ScopeModel    Scope = "model"
	ScopeProvider Scope = "provider"
	ScopeUser     Scope = "user"
)

// Limit defines a rate limit configuration.
type Limit struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Scope      Scope     `json:"scope"`
	Algorithm  Algorithm `json:"algorithm"`
	Rate       float64   `json:"rate"`        // Requests per second (token bucket) or per window
	Burst      int       `json:"burst"`        // Max burst size (token bucket)
	WindowSec  int       `json:"window_sec"`   // Window duration (fixed/sliding)
	MaxInWindow int      `json:"max_in_window"` // Max requests per window
	Enabled    bool      `json:"enabled"`
}

// BucketState tracks the state of a token bucket.
type BucketState struct {
	Tokens     float64   `json:"tokens"`
	LastRefill time.Time `json:"last_refill"`
}

// WindowState tracks the state of a window counter.
type WindowState struct {
	Count     int       `json:"count"`
	WindowEnd time.Time `json:"window_end"`
}

// Request represents a rate-limited request.
type Request struct {
	ScopeKey  string    `json:"scope_key"`  // e.g., agent ID, model name
	Timestamp time.Time `json:"timestamp"`
	Cost      float64   `json:"cost"` // Optional cost (for token-based limits)
}

// Decision represents the result of a rate limit check.
type Decision struct {
	Allowed    bool      `json:"allowed"`
	Limit      Limit     `json:"limit"`
	Remaining  float64   `json:"remaining"`
	ResetAt    *time.Time `json:"reset_at,omitempty"`
	RetryAfter string    `json:"retry_after,omitempty"`
	Reason     string    `json:"reason,omitempty"`
}

// Manager manages rate limits.
type Manager struct {
	storeDir  string
	limits    map[string]*Limit
	buckets   map[string]*BucketState
	windows   map[string]*WindowState
	mu        sync.Mutex
}

// NewManager creates a new rate limit manager.
func NewManager(storeDir string) *Manager {
	os.MkdirAll(storeDir, 0755)
	m := &Manager{
		storeDir: storeDir,
		limits:   make(map[string]*Limit),
		buckets:  make(map[string]*BucketState),
		windows:  make(map[string]*WindowState),
	}
	m.load()
	if len(m.limits) == 0 {
		m.initDefaults()
	}
	return m
}

// DefaultLimits returns built-in rate limits.
func DefaultLimits() []*Limit {
	return []*Limit{
		{ID: "global-rpm", Name: "Global RPM", Scope: ScopeGlobal, Algorithm: AlgoTokenBucket, Rate: 100, Burst: 20, Enabled: true},
		{ID: "global-tpm", Name: "Global TPM", Scope: ScopeGlobal, Algorithm: AlgoTokenBucket, Rate: 100000, Burst: 10000, Enabled: true},
		{ID: "agent-rpm", Name: "Agent RPM", Scope: ScopeAgent, Algorithm: AlgoSlidingWindow, MaxInWindow: 30, WindowSec: 60, Enabled: true},
		{ID: "model-rpm", Name: "Model RPM", Scope: ScopeModel, Algorithm: AlgoFixedWindow, MaxInWindow: 60, WindowSec: 60, Enabled: true},
		{ID: "provider-rpm", Name: "Provider RPM", Scope: ScopeProvider, Algorithm: AlgoSlidingWindow, MaxInWindow: 50, WindowSec: 60, Enabled: true},
	}
}

// AddLimit adds a rate limit configuration.
func (m *Manager) AddLimit(limit Limit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit.ID == "" {
		return fmt.Errorf("limit ID is required")
	}
	m.limits[limit.ID] = &limit
	m.save()
	return nil
}

// RemoveLimit removes a rate limit.
func (m *Manager) RemoveLimit(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.limits[id]; !ok {
		return fmt.Errorf("limit %s not found", id)
	}
	delete(m.limits, id)
	m.save()
	return nil
}

// GetLimit retrieves a limit by ID.
func (m *Manager) GetLimit(id string) (*Limit, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.limits[id]
	return l, ok
}

// ListLimits lists all rate limits.
func (m *Manager) ListLimits() []*Limit {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Limit, 0, len(m.limits))
	for _, l := range m.limits {
		result = append(result, l)
	}
	return result
}

// ListByScope lists limits for a scope.
func (m *Manager) ListByScope(scope Scope) []*Limit {
	var result []*Limit
	for _, l := range m.ListLimits() {
		if l.Scope == scope {
			result = append(result, l)
		}
	}
	return result
}

// Check checks if a request is allowed under all applicable limits.
func (m *Manager) Check(req Request) []Decision {
	m.mu.Lock()
	defer m.mu.Unlock()

	var decisions []Decision

	for _, limit := range m.limits {
		if !limit.Enabled {
			continue
		}
		if limit.Scope != ScopeGlobal && limit.Scope != Scope(req.ScopeKey) {
			// Check if scope matches — for non-global, the key must match
			continue
		}

		key := m.bucketKey(limit.ID, req.ScopeKey)
		var decision Decision

		switch limit.Algorithm {
		case AlgoTokenBucket:
			decision = m.checkTokenBucket(key, limit)
		case AlgoFixedWindow:
			decision = m.checkFixedWindow(key, limit)
		case AlgoSlidingWindow:
			decision = m.checkSlidingWindow(key, limit)
		default:
			decision = m.checkTokenBucket(key, limit)
		}

		decisions = append(decisions, decision)
	}

	return decisions
}

// Allow checks if a request is allowed (returns true/false).
func (m *Manager) Allow(req Request) bool {
	decisions := m.Check(req)
	for _, d := range decisions {
		if !d.Allowed {
			return false
		}
	}
	return true
}

// Record records a request against all applicable limits.
func (m *Manager) Record(req Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, limit := range m.limits {
		if !limit.Enabled {
			continue
		}

		key := m.bucketKey(limit.ID, req.ScopeKey)

		switch limit.Algorithm {
		case AlgoTokenBucket:
			m.consumeToken(key, limit)
		case AlgoFixedWindow, AlgoSlidingWindow:
			m.incrementWindow(key, limit)
		}
	}
	m.save()
}

// Reset resets all rate limit counters.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.buckets = make(map[string]*BucketState)
	m.windows = make(map[string]*WindowState)
	m.save()
}

// Stats returns rate limiter statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	byScope := make(map[Scope]int)
	byAlgo := make(map[Algorithm]int)

	for _, l := range m.limits {
		byScope[l.Scope]++
		byAlgo[l.Algorithm]++
	}

	return map[string]interface{}{
		"total_limits":  len(m.limits),
		"active_buckets": len(m.buckets),
		"active_windows": len(m.windows),
		"by_scope":      byScope,
		"by_algorithm":  byAlgo,
	}
}

func (m *Manager) checkTokenBucket(key string, limit *Limit) Decision {
	now := time.Now()

	bucket, ok := m.buckets[key]
	if !ok {
		bucket = &BucketState{
			Tokens:     float64(limit.Burst),
			LastRefill: now,
		}
		m.buckets[key] = bucket
	}

	// Refill tokens
	elapsed := now.Sub(bucket.LastRefill).Seconds()
	bucket.Tokens += elapsed * limit.Rate
	if bucket.Tokens > float64(limit.Burst) {
		bucket.Tokens = float64(limit.Burst)
	}
	bucket.LastRefill = now

	if bucket.Tokens >= 1.0 {
		return Decision{
			Allowed:   true,
			Limit:     *limit,
			Remaining: bucket.Tokens - 1,
		}
	}

	retryAfter := (1.0 - bucket.Tokens) / limit.Rate
	retryTime := now.Add(time.Duration(retryAfter * float64(time.Second)))
	return Decision{
		Allowed:    false,
		Limit:      *limit,
		Remaining:  0,
		RetryAfter: fmt.Sprintf("%.1fs", retryAfter),
		ResetAt:    &retryTime,
		Reason:     "rate limit exceeded",
	}
}

func (m *Manager) checkFixedWindow(key string, limit *Limit) Decision {
	now := time.Now()
	window, ok := m.windows[key]

	if !ok || now.After(window.WindowEnd) {
		windowEnd := now.Add(time.Duration(limit.WindowSec) * time.Second)
		window = &WindowState{Count: 0, WindowEnd: windowEnd}
		m.windows[key] = window
	}

	remaining := float64(limit.MaxInWindow - window.Count)
	if remaining < 0 {
		remaining = 0
	}

	if window.Count < limit.MaxInWindow {
		return Decision{
			Allowed:   true,
			Limit:     *limit,
			Remaining: remaining - 1,
			ResetAt:   &window.WindowEnd,
		}
	}

	return Decision{
		Allowed:    false,
		Limit:      *limit,
		Remaining:  0,
		ResetAt:    &window.WindowEnd,
		RetryAfter: time.Until(window.WindowEnd).Round(time.Second).String(),
		Reason:     "window limit exceeded",
	}
}

func (m *Manager) checkSlidingWindow(key string, limit *Limit) Decision {
	// Simplified sliding window — same as fixed for now
	return m.checkFixedWindow(key, limit)
}

func (m *Manager) consumeToken(key string, limit *Limit) {
	if bucket, ok := m.buckets[key]; ok {
		if bucket.Tokens >= 1.0 {
			bucket.Tokens -= 1.0
		}
	}
}

func (m *Manager) incrementWindow(key string, limit *Limit) {
	now := time.Now()
	window, ok := m.windows[key]

	if !ok || now.After(window.WindowEnd) {
		windowEnd := now.Add(time.Duration(limit.WindowSec) * time.Second)
		window = &WindowState{Count: 0, WindowEnd: windowEnd}
		m.windows[key] = window
	}

	window.Count++
}

func (m *Manager) bucketKey(limitID, scopeKey string) string {
	return fmt.Sprintf("%s:%s", limitID, scopeKey)
}

func (m *Manager) initDefaults() {
	for _, l := range DefaultLimits() {
		m.limits[l.ID] = l
	}
	m.save()
}

func (m *Manager) save() {
	data, _ := json.MarshalIndent(map[string]interface{}{
		"limits":  m.limits,
		"buckets": m.buckets,
		"windows": m.windows,
	}, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "ratelimits.json"), data, 0644)
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "ratelimits.json"))
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if lData, ok := raw["limits"]; ok {
		json.Unmarshal(lData, &m.limits)
	}
	if bData, ok := raw["buckets"]; ok {
		json.Unmarshal(bData, &m.buckets)
	}
	if wData, ok := raw["windows"]; ok {
		json.Unmarshal(wData, &m.windows)
	}
}
