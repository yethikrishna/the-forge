// Package circuit provides circuit breakers for agent and API calls.
// Prevents cascading failures when downstream services are unhealthy.
//
// When an agent keeps failing, stop calling it. Let it recover.
package circuit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State string

const (
	StateClosed    State = "closed"    // Normal operation
	StateOpen      State = "open"      // Failing, reject calls
	StateHalfOpen  State = "half_open" // Testing if service recovered
)

// Event represents a circuit breaker event.
type Event struct {
	Type      string    `json:"type"` // success, failure, timeout, rejected, state_change
	Service   string    `json:"service"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
	OldState  State     `json:"old_state,omitempty"`
	NewState  State     `json:"new_state,omitempty"`
	Duration  string    `json:"duration,omitempty"`
}

// Config holds circuit breaker configuration.
type Config struct {
	Name             string        `json:"name"`
	FailureThreshold int           `json:"failure_threshold"` // failures before opening
	SuccessThreshold int           `json:"success_threshold"` // successes in half-open to close
	Timeout          time.Duration `json:"timeout"`           // how long to stay open
	HalfOpenMax      int           `json:"half_open_max"`     // max calls in half-open
}

// DefaultConfig returns sensible defaults.
func DefaultConfig(name string) Config {
	return Config{
		Name:             name,
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		HalfOpenMax:      1,
	}
}

// Breaker is a circuit breaker instance.
type Breaker struct {
	mu             sync.Mutex
	config         Config
	state          State
	failures       int
	successes      int
	halfOpenCalls  int
	lastFailure    time.Time
	lastStateChange time.Time
	events         []Event
	totalCalls     int64
	totalFailures  int64
	totalRejected  int64
}

// NewBreaker creates a circuit breaker with the given config.
func NewBreaker(config Config) *Breaker {
	return &Breaker{
		config:         config,
		state:          StateClosed,
		lastStateChange: time.Now(),
	}
}

// Execute runs a function through the circuit breaker.
func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()

	b.totalCalls++

	switch b.state {
	case StateOpen:
		if time.Since(b.lastFailure) > b.config.Timeout {
			b.setState(StateHalfOpen)
		} else {
			b.totalRejected++
			b.recordEvent(Event{Type: "rejected", Service: b.config.Name, Timestamp: time.Now()})
			b.mu.Unlock()
			return fmt.Errorf("circuit breaker %q is open", b.config.Name)
		}

	case StateHalfOpen:
		if b.halfOpenCalls >= b.config.HalfOpenMax {
			b.totalRejected++
			b.recordEvent(Event{Type: "rejected", Service: b.config.Name, Timestamp: time.Now()})
			b.mu.Unlock()
			return fmt.Errorf("circuit breaker %q is half-open and at capacity", b.config.Name)
		}
		b.halfOpenCalls++
	}

	b.mu.Unlock()

	// Execute the function
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.onFailure(err, duration)
		return err
	}

	b.onSuccess(duration)
	return nil
}

// State returns the current state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Stats returns current breaker statistics.
func (b *Breaker) Stats() map[string]interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()

	return map[string]interface{}{
		"name":           b.config.Name,
		"state":          string(b.state),
		"failures":       b.failures,
		"successes":      b.successes,
		"total_calls":    b.totalCalls,
		"total_failures": b.totalFailures,
		"total_rejected": b.totalRejected,
		"last_failure":   b.lastFailure.Format(time.RFC3339),
		"last_state_change": b.lastStateChange.Format(time.RFC3339),
	}
}

// Events returns recent events.
func (b *Breaker) Events(limit int) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	if limit <= 0 || limit > len(b.events) {
		limit = len(b.events)
	}

	result := make([]Event, limit)
	copy(result, b.events[len(b.events)-limit:])
	return result
}

// Reset manually resets the breaker to closed state.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	old := b.state
	b.state = StateClosed
	b.failures = 0
	b.successes = 0
	b.halfOpenCalls = 0
	b.lastStateChange = time.Now()
	b.recordEvent(Event{
		Type: "state_change", Service: b.config.Name,
		Timestamp: time.Now(), OldState: old, NewState: StateClosed,
	})
}

// Trip manually trips the breaker to open state.
func (b *Breaker) Trip() {
	b.mu.Lock()
	defer b.mu.Unlock()

	old := b.state
	b.state = StateOpen
	b.lastFailure = time.Now()
	b.lastStateChange = time.Now()
	b.recordEvent(Event{
		Type: "state_change", Service: b.config.Name,
		Timestamp: time.Now(), OldState: old, NewState: StateOpen,
	})
}

// Allow checks if a request is allowed through the circuit breaker.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.lastFailure) > b.config.Timeout {
			return true // transition to half-open
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful operation (for manual tracking).
func (b *Breaker) RecordSuccess() {
	b.onSuccess(0)
}

// RecordFailure records a failed operation (for manual tracking).
func (b *Breaker) RecordFailure() {
	b.onFailure(fmt.Errorf("manual failure record"), 0)
}

func (b *Breaker) onSuccess(duration time.Duration) {
	b.recordEvent(Event{
		Type: "success", Service: b.config.Name,
		Timestamp: time.Now(), Duration: duration.String(),
	})

	switch b.state {
	case StateClosed:
		b.failures = 0

	case StateHalfOpen:
		b.successes++
		if b.successes >= b.config.SuccessThreshold {
			b.setState(StateClosed)
		}
	}
}

func (b *Breaker) onFailure(err error, duration time.Duration) {
	b.totalFailures++
	b.lastFailure = time.Now()
	b.recordEvent(Event{
		Type: "failure", Service: b.config.Name,
		Timestamp: time.Now(), Error: err.Error(), Duration: duration.String(),
	})

	switch b.state {
	case StateClosed:
		b.failures++
		if b.failures >= b.config.FailureThreshold {
			b.setState(StateOpen)
		}

	case StateHalfOpen:
		b.setState(StateOpen)
	}
}

func (b *Breaker) setState(newState State) {
	old := b.state
	b.state = newState
	b.lastStateChange = time.Now()
	b.successes = 0
	b.halfOpenCalls = 0

	if newState == StateClosed {
		b.failures = 0
	}

	b.recordEvent(Event{
		Type: "state_change", Service: b.config.Name,
		Timestamp: time.Now(), OldState: old, NewState: newState,
	})
}

func (b *Breaker) recordEvent(event Event) {
	b.events = append(b.events, event)
	// Keep only last 1000 events
	if len(b.events) > 1000 {
		b.events = b.events[len(b.events)-1000:]
	}
}

// Registry manages multiple circuit breakers.
type Registry struct {
	mu       sync.Mutex
	breakers map[string]*Breaker
	dir      string // persistence directory
}

// NewRegistry creates a circuit breaker registry.
func NewRegistry(dir string) *Registry {
	return &Registry{
		breakers: make(map[string]*Breaker),
		dir:      dir,
	}
}

// GetOrCreate returns an existing breaker or creates one with the given config.
func (r *Registry) GetOrCreate(config Config) *Breaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if b, ok := r.breakers[config.Name]; ok {
		return b
	}

	b := NewBreaker(config)
	r.breakers[config.Name] = b
	return b
}

// Get returns a breaker by name.
func (r *Registry) Get(name string) (*Breaker, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.breakers[name]
	return b, ok
}

// List returns all breaker names.
func (r *Registry) List() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	names := make([]string, 0, len(r.breakers))
	for name := range r.breakers {
		names = append(names, name)
	}
	return names
}

// AllStats returns stats for all breakers.
func (r *Registry) AllStats() []map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := make([]map[string]interface{}, 0, len(r.breakers))
	for _, b := range r.breakers {
		stats = append(stats, b.Stats())
	}
	return stats
}

// Save persists breaker states to disk.
func (r *Registry) Save() error {
	if r.dir == "" {
		return nil
	}

	os.MkdirAll(r.dir, 0o755)

	r.mu.Lock()
	defer r.mu.Unlock()

	for name, b := range r.breakers {
		data, err := json.MarshalIndent(b.Stats(), "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal breaker %q: %w", name, err)
		}
		path := filepath.Join(r.dir, name+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("failed to write breaker %q: %w", name, err)
		}
	}

	return nil
}

// FormatState renders a circuit breaker state for display.
func FormatState(state State) string {
	switch state {
	case StateClosed:
		return "✅ Closed (healthy)"
	case StateOpen:
		return "🔴 Open (failing)"
	case StateHalfOpen:
		return "🟡 Half-Open (testing)"
	default:
		return string(state)
	}
}
