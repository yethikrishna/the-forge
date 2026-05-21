package ratelimit

import (
	"testing"
	"time"
)

func TestDefaultLimits(t *testing.T) {
	limits := DefaultLimits()
	if len(limits) < 3 {
		t.Errorf("expected at least 3 default limits, got %d", len(limits))
	}
	for _, l := range limits {
		if l.ID == "" {
			t.Error("expected non-empty limit ID")
		}
		if !l.Enabled {
			t.Errorf("expected default limit %s to be enabled", l.Name)
		}
	}
}

func TestAddLimit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.AddLimit(Limit{
		ID:        "test-limit",
		Name:      "Test",
		Scope:     ScopeAgent,
		Algorithm: AlgoTokenBucket,
		Rate:      10,
		Burst:     5,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := m.GetLimit("test-limit")
	if !ok {
		t.Fatal("expected to find limit")
	}
	if got.Rate != 10 {
		t.Errorf("expected rate 10, got %f", got.Rate)
	}
}

func TestAddLimitEmptyID(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.AddLimit(Limit{Name: "No ID"})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestRemoveLimit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddLimit(Limit{ID: "test", Name: "Test", Scope: ScopeGlobal, Algorithm: AlgoTokenBucket, Rate: 10, Burst: 5, Enabled: true})
	m.RemoveLimit("test")

	_, ok := m.GetLimit("test")
	if ok {
		t.Error("expected limit to be removed")
	}
}

func TestListLimits(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Has defaults
	limits := m.ListLimits()
	if len(limits) < 3 {
		t.Errorf("expected default limits, got %d", len(limits))
	}
}

func TestListByScope(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	global := m.ListByScope(ScopeGlobal)
	if len(global) < 1 {
		t.Errorf("expected at least 1 global limit, got %d", len(global))
	}
}

func TestTokenBucket(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Add a restrictive limit
	m.AddLimit(Limit{
		ID:        "test-bucket",
		Name:      "Test Bucket",
		Scope:     ScopeGlobal,
		Algorithm: AlgoTokenBucket,
		Rate:      1, // 1 token/sec
		Burst:     3, // Max 3 tokens
		Enabled:   true,
	})

	req := Request{ScopeKey: "global", Timestamp: time.Now()}

	// Should allow first 3 requests (burst)
	for i := 0; i < 3; i++ {
		if !m.Allow(req) {
			t.Errorf("expected request %d to be allowed", i+1)
		}
		m.Record(req)
	}

	// 4th should be denied (burst exhausted)
	if m.Allow(req) {
		t.Error("expected 4th request to be denied")
	}
}

func TestFixedWindow(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddLimit(Limit{
		ID:          "test-window",
		Name:        "Test Window",
		Scope:       ScopeGlobal,
		Algorithm:   AlgoFixedWindow,
		MaxInWindow: 3,
		WindowSec:   60,
		Enabled:     true,
	})

	req := Request{ScopeKey: "global", Timestamp: time.Now()}

	for i := 0; i < 3; i++ {
		if !m.Allow(req) {
			t.Errorf("expected request %d to be allowed", i+1)
		}
		m.Record(req)
	}

	if m.Allow(req) {
		t.Error("expected 4th request to be denied in fixed window")
	}
}

func TestCheckReturnsDecisions(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddLimit(Limit{
		ID:        "check-test",
		Name:      "Check Test",
		Scope:     ScopeGlobal,
		Algorithm: AlgoTokenBucket,
		Rate:      10,
		Burst:     100,
		Enabled:   true,
	})

	req := Request{ScopeKey: "global", Timestamp: time.Now()}
	decisions := m.Check(req)

	// At least one decision (from default + custom)
	if len(decisions) == 0 {
		t.Error("expected at least one decision")
	}
}

func TestReset(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	req := Request{ScopeKey: "global", Timestamp: time.Now()}

	// Exhaust some limits
	for i := 0; i < 100; i++ {
		m.Record(req)
	}

	m.Reset()

	// Should be allowed again after reset
	if !m.Allow(req) {
		t.Error("expected requests to be allowed after reset")
	}
}

func TestDisabledLimit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddLimit(Limit{
		ID:        "disabled",
		Name:      "Disabled",
		Scope:     ScopeGlobal,
		Algorithm: AlgoTokenBucket,
		Rate:      0,
		Burst:     0,
		Enabled:   false,
	})

	req := Request{ScopeKey: "global", Timestamp: time.Now()}
	decisions := m.Check(req)

	// Disabled limits should not produce decisions
	for _, d := range decisions {
		if d.Limit.ID == "disabled" {
			t.Error("disabled limit should not produce a decision")
		}
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	stats := m.Stats()
	if stats["total_limits"] == nil {
		t.Error("expected total_limits in stats")
	}
}

func TestRetryAfter(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddLimit(Limit{
		ID:        "retry-test",
		Name:      "Retry Test",
		Scope:     ScopeGlobal,
		Algorithm: AlgoTokenBucket,
		Rate:      1,
		Burst:     1,
		Enabled:   true,
	})

	req := Request{ScopeKey: "global", Timestamp: time.Now()}

	// Use up the burst
	m.Record(req)

	// Next should be denied with retry-after
	decisions := m.Check(req)
	for _, d := range decisions {
		if d.Limit.ID == "retry-test" && !d.Allowed {
			if d.RetryAfter == "" {
				t.Error("expected retry-after for denied request")
			}
		}
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	m1.AddLimit(Limit{
		ID:        "persistent",
		Name:      "Persistent",
		Scope:     ScopeAgent,
		Algorithm: AlgoTokenBucket,
		Rate:      5,
		Burst:     10,
		Enabled:   true,
	})

	m2 := NewManager(dir)
	_, ok := m2.GetLimit("persistent")
	if !ok {
		t.Error("expected limit to persist after reload")
	}
}
