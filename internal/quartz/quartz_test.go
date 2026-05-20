package quartz_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/forge/sword/internal/quartz"
)

func TestRealClock(t *testing.T) {
	clock := quartz.RealClock{}
	now := clock.Now()
	if now.IsZero() {
		t.Error("real clock should return non-zero time")
	}
}

func TestFakeClockNow(t *testing.T) {
	start := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	fc := quartz.NewFakeClock(start)

	if fc.Now() != start {
		t.Errorf("expected %v, got %v", start, fc.Now())
	}
}

func TestFakeClockAdvance(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := quartz.NewFakeClock(start)

	fc.Advance(time.Hour)
	expected := start.Add(time.Hour)
	if fc.Now() != expected {
		t.Errorf("expected %v, got %v", expected, fc.Now())
	}
}

func TestFakeClockSet(t *testing.T) {
	fc := quartz.NewFakeClock()
	target := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
	fc.Set(target)

	if fc.Now() != target {
		t.Errorf("expected %v, got %v", target, fc.Now())
	}
}

func TestFakeClockSince(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := quartz.NewFakeClock(start)

	fc.Advance(2 * time.Hour)
	since := fc.Since(start)
	if since != 2*time.Hour {
		t.Errorf("expected 2h, got %v", since)
	}
}

func TestFakeClockAfter(t *testing.T) {
	fc := quartz.NewFakeClock()
	ch := fc.After(5 * time.Minute)

	select {
	case <-ch:
		t.Error("should not fire before advance")
	default:
	}

	fc.Advance(5 * time.Minute)

	select {
	case <-ch:
		// Expected
	default:
		t.Error("should have fired after advance")
	}
}

func TestFakeClockAfterFunc(t *testing.T) {
	fc := quartz.NewFakeClock()
	var called atomic.Int32

	fc.AfterFunc(10*time.Minute, func() {
		called.Add(1)
	})

	if called.Load() != 0 {
		t.Error("should not be called yet")
	}

	fc.Advance(10 * time.Minute)

	if called.Load() != 1 {
		t.Errorf("expected 1 call, got %d", called.Load())
	}
}

func TestFakeClockSleep(t *testing.T) {
	fc := quartz.NewFakeClock()
	before := fc.Now()
	fc.Sleep(time.Hour)
	after := fc.Now()

	if after.Sub(before) != time.Hour {
		t.Errorf("expected 1h advance, got %v", after.Sub(before))
	}
}

func TestClockFunc(t *testing.T) {
	fixed := time.Date(2024, 7, 4, 12, 0, 0, 0, time.UTC)
	cf := quartz.ClockFunc(func() time.Time { return fixed })

	if cf.Now() != fixed {
		t.Errorf("expected %v, got %v", fixed, cf.Now())
	}
}
