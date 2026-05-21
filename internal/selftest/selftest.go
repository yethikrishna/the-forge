// Package selftest provides agent self-diagnostic and health check capabilities.
// It verifies that all subsystems are functioning correctly, measures
// response times, and produces a health report.
//
// "Before you trust the agent, let the agent prove itself."
package selftest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Status represents the result of a diagnostic check.
type Status string

const (
	StatusPass    Status = "PASS"
	StatusWarn    Status = "WARN"
	StatusFail    Status = "FAIL"
	StatusSkip    Status = "SKIP"
	StatusTimeout Status = "TIMEOUT"
)

// Category represents a diagnostic category.
type Category string

const (
	CatCore       Category = "core"
	CatRuntime    Category = "runtime"
	CatNetwork    Category = "network"
	CatStorage    Category = "storage"
	CatSecurity   Category = "security"
	CatDependency Category = "dependency"
	CatAgent      Category = "agent"
	CatBuild      Category = "build"
)

// Check represents a single diagnostic check.
type Check struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Status      Status   `json:"status"`
	Duration    time.Duration `json:"duration"`
	Message     string   `json:"message,omitempty"`
	Detail      string   `json:"detail,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
	Critical    bool     `json:"critical"`
}

// Report represents a complete self-test report.
type Report struct {
	Timestamp  time.Time `json:"timestamp"`
	Version    string    `json:"version"`
	GoVersion  string    `json:"go_version"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
	Hostname   string    `json:"hostname"`
	Checks     []*Check  `json:"checks"`
	Summary    *Summary  `json:"summary"`
	Passed     bool      `json:"passed"`
}

// Summary holds aggregated results.
type Summary struct {
	Total   int            `json:"total"`
	Pass    int            `json:"pass"`
	Warn    int            `json:"warn"`
	Fail    int            `json:"fail"`
	Skip    int            `json:"skip"`
	Timeout int            `json:"timeout"`
	ByCategory map[Category]int `json:"by_category,omitempty"`
	Duration   time.Duration    `json:"duration"`
}

// Runner executes diagnostic checks.
type Runner struct {
	mu       sync.Mutex
	checks   []CheckFunc
	timeout  time.Duration
	version  string
	baseDir  string
}

// CheckFunc is a function that performs a diagnostic check.
type CheckFunc func(ctx context.Context) *Check

// NewRunner creates a new self-test runner.
func NewRunner(version, baseDir string) *Runner {
	r := &Runner{
		timeout: 30 * time.Second,
		version: version,
		baseDir: baseDir,
	}
	r.registerDefaultChecks()
	return r
}

// SetTimeout configures the per-check timeout.
func (r *Runner) SetTimeout(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.timeout = d
}

// AddCheck registers a custom diagnostic check.
func (r *Runner) AddCheck(fn CheckFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = append(r.checks, fn)
}

