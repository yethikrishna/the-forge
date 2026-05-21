package resilience_test

import (
	"testing"

	"github.com/forge/sword/internal/resilience/circuit"
	"github.com/forge/sword/internal/resilience/ratelimit"
	"github.com/forge/sword/internal/resilience/runaway"
)

func TestCircuitBreakerCreation(t *testing.T) {
	cfg := circuit.DefaultConfig("test-provider")
	b := circuit.NewBreaker(cfg)
	if b == nil {
		t.Fatal("NewBreaker should return a breaker")
	}
	if b.State() != circuit.StateClosed {
		t.Errorf("Initial state = %q, want %q", b.State(), circuit.StateClosed)
	}
}

func TestCircuitBreakerTrip(t *testing.T) {
	cfg := circuit.DefaultConfig("test-provider")
	b := circuit.NewBreaker(cfg)

	b.Trip()
	if b.State() != circuit.StateOpen {
		t.Errorf("After Trip(), state = %q, want %q", b.State(), circuit.StateOpen)
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cfg := circuit.DefaultConfig("test-provider")
	b := circuit.NewBreaker(cfg)

	b.Trip()
	b.Reset()
	if b.State() != circuit.StateClosed {
		t.Errorf("After Reset(), state = %q, want %q", b.State(), circuit.StateClosed)
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
	for i := 0; i < 5; i++ {
		if limiter.Allow(ratelimit.Request{ScopeKey: "test"}) {
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
