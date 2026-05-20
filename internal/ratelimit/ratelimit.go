// Package ratelimit provides token bucket rate limiting for providers, agents, and users.
// Supports per-provider, per-agent, and per-user rate limits with configurable
// burst sizes and refill rates.
//
// Throttle fast, fail gracefully.
package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// Bucket represents a token bucket for rate limiting.
type Bucket struct {
	Name       string    `json:"name"`
	Tokens     float64   `json:"tokens"`
	MaxTokens  float64   `json:"max_tokens"`
	RefillRate float64   `json:"refill_rate"` // tokens per second
	LastRefill time.Time `json:"last_refill"`
}

// NewBucket creates a token bucket with the given capacity and refill rate.
func NewBucket(name string, maxTokens, refillRate float64) *Bucket {
	return &Bucket{
		Name:       name,
		Tokens:     maxTokens,
		MaxTokens:  maxTokens,
		RefillRate: refillRate,
		LastRefill: time.Now(),
	}
}

// Allow attempts to consume n tokens. Returns true if allowed.
func (b *Bucket) Allow(n float64) bool {
	b.refill()

	if b.Tokens >= n {
		b.Tokens -= n
		return true
	}
	return false
}

// Wait blocks until n tokens are available, then consumes them.
func (b *Bucket) Wait(n float64) {
	for {
		b.refill()
		if b.Tokens >= n {
			b.Tokens -= n
			return
		}
		// Calculate wait time
		deficit := n - b.Tokens
		waitTime := time.Duration(deficit / b.RefillRate * float64(time.Second))
		if waitTime < time.Millisecond {
			waitTime = time.Millisecond
		}
		time.Sleep(waitTime)
	}
}

// Remaining returns the current number of available tokens.
func (b *Bucket) Remaining() float64 {
	b.refill()
	return b.Tokens
}

// Reset refills the bucket to maximum capacity.
func (b *Bucket) Reset() {
	b.Tokens = b.MaxTokens
	b.LastRefill = time.Now()
}

func (b *Bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.LastRefill).Seconds()
	b.Tokens += elapsed * b.RefillRate
	if b.Tokens > b.MaxTokens {
		b.Tokens = b.MaxTokens
	}
	b.LastRefill = now
}

// LimitConfig defines a rate limit configuration.
type LimitConfig struct {
	MaxTokens  float64 `json:"max_tokens"`  // burst capacity
	RefillRate float64 `json:"refill_rate"` // tokens per second
}

// Scope defines what a rate limit applies to.
type Scope string

const (
	ScopeProvider Scope = "provider"
	ScopeAgent    Scope = "agent"
	ScopeUser     Scope = "user"
	ScopeGlobal   Scope = "global"
)

// Limiter manages rate limits across multiple scopes and names.
type Limiter struct {
	mu      sync.RWMutex
	buckets map[string]*Bucket          // key: "scope:name"
	configs map[Scope]map[string]LimitConfig // default configs per scope
}

// NewLimiter creates a rate limiter.
func NewLimiter() *Limiter {
	return &Limiter{
		buckets: make(map[string]*Bucket),
		configs: make(map[Scope]map[string]LimitConfig),
	}
}

// SetDefault sets the default rate limit for a scope.
func (l *Limiter) SetDefault(scope Scope, config LimitConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.configs[scope] == nil {
		l.configs[scope] = make(map[string]LimitConfig)
	}
	l.configs[scope]["_default"] = config
}

// SetLimit sets the rate limit for a specific name within a scope.
func (l *Limiter) SetLimit(scope Scope, name string, config LimitConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.configs[scope] == nil {
		l.configs[scope] = make(map[string]LimitConfig)
	}
	l.configs[scope][name] = config

	// If bucket already exists, update it
	key := l.key(scope, name)
	if bucket, exists := l.buckets[key]; exists {
		bucket.MaxTokens = config.MaxTokens
		bucket.RefillRate = config.RefillRate
		if bucket.Tokens > config.MaxTokens {
			bucket.Tokens = config.MaxTokens
		}
	}
}

