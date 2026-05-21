// Package workspace provides CPU/memory/GPU resource tracking per workspace.
// Monitors utilization, enforces limits, and reports metrics.
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ResourceSnapshot is a point-in-time measurement of resource usage.
type ResourceSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	CPUPct    float64   `json:"cpu_pct"`
	MemoryMB  int64     `json:"memory_mb"`
	DiskMB    int64     `json:"disk_mb"`
	GPUUsage  float64   `json:"gpu_usage,omitempty"`
	GPUMemMB  int64     `json:"gpu_mem_mb,omitempty"`
	NetRxMB   int64     `json:"net_rx_mb"`
	NetTxMB   int64     `json:"net_tx_mb"`
}

// ResourceLimits defines hard resource boundaries.
type ResourceLimits struct {
	CPUCores float64 `json:"cpu_cores"`
	MemoryMB int64   `json:"memory_mb"`
	DiskMB   int64   `json:"disk_mb"`
	GPUs     int     `json:"gpus"`
}

// ResourceMetrics tracks resource usage over time for one environment.
type ResourceMetrics struct {
	EnvName   string            `json:"env_name"`
	Limits    ResourceLimits    `json:"limits"`
	Current   ResourceSnapshot  `json:"current"`
	Peak      ResourceSnapshot  `json:"peak"`
	History   []ResourceSnapshot `json:"history,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// ResourceTracker monitors resource usage across environments.
type ResourceTracker struct {
	mu      sync.RWMutex
	metrics map[string]*ResourceMetrics
	maxHist int
}

// NewResourceTracker creates a resource tracker.
func NewResourceTracker() *ResourceTracker {
	return &ResourceTracker{
		metrics: make(map[string]*ResourceMetrics),
		maxHist: 288, // 24h at 5-min intervals
	}
}

// SetLimits configures resource limits for an environment.
func (rt *ResourceTracker) SetLimits(envName string, limits ResourceLimits) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	m, ok := rt.metrics[envName]
	if !ok {
		m = &ResourceMetrics{EnvName: envName}
		rt.metrics[envName] = m
	}
	m.Limits = limits
}

// Record captures a resource snapshot for an environment.
func (rt *ResourceTracker) Record(envName string, snap ResourceSnapshot) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	m, ok := rt.metrics[envName]
	if !ok {
		m = &ResourceMetrics{EnvName: envName}
		rt.metrics[envName] = m
	}
	snap.Timestamp = time.Now()
	m.Current = snap
	m.UpdatedAt = time.Now()

	// Track peak
	if snap.CPUPct > m.Peak.CPUPct {
		m.Peak.CPUPct = snap.CPUPct
	}
	if snap.MemoryMB > m.Peak.MemoryMB {
		m.Peak.MemoryMB = snap.MemoryMB
	}
	if snap.GPUUsage > m.Peak.GPUUsage {
		m.Peak.GPUUsage = snap.GPUUsage
	}

	// Append to history, trim if needed
	m.History = append(m.History, snap)
	if len(m.History) > rt.maxHist {
		m.History = m.History[len(m.History)-rt.maxHist:]
	}
}

// GetMetrics returns current metrics for an environment.
func (rt *ResourceTracker) GetMetrics(envName string) (*ResourceMetrics, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	m, ok := rt.metrics[envName]
	if !ok {
		return nil, false
	}
	cp := *m
	return &cp, true
}

// AllMetrics returns metrics for all tracked environments.
func (rt *ResourceTracker) AllMetrics() map[string]*ResourceMetrics {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	result := make(map[string]*ResourceMetrics, len(rt.metrics))
	for k, v := range rt.metrics {
		cp := *v
		result[k] = &cp
	}
	return result
}

// IsOverLimit checks if an environment exceeds any resource limit.
func (rt *ResourceTracker) IsOverLimit(envName string) (over bool, details []string) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	m, ok := rt.metrics[envName]
	if !ok {
		return false, nil
	}

	limits := m.Limits
	cur := m.Current

	if limits.CPUCores > 0 && cur.CPUPct > limits.CPUCores*100 {
		over = true
		details = append(details, fmt.Sprintf("CPU: %.1f%% exceeds %.1f%% limit", cur.CPUPct, limits.CPUCores*100))
	}
	if limits.MemoryMB > 0 && cur.MemoryMB > limits.MemoryMB {
		over = true
		details = append(details, fmt.Sprintf("Memory: %dMB exceeds %dMB limit", cur.MemoryMB, limits.MemoryMB))
	}
	if limits.DiskMB > 0 && cur.DiskMB > limits.DiskMB {
		over = true
		details = append(details, fmt.Sprintf("Disk: %dMB exceeds %dMB limit", cur.DiskMB, limits.DiskMB))
	}
	return
}

// History returns the usage history for an env within the time range.
func (rt *ResourceTracker) History(envName string, from, to time.Time) []ResourceSnapshot {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	m, ok := rt.metrics[envName]
	if !ok {
		return nil
	}

	var result []ResourceSnapshot
	for _, s := range m.History {
		if (from.IsZero() || !s.Timestamp.Before(from)) && (to.IsZero() || !s.Timestamp.After(to)) {
			result = append(result, s)
		}
	}
	return result
}

// CollectDockerStats reads resource stats from a Docker container.
func CollectDockerStats(containerID string) (ResourceSnapshot, error) {
	var snap ResourceSnapshot

	out, err := exec.Command("docker", "stats",
		"--no-stream",
		"--format",
		`{"cpu":"{{.CPUPerc}}","mem":"{{.MemUsage}}","net":"{{.NetIO}}"}`,
		containerID,
	).CombinedOutput()

	if err != nil {
		return snap, fmt.Errorf("docker stats: %w", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(out, &raw); err != nil {
		return snap, fmt.Errorf("parse docker stats: %w", err)
	}

	// Parse CPU percentage
	if cpu, ok := raw["cpu"]; ok {
		snap.CPUPct, _ = strconv.ParseFloat(strings.TrimSuffix(cpu, "%"), 64)
	}

	// Parse memory usage "50MiB / 1GiB"
	if mem, ok := raw["mem"]; ok {
		parts := strings.SplitN(mem, "/", 2)
		if len(parts) > 0 {
			snap.MemoryMB = parseMemMB(strings.TrimSpace(parts[0]))
		}
	}

	// Parse network I/O "1.2kB / 3.4kB"
	if net, ok := raw["net"]; ok {
		parts := strings.SplitN(net, "/", 2)
		if len(parts) == 2 {
			snap.NetRxMB = parseBytesMB(strings.TrimSpace(parts[0]))
			snap.NetTxMB = parseBytesMB(strings.TrimSpace(parts[1]))
		}
	}

	snap.Timestamp = time.Now()
	return snap, nil
}

// parseMemMB parses Docker memory strings like "50MiB", "1.5GiB".
func parseMemMB(s string) int64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "GiB") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "GiB"), 64)
		return int64(v * 1024)
	}
	if strings.HasSuffix(s, "MiB") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "MiB"), 64)
		return int64(v)
	}
	if strings.HasSuffix(s, "KiB") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "KiB"), 64)
		return int64(v / 1024)
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// parseBytesMB parses byte strings like "1.2kB", "3.4MB".
func parseBytesMB(s string) int64 {
	s = strings.TrimSpace(s)
	multipliers := map[string]float64{
		"B":  1.0 / (1024 * 1024),
		"kB": 1.0 / 1024,
		"MB": 1.0,
		"GB": 1024.0,
	}
	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			v, _ := strconv.ParseFloat(strings.TrimSuffix(s, suffix), 64)
			return int64(v * mult)
		}
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v / (1024 * 1024)
}

// ReadProcessStats reads resource stats from /proc for a PID.
func ReadProcessStats(pid int) (ResourceSnapshot, error) {
	var snap ResourceSnapshot

	// Read /proc/[pid]/stat for CPU
	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return snap, err
	}

	fields := strings.Fields(string(statData))
	if len(fields) < 24 {
		return snap, fmt.Errorf("unexpected /proc/pid/stat format")
	}

	// utime + stime (fields 14,15 in 1-indexed = 13,14 in 0-indexed)
	utime, _ := strconv.ParseInt(fields[13], 10, 64)
	stime, _ := strconv.ParseInt(fields[14], 10, 64)
	totalTicks := utime + stime

	// Read /proc/[pid]/status for memory
	statusData, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	for _, line := range strings.Split(string(statusData), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, _ := strconv.ParseInt(parts[1], 10, 64)
				snap.MemoryMB = kb / 1024
			}
		}
	}

	snap.CPUPct = float64(totalTicks) / 100.0 // approximate
	snap.Timestamp = time.Now()
	return snap, nil
}
