package exectrace_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/exectrace"
)

func TestNewTracer(t *testing.T) {
	tracer := exectrace.NewTracer()
	if tracer == nil {
		t.Fatal("tracer should not be nil")
	}
}

func TestTracerStartStop(t *testing.T) {
	tracer := exectrace.NewTracer()
	if err := tracer.Start(nil); err != nil {
		t.Fatalf("start error: %v", err)
	}
	if err := tracer.Start(nil); err == nil {
		t.Error("should error on double start")
	}
	if err := tracer.Stop(); err != nil {
		t.Fatalf("stop error: %v", err)
	}
}

func TestTracerEvents(t *testing.T) {
	tracer := exectrace.NewTracer()
	events := tracer.Events()
	if len(events) != 0 {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestTracerFilter(t *testing.T) {
	tracer := exectrace.NewTracer()
	filtered := tracer.Filter(func(e exectrace.Event) bool {
		return e.Comm == "test"
	})
	if len(filtered) != 0 {
		t.Errorf("expected no filtered events, got %d", len(filtered))
	}
}

func TestTracerStats(t *testing.T) {
	tracer := exectrace.NewTracer()
	stats := tracer.GetStats()
	if stats.TotalEvents != 0 {
		t.Errorf("expected 0 total events, got %d", stats.TotalEvents)
	}
}

func TestEventType(t *testing.T) {
	types := []exectrace.EventType{
		exectrace.EventExec,
		exectrace.EventExit,
		exectrace.EventFork,
		exectrace.EventSignal,
	}
	for _, et := range types {
		if et == "" {
			t.Error("event type should not be empty")
		}
	}
}
