package ratelimit

import (
	"testing"
	"time"
)

func TestBucketAllow(t *testing.T) {
	b := NewBucket("test", 10, 1) // 10 tokens, 1/sec

	if !b.Allow(5) {
		t.Error("should allow 5 tokens from bucket of 10")
	}
	if !b.Allow(5) {
		t.Error("should allow 5 more tokens")
	}
	if b.Allow(1) {
		t.Error("should not allow when empty")
	}
}

func TestBucketRefill(t *testing.T) {
	b := NewBucket("test", 10, 100) // 10 tokens, 100/sec

	b.Allow(10) // drain
	if b.Allow(1) {
		t.Error("bucket should be empty")
	}

	time.Sleep(50 * time.Millisecond) // should refill ~5 tokens
	if !b.Allow(1) {
		t.Error("bucket should have refilled")
	}
}

func TestBucketMaxCapacity(t *testing.T) {
	b := NewBucket("test", 5, 1000) // 5 max, fast refill
	time.Sleep(20 * time.Millisecond)

	// Should not exceed max tokens
	if b.Remaining() > 5.1 { // small float tolerance
		t.Errorf("bucket should not exceed max capacity, got %.2f", b.Remaining())
	}
}

func TestBucketReset(t *testing.T) {
	b := NewBucket("test", 10, 1)
	b.Allow(10)
	b.Reset()

	if b.Remaining() != 10 {
		t.Errorf("expected 10 after reset, got %.2f", b.Remaining())
	}
}

func TestBucketWait(t *testing.T) {
	b := NewBucket("test", 0, 1000) // starts empty, fast refill
	start := time.Now()
	b.Wait(1)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}
}

func TestLimiterAllow(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 5, RefillRate: 1})

	for i := 0; i < 5; i++ {
		if err := l.Allow(ScopeProvider, "openai", 1); err != nil {
			t.Errorf("request %d should be allowed: %v", i+1, err)
		}
	}

	if err := l.Allow(ScopeProvider, "openai", 1); err == nil {
		t.Error("6th request should be rate limited")
	}
}

func TestLimiterDifferentScopes(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 2, RefillRate: 1})
	l.SetLimit(ScopeAgent, "reviewer", LimitConfig{MaxTokens: 2, RefillRate: 1})

	// Different scopes are independent
	if err := l.Allow(ScopeProvider, "openai", 1); err != nil {
		t.Errorf("provider request should be allowed: %v", err)
	}
	if err := l.Allow(ScopeAgent, "reviewer", 1); err != nil {
		t.Errorf("agent request should be allowed: %v", err)
	}
}

func TestLimiterDefault(t *testing.T) {
	l := NewLimiter()
	l.SetDefault(ScopeProvider, LimitConfig{MaxTokens: 3, RefillRate: 1})

	// Unknown provider should use default
	for i := 0; i < 3; i++ {
		if err := l.Allow(ScopeProvider, "unknown", 1); err != nil {
			t.Errorf("request %d should be allowed: %v", i+1, err)
		}
	}
	if err := l.Allow(ScopeProvider, "unknown", 1); err == nil {
		t.Error("should be rate limited after default capacity")
	}
}

func TestLimiterReset(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 3, RefillRate: 1})

	l.Allow(ScopeProvider, "openai", 1)
	l.Allow(ScopeProvider, "openai", 1)
	l.Allow(ScopeProvider, "openai", 1)

	l.Reset(ScopeProvider, "openai")

	if err := l.Allow(ScopeProvider, "openai", 1); err != nil {
		t.Errorf("should be allowed after reset: %v", err)
	}
}

func TestLimiterResetAll(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 1, RefillRate: 0})
	l.SetLimit(ScopeProvider, "anthropic", LimitConfig{MaxTokens: 1, RefillRate: 0})

	l.Allow(ScopeProvider, "openai", 1)
	l.Allow(ScopeProvider, "anthropic", 1)

	l.ResetAll()

	if err := l.Allow(ScopeProvider, "openai", 1); err != nil {
		t.Errorf("should be allowed after reset all: %v", err)
	}
	if err := l.Allow(ScopeProvider, "anthropic", 1); err != nil {
		t.Errorf("should be allowed after reset all: %v", err)
	}
}

func TestLimiterStats(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 100, RefillRate: 10})
	l.Allow(ScopeProvider, "openai", 50)

	stats := l.Stats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Name != "openai" {
		t.Errorf("expected openai, got %s", stats[0].Name)
	}
	if stats[0].Tokens > 55 { // 50 consumed, some refill
		t.Errorf("expected ~50 tokens remaining, got %.1f", stats[0].Tokens)
	}
}

func TestLimiterRemove(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 1, RefillRate: 0})
	l.Allow(ScopeProvider, "openai", 1)

	l.Remove(ScopeProvider, "openai")

	// Creating a new bucket for the same key
	stats := l.Stats()
	if len(stats) != 0 {
		t.Errorf("expected 0 stats after remove, got %d", len(stats))
	}
}

func TestLimiterSetLimitUpdatesExisting(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 2, RefillRate: 1})

	l.Allow(ScopeProvider, "openai", 1)

	// Update config
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 10, RefillRate: 1})

	remaining := l.Remaining(ScopeProvider, "openai")
	if remaining > 10 {
		t.Errorf("remaining should not exceed new max, got %.1f", remaining)
	}
}

func TestPresetConfigs(t *testing.T) {
	presets := PresetConfigs()
	if len(presets) < 5 {
		t.Errorf("expected at least 5 presets, got %d", len(presets))
	}

	if _, ok := presets["openai-standard"]; !ok {
		t.Error("expected openai-standard preset")
	}
}

func TestBucketZeroRefill(t *testing.T) {
	b := NewBucket("test", 5, 0) // no refill

	for i := 0; i < 5; i++ {
		if !b.Allow(1) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
	if b.Allow(1) {
		t.Error("should not allow when empty with zero refill")
	}

	time.Sleep(20 * time.Millisecond)
	if b.Allow(1) {
		t.Error("still should not allow with zero refill")
	}
}

func TestLimiterFractionalTokens(t *testing.T) {
	l := NewLimiter()
	l.SetLimit(ScopeProvider, "openai", LimitConfig{MaxTokens: 1, RefillRate: 1})

	if err := l.Allow(ScopeProvider, "openai", 0.5); err != nil {
		t.Errorf("0.5 tokens should be allowed: %v", err)
	}
	if err := l.Allow(ScopeProvider, "openai", 0.5); err != nil {
		t.Errorf("0.5 more tokens should be allowed: %v", err)
	}
	if err := l.Allow(ScopeProvider, "openai", 0.1); err == nil {
		t.Error("should be rate limited after using 1.0 tokens total")
	}
}
