// Package quartz provides deterministic time/clock mocking for tests.
// Control time itself — even a forge master needs that power.
package quartz

import (
	"sync"
	"time"
)

// Clock provides the current time. Use RealClock in production
// and FakeClock in tests.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
	Since(t time.Time) time.Duration
	Until(t time.Time) time.Duration
	After(d time.Duration) <-chan time.Time
	AfterFunc(d time.Duration, f func()) *Timer
}

// Timer wraps time.Timer for clock abstraction.
type Timer struct {
	C <-chan time.Time
}

// RealClock uses the system clock.
type RealClock struct{}

// Now returns the current real time.
func (RealClock) Now() time.Time { return time.Now() }

// Sleep sleeps for the given duration.
func (RealClock) Sleep(d time.Duration) { time.Sleep(d) }

// Since returns time since the given time.
func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }

// Until returns duration until the given time.
func (RealClock) Until(t time.Time) time.Duration { return time.Until(t) }

// After returns a channel that fires after the duration.
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// AfterFunc runs f after the duration.
func (RealClock) AfterFunc(d time.Duration, f func()) *Timer {
	t := time.AfterFunc(d, f)
	return &Timer{C: t.C}
}

// FakeClock provides a controllable clock for testing.
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

// NewFakeClock creates a FakeClock starting at the given time.
// If no time is provided, starts at the zero time.
func NewFakeClock(now ...time.Time) *FakeClock {
	var t time.Time
	if len(now) > 0 {
		t = now[0]
	} else {
		t = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return &FakeClock{now: t}
}

// Now returns the fake current time.
func (fc *FakeClock) Now() time.Time {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.now
}

// Sleep is a no-op for FakeClock (use Advance in tests).
func (fc *FakeClock) Sleep(d time.Duration) {
	fc.Advance(d)
}

// Since returns the duration since the given time.
func (fc *FakeClock) Since(t time.Time) time.Duration {
	return fc.Now().Sub(t)
}

// Until returns the duration until the given time.
func (fc *FakeClock) Until(t time.Time) time.Duration {
	return t.Sub(fc.Now())
}

// After returns a channel that fires when the clock advances enough.
func (fc *FakeClock) After(d time.Duration) <-chan time.Time {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	ch := make(chan time.Time, 1)
	fireAt := fc.now.Add(d)
	fc.timers = append(fc.timers, &fakeTimer{
		fireAt: fireAt,
		ch:     ch,
	})
	return ch
}

// AfterFunc schedules f to run when the clock advances enough.
func (fc *FakeClock) AfterFunc(d time.Duration, f func()) *Timer {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fireAt := fc.now.Add(d)
	ch := make(chan time.Time, 1)
	fc.timers = append(fc.timers, &fakeTimer{
		fireAt: fireAt,
		ch:     ch,
		fn:     f,
	})
	return &Timer{C: ch}
}

// Advance moves the fake clock forward by the given duration.
// Fires any timers whose time has come.
func (fc *FakeClock) Advance(d time.Duration) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.now = fc.now.Add(d)
	fc.fireTimers()
}

// Set sets the fake clock to a specific time.
func (fc *FakeClock) Set(t time.Time) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.now = t
	fc.fireTimers()
}

// fireTimers fires any timers that are due.
func (fc *FakeClock) fireTimers() {
	var remaining []*fakeTimer
	for _, timer := range fc.timers {
		if !fc.now.Before(timer.fireAt) {
			timer.ch <- fc.now
			if timer.fn != nil {
				timer.fn()
			}
		} else {
			remaining = append(remaining, timer)
		}
	}
	fc.timers = remaining
}

type fakeTimer struct {
	fireAt time.Time
	ch     chan time.Time
	fn     func()
}

// ClockFunc is a Clock implementation backed by a function.
type ClockFunc func() time.Time

// Now returns the time from the function.
func (cf ClockFunc) Now() time.Time { return cf() }

// Sleep sleeps for the given duration.
func (cf ClockFunc) Sleep(d time.Duration) { time.Sleep(d) }

// Since returns time since t using the function's clock.
func (cf ClockFunc) Since(t time.Time) time.Duration { return cf().Sub(t) }

// Until returns duration until t using the function's clock.
func (cf ClockFunc) Until(t time.Time) time.Duration { return t.Sub(cf()) }

// After returns a channel that fires after d.
func (cf ClockFunc) After(d time.Duration) <-chan time.Time { return time.After(d) }

// AfterFunc runs f after d.
func (cf ClockFunc) AfterFunc(d time.Duration, f func()) *Timer {
	t := time.AfterFunc(d, f)
	return &Timer{C: t.C}
}
