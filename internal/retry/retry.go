// Package retry provides configurable retry logic with exponential backoff
// and jitter. Every good sword strikes more than once.
package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Config controls retry behavior.
type Config struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	// 0 means unlimited retries (use with MaxDuration).
	MaxAttempts int

	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay caps the delay between retries.
	MaxDelay time.Duration

	// Multiplier controls backoff growth. Default 2.0.
	Multiplier float64

	// Jitter adds randomness to prevent thundering herd.
	// 0 = no jitter, 1 = full jitter. Default 0.
	Jitter float64

	// MaxDuration caps total elapsed time across all attempts.
	// 0 means no duration limit.
	MaxDuration time.Duration
}

// DefaultConfig returns sensible defaults: 3 attempts, 100ms initial,
// 10s max, 2x multiplier, no jitter.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0,
		MaxDuration:  0,
	}
}

// Option modifies a Config.
type Option func(*Config)

// WithMaxAttempts sets the max number of attempts.
func WithMaxAttempts(n int) Option {
	return func(c *Config) { c.MaxAttempts = n }
}

// WithInitialDelay sets the initial backoff delay.
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) { c.InitialDelay = d }
}

// WithMaxDelay sets the maximum backoff delay.
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) { c.MaxDelay = d }
}

// WithMultiplier sets the backoff multiplier.
func WithMultiplier(m float64) Option {
	return func(c *Config) { c.Multiplier = m }
}

// WithJitter enables jitter (0-1 range).
func WithJitter(j float64) Option {
	return func(c *Config) { c.Jitter = j }
}

// WithMaxDuration sets the total elapsed time limit.
func WithMaxDuration(d time.Duration) Option {
	return func(c *Config) { c.MaxDuration = d }
}

// RetryableFunc is a function that can be retried.
// Return nil on success, or an error to retry.
type RetryableFunc func() error

// IsRetryable determines if an error warrants a retry.
type IsRetryable func(error) bool

// AlwaysRetry considers all errors retryable.
func AlwaysRetry(err error) bool { return true }

// NeverRetry considers no errors retryable.
func NeverRetry(err error) bool { return false }

// Do executes the function with retries according to the config.
// Returns the last error if all attempts fail.
func Do(ctx context.Context, fn RetryableFunc, opts ...Option) error {
	cfg := DefaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	return doRetry(ctx, fn, cfg, AlwaysRetry)
}

// DoIf retries only when the predicate returns true for the error.
func DoIf(ctx context.Context, fn RetryableFunc, predicate IsRetryable, opts ...Option) error {
	cfg := DefaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	return doRetry(ctx, fn, cfg, predicate)
}

func doRetry(ctx context.Context, fn RetryableFunc, cfg Config, predicate IsRetryable) error {
	var lastErr error
	start := time.Now()

	for attempt := 0; ; attempt++ {
		// Check max attempts
		if cfg.MaxAttempts > 0 && attempt >= cfg.MaxAttempts {
			if lastErr != nil {
				return fmt.Errorf("retry: max attempts (%d) reached: %w", cfg.MaxAttempts, lastErr)
			}
			return fmt.Errorf("retry: max attempts (%d) reached", cfg.MaxAttempts)
		}

		// Check max duration
		if cfg.MaxDuration > 0 && time.Since(start) >= cfg.MaxDuration {
			if lastErr != nil {
				return fmt.Errorf("retry: max duration (%s) exceeded: %w", cfg.MaxDuration, lastErr)
			}
			return fmt.Errorf("retry: max duration (%s) exceeded", cfg.MaxDuration)
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if retryable
		if !predicate(err) {
			return fmt.Errorf("retry: non-retryable error: %w", err)
		}

		// Calculate delay
		delay := backoffDelay(attempt, cfg)

		// Wait or cancel
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry: context cancelled: %w", ctx.Err())
		case <-time.After(delay):
		}
	}
}

// backoffDelay calculates the delay for the given attempt number.
func backoffDelay(attempt int, cfg Config) time.Duration {
	if attempt == 0 {
		return 0
	}

	// Exponential backoff
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))

	// Cap at max delay
	if cfg.MaxDelay > 0 && delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Apply jitter
	if cfg.Jitter > 0 {
		jitterRange := delay * cfg.Jitter
		delay = delay - jitterRange/2 + rand.Float64()*jitterRange
	}

	d := time.Duration(delay)
	if d < 0 {
		return 0
	}
	return d
}