// Allow checks if a request is allowed for the given scope/name.
func (l *Limiter) Allow(scope Scope, name string, n float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.key(scope, name)
	bucket, exists := l.buckets[key]
	if !exists {
		bucket = l.createBucket(scope, name)
		l.buckets[key] = bucket
	}

	if !bucket.Allow(n) {
		return fmt.Errorf("rate limit exceeded for %s %q (%.1f tokens remaining, %.1f required)",
			scope, name, bucket.Tokens, n)
	}
	return nil
}

// Remaining returns the remaining tokens for a scope/name.
func (l *Limiter) Remaining(scope Scope, name string) float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	key := l.key(scope, name)
	bucket, exists := l.buckets[key]
	if !exists {
		return 0
	}
	return bucket.Remaining()
}

// Reset resets the bucket for a scope/name.
func (l *Limiter) Reset(scope Scope, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.key(scope, name)
	if bucket, exists := l.buckets[key]; exists {
		bucket.Reset()
	}
}

// ResetAll resets all buckets.
func (l *Limiter) ResetAll() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, bucket := range l.buckets {
		bucket.Reset()
	}
}

// Stats returns rate limit statistics for all buckets.
type BucketStats struct {
	Scope     Scope   `json:"scope"`
	Name      string  `json:"name"`
	Tokens    float64 `json:"tokens"`
	MaxTokens float64 `json:"max_tokens"`
	RefillRate float64 `json:"refill_rate"`
	PercentRemaining float64 `json:"percent_remaining"`
}

// Stats returns statistics for all buckets.
func (l *Limiter) Stats() []BucketStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := make([]BucketStats, 0, len(l.buckets))
	for key, bucket := range l.buckets {
		remaining := bucket.Remaining()
		percent := 0.0
		if bucket.MaxTokens > 0 {
			percent = (remaining / bucket.MaxTokens) * 100
		}
		scope, name := l.parseKey(key)
		stats = append(stats, BucketStats{
			Scope:     scope,
			Name:      name,
			Tokens:    remaining,
			MaxTokens: bucket.MaxTokens,
			RefillRate: bucket.RefillRate,
			PercentRemaining: percent,
		})
	}
	return stats
}

// Remove removes a rate limit bucket.
func (l *Limiter) Remove(scope Scope, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.key(scope, name)
	delete(l.buckets, key)
}

func (l *Limiter) key(scope Scope, name string) string {
	return string(scope) + ":" + name
}

func (l *Limiter) parseKey(key string) (Scope, string) {
	for _, scope := range []Scope{ScopeProvider, ScopeAgent, ScopeUser, ScopeGlobal} {
		prefix := string(scope) + ":"
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			return scope, key[len(prefix):]
		}
	}
	return ScopeGlobal, key
}

func (l *Limiter) createBucket(scope Scope, name string) *Bucket {
	config := LimitConfig{MaxTokens: 100, RefillRate: 10} // sensible default

	if scopeConfigs, ok := l.configs[scope]; ok {
		if c, ok := scopeConfigs[name]; ok {
			config = c
		} else if c, ok := scopeConfigs["_default"]; ok {
			config = c
		}
	}

	return NewBucket(l.key(scope, name), config.MaxTokens, config.RefillRate)
}

// PresetConfigs returns common rate limit configurations.
func PresetConfigs() map[string]LimitConfig {
	return map[string]LimitConfig{
		"openai-conservative": {MaxTokens: 60, RefillRate: 1},    // 60 RPM, 1/sec
		"openai-standard":     {MaxTokens: 500, RefillRate: 8.3},  // 500 RPM
		"anthropic-standard":  {MaxTokens: 1000, RefillRate: 16.6}, // 1000 RPM
		"google-standard":     {MaxTokens: 300, RefillRate: 5},    // 300 RPM
		"agent-default":       {MaxTokens: 30, RefillRate: 0.5},   // 30 RPM per agent
		"user-default":        {MaxTokens: 100, RefillRate: 2},    // 100 RPM per user
		"global-default":      {MaxTokens: 1000, RefillRate: 16.6}, // 1000 RPM total
	}
}
