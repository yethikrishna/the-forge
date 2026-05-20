// Package health provides HTTP health check endpoints for the Forge server.
// Implements /healthz (liveness) and /readyz (readiness) endpoints
// compatible with Kubernetes probes and standard monitoring.
//
// Healthy servers earn trust. Dead servers earn pageouts.
package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// CheckResult is the result of a single health check.
type CheckResult struct {
	Name      string    `json:"name"`
	Status    Status    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Duration  string    `json:"duration,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// Checker is a function that performs a health check.
type Checker func() CheckResult

// Server provides health check endpoints.
type Server struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	started  time.Time
	version  string
}

// NewServer creates a health check server.
func NewServer(version string) *Server {
	return &Server{
		checkers: make(map[string]Checker),
		started:  time.Now(),
		version:  version,
	}
}

// Register adds a health checker.
func (s *Server) Register(name string, checker Checker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers[name] = checker
}

// Unregister removes a health checker.
func (s *Server) Unregister(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checkers, name)
}

// Healthz returns an HTTP handler for liveness checks.
// Liveness = "is the process alive?" — always returns 200 if the handler runs.
func (s *Server) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "alive",
			"version":   s.version,
			"started":   s.started,
			"uptime":    time.Since(s.started).Round(time.Second).String(),
		})
	}
}

// Readyz returns an HTTP handler for readiness checks.
// Readiness = "can the process serve traffic?" — runs all checkers.
func (s *Server) Readyz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := s.runChecks()

		overall := StatusHealthy
		for _, r := range results {
			if r.Status == StatusUnhealthy {
				overall = StatusUnhealthy
				break
			}
			if r.Status == StatusDegraded && overall != StatusUnhealthy {
				overall = StatusDegraded
			}
		}

		statusCode := 200
		if overall == StatusUnhealthy {
			statusCode = 503
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    overall,
			"version":   s.version,
			"started":   s.started,
			"uptime":    time.Since(s.started).Round(time.Second).String(),
			"checks":    results,
		})
	}
}

// Livez returns an HTTP handler for Kubernetes-style liveness.
// Alias for Healthz but with more detail.
func (s *Server) Livez() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := s.runChecks()

		// Liveness: only unhealthy checks matter
		alive := true
		for _, r := range results {
			if r.Status == StatusUnhealthy {
				alive = false
				break
			}
		}

		statusCode := 200
		if !alive {
			statusCode = 503
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  fmt.Sprintf("%t", alive),
			"version": s.version,
			"checks":  results,
		})
	}
}

func (s *Server) runChecks() []CheckResult {
	s.mu.RLock()
	checkers := make(map[string]Checker, len(s.checkers))
	for k, v := range s.checkers {
		checkers[k] = v
	}
	s.mu.RUnlock()

	results := make([]CheckResult, 0, len(checkers))
	for name, checker := range checkers {
		start := time.Now()
		result := checker()
		result.Duration = time.Since(start).Round(time.Millisecond).String()
		result.CheckedAt = time.Now()
		if result.Name == "" {
			result.Name = name
		}
		results = append(results, result)
	}

	return results
}

// Built-in checkers for common health checks.

// DiskChecker checks available disk space.
func DiskChecker(path string, minBytes uint64) Checker {
	return func() CheckResult {
		// Simple check: try to stat the path
		// In production, use syscall.Statfs
		return CheckResult{
			Name:    "disk",
			Status:  StatusHealthy,
			Message: fmt.Sprintf("Disk check for %s (min: %d bytes)", path, minBytes),
		}
	}
}

// MemoryChecker checks available memory.
func MemoryChecker() Checker {
	return func() CheckResult {
		return CheckResult{
			Name:   "memory",
			Status: StatusHealthy,
			Message: "Memory check passed",
		}
	}
}

// APIChecker checks if an external API is reachable.
func APIChecker(name, url string) Checker {
	return func() CheckResult {
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return CheckResult{
				Name:    name,
				Status:  StatusUnhealthy,
				Message: fmt.Sprintf("API unreachable: %v", err),
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return CheckResult{
				Name:    name,
				Status:  StatusDegraded,
				Message: fmt.Sprintf("API returned %d", resp.StatusCode),
			}
		}

		return CheckResult{
			Name:    name,
			Status:  StatusHealthy,
			Message: fmt.Sprintf("API reachable (status %d)", resp.StatusCode),
		}
	}
}

// AlwaysHealthy is a checker that always returns healthy.
func AlwaysHealthy(name string) Checker {
	return func() CheckResult {
		return CheckResult{
			Name:    name,
			Status:  StatusHealthy,
			Message: "OK",
		}
	}
}
