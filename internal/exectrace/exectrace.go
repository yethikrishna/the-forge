// Package exectrace provides eBPF-based process tracing utilities.
// Trace every process the forge spawns. (Stubs for now — real eBPF
// requires kernel support and BTF data.)
package exectrace

import (
	"context"
	"fmt"
	"time"
)

// Event represents a traced process event.
type Event struct {
	Timestamp time.Time
	PID       int
	PPID      int
	Comm      string // Process command name
	Type      EventType
	ExitCode  int
	Duration  time.Duration
}

// EventType identifies the type of process event.
type EventType string

const (
	EventExec   EventType = "exec"
	EventExit   EventType = "exit"
	EventFork   EventType = "fork"
	EventSignal EventType = "signal"
)

// Tracer traces process execution events.
type Tracer struct {
	events []Event
	running bool
}

// NewTracer creates a new process tracer.
// Note: Real eBPF tracing requires Linux 5.2+ with BTF support.
// This implementation uses /proc for compatibility.
func NewTracer() *Tracer {
	return &Tracer{
		events: make([]Event, 0),
	}
}

// Start begins tracing process events.
func (t *Tracer) Start(ctx context.Context) error {
	if t.running {
		return fmt.Errorf("exectrace: already running")
	}
	t.running = true
	return nil
}

// Stop stops tracing.
func (t *Tracer) Stop() error {
	t.running = false
	return nil
}

// Events returns all collected events.
func (t *Tracer) Events() []Event {
	return t.events
}

// Filter returns events matching the given predicate.
func (t *Tracer) Filter(fn func(Event) bool) []Event {
	var result []Event
	for _, e := range t.events {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

// ByComm returns events for a specific command name.
func (t *Tracer) ByComm(comm string) []Event {
	return t.Filter(func(e Event) bool {
		return e.Comm == comm
	})
}

// ByPID returns events for a specific PID.
func (t *Tracer) ByPID(pid int) []Event {
	return t.Filter(func(e Event) bool {
		return e.PID == pid
	})
}

// Stats returns tracing statistics.
type Stats struct {
	TotalEvents    int
	ExecEvents     int
	ExitEvents     int
	AvgDuration    time.Duration
	UniqueCommands int
}

// GetStats returns tracing statistics.
func (t *Tracer) GetStats() Stats {
	stats := Stats{TotalEvents: len(t.events)}
	comms := make(map[string]bool)
	var totalDur time.Duration
	var durCount int

	for _, e := range t.events {
		comms[e.Comm] = true
		switch e.Type {
		case EventExec:
			stats.ExecEvents++
		case EventExit:
			stats.ExitEvents++
			if e.Duration > 0 {
				totalDur += e.Duration
				durCount++
			}
		}
	}

	stats.UniqueCommands = len(comms)
	if durCount > 0 {
		stats.AvgDuration = totalDur / time.Duration(durCount)
	}

	return stats
}

// ProcessInfo holds information about a running process.
type ProcessInfo struct {
	PID     int
	Comm    string
	Cmdline string
	Status  string
}

// ListProcesses lists current processes (using /proc).
func ListProcesses() ([]ProcessInfo, error) {
	// This is a stub — real implementation would read /proc
	return nil, fmt.Errorf("exectrace: process listing not implemented (requires /proc access)")
}
