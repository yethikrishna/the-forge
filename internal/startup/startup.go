// Package startup provides sub-100ms command startup optimization.
// Measures init time per module, identifies slow loads, and provides
// lazy loading wrappers to defer module initialization.
//
// Fast is a feature.
package startup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Module represents a loaded module with timing.
type Module struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
	Lazy     bool          `json:"lazy"`
	Loaded   bool          `json:"loaded"`
	Category string        `json:"category"`
}

// Benchmark is a single startup benchmark run.
type Benchmark struct {
	Timestamp time.Time `json:"timestamp"`
	TotalMs   float64   `json:"total_ms"`
	Modules   []Module  `json:"modules"`
	TargetMs  float64   `json:"target_ms"`
	Pass      bool      `json:"pass"`
}

// Tracker tracks module initialization times.
type Tracker struct {
	modules  []Module
	start    time.Time
	targetMs float64
	mu       sync.Mutex
}

// NewTracker creates a startup time tracker.
func NewTracker(targetMs float64) *Tracker {
	return &Tracker{
		targetMs: targetMs,
	}
}

// Start begins timing.
func (t *Tracker) Start() {
	t.start = time.Now()
}

// Record records a module's init time.
func (t *Tracker) Record(name string, duration time.Duration, category string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.modules = append(t.modules, Module{
		Name:     name,
		Duration: duration,
		Category: category,
		Loaded:   true,
	})
}

// Measure runs fn and records its duration.
func (t *Tracker) Measure(name, category string, fn func()) {
	start := time.Now()
	fn()
	t.Record(name, time.Since(start), category)
}

// MeasureLazy records a lazy module (not yet loaded).
func (t *Tracker) MeasureLazy(name, category string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.modules = append(t.modules, Module{
		Name:     name,
		Duration: 0,
		Category: category,
		Lazy:     true,
		Loaded:   false,
	})
}

// LazyInit creates a lazy initialization wrapper.
func LazyInit(name string, init func() error) *LazyModule {
	return &LazyModule{
		name: name,
		init: init,
	}
}

// LazyModule defers initialization until first use.
type LazyModule struct {
	name   string
	init   func() error
	loaded bool
	err    error
	mu     sync.Mutex
}

// Get ensures the module is loaded and returns any error.
func (l *LazyModule) Get() error {
	if l.loaded {
		return l.err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loaded {
		return l.err
	}
	l.err = l.init()
	l.loaded = true
	return l.err
}

// Loaded returns whether the module has been initialized.
func (l *LazyModule) Loaded() bool {
	return l.loaded
}

// Snapshot returns the current benchmark.
func (t *Tracker) Snapshot() Benchmark {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := time.Since(t.start)
	mods := make([]Module, len(t.modules))
	copy(mods, t.modules)

	return Benchmark{
		Timestamp: time.Now(),
		TotalMs:   float64(total.Milliseconds()),
		Modules:   mods,
		TargetMs:  t.targetMs,
		Pass:      float64(total.Milliseconds()) <= t.targetMs,
	}
}

// Report generates a startup report.
func (t *Tracker) Report() string {
	b := t.Snapshot()
	return FormatBenchmark(&b)
}

// SlowModules returns modules exceeding a threshold.
func (t *Tracker) SlowModules(thresholdMs float64) []Module {
	t.mu.Lock()
	defer t.mu.Unlock()

	var slow []Module
	for _, m := range t.modules {
		if float64(m.Duration.Milliseconds()) > thresholdMs {
			slow = append(slow, m)
		}
	}
	sort.Slice(slow, func(i, j int) bool {
		return slow[i].Duration > slow[j].Duration
	})
	return slow
}

// Modules returns all modules sorted by duration (slowest first).
func (t *Tracker) Modules() []Module {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]Module, len(t.modules))
	copy(result, t.modules)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Duration > result[j].Duration
	})
	return result
}

// Save saves a benchmark to a file.
func SaveBenchmark(b *Benchmark, dir string) error {
	os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(b, "", "  ")
	return os.WriteFile(filepath.Join(dir, "startup_benchmark.json"), data, 0644)
}

// LoadBenchmark loads a benchmark from a file.
func LoadBenchmark(dir string) (*Benchmark, error) {
	data, err := os.ReadFile(filepath.Join(dir, "startup_benchmark.json"))
	if err != nil {
		return nil, err
	}
	var b Benchmark
	json.Unmarshal(data, &b)
	return &b, nil
}

// FormatBenchmark formats a benchmark for display.
func FormatBenchmark(b *Benchmark) string {
	var s strings.Builder

	status := "PASS"
	if !b.Pass {
		status = "FAIL"
	}

	s.WriteString(fmt.Sprintf("Startup Benchmark [%s]\n", status))
	s.WriteString(fmt.Sprintf("Total: %.1fms (target: %.0fms)\n\n", b.TotalMs, b.TargetMs))

	// Sort modules by duration
	mods := make([]Module, len(b.Modules))
	copy(mods, b.Modules)
	sort.Slice(mods, func(i, j int) bool {
		return mods[i].Duration > mods[j].Duration
	})

	s.WriteString("Module breakdown:\n")
	for _, m := range mods {
		ms := float64(m.Duration.Milliseconds())
		status := ""
		if m.Lazy {
			status = " (lazy)"
		}
		bar := strings.Repeat("█", int(ms/2))
		if len(bar) > 40 {
			bar = bar[:40]
		}
		s.WriteString(fmt.Sprintf("  %-25s %6.1fms %s%s\n", m.Name, ms, bar, status))
	}

	return s.String()
}

// CompareBenchmarks compares two benchmark runs.
func CompareBenchmarks(before, after *Benchmark) string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("Before: %.1fms → After: %.1fms (Δ: %.1fms)\n",
		before.TotalMs, after.TotalMs, after.TotalMs-before.TotalMs))

	// Build map of before modules
	beforeMap := make(map[string]Module)
	for _, m := range before.Modules {
		beforeMap[m.Name] = m
	}

	for _, m := range after.Modules {
		if before, ok := beforeMap[m.Name]; ok {
			delta := float64(m.Duration.Milliseconds()) - float64(before.Duration.Milliseconds())
			indicator := ""
			if delta > 1 {
				indicator = "↑"
			} else if delta < -1 {
				indicator = "↓"
			}
			s.WriteString(fmt.Sprintf("  %-25s %.1fms → %.1fms (%.1fms) %s\n",
				m.Name, float64(before.Duration.Milliseconds()),
				float64(m.Duration.Milliseconds()), delta, indicator))
		}
	}

	return s.String()
}
