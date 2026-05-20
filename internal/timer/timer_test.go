package timer_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/timer"
)

func TestTimerBasic(t *testing.T) {
	tm := timer.New()
	time.Sleep(10 * time.Millisecond)
	elapsed := tm.Stop()
	if elapsed < 10*time.Millisecond {
		t.Fatalf("elapsed should be at least 10ms, got %v", elapsed)
	}
}

func TestTimerString(t *testing.T) {
	tm := timer.New()
	s := tm.String()
	if s == "" {
		t.Fatal("string should not be empty")
	}
}

func TestTimerReset(t *testing.T) {
	tm := timer.New()
	time.Sleep(10 * time.Millisecond)
	tm.Stop()
	tm.Reset()
	// After reset, should be running again
	elapsed := tm.Elapsed()
	if elapsed > time.Second {
		t.Fatalf("after reset, elapsed should be small, got %v", elapsed)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		contains string
	}{
		{100 * time.Nanosecond, "ns"},
		{10 * time.Microsecond, "µs"},
		{10 * time.Millisecond, "ms"},
		{1500 * time.Millisecond, "s"},
		{90 * time.Second, "m"},
	}
	for _, tt := range tests {
		result := timer.FormatDuration(tt.d)
		if result == "" {
			t.Errorf("FormatDuration(%v) returned empty string", tt.d)
		}
	}
}

func TestTrack(t *testing.T) {
	d := timer.Track(func() {
		time.Sleep(5 * time.Millisecond)
	})
	if d < 5*time.Millisecond {
		t.Fatalf("tracked duration should be >= 5ms, got %v", d)
	}
}

func TestTrackError(t *testing.T) {
	d, err := timer.TrackError(func() error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d < 5*time.Millisecond {
		t.Fatalf("tracked duration should be >= 5ms, got %v", d)
	}
}

func TestLapTimer(t *testing.T) {
	lt := timer.NewLapTimer()
	time.Sleep(5 * time.Millisecond)
	lt.Lap("first")
	time.Sleep(5 * time.Millisecond)
	lt.Lap("second")

	laps := lt.Laps()
	if len(laps) != 2 {
		t.Fatalf("expected 2 laps, got %d", len(laps))
	}
	if laps[0].Name != "first" || laps[1].Name != "second" {
		t.Fatalf("lap names wrong: %v", laps)
	}

	total := lt.Total()
	if total < 10*time.Millisecond {
		t.Fatalf("total should be >= 10ms, got %v", total)
	}

	s := lt.String()
	if s == "" {
		t.Fatal("string should not be empty")
	}
}
