// Package resource monitors disk, memory, and goroutine usage
// with configurable thresholds and auto-cleanup.
//
// Know your limits. Stay within them.
package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents a resource usage level.
type Level string

const (
	LevelOK       Level = "ok"
	LevelWarn     Level = "warn"
	LevelCritical Level = "critical"
)

// Thresholds defines when to trigger warnings.
type Thresholds struct {
	MemoryPercent float64 `json:"memory_percent"` // default: 80
	DiskPercent   float64 `json:"disk_percent"`   // default: 90
	Goroutines    int     `json:"goroutines"`     // default: 1000
	OpenFiles     int     `json:"open_files"`     // default: 500
}

// DefaultThresholds returns sensible defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MemoryPercent: 80,
		DiskPercent:   90,
		Goroutines:    1000,
		OpenFiles:     500,
	}
}

// Snapshot represents a resource usage snapshot.
type Snapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	MemoryUsed  uint64    `json:"memory_used"`
	MemoryTotal uint64    `json:"memory_total"`
	MemoryPct   float64   `json:"memory_pct"`
	DiskUsed    uint64    `json:"disk_used"`
	DiskTotal   uint64    `json:"disk_total"`
	DiskPct     float64   `json:"disk_pct"`
	Goroutines  int       `json:"goroutines"`
	OpenFiles   int       `json:"open_files"`
	CPUCount    int       `json:"cpu_count"`
	Level       Level     `json:"level"`
}

