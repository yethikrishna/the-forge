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
		t.Errorf("After successes, state = %q, want %q", b.State(), circuit.StateClosed)
	}
}

func TestCircuitBreakerEvents(t *testing.T) {
	cfg := circuit.DefaultConfig("event-test")
	cfg.FailureThreshold = 2
	b := circuit.NewBreaker(cfg)

	b.RecordFailure()
	b.RecordFailure()

	events := b.Events()
	if len(events) < 2 {
		t.Errorf("Events() returned %d events, want at least 2", len(events))
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cfg := circuit.DefaultConfig("stats-test")
	b := circuit.NewBreaker(cfg)

	b.RecordSuccess()
	b.RecordSuccess()
	b.RecordFailure()

	stats := b.Stats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
}

func TestRateLimiterIntegration(t *testing.T) {
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		Rate:   5,
		Burst:  5,
		Scope:  ratelimit.ScopeGlobal,
	})

	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow(ratelimit.Request{ScopeKey: "test", Timestamp: time.Now()}) {
			allowed++
		}
	}
	if allowed != 5 {
		t.Errorf("Allowed %d requests, want 5 (burst)", allowed)
	}
}

func TestRateLimiterWait(t *testing.T) {
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		Rate:   1000,
		Burst:  1,
		Scope:  ratelimit.ScopeGlobal,
	})

	limiter.Allow(ratelimit.Request{ScopeKey: "test", Timestamp: time.Now()})

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
		ContextExplosion: 100000,
	}
	detector := runaway.NewDetector(cfg)

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

	for i := 0; i < 4; i++ {
		detector.Check(runaway.IterationInfo{
			Iteration:  i + 1,
			Output:     "same output repeated",
			TokensUsed: 100,
		})
	}
	// After 3+ repeats, should detect runaway
	result := detector.Check(runaway.IterationInfo{
		Iteration:  5,
		Output:     "same output repeated",
		TokensUsed: 100,
	})
	if !result.IsRunaway {
		t.Error("Should detect runaway from repetition")
	}
}
