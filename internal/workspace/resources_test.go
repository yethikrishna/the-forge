package workspace

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResourceTrackerSetLimits(t *testing.T) {
	rt := NewResourceTracker()
	rt.SetLimits("env-1", ResourceLimits{CPUCores: 2, MemoryMB: 4096})

	m, ok := rt.GetMetrics("env-1")
	if !ok {
		t.Fatal("expected metrics to exist")
	}
	if m.Limits.CPUCores != 2 {
		t.Errorf("expected 2 CPU cores, got %f", m.Limits.CPUCores)
	}
	if m.Limits.MemoryMB != 4096 {
		t.Errorf("expected 4096 MB, got %d", m.Limits.MemoryMB)
	}
}

func TestResourceTrackerRecord(t *testing.T) {
	rt := NewResourceTracker()

	snap := ResourceSnapshot{
		CPUPct:   45.2,
		MemoryMB: 2048,
		DiskMB:   500,
	}
	rt.Record("env-1", snap)

	m, ok := rt.GetMetrics("env-1")
	if !ok {
		t.Fatal("expected metrics")
	}
	if m.Current.CPUPct != 45.2 {
		t.Errorf("expected 45.2 CPU, got %f", m.Current.CPUPct)
	}
	if m.Current.MemoryMB != 2048 {
		t.Errorf("expected 2048 MB, got %d", m.Current.MemoryMB)
	}
}

func TestResourceTrackerPeak(t *testing.T) {
	rt := NewResourceTracker()

	rt.Record("env-1", ResourceSnapshot{CPUPct: 30, MemoryMB: 1000})
	rt.Record("env-1", ResourceSnapshot{CPUPct: 80, MemoryMB: 2000})
	rt.Record("env-1", ResourceSnapshot{CPUPct: 20, MemoryMB: 3000})

	m, _ := rt.GetMetrics("env-1")
	if m.Peak.CPUPct != 80 {
		t.Errorf("expected peak CPU 80, got %f", m.Peak.CPUPct)
	}
	if m.Peak.MemoryMB != 3000 {
		t.Errorf("expected peak mem 3000, got %d", m.Peak.MemoryMB)
	}
}

func TestResourceTrackerOverLimit(t *testing.T) {
	rt := NewResourceTracker()
	rt.SetLimits("env-1", ResourceLimits{CPUCores: 2, MemoryMB: 4096})

	rt.Record("env-1", ResourceSnapshot{CPUPct: 250, MemoryMB: 3000})

	over, details := rt.IsOverLimit("env-1")
	if !over {
		t.Error("expected over limit")
	}
	if len(details) != 1 {
		t.Errorf("expected 1 detail, got %d: %v", len(details), details)
	}
}

func TestResourceTrackerUnderLimit(t *testing.T) {
	rt := NewResourceTracker()
	rt.SetLimits("env-1", ResourceLimits{CPUCores: 4, MemoryMB: 8192})

	rt.Record("env-1", ResourceSnapshot{CPUPct: 50, MemoryMB: 2000})

	over, _ := rt.IsOverLimit("env-1")
	if over {
		t.Error("expected under limit")
	}
}

func TestResourceTrackerNoLimits(t *testing.T) {
	rt := NewResourceTracker()
	rt.Record("env-1", ResourceSnapshot{CPUPct: 99, MemoryMB: 99999})

	over, _ := rt.IsOverLimit("env-1")
	if over {
		t.Error("expected not over when no limits set")
	}
}

func TestResourceTrackerHistory(t *testing.T) {
	rt := NewResourceTracker()

	now := time.Now()
	t1 := now.Add(-30 * time.Minute)
	t2 := now.Add(-15 * time.Minute)
	t3 := now

	s1 := ResourceSnapshot{CPUPct: 10}
	s1.Timestamp = t1
	s2 := ResourceSnapshot{CPUPct: 20}
	s2.Timestamp = t2
	s3 := ResourceSnapshot{CPUPct: 30}
	s3.Timestamp = t3

	rt.Record("env-1", s1)
	rt.Record("env-1", s2)
	rt.Record("env-1", s3)

	// Get history from t2 onwards
	hist := rt.History("env-1", t2, time.Time{})
	if len(hist) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(hist))
	}

	// Get all history
	all := rt.History("env-1", time.Time{}, time.Time{})
	if len(all) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(all))
	}
}

func TestResourceTrackerHistoryTrim(t *testing.T) {
	rt := NewResourceTracker()
	rt.maxHist = 5

	for i := 0; i < 10; i++ {
		rt.Record("env-1", ResourceSnapshot{CPUPct: float64(i)})
	}

	m, _ := rt.GetMetrics("env-1")
	if len(m.History) != 5 {
		t.Errorf("expected 5 history entries (trimmed), got %d", len(m.History))
	}
}

func TestResourceTrackerAllMetrics(t *testing.T) {
	rt := NewResourceTracker()
	rt.Record("a", ResourceSnapshot{CPUPct: 10})
	rt.Record("b", ResourceSnapshot{CPUPct: 20})

	all := rt.AllMetrics()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestResourceTrackerNotFound(t *testing.T) {
	rt := NewResourceTracker()
	_, ok := rt.GetMetrics("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestParseMemMB(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1GiB", 1024},
		{"512MiB", 512},
		{"2048KiB", 2},
		{"4096", 4096},
	}

	for _, tt := range tests {
		got := parseMemMB(tt.input)
		if got != tt.expected {
			t.Errorf("parseMemMB(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseBytesMB(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1024kB", 1},
		{"1MB", 1},
		{"1GB", 1024},
		{"500B", 0},
	}

	for _, tt := range tests {
		got := parseBytesMB(tt.input)
		if got != tt.expected {
			t.Errorf("parseBytesMB(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestResourceSnapshotSerialization(t *testing.T) {
	now := time.Now()
	snap := ResourceSnapshot{
		Timestamp: now,
		CPUPct:    55.5,
		MemoryMB:  2048,
		DiskMB:    10240,
		GPUUsage:  30.0,
		GPUMemMB:  512,
		NetRxMB:   100,
		NetTxMB:   50,
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}

	var snap2 ResourceSnapshot
	if err := json.Unmarshal(data, &snap2); err != nil {
		t.Fatal(err)
	}

	if snap2.CPUPct != 55.5 {
		t.Errorf("expected 55.5, got %f", snap2.CPUPct)
	}
	if snap2.MemoryMB != 2048 {
		t.Errorf("expected 2048, got %d", snap2.MemoryMB)
	}
	if snap2.GPUUsage != 30.0 {
		t.Errorf("expected 30.0, got %f", snap2.GPUUsage)
	}
}
