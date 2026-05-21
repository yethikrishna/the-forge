// Package clistat provides system resource monitoring utilities.
// Know the state of the forge at all times.
package clistat

import (
	"fmt"
	"runtime"
	"time"
)

// Stats holds system resource statistics.
type Stats struct {
	Timestamp   time.Time
	Goroutines  int
	MemoryAlloc uint64
	MemorySys   uint64
	MemoryGC    uint32
	CPUUsage    float64 // Approximate, requires sampling
}

// Collect gathers current system statistics.
func Collect() Stats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return Stats{
		Timestamp:   time.Now(),
		Goroutines:  runtime.NumGoroutine(),
		MemoryAlloc: m.Alloc,
		MemorySys:   m.Sys,
		MemoryGC:    m.NumGC,
	}
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Snapshot takes a snapshot at the current time.
type Snapshot struct {
	Time   time.Time
	Alloc  uint64
	Gorout int
}

// Tracker periodically collects stats.
type Tracker struct {
	interval  time.Duration
	snapshots []Snapshot
	stopCh    chan struct{}
}

// NewTracker creates a stats tracker with the given interval.
func NewTracker(interval time.Duration) *Tracker {
	return &Tracker{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins collecting stats.
func (t *Tracker) Start() {
	ticker := time.NewTicker(t.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				t.snapshots = append(t.snapshots, Snapshot{
					Time:   time.Now(),
					Alloc:  m.Alloc,
					Gorout: runtime.NumGoroutine(),
				})
			case <-t.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops collecting stats.
func (t *Tracker) Stop() {
	close(t.stopCh)
}

// Snapshots returns all collected snapshots.
func (t *Tracker) Snapshots() []Snapshot {
	return t.snapshots
}

// Summary returns a summary of collected stats.
func (t *Tracker) Summary() string {
	if len(t.snapshots) == 0 {
		return "no snapshots collected"
	}

	var maxAlloc, minAlloc uint64
	maxAlloc = 0
	minAlloc = ^uint64(0)
	var maxGorout, minGorout int
	maxGorout = 0
	minGorout = int(^uint(0) >> 1)

	for _, s := range t.snapshots {
		if s.Alloc > maxAlloc {
			maxAlloc = s.Alloc
		}
		if s.Alloc < minAlloc {
			minAlloc = s.Alloc
		}
		if s.Gorout > maxGorout {
			maxGorout = s.Gorout
		}
		if s.Gorout < minGorout {
			minGorout = s.Gorout
		}
	}

	return fmt.Sprintf("Snapshots: %d | Memory: %s-%s | Goroutines: %d-%d",
		len(t.snapshots),
		FormatBytes(minAlloc), FormatBytes(maxAlloc),
		minGorout, maxGorout)
}
