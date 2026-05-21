package system_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/system/clistat"
	"github.com/forge/sword/internal/system/monitor"
	"github.com/forge/sword/internal/system/resource"
)

func TestClistatCollect(t *testing.T) {
	stats := clistat.Collect()
	if stats.Goroutines == 0 {
		t.Error("Collect().Goroutines should not be zero")
	}
}

func TestClistatFormatBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 B"},
		{1024, "1.0 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, tt := range tests {
		got := clistat.FormatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestClistatTracker(t *testing.T) {
	tracker := clistat.NewTracker(10 * time.Millisecond)
	tracker.Start()
	time.Sleep(30 * time.Millisecond)
	tracker.Stop()

	snaps := tracker.Snapshots()
	if len(snaps) == 0 {
		t.Error("Tracker should have snapshots after running")
	}

	summary := tracker.Summary()
	if summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestResourceMonitor(t *testing.T) {
	mon, err := resource.NewMonitor(t.TempDir())
	if err != nil {
		t.Fatalf("NewMonitor error: %v", err)
	}

	snap := mon.Snapshot()
	if snap == nil {
		t.Fatal("Snapshot should not be nil")
	}
}

func TestResourceMonitorHistory(t *testing.T) {
	mon, _ := resource.NewMonitor(t.TempDir())
	mon.Snapshot()
	mon.Snapshot()

	history := mon.GetHistory(10)
	if len(history) < 2 {
		t.Errorf("History = %d snapshots, want at least 2", len(history))
	}
}

func TestResourceMonitorCleanup(t *testing.T) {
	mon, _ := resource.NewMonitor(t.TempDir())

	cleaned := mon.Cleanup()
	// Cleanup may or may not clean anything, just verify it doesn't panic
	_ = cleaned
}

func TestResourceMonitorThresholds(t *testing.T) {
	mon, _ := resource.NewMonitor(t.TempDir())
	thresholds := resource.DefaultThresholds()
	mon.SetThresholds(thresholds)
	// Verify it doesn't panic
	mon.Snapshot()
}

func TestResourceMonitorRegisterCleaner(t *testing.T) {
	mon, _ := resource.NewMonitor(t.TempDir())
	mon.RegisterCleaner("test-cleaner", func() (int, error) {
		return 0, nil
	})
	result := mon.Cleanup()
	if count, ok := result["test-cleaner"]; ok && count != 0 {
		t.Errorf("Test cleaner count = %d, want 0", count)
	}
}

func TestMonitorSnapshot(t *testing.T) {
	m := monitor.NewMonitor(monitor.DefaultMonitorConfig())

	snap := m.Snapshot()
	_ = snap // just verify it doesn't panic
}

func TestMonitorCurrent(t *testing.T) {
	m := monitor.NewMonitor(monitor.DefaultMonitorConfig())

	current := m.Current()
	_ = current
}

func TestMonitorHistory(t *testing.T) {
	m := monitor.NewMonitor(monitor.DefaultMonitorConfig())
	m.Snapshot()
	m.Snapshot()

	history := m.History(10)
	if len(history) < 2 {
		t.Errorf("History = %d snapshots, want at least 2", len(history))
	}
}

func TestMonitorAlertHandler(t *testing.T) {
	m := monitor.NewMonitor(monitor.DefaultMonitorConfig())

	alertReceived := false
	m.OnAlert(func(alert monitor.Alert) {
		alertReceived = true
	})
	_ = alertReceived
}

func TestMonitorSetThresholds(t *testing.T) {
	m := monitor.NewMonitor(monitor.DefaultMonitorConfig())
	thresholds := monitor.DefaultThresholds()
	m.SetThresholds(thresholds)
	m.Snapshot()
}
