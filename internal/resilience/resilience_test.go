package resilience_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/resilience/circuit"
	"github.com/forge/sword/internal/resilience/ratelimit"
	"github.com/forge/sword/internal/resilience/runaway"
)

func TestCircuitBreakerIntegration(t *testing.T) {
	cfg := circuit.DefaultConfig("test-provider")
	cfg.FailureThreshold = 3
	cfg.SuccessThreshold = 2
	cfg.Timeout = 100 * time.Millisecond

	b := circuit.NewBreaker(cfg)

	// Should start closed
	if b.State() != circuit.StateClosed {
		t.Errorf("Initial state = %q, want %q", b.State(), circuit.StateClosed)
	}

	// Record failures to trip the breaker
	for i := 0; i < cfg.FailureThreshold; i++ {
		b.Trip()
	}
	if b.State() != circuit.StateOpen {
		t.Errorf("After %d failures, state = %q, want %q", cfg.FailureThreshold, b.State(), circuit.StateOpen)
	}

	// Should reject calls when open
	if b.Allow() {
		t.Error("Allow() should return false when circuit is open")
	}

	// Wait for timeout to transition to half-open
	time.Sleep(cfg.Timeout + 10*time.Millisecond)

	// Now should be in half-open state and allow one call
	if !b.Allow() {
		t.Error("Allow() should return true in half-open state after timeout")
	}

	// Record successes to close the circuit
	for i := 0; i < cfg.SuccessThreshold; i++ {
		b.RecordSuccess()
	}
	if b.State() != circuit.StateClosed {
		t.Errorf("After successes, state = %q, want %q", b.State(), circuit.StateClosed)
	}
}

func TestCircuitBreakerEvents(t *testing.T) {
	cfg := circuit.DefaultConfig("event-test")
	cfg.FailureThreshold = 2
	b := circuit.NewBreaker(cfg)

	b.Trip()
	b.Trip()

	events := b.Events(10)
	if len(events) < 2 {
		t.Errorf("Events() returned %d events, want at least 2", len(events))
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cfg := circuit.DefaultConfig("stats-test")
	b := circuit.NewBreaker(cfg)

	b.RecordSuccess()
	b.RecordSuccess()
	b.Trip()

	stats := b.Stats()
	if stats["total_calls"].(int) < 3 {
		t.Errorf("Stats.TotalCalls = %v, want >= 3", stats["total_calls"])
	}
}

func TestRateLimiterIntegration(t *testing.T) {
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		Rate:   5,
		Burst:  5,
		Scope:  ratelimit.ScopeGlobal,
	})

	// Should allow up to burst
	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow() {
			allowed++
		}
	}
	if allowed != 5 {
		t.Errorf("Allowed %d requests, want 5 (burst)", allowed)
	}
}

func TestRateLimiterWait(t *testing.T) {
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		Rate:   1000, // high rate for fast test
		Burst:  1,
		Scope:  ratelimit.ScopeGlobal,
	})

	// Exhaust burst
	limiter.Allow()

	// Wait should succeed within reasonable time
	err := limiter.Wait(t.Context(), 500*time.Millisecond)
	if err != nil {
		t.Errorf("Wait error: %v", err)
	}
}

func TestRunawayDetector(t *testing.T) {
	cfg := runaway.Config{
		MaxIterations:    5,
		MaxDuration:      10 * time.Second,
		RepeatThreshold:  3,
		ContextExplosion: 100000, // tokens
	}
	detector := runaway.NewDetector(cfg)

	// Simulate normal operation - should not detect runaway
	for i := 0; i < 4; i++ {
		result := detector.Check(runaway.IterationInfo{
			Iteration:  i + 1,
			Output:     "different output each time",
			TokensUsed: 100,
		})
		if result.IsRunaway {
			t.Errorf("Iteration %d: should not detect runaway", i+1)
		}
	}

	// Exceed iteration limit
	result := detector.Check(runaway.IterationInfo{
		Iteration:  6,
		Output:     "output",
		TokensUsed: 100,
	})
	if !result.IsRunaway {
		t.Error("Should detect runaway after exceeding max iterations")
	}
}

func TestRunawayDetectorRepetition(t *testing.T) {
	cfg := runaway.Config{
		MaxIterations:    100,
		MaxDuration:      10 * time.Second,
		RepeatThreshold:  3,
		ContextExplosion: 100000,
	}
	detector := runaway.NewDetector(cfg)

	// Send same output multiple times
	for i := 0; i < 4; i++ {
		result := detector.Check(runaway.IterationInfo{
			Iteration:  i + 1,
			Output:     "same output repeated",
			TokensUsed: 100,
		})
		if i >= 2 && !result.IsRunaway {
			t.Errorf("Iteration %d: should detect runaway from repetition", i+1)
		}
	}
}
