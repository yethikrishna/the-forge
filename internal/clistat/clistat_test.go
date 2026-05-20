package clistat_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/clistat"
)

func TestCollect(t *testing.T) {
	stats := clistat.Collect()
	if stats.Goroutines <= 0 {
		t.Error("should have at least 1 goroutine")
	}
	if stats.MemoryAlloc == 0 {
		t.Error("should have some memory allocated")
	}
	if stats.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		contains string
	}{
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, tt := range tests {
		result := clistat.FormatBytes(tt.bytes)
		if result == "" {
			t.Errorf("FormatBytes(%d) returned empty", tt.bytes)
		}
	}
}

func TestTracker(t *testing.T) {
	tracker := clistat.NewTracker(10 * time.Millisecond)
	tracker.Start()
	time.Sleep(50 * time.Millisecond)
	tracker.Stop()

	snapshots := tracker.Snapshots()
	if len(snapshots) == 0 {
		t.Error("should have collected at least one snapshot")
	}

	summary := tracker.Summary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
}
