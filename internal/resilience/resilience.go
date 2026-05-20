// Package resilience provides circuit breaker and fallback patterns for provider reliability.
// When a provider (OpenAI, Anthropic, etc.) fails repeatedly, the circuit breaker
// trips and routes requests to fallback providers automatically.
//
// Failures are inevitable. Recovery must be automatic.
package resilience

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"   // Normal operation
	CircuitOpen     CircuitState = "open"      // Failing, reject requests
	CircuitHalfOpen CircuitState = "half_open" // Testing if provider recovered
)

// ProviderStats tracks success/failure metrics for a provider.
type ProviderStats struct {
	Provider       string        `json:"provider"`
	State          CircuitState  `json:"state"`
	Successes      int64         `json:"successes"`
	Failures       int64         `json:"failures"`
	ConsecutiveFails int64       `json:"consecutive_fails"`
	LastSuccess    time.Time     `json:"last_success,omitempty"`
	LastFailure    time.Time     `json:"last_failure,omitempty"`
	LastStateChange time.Time    `json:"last_state_change"`
	TotalRequests  int64         `json:"total_requests"`
	FailureRate    float64       `json:"failure_rate"` // last window
}

// CircuitConfig configures a circuit breaker.
type CircuitConfig struct {
	FailureThreshold int64         `json:"failure_threshold"` // consecutive failures to trip (default: 5)
	SuccessThreshold int64         `json:"success_threshold"` // successes in half-open to close (default: 3)
	Timeout          time.Duration `json:"timeout"`           // time in open before half-open (default: 30s)
	Window           time.Duration `json:"window"`            // rolling window for rate calc (default: 60s)
}

// DefaultCircuitConfig returns sensible defaults.
func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		Window:           60 * time.Second,
	}
}

// windowEntry tracks a single request outcome within a time window.
type windowEntry struct {
	time    time.Time
	success bool
}

// CircuitBreaker implements the circuit breaker pattern for a single provider.
type CircuitBreaker struct {
	mu       sync.RWMutex
	provider string
	config   CircuitConfig
	stats    ProviderStats
	window   []windowEntry
	halfOpenSuccesses int64
}

// NewCircuitBreaker creates a circuit breaker for a provider.
func NewCircuitBreaker(provider string, config CircuitConfig) *CircuitBreaker {
	now := time.Now()
	return &CircuitBreaker{
		provider: provider,
		config:   config,
		stats: ProviderStats{
			Provider:        provider,
			State:           CircuitClosed,
			LastStateChange: now,
		},
		window: make([]windowEntry, 0, 100),
	}
}

// Allow checks if a request should proceed.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.stats.State {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		if time.Since(cb.stats.LastStateChange) > cb.config.Timeout {
			cb.transitionTo(CircuitHalfOpen)
			return nil // allow one request through
		}
		return fmt.Errorf("circuit open for provider %q (since %s, retry after %s)",
			cb.provider, cb.stats.LastStateChange.Format(time.RFC3339), cb.config.Timeout)
	case CircuitHalfOpen:
		return nil // allow limited requests through
	default:
		return nil
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.stats.Successes++
	cb.stats.TotalRequests++
	cb.stats.ConsecutiveFails = 0
	cb.stats.LastSuccess = time.Now()

	cb.window = append(cb.window, windowEntry{time: time.Now(), success: true})

	if cb.stats.State == CircuitHalfOpen {
		cb.halfOpenSuccesses++
		if cb.halfOpenSuccesses >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	}

	cb.pruneWindow()
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.stats.Failures++
	cb.stats.TotalRequests++
	cb.stats.ConsecutiveFails++
	cb.stats.LastFailure = time.Now()

	cb.window = append(cb.window, windowEntry{time: time.Now(), success: false})

	if cb.stats.State == CircuitHalfOpen {
		// Failed during half-open → back to open
		cb.transitionTo(CircuitOpen)
	} else if cb.stats.State == CircuitClosed && cb.stats.ConsecutiveFails >= cb.config.FailureThreshold {
		cb.transitionTo(CircuitOpen)
	}

	cb.pruneWindow()
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.stats.State
}