// Alert represents a resource alert.
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // memory, disk, goroutines, files
	Level     Level     `json:"level"`
	Message   string    `json:"message"`
	Value     string    `json:"value"`
	Threshold string    `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// Cleaner defines a cleanup action.
type Cleaner struct {
	Name string
	Fn   func() (int, error) // returns items cleaned
}

// Monitor tracks resource usage.
type Monitor struct {
	mu         sync.Mutex
	dir        string
	thresholds Thresholds
	cleaners   []Cleaner
	history    []*Snapshot
	alerts     []*Alert
	maxHistory int
}

// NewMonitor creates a resource monitor.
func NewMonitor(dir string) (*Monitor, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Monitor{
		dir:        dir,
		thresholds: DefaultThresholds(),
		history:    make([]*Snapshot, 0),
		alerts:     make([]*Alert, 0),
		maxHistory: 100,
	}, nil
}

// SetThresholds configures alert thresholds.
func (m *Monitor) SetThresholds(t Thresholds) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.thresholds = t
}

// RegisterCleaner adds a cleanup action.
func (m *Monitor) RegisterCleaner(name string, fn func() (int, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleaners = append(m.cleaners, Cleaner{Name: name, Fn: fn})
}

// Snapshot captures current resource usage.
func (m *Monitor) Snapshot() *Snapshot {
	s := &Snapshot{
		Timestamp:  time.Now(),
		CPUCount:   runtime.NumCPU(),
		Goroutines: runtime.NumGoroutine(),
	}

	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	s.MemoryUsed = memStats.Alloc
	s.MemoryTotal = memStats.Sys
	if s.MemoryTotal > 0 {
		s.MemoryPct = float64(s.MemoryUsed) / float64(s.MemoryTotal) * 100
	}

	// Disk usage (workspace dir)
	s.DiskUsed, s.DiskTotal = getDiskUsage(m.dir)
	if s.DiskTotal > 0 {
		s.DiskPct = float64(s.DiskUsed) / float64(s.DiskTotal) * 100
	}

	// Open files (best effort)
	s.OpenFiles = countOpenFiles()

	// Determine overall level
	s.Level = m.evaluateLevel(s)

	// Store
	m.mu.Lock()
	m.history = append(m.history, s)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
	m.mu.Unlock()

	// Check thresholds and generate alerts
	m.checkThresholds(s)

	return s
}

func (m *Monitor) evaluateLevel(s *Snapshot) Level {
	level := LevelOK
	if s.MemoryPct > m.thresholds.MemoryPercent {
		level = LevelWarn
	}
	if s.DiskPct > m.thresholds.DiskPercent {
		level = LevelWarn
	}
	if s.Goroutines > m.thresholds.Goroutines {
		level = LevelWarn
	}
	if s.OpenFiles > m.thresholds.OpenFiles {
		level = LevelWarn
	}
	// Critical if >20% above threshold
	if s.MemoryPct > m.thresholds.MemoryPercent*1.2 {
		level = LevelCritical
	}
	if s.DiskPct > m.thresholds.DiskPercent*1.05 {
		level = LevelCritical
	}
	return level
}

func (m *Monitor) checkThresholds(s *Snapshot) {
	if s.MemoryPct > m.thresholds.MemoryPercent {
		m.alert("memory", s.MemoryPct > m.thresholds.MemoryPercent*1.2, s)
	}
	if s.DiskPct > m.thresholds.DiskPercent {
		m.alert("disk", s.DiskPct > m.thresholds.DiskPercent*1.05, s)
	}
	if s.Goroutines > m.thresholds.Goroutines {
		m.alert("goroutines", s.Goroutines > m.thresholds.Goroutines*2, s)
	}
	if s.OpenFiles > m.thresholds.OpenFiles {
		m.alert("files", s.OpenFiles > m.thresholds.OpenFiles*2, s)
	}
}

func (m *Monitor) alert(typ string, critical bool, s *Snapshot) {
	level := LevelWarn
	if critical {
		level = LevelCritical
	}

	var value, threshold string
	switch typ {
	case "memory":
		value = fmt.Sprintf("%.1f%%", s.MemoryPct)
		threshold = fmt.Sprintf("%.1f%%", m.thresholds.MemoryPercent)
	case "disk":
		value = fmt.Sprintf("%.1f%%", s.DiskPct)
		threshold = fmt.Sprintf("%.1f%%", m.thresholds.DiskPercent)
	case "goroutines":
		value = fmt.Sprintf("%d", s.Goroutines)
		threshold = fmt.Sprintf("%d", m.thresholds.Goroutines)
	case "files":
		value = fmt.Sprintf("%d", s.OpenFiles)
		threshold = fmt.Sprintf("%d", m.thresholds.OpenFiles)
	}

	a := &Alert{
		ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Type:      typ,
		Level:     level,
		Message:   fmt.Sprintf("%s usage at %s (threshold: %s)", typ, value, threshold),
		Value:     value,
		Threshold: threshold,
		Timestamp: time.Now(),
	}

	m.mu.Lock()
	m.alerts = append(m.alerts, a)
	if len(m.alerts) > 200 {
		m.alerts = m.alerts[len(m.alerts)-100:]
	}
	m.mu.Unlock()
}

// Cleanup runs all registered cleaners.
func (m *Monitor) Cleanup() map[string]int {
	results := make(map[string]int)
	for _, c := range m.cleaners {
		n, err := c.Fn()
		if err == nil {
			results[c.Name] = n
		}
	}
	return results
}

// GetHistory returns recent snapshots.
func (m *Monitor) GetHistory(limit int) []*Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}
	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*Snapshot, len(m.history[start:]))
	copy(result, m.history[start:])
	return result
}

// GetAlerts returns recent alerts.
func (m *Monitor) GetAlerts(limit int) []*Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}
	start := len(m.alerts) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*Alert, len(m.alerts[start:]))
	copy(result, m.alerts[start:])
	return result
}

// FormatSnapshot renders a snapshot for display.
func FormatSnapshot(s *Snapshot) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Resource Usage — %s\n", s.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Memory:     %s / %s (%.1f%%)\n", formatBytes(s.MemoryUsed), formatBytes(s.MemoryTotal), s.MemoryPct))
	sb.WriteString(fmt.Sprintf("  Disk:       %s / %s (%.1f%%)\n", formatBytes(s.DiskUsed), formatBytes(s.DiskTotal), s.DiskPct))
	sb.WriteString(fmt.Sprintf("  Goroutines: %d\n", s.Goroutines))
	sb.WriteString(fmt.Sprintf("  Open Files: %d\n", s.OpenFiles))
	sb.WriteString(fmt.Sprintf("  CPUs:       %d\n", s.CPUCount))
	sb.WriteString(fmt.Sprintf("  Status:     %s\n", s.Level))
	return sb.String()
}

// FormatAlert renders an alert for display.
func FormatAlert(a *Alert) string {
	prefix := "⚠️"
	if a.Level == LevelCritical {
		prefix = "🔴"
	}
	return fmt.Sprintf("%s [%s] %s — %s (threshold: %s)", prefix, a.Type, a.Message, a.Value, a.Threshold)
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func getDiskUsage(path string) (used, total uint64) {
	// Use df-like approach
	out, err := exec.Command("df", "-B1", path).Output()
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return 0, 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, 0
	}
	fmt.Sscanf(fields[1], "%d", &total)
	fmt.Sscanf(fields[2], "%d", &used)
	return used, total
}

func countOpenFiles() int {
	out, err := exec.Command("sh", "-c", fmt.Sprintf("ls /proc/%d/fd 2>/dev/null | wc -l", os.Getpid())).Output()
	if err != nil {
		return 0
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count
}

// SaveSnapshot persists a snapshot to disk.
func (m *Monitor) SaveSnapshot(s *Snapshot) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dir+"/latest.json", data, 0o644)
}
