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

	if b.State() != circuit.StateClosed {
		t.Errorf("Initial state = %q, want %q", b.State(), circuit.StateClosed)
	}

	for i := 0; i < cfg.FailureThreshold; i++ {
		b.RecordFailure()
	}
	if b.State() != circuit.StateOpen {
		t.Errorf("After %d failures, state = %q, want %q", cfg.FailureThreshold, b.State(), circuit.StateOpen)
	}

	if b.Allow() {
		t.Error("Allow() should return false when circuit is open")
	}

	time.Sleep(cfg.Timeout + 10*time.Millisecond)

	if !b.Allow() {
		t.Error("Allow() should return true in half-open state after timeout")
	}

	for i := 0; i < cfg.SuccessThreshold; i++ {
		b.RecordSuccess()
	}
	if b.State() != circuit.StateClosed {
		t.Errorf("After %d successes, state = %q, want %q", cfg.SuccessThreshold, b.State(), circuit.StateClosed)
	}
}

func TestCircuitBreakerEvents(t *testing.T) {
	cfg := circuit.DefaultConfig("test-provider")
	b := circuit.NewBreaker(cfg)

	b.RecordSuccess()
	b.RecordFailure()

	events := b.Events(10)
	if len(events) < 2 {
		t.Errorf("Expected at least 2 events, got %d", len(events))
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cfg := circuit.DefaultConfig("test-provider")
	b := circuit.NewBreaker(cfg)

	b.RecordSuccess()
	b.RecordSuccess()

	stats := b.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}
}

func TestRateLimiterIntegration(t *testing.T) {
	limiter := ratelimit.NewManager(t.TempDir())

	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow(ratelimit.Request{ScopeKey: "test", Timestamp: time.Now()}) {
			allowed++
		}
	}
	if allowed == 0 {
		t.Error("Should allow at least some requests")
	}
}

func TestRunawayDetector(t *testing.T) {
	cfg := runaway.DefaultConfig()
	detector := runaway.NewDetector(cfg)
	if detector == nil {
		t.Fatal("NewDetector should return a detector")
	}

	issues := detector.Check("test-agent")
	_ = issues
}