// Stats returns current provider statistics.
func (cb *CircuitBreaker) Stats() ProviderStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Calculate failure rate from window
	cb.calculateFailureRate()
	return cb.stats
}

// Reset forces the circuit to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(CircuitClosed)
	cb.stats.ConsecutiveFails = 0
	cb.halfOpenSuccesses = 0
}

func (cb *CircuitBreaker) transitionTo(state CircuitState) {
	cb.stats.State = state
	cb.stats.LastStateChange = time.Now()
	if state == CircuitHalfOpen {
		cb.halfOpenSuccesses = 0
	}
}

func (cb *CircuitBreaker) pruneWindow() {
	cutoff := time.Now().Add(-cb.config.Window)
	i := 0
	for i < len(cb.window) && cb.window[i].time.Before(cutoff) {
		i++
	}
	if i > 0 {
		cb.window = cb.window[i:]
	}
}

func (cb *CircuitBreaker) calculateFailureRate() {
	if len(cb.window) == 0 {
		cb.stats.FailureRate = 0
		return
	}
	var failures int64
	for _, e := range cb.window {
		if !e.success {
			failures++
		}
	}
	cb.stats.FailureRate = float64(failures) / float64(len(cb.window))
}

// ProviderRouter routes requests across providers with circuit breakers.
type ProviderRouter struct {
	mu        sync.RWMutex
	breakers  map[string]*CircuitBreaker
	priority  []string // provider priority order
	config    CircuitConfig
}

// NewProviderRouter creates a router with circuit breakers per provider.
func NewProviderRouter(config CircuitConfig) *ProviderRouter {
	return &ProviderRouter{
		breakers: make(map[string]*CircuitBreaker),
		priority: make([]string, 0),
		config:   config,
	}
}

// AddProvider adds a provider with the given priority position.
func (r *ProviderRouter) AddProvider(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.breakers[provider]; !exists {
		r.breakers[provider] = NewCircuitBreaker(provider, r.config)
		r.priority = append(r.priority, provider)
	}
}

// RemoveProvider removes a provider.
func (r *ProviderRouter) RemoveProvider(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.breakers, provider)
	for i, p := range r.priority {
		if p == provider {
			r.priority = append(r.priority[:i], r.priority[i+1:]...)
			break
		}
	}
}

// Next returns the next available provider, respecting circuit states.
// It tries providers in priority order, skipping open circuits.
func (r *ProviderRouter) Next() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.priority) == 0 {
		return "", fmt.Errorf("no providers configured")
	}

	// First pass: try closed circuits
	for _, p := range r.priority {
		cb := r.breakers[p]
		if cb.State() == CircuitClosed {
			return p, nil
		}
	}

	// Second pass: try half-open circuits
	for _, p := range r.priority {
		cb := r.breakers[p]
		if cb.State() == CircuitHalfOpen {
			return p, nil
		}
	}

	// Third pass: check if any open circuit has timed out
	for _, p := range r.priority {
		cb := r.breakers[p]
		if err := cb.Allow(); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("all providers have open circuits")
}

// RecordSuccess records a successful request for a provider.
func (r *ProviderRouter) RecordSuccess(provider string) {
	r.mu.RLock()
	cb, exists := r.breakers[provider]
	r.mu.RUnlock()

	if exists {
		cb.RecordSuccess()
	}
}

// RecordFailure records a failed request for a provider.
func (r *ProviderRouter) RecordFailure(provider string) {
	r.mu.RLock()
	cb, exists := r.breakers[provider]
	r.mu.RUnlock()

	if exists {
		cb.RecordFailure()
	}
}

// Stats returns statistics for all providers.
func (r *ProviderRouter) Stats() []ProviderStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make([]ProviderStats, 0, len(r.breakers))
	for _, p := range r.priority {
		cb := r.breakers[p]
		stats = append(stats, cb.Stats())
	}
	return stats
}

// SetPriority changes the provider priority order.
func (r *ProviderRouter) SetPriority(providers []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.priority = make([]string, len(providers))
	copy(r.priority, providers)
}
