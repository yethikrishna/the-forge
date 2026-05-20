// Package timer provides command timing utilities.
// Time how long the forge takes to strike.
package timer

import (
	"fmt"
	"time"
)

// Timer tracks elapsed time for operations.
type Timer struct {
	start   time.Time
	elapsed time.Duration
	running bool
}

// New creates and starts a new Timer.
func New() *Timer {
	return &Timer{
		start:   time.Now(),
		running: true,
	}
}

// Stop stops the timer and returns elapsed duration.
func (t *Timer) Stop() time.Duration {
	if t.running {
		t.elapsed = time.Since(t.start)
		t.running = false
	}
	return t.elapsed
}

// Elapsed returns the current elapsed time.
// If the timer is still running, returns time since start.
// If stopped, returns the final duration.
func (t *Timer) Elapsed() time.Duration {
	if t.running {
		return time.Since(t.start)
	}
	return t.elapsed
}

// Reset restarts the timer.
func (t *Timer) Reset() *Timer {
	t.start = time.Now()
	t.elapsed = 0
	t.running = true
	return t
}

// String returns a human-readable elapsed time.
func (t *Timer) String() string {
	return FormatDuration(t.Elapsed())
}

// FormatDuration returns a human-readable duration string.
func FormatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1000)
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1e6)
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", d.Seconds())
	default:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
}

// Track runs a function and returns its duration.
func Track(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// TrackError runs a function and returns its duration and error.
func TrackError(fn func() error) (time.Duration, error) {
	start := time.Now()
	err := fn()
	return time.Since(start), err
}

// LapTimer tracks multiple lap times.
type LapTimer struct {
	start time.Time
	laps  []Lap
}

// Lap records a named time point.
type Lap struct {
	Name     string
	Duration time.Duration
}

// NewLapTimer creates a new lap timer.
func NewLapTimer() *LapTimer {
	return &LapTimer{
		start: time.Now(),
	}
}

// Lap records a named checkpoint.
func (lt *LapTimer) Lap(name string) Lap {
	l := Lap{
		Name:     name,
		Duration: time.Since(lt.start),
	}
	lt.laps = append(lt.laps, l)
	return l
}

// Laps returns all recorded laps.
func (lt *LapTimer) Laps() []Lap {
	return lt.laps
}

// Total returns the total elapsed time.
func (lt *LapTimer) Total() time.Duration {
	return time.Since(lt.start)
}

// String returns a formatted summary of all laps.
func (lt *LapTimer) String() string {
	if len(lt.laps) == 0 {
		return "no laps recorded"
	}
	var result string
	for i, lap := range lt.laps {
		result += fmt.Sprintf("  lap %d: %s — %s\n", i+1, lap.Name, FormatDuration(lap.Duration))
	}
	result += fmt.Sprintf("  total: %s", FormatDuration(lt.Total()))
	return result
}
