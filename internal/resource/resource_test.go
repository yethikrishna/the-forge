package resource

import (
	"testing"
)

func TestNewMonitor(t *testing.T) {
	m, err := NewMonitor(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected non-nil monitor")
	}
}

func TestSnapshot(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	s := m.Snapshot()
	if s == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if s.CPUCount <= 0 {
		t.Error("expected positive CPU count")
	}
	if s.Timestamp.IsZero() {
		t.Error("expected timestamp")
	}
}

func TestMemorySnapshot(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	s := m.Snapshot()
	if s.MemoryTotal == 0 {
		t.Error("expected non-zero memory total")
	}
	if s.MemoryPct < 0 || s.MemoryPct > 100 {
		t.Errorf("memory percent out of range: %.1f", s.MemoryPct)
	}
}

func TestGoroutines(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	s := m.Snapshot()
	if s.Goroutines <= 0 {
		t.Error("expected positive goroutine count")
	}
}

func TestLevelOK(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	m.SetThresholds(Thresholds{MemoryPercent: 99, DiskPercent: 99, Goroutines: 10000, OpenFiles: 10000})
	s := m.Snapshot()
	if s.Level != LevelOK {
		t.Errorf("expected ok with high thresholds, got %s", s.Level)
	}
}

func TestThresholdAlerts(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	m.SetThresholds(Thresholds{MemoryPercent: 0.001, DiskPercent: 0.001, Goroutines: 1, OpenFiles: 1})
	_ = m.Snapshot()
	alerts := m.GetAlerts(10)
	if len(alerts) == 0 {
		t.Error("expected alerts with low thresholds")
	}
}

func TestHistory(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	m.Snapshot()
	m.Snapshot()
	m.Snapshot()
	history := m.GetHistory(0)
	if len(history) != 3 {
		t.Errorf("expected 3, got %d", len(history))
	}
	history = m.GetHistory(2)
	if len(history) != 2 {
		t.Errorf("expected 2, got %d", len(history))
	}
}

func TestCleanup(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	m.RegisterCleaner("test", func() (int, error) { return 5, nil })
	results := m.Cleanup()
	if results["test"] != 5 {
		t.Errorf("expected 5, got %d", results["test"])
	}
}

func TestFormatSnapshot(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	s := m.Snapshot()
	output := FormatSnapshot(s)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatAlert(t *testing.T) {
	a := &Alert{Type: "memory", Level: LevelWarn, Message: "high memory", Value: "85%", Threshold: "80%"}
	output := FormatAlert(a)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, got, tt.want)
		}
	}
}

func TestSaveSnapshot(t *testing.T) {
	m, _ := NewMonitor(t.TempDir())
	s := m.Snapshot()
	if err := m.SaveSnapshot(s); err != nil {
		t.Fatal(err)
	}
}
