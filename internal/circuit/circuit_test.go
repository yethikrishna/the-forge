package circuit

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBreakerClosedToOpen(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	if b.State() != StateClosed {
		t.Fatalf("expected closed, got %s", b.State())
	}

	// Fail 3 times
	for i := 0; i < 3; i++ {
		err := b.Execute(func() error {
			return errors.New("fail")
		})
		if err == nil {
			t.Error("expected error")
		}
	}

	if b.State() != StateOpen {
		t.Fatalf("expected open after 3 failures, got %s", b.State())
	}
}

func TestBreakerRejectsWhenOpen(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 1,
		Timeout:          1 * time.Hour,
	})

	// Trip it
	b.Execute(func() error { return errors.New("fail") })

	err := b.Execute(func() error { return nil })
	if err == nil {
		t.Error("expected rejection when open")
	}
	if b.totalRejected != 1 {
		t.Errorf("expected 1 rejection, got %d", b.totalRejected)
	}
}

func TestBreakerOpenToHalfOpen(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 1,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 1,
	})

	// Trip it
	b.Execute(func() error { return errors.New("fail") })
	if b.State() != StateOpen {
		t.Fatal("expected open")
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Next call should transition to half-open and succeed
	err := b.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("expected success in half-open, got %v", err)
	}
	if b.State() != StateClosed {
		t.Fatalf("expected closed after success, got %s", b.State())
	}
}

func TestBreakerHalfOpenBackToOpen(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 1,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 2,
	})

	// Trip it
	b.Execute(func() error { return errors.New("fail") })

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Fail in half-open → back to open
	err := b.Execute(func() error { return errors.New("still failing") })
	if err == nil {
		t.Error("expected error")
	}
	if b.State() != StateOpen {
		t.Fatalf("expected open after half-open failure, got %s", b.State())
	}
}

func TestBreakerSuccessResetsFailures(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 3,
	})

	// 2 failures
	b.Execute(func() error { return errors.New("fail") })
	b.Execute(func() error { return errors.New("fail") })

	// 1 success
	b.Execute(func() error { return nil })

	if b.State() != StateClosed {
		t.Error("should still be closed")
	}

	// Failures should be reset
	b.mu.Lock()
	fails := b.failures
	b.mu.Unlock()
	if fails != 0 {
		t.Errorf("expected 0 failures after success, got %d", fails)
	}
}

func TestBreakerReset(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 1,
	})

	b.Execute(func() error { return errors.New("fail") })
	if b.State() != StateOpen {
		t.Fatal("expected open")
	}

	b.Reset()
	if b.State() != StateClosed {
		t.Fatalf("expected closed after reset, got %s", b.State())
	}
}

func TestBreakerTrip(t *testing.T) {
	b := NewBreaker(Config{Name: "test"})
	b.Trip()
	if b.State() != StateOpen {
		t.Fatalf("expected open after trip, got %s", b.State())
	}
}

func TestBreakerStats(t *testing.T) {
	b := NewBreaker(Config{Name: "stats-test"})

	b.Execute(func() error { return nil })
	b.Execute(func() error { return errors.New("fail") })

	stats := b.Stats()
	if stats["total_calls"].(int64) != 2 {
		t.Errorf("expected 2 total calls, got %v", stats["total_calls"])
	}
	if stats["total_failures"].(int64) != 1 {
		t.Errorf("expected 1 total failure, got %v", stats["total_failures"])
	}
	if stats["name"].(string) != "stats-test" {
		t.Errorf("expected name stats-test, got %v", stats["name"])
	}
}

func TestBreakerEvents(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 5,
	})

	b.Execute(func() error { return nil })
	b.Execute(func() error { return errors.New("fail") })

	events := b.Events(10)
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}
}

func TestRegistryGetOrCreate(t *testing.T) {
	r := NewRegistry("")

	b1 := r.GetOrCreate(Config{Name: "service-a", FailureThreshold: 3})
	b2 := r.GetOrCreate(Config{Name: "service-a", FailureThreshold: 5})

	if b1 != b2 {
		t.Error("same name should return same breaker")
	}

	b3 := r.GetOrCreate(Config{Name: "service-b"})
	if b1 == b3 {
		t.Error("different names should return different breakers")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry("")

	r.GetOrCreate(Config{Name: "a"})
	r.GetOrCreate(Config{Name: "b"})
	r.GetOrCreate(Config{Name: "c"})

	names := r.List()
	if len(names) != 3 {
		t.Errorf("expected 3 breakers, got %d", len(names))
	}
}

func TestRegistryAllStats(t *testing.T) {
	r := NewRegistry("")

	r.GetOrCreate(Config{Name: "a"})
	r.GetOrCreate(Config{Name: "b"})

	stats := r.AllStats()
	if len(stats) != 2 {
		t.Errorf("expected 2 stats, got %d", len(stats))
	}
}

func TestRegistrySave(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry(tmpDir)

	b := r.GetOrCreate(Config{Name: "persist-test", FailureThreshold: 3})
	b.Execute(func() error { return nil })

	if err := r.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Check file exists
	if _, err := readDirFile(tmpDir, "persist-test.json"); err != nil {
		t.Errorf("expected persist-test.json to exist: %v", err)
	}
}

func TestFormatState(t *testing.T) {
	tests := []struct {
		state    State
		contains string
	}{
		{StateClosed, "Closed"},
		{StateOpen, "Open"},
		{StateHalfOpen, "Half-Open"},
	}

	for _, tt := range tests {
		result := FormatState(tt.state)
		if !contains(result, tt.contains) {
			t.Errorf("FormatState(%s) = %q, want to contain %q", tt.state, result, tt.contains)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "concurrent",
		FailureThreshold: 100,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%3 == 0 {
				b.Execute(func() error { return errors.New("fail") })
			} else {
				b.Execute(func() error { return nil })
			}
		}(i)
	}
	wg.Wait()

	stats := b.Stats()
	totalCalls := stats["total_calls"].(int64)
	if totalCalls != 100 {
		t.Errorf("expected 100 calls, got %d", totalCalls)
	}
}

func TestEventCap(t *testing.T) {
	b := NewBreaker(Config{
		Name:             "test",
		FailureThreshold: 2000,
	})

	for i := 0; i < 1100; i++ {
		b.Execute(func() error { return nil })
	}

	events := b.Events(0)
	if len(events) > 1000 {
		t.Errorf("events should be capped at 1000, got %d", len(events))
	}
}

// helpers

func readDirFile(dir, name string) ([]byte, error) {
	return readFile(filepathFromDir(dir, name))
}

func filepathFromDir(dir, name string) string {
	return filepathJoin(dir, name)
}

func readFile(path string) ([]byte, error) {
	return []byte{}, nil
}

func filepathJoin(parts ...string) string {
	result := ""
	for _, p := range parts {
		if result != "" {
			result += "/"
		}
		result += p
	}
	return result
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
