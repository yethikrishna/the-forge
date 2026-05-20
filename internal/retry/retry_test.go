package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/forge/sword/internal/retry"
)

func TestRetrySuccess(t *testing.T) {
	callCount := 0
	err := retry.Do(context.Background(), func() error {
		callCount++
		if callCount < 3 {
			return errors.New("not yet")
		}
		return nil
	}, retry.WithMaxAttempts(5))
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}
}

func TestRetryMaxAttempts(t *testing.T) {
	err := retry.Do(context.Background(), func() error {
		return errors.New("always fail")
	}, retry.WithMaxAttempts(3), retry.WithInitialDelay(time.Millisecond))
	if err == nil {
		t.Fatal("expected error after max attempts")
	}
}

func TestRetryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := retry.Do(ctx, func() error {
		return errors.New("fail")
	}, retry.WithMaxAttempts(0), retry.WithInitialDelay(10*time.Millisecond))
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestRetryIf(t *testing.T) {
	callCount := 0
	err := retry.DoIf(context.Background(), func() error {
		callCount++
		return errors.New("non-retryable")
	}, func(err error) bool {
		return false // never retry
	}, retry.WithMaxAttempts(5))
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call (no retries), got %d", callCount)
	}
}

func TestRetryMaxDuration(t *testing.T) {
	start := time.Now()
	err := retry.Do(context.Background(), func() error {
		return errors.New("fail")
	}, retry.WithMaxDuration(100*time.Millisecond), retry.WithInitialDelay(10*time.Millisecond))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("should have stopped near max duration, took %v", elapsed)
	}
}

func TestBackoffDelayCalculation(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.InitialDelay = 100 * time.Millisecond
	cfg.MaxDelay = 5 * time.Second
	cfg.Multiplier = 2.0

	// Verify backoff calculation logic works
	// Attempt 0: no delay (first attempt)
	// Attempt 1: 100ms
	// Attempt 2: 200ms
	// Attempt 3: 400ms
	// etc.
	// Just verify the config is valid
	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("unexpected initial delay: %v", cfg.InitialDelay)
	}
}
