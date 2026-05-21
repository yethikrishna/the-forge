package costlive

import (
	"fmt"
	"os"
	"testing"
)

func newBenchTracker(b *testing.B, budget float64) *LiveTracker {
	b.Helper()
	dir, err := os.MkdirTemp("", "costlive-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })
	lt, err := NewLiveTracker(dir, budget)
	if err != nil {
		b.Fatal(err)
	}
	return lt
}

// BenchmarkRecord measures the cost of recording a single usage snapshot.
func BenchmarkRecord(b *testing.B) {
	lt := newBenchTracker(b, 100.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lt.Record("agent-1", "claude-3-5-sonnet", 1000, 500, 0.005, "chat")
	}
}

// BenchmarkRecord_MultiAgent measures recording across many agents/models.
func BenchmarkRecord_MultiAgent(b *testing.B) {
	lt := newBenchTracker(b, 500.0)
	agents := []string{"forge-main", "forge-coder", "forge-reviewer", "forge-planner", "forge-critic"}
	models := []string{"claude-3-5-sonnet", "claude-3-haiku", "gpt-4o", "gpt-4o-mini", "gemini-1.5-pro"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lt.Record(
			agents[i%len(agents)],
			models[i%len(models)],
			500+i%2000,
			200+i%800,
			float64(i%100)*0.001,
			"inference",
		)
	}
}

// BenchmarkStats_EmptyTracker measures stats computation on an empty tracker.
func BenchmarkStats_EmptyTracker(b *testing.B) {
	lt := newBenchTracker(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lt.Stats()
	}
}

// BenchmarkStats_100Snapshots measures stats with 100 pre-recorded snapshots.
func BenchmarkStats_100Snapshots(b *testing.B) {
	lt := newBenchTracker(b, 200.0)
	for i := 0; i < 100; i++ {
		lt.Record(fmt.Sprintf("agent-%d", i%5), "claude-3-5-sonnet", 1000, 500, 0.005, "chat")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lt.Stats()
	}
}

// BenchmarkStats_1000Snapshots measures stats at moderate scale (1 000 snapshots).
func BenchmarkStats_1000Snapshots(b *testing.B) {
	lt := newBenchTracker(b, 1000.0)
	for i := 0; i < 1000; i++ {
		lt.Record(
			fmt.Sprintf("agent-%d", i%10),
			[]string{"claude-3-5-sonnet", "claude-3-haiku", "gpt-4o"}[i%3],
			500+i%3000,
			100+i%1000,
			float64(i%50)*0.0001,
			"inference",
		)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lt.Stats()
	}
}

// BenchmarkStats_WithBudget measures budget calculation overhead.
func BenchmarkStats_WithBudget(b *testing.B) {
	lt := newBenchTracker(b, 50.0)
	for i := 0; i < 200; i++ {
		lt.Record("agent-budget", "gpt-4o", 800, 400, 0.01, "tool")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lt.Stats()
	}
}

// BenchmarkFormatLiveStats measures terminal formatting performance.
func BenchmarkFormatLiveStats(b *testing.B) {
	lt := newBenchTracker(b, 100.0)
	for i := 0; i < 50; i++ {
		lt.Record(fmt.Sprintf("agent-%d", i%5), "claude-3-5-sonnet", 1000, 500, 0.005, "chat")
	}
	stats := lt.Stats()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatLiveStats(stats)
	}
}

// BenchmarkFormatNumber measures the comma-formatting helper.
func BenchmarkFormatNumber(b *testing.B) {
	nums := []int{0, 999, 1000, 1000000, 1234567890}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatNumber(nums[i%len(nums)])
	}
}

// BenchmarkDaysInMonth measures month-length calculation.
func BenchmarkDaysInMonth(b *testing.B) {
	months := [][2]int{{2024, 2}, {2025, 1}, {2025, 12}, {2024, 2}} // includes leap year
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := months[i%len(months)]
		daysInMonth(m[0], m[1])
	}
}

// BenchmarkRecord_Parallel measures concurrent recording throughput.
func BenchmarkRecord_Parallel(b *testing.B) {
	lt := newBenchTracker(b, 0)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lt.Record(fmt.Sprintf("agent-%d", i%5), "claude-3-5-sonnet", 1000, 500, 0.005, "chat")
			i++
		}
	})
}
