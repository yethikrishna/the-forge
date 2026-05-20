package resilience

import (
	"testing"
	"time"
)

func TestCircuitBreakerClosedToOpen(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	if cb.State() != CircuitClosed {
		t.Error("expected initial state to be closed")
	}

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.State() != CircuitOpen {
		t.Error("expected state to be open after threshold failures")
	}
}

func TestCircuitBreakerRejectsWhenOpen(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 2,
		Timeout:          100 * time.Millisecond,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	cb.RecordFailure()
	cb.RecordFailure()

	err := cb.Allow()
	if err == nil {
		t.Error("expected rejection when circuit is open")
	}
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	err := cb.Allow()
	if err != nil {
		t.Error("expected allow after timeout (half-open)")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenToClosed(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	// Trip the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	// Record enough successes
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after success threshold, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenToOpen(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	// Trip the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	// Failure during half-open → back to open
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("expected open after failure in half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerConsecutiveFailsReset(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 5,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}

	stats := cb.Stats()
	if stats.ConsecutiveFails != 4 {
		t.Errorf("expected 4 consecutive fails, got %d", stats.ConsecutiveFails)
	}

	// Success resets
	cb.RecordSuccess()
	stats = cb.Stats()
	if stats.ConsecutiveFails != 0 {
		t.Errorf("expected 0 consecutive fails after success, got %d", stats.ConsecutiveFails)
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 10,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordFailure()

	stats := cb.Stats()
	if stats.Successes != 2 {
		t.Errorf("expected 2 successes, got %d", stats.Successes)
	}
	if stats.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", stats.Failures)
	}
	if stats.TotalRequests != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalRequests)
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Error("expected closed after reset")
	}
}

func TestProviderRouterNext(t *testing.T) {
	config := DefaultCircuitConfig()
	router := NewProviderRouter(config)
	router.AddProvider("openai")
	router.AddProvider("anthropic")
	router.AddProvider("google")

	provider, err := router.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if provider != "openai" {
		t.Errorf("expected first provider, got %s", provider)
	}
}

func TestProviderRouterFallbackOnCircuitOpen(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          5 * time.Minute, // long timeout so it stays open
		Window:           time.Minute,
	}
	router := NewProviderRouter(config)
	router.AddProvider("openai")
	router.AddProvider("anthropic")

	// Trip openai's circuit
	router.RecordFailure("openai")

	provider, err := router.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if provider != "anthropic" {
		t.Errorf("expected fallback to anthropic, got %s", provider)
	}
}

func TestProviderRouterAllOpen(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 1,
		Timeout:          5 * time.Minute,
		Window:           time.Minute,
	}
	router := NewProviderRouter(config)
	router.AddProvider("openai")
	router.AddProvider("anthropic")

	router.RecordFailure("openai")
	router.RecordFailure("anthropic")

	_, err := router.Next()
	if err == nil {
		t.Error("expected error when all providers have open circuits")
	}
}

func TestProviderRouterNoProviders(t *testing.T) {
	config := DefaultCircuitConfig()
	router := NewProviderRouter(config)

	_, err := router.Next()
	if err == nil {
		t.Error("expected error with no providers")
	}
}

func TestProviderRouterRecordSuccess(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		Window:           time.Minute,
	}
	router := NewProviderRouter(config)
	router.AddProvider("openai")

	router.RecordFailure("openai")
	router.RecordFailure("openai")

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Success should close the circuit
	router.RecordSuccess("openai")

	provider, err := router.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if provider != "openai" {
		t.Errorf("expected openai to be available after recovery, got %s", provider)
	}
}

func TestProviderRouterStats(t *testing.T) {
	config := DefaultCircuitConfig()
	router := NewProviderRouter(config)
	router.AddProvider("openai")
	router.AddProvider("anthropic")

	router.RecordSuccess("openai")
	router.RecordFailure("openai")
	router.RecordSuccess("anthropic")

	stats := router.Stats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 provider stats, got %d", len(stats))
	}
	if stats[0].Provider != "openai" {
		t.Errorf("expected first stat to be openai, got %s", stats[0].Provider)
	}
}

func TestProviderRouterSetPriority(t *testing.T) {
	config := DefaultCircuitConfig()
	router := NewProviderRouter(config)
	router.AddProvider("openai")
	router.AddProvider("anthropic")

	router.SetPriority([]string{"anthropic", "openai"})

	provider, _ := router.Next()
	if provider != "anthropic" {
		t.Errorf("expected anthropic after priority change, got %s", provider)
	}
}

func TestProviderRouterRemoveProvider(t *testing.T) {
	config := DefaultCircuitConfig()
	router := NewProviderRouter(config)
	router.AddProvider("openai")
	router.AddProvider("anthropic")

	router.RemoveProvider("openai")

	stats := router.Stats()
	if len(stats) != 1 {
		t.Errorf("expected 1 provider after remove, got %d", len(stats))
	}
}

func TestCircuitBreakerFailureRate(t *testing.T) {
	config := CircuitConfig{
		FailureThreshold: 100,
		Window:           time.Minute,
	}
	cb := NewCircuitBreaker("test-provider", config)

	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordFailure()

	stats := cb.Stats()
	if stats.FailureRate < 0.3 || stats.FailureRate > 0.4 {
		t.Errorf("expected ~0.33 failure rate, got %.2f", stats.FailureRate)
	}
}
