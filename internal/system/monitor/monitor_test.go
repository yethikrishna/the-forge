package monitor

import (
	"testing"
	"time"
)

func TestNewMonitor(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	if m == nil {
		t.Fatal("expected non-nil monitor")
	}
}

func TestSnapshot(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	snap := m.Snapshot()

	if snap.Goroutines <= 0 {
		t.Error("expected positive goroutine count")
	}
	if snap.HeapAllocMB <= 0 {
		t.Error("expected positive heap allocation")
	}
	if snap.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestHistory(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	m.Snapshot()
	m.Snapshot()
	m.Snapshot()

	history := m.History(2)
	if len(history) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(history))
	}

	all := m.History(0)
	if len(all) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(all))
	}
}

func TestCurrent(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	snap := m.Current()
	if snap.Goroutines <= 0 {
		t.Error("expected current snapshot")
	}
}

func TestStats(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	m.Snapshot()
	m.Snapshot()

	stats := m.Stats()
	if stats.SnapshotCount != 2 {
		t.Errorf("expected 2 snapshots, got %d", stats.SnapshotCount)
	}
	if stats.AvgHeapMB <= 0 {
		t.Error("expected positive average heap")
	}
	if stats.PeakHeapMB <= 0 {
		t.Error("expected positive peak heap")
	}
}

func TestForceGC(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	snap := m.ForceGC()
	if snap.NumGC < 1 {
		t.Error("expected at least 1 GC after ForceGC")
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()
	if len(thresholds) == 0 {
		t.Error("expected default thresholds")
	}
	for _, th := range thresholds {
		if th.WarnLevel <= 0 || th.CritLevel <= 0 {
			t.Errorf("threshold %s has invalid levels", th.Metric)
		}
	}
}

func TestSetThresholds(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	custom := []AlertThreshold{
		{Metric: "goroutines", WarnLevel: 100, CritLevel: 500},
	}
	m.SetThresholds(custom)

	// Thresholds are set — verify by checking no panic on snapshot
	m.Snapshot()
}

func TestOnAlert(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	// Set very low thresholds to trigger alerts
	m.SetThresholds([]AlertThreshold{
		{Metric: "goroutines", WarnLevel: 1, CritLevel: 50000},
	})

	alertFired := false
	m.OnAlert(func(a Alert) {
		alertFired = true
	})

	m.Snapshot()

	if !alertFired {
		t.Error("expected alert to fire with low threshold")
	}
}

func TestAlertLevels(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	var alerts []Alert
	m.OnAlert(func(a Alert) {
		alerts = append(alerts, a)
	})

	// Set critical threshold at 1 goroutine
	m.SetThresholds([]AlertThreshold{
		{Metric: "goroutines", WarnLevel: 50000, CritLevel: 1},
	})

	m.Snapshot()

	foundCritical := false
	for _, a := range alerts {
		if a.Level == "critical" {
			foundCritical = true
		}
	}
	if !foundCritical {
		t.Error("expected critical alert")
	}
}

func TestSaveAndLoad(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	m.Snapshot()
	m.Snapshot()

	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestMaxSnapshots(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.MaxSnapshots = 3
	cfg.StateDir = t.TempDir()
	m := NewMonitor(cfg)

	for i := 0; i < 5; i++ {
		m.Snapshot()
	}

	history := m.History(0)
	if len(history) != 3 {
		t.Errorf("expected 3 snapshots (max), got %d", len(history))
	}
}

func TestFormatSnapshot(t *testing.T) {
	snap := ResourceSnapshot{
		Timestamp:     time.Now(),
		Goroutines:    42,
		HeapAllocMB:   128.5,
		StackInUseMB:  8.2,
		NumGC:         15,
		GCPauseMs:     3.7,
		UptimeSeconds: 3600,
	}
	output := FormatSnapshot(snap)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatStats(t *testing.T) {
	stats := MonitorStats{
		SnapshotCount:  100,
		AvgHeapMB:      256.3,
		PeakHeapMB:     512.1,
		AvgGoroutines:  45,
		PeakGoroutines: 120,
	}
	output := FormatStats(stats)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatAlert(t *testing.T) {
	alert := Alert{
		Level:     "warn",
		Metric:    "goroutines",
		Value:     600,
		Threshold: 500,
		Message:   "goroutines at 600 (warning threshold: 500)",
		Timestamp: time.Now(),
	}
	output := FormatAlert(alert)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