// Run executes all registered checks and returns a report.
func (r *Runner) Run(ctx context.Context) *Report {
	r.mu.Lock()
	checks := make([]CheckFunc, len(r.checks))
	copy(checks, r.checks)
	timeout := r.timeout
	r.mu.Unlock()

	report := &Report{
		Timestamp: time.Now(),
		Version:   r.version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	hostname, _ := os.Hostname()
	report.Hostname = hostname

	start := time.Now()

	var wg sync.WaitGroup
	results := make(chan *Check, len(checks))

	for i, checkFn := range checks {
		wg.Add(1)
		go func(idx int, fn CheckFunc) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			done := make(chan *Check, 1)
			go func() {
				done <- fn(checkCtx)
			}()

			select {
			case result := <-done:
				results <- result
			case <-checkCtx.Done():
				results <- &Check{
					ID:       fmt.Sprintf("check-%03d", idx),
					Status:   StatusTimeout,
					Message:  "check timed out",
					Critical: true,
				}
			}
		}(i, checkFn)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for check := range results {
		report.Checks = append(report.Checks, check)
	}

	report.Summary = computeSummary(report.Checks, time.Since(start))
	report.Passed = report.Summary.Fail == 0 && report.Summary.Timeout == 0

	// Sort checks by category then ID
	sort.Slice(report.Checks, func(i, j int) bool {
		if report.Checks[i].Category != report.Checks[j].Category {
			return report.Checks[i].Category < report.Checks[j].Category
		}
		return report.Checks[i].ID < report.Checks[j].ID
	})

	return report
}

func computeSummary(checks []*Check, totalDuration time.Duration) *Summary {
	s := &Summary{
		Total:      len(checks),
		ByCategory: make(map[Category]int),
		Duration:   totalDuration,
	}

	for _, c := range checks {
		switch c.Status {
		case StatusPass:
			s.Pass++
		case StatusWarn:
			s.Warn++
		case StatusFail:
			s.Fail++
		case StatusSkip:
			s.Skip++
		case StatusTimeout:
			s.Timeout++
		}
		s.ByCategory[c.Category]++
	}

	return s
}

// Default checks

func (r *Runner) registerDefaultChecks() {
	r.checks = []CheckFunc{
		r.checkGoVersion,
		r.checkMemory,
		r.checkGoroutines,
		r.checkDiskSpace,
		r.checkBuild,
		r.checkGoMod,
		r.checkNetworkDNS,
		r.checkFileSystemPerms,
		r.checkCGO,
	}
}

func (r *Runner) checkGoVersion(_ context.Context) *Check {
	c := &Check{
		ID:       "go-version",
		Name:     "Go Version",
		Category: CatRuntime,
		Critical: true,
	}

	out, err := exec.Command("go", "version").Output()
	if err != nil {
		c.Status = StatusFail
		c.Message = fmt.Sprintf("go not found: %v", err)
		c.Suggestion = "Install Go 1.21 or later"
		return c
	}

	c.Status = StatusPass
	c.Message = string(out[:len(out)-1]) // trim newline
	return c
}

func (r *Runner) checkMemory(_ context.Context) *Check {
	c := &Check{
		ID:       "memory",
		Name:     "Memory Stats",
		Category: CatRuntime,
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB := m.Alloc / 1024 / 1024
	sysMB := m.Sys / 1024 / 1024

	c.Status = StatusPass
	c.Message = fmt.Sprintf("Alloc: %dMB, Sys: %dMB, GC: %d", allocMB, sysMB, m.NumGC)
	c.Detail = fmt.Sprintf("HeapAlloc=%d HeapSys=%d HeapIdle=%d StackInUse=%d",
		m.HeapAlloc, m.HeapSys, m.HeapIdle, m.StackInuse)

	if sysMB > 1024 {
		c.Status = StatusWarn
		c.Suggestion = "High memory usage — consider reducing agent concurrency"
	}

	return c
}

func (r *Runner) checkGoroutines(_ context.Context) *Check {
	c := &Check{
		ID:       "goroutines",
		Name:     "Goroutine Count",
		Category: CatRuntime,
	}

	count := runtime.NumGoroutine()
	c.Message = fmt.Sprintf("%d goroutines", count)

	if count > 1000 {
		c.Status = StatusWarn
		c.Suggestion = "High goroutine count — possible leak"
	} else if count > 5000 {
		c.Status = StatusFail
		c.Suggestion = "Excessive goroutines — likely goroutine leak"
	} else {
		c.Status = StatusPass
	}

	return c
}

func (r *Runner) checkDiskSpace(_ context.Context) *Check {
	c := &Check{
		ID:       "disk-space",
		Name:     "Disk Space",
		Category: CatStorage,
	}

	var stat syscallStat
	if err := getDiskStats(r.baseDir, &stat); err != nil {
		c.Status = StatusWarn
		c.Message = fmt.Sprintf("Could not check disk: %v", err)
		return c
	}

	freeGB := stat.Free / 1024 / 1024 / 1024
	totalGB := stat.Total / 1024 / 1024 / 1024
	c.Message = fmt.Sprintf("%dGB free / %dGB total", freeGB, totalGB)

	if freeGB < 1 {
		c.Status = StatusFail
		c.Suggestion = "Less than 1GB free — agent may fail"
	} else if freeGB < 5 {
		c.Status = StatusWarn
		c.Suggestion = "Low disk space — consider cleanup"
	} else {
		c.Status = StatusPass
	}

	return c
}

func (r *Runner) checkBuild(_ context.Context) *Check {
	c := &Check{
		ID:       "build",
		Name:     "Build Check",
		Category: CatBuild,
		Critical: true,
	}

	start := time.Now()
	out, err := exec.Command("go", "build", "./...").CombinedOutput()
	c.Duration = time.Since(start)

	if err != nil {
		c.Status = StatusFail
		c.Message = "Build failed"
		c.Detail = string(out)
		c.Suggestion = "Fix build errors before deploying"
	} else {
		c.Status = StatusPass
		c.Message = fmt.Sprintf("Build passed in %s", c.Duration.Round(time.Millisecond))
	}

	return c
}

func (r *Runner) checkGoMod(_ context.Context) *Check {
	c := &Check{
		ID:       "go-mod",
		Name:     "Go Module Status",
		Category: CatDependency,
	}

	out, err := exec.Command("go", "mod", "verify").CombinedOutput()
	if err != nil {
		c.Status = StatusFail
		c.Message = "Module verification failed"
		c.Detail = string(out)
		c.Suggestion = "Run 'go mod tidy' and check for corrupted dependencies"
	} else {
		c.Status = StatusPass
		c.Message = "All modules verified"
	}

	return c
}

func (r *Runner) checkNetworkDNS(_ context.Context) *Check {
	c := &Check{
		ID:       "dns",
		Name:     "DNS Resolution",
		Category: CatNetwork,
	}

	start := time.Now()
	out, err := exec.Command("nslookup", "github.com").CombinedOutput()
	c.Duration = time.Since(start)

	if err != nil {
		c.Status = StatusWarn
		c.Message = "DNS resolution failed"
		c.Detail = string(out)
		c.Suggestion = "Check network connectivity"
	} else {
		c.Status = StatusPass
		c.Message = fmt.Sprintf("DNS resolved in %s", c.Duration.Round(time.Millisecond))
	}

	return c
}

func (r *Runner) checkFileSystemPerms(_ context.Context) *Check {
	c := &Check{
		ID:       "fs-perms",
		Name:     "File System Permissions",
		Category: CatStorage,
	}

	testFile := fmt.Sprintf("%s/.selftest-perm-check", r.baseDir)
	err := os.WriteFile(testFile, []byte("test"), 0o644)
	if err != nil {
		c.Status = StatusFail
		c.Message = fmt.Sprintf("Cannot write to %s: %v", r.baseDir, err)
		c.Suggestion = "Check directory permissions"
	} else {
		os.Remove(testFile)
		c.Status = StatusPass
		c.Message = "Read/write access confirmed"
	}

	return c
}

func (r *Runner) checkCGO(_ context.Context) *Check {
	c := &Check{
		ID:       "cgo",
		Name:     "CGO Status",
		Category: CatCore,
	}

	out, err := exec.Command("go", "env", "CGO_ENABLED").Output()
	if err != nil {
		c.Status = StatusSkip
		c.Message = "Could not determine CGO status"
		return c
	}

	c.Message = fmt.Sprintf("CGO_ENABLED=%s", string(out[:len(out)-1]))
	c.Status = StatusPass
	return c
}
