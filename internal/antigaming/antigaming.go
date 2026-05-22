// Package antigaming provides random audits, red team capabilities, and
// metric gaming detection (Goodhart's law). It closes the gap in metric
// integrity by detecting when optimization targets are being gamed rather
// than genuinely improved, running red team exercises, and performing random
// audits—ensuring organizational metrics reflect reality, not manipulation.
package antigaming

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// AuditStatus represents the state of an audit.
type AuditStatus string

const (
	AuditScheduled AuditStatus = "scheduled"
	AuditRunning   AuditStatus = "running"
	AuditComplete  AuditStatus = "complete"
	AuditFailed    AuditStatus = "failed"
)

// GamingSeverity represents how severe gaming detection is.
type GamingSeverity string

const (
	GamingLow      GamingSeverity = "low"
	GamingMedium   GamingSeverity = "medium"
	GamingHigh     GamingSeverity = "high"
	GamingCritical GamingSeverity = "critical"
)

// MetricAudit represents a random audit of a metric or process.
type MetricAudit struct {
	ID          string      `json:"id"`
	Target      string      `json:"target"`
	Type        string      `json:"type"` // metric, process, behavior
	Status      AuditStatus `json:"status"`
	Auditor     string      `json:"auditor"`
	Findings    []string    `json:"findings"`
	Score       float64     `json:"score"` // 0.0-1.0, 1.0 = clean
	ScheduledAt time.Time   `json:"scheduled_at"`
	CompletedAt time.Time   `json:"completed_at"`
	CreatedAt   time.Time   `json:"created_at"`
}

// GamingDetection represents a detected instance of metric gaming.
type GamingDetection struct {
	ID           string         `json:"id"`
	MetricName   string         `json:"metric_name"`
	Severity     GamingSeverity `json:"severity"`
	Description  string         `json:"description"`
	Evidence     []string       `json:"evidence"`
	SuspectedCause string       `json:"suspected_cause"`
	DetectedAt   time.Time      `json:"detected_at"`
	Resolved     bool           `json:"resolved"`
	Resolution   string         `json:"resolution"`
}

// RedTeamFinding represents a finding from a red team exercise.
type RedTeamFinding struct {
	ID          string    `json:"id"`
	ExerciseID  string    `json:"exercise_id"`
	Target      string    `json:"target"`
	Finding     string    `json:"finding"`
	Impact      string    `json:"impact"`
	Mitigation  string    `json:"mitigation"`
	Severity    GamingSeverity `json:"severity"`
	IsFixed     bool      `json:"is_fixed"`
	CreatedAt   time.Time `json:"created_at"`
}

// GoodhartScan represents a scan for Goodhart's law violations.
type GoodhartScan struct {
	ID              string    `json:"id"`
	TargetMetric    string    `json:"target_metric"`
	ProxyMetric     string    `json:"proxy_metric"`
	CorrelationScore float64  `json:"correlation_score"` // 0.0-1.0, low = likely gaming
	DivergenceDetected bool   `json:"divergence_detected"`
	Description     string    `json:"description"`
	ScannedAt       time.Time `json:"scanned_at"`
}

// Store persists anti-gaming data.
type Store struct {
	mu        sync.Mutex
	filePath  string
	Audits    map[string]MetricAudit     `json:"audits"`
	Detections map[string]GamingDetection `json:"detections"`
	RedTeamFindings map[string]RedTeamFinding `json:"red_team_findings"`
	GoodhartScans map[string]GoodhartScan `json:"goodhart_scans"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath:       filePath,
		Audits:         make(map[string]MetricAudit),
		Detections:     make(map[string]GamingDetection),
		RedTeamFindings: make(map[string]RedTeamFinding),
		GoodhartScans:  make(map[string]GoodhartScan),
	}
}

// Load reads the store from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, s)
}

// Save writes the store to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// RunAudit creates and completes a metric audit.
func (s *Store) RunAudit(a MetricAudit) MetricAudit {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.CreatedAt = time.Now().UTC()
	if a.Status == "" {
		a.Status = AuditScheduled
	}
	s.Audits[a.ID] = a
	return a
}

// DetectGaming records a gaming detection event.
func (s *Store) DetectGaming(d GamingDetection) GamingDetection {
	s.mu.Lock()
	defer s.mu.Unlock()
	d.DetectedAt = time.Now().UTC()
	s.Detections[d.ID] = d
	return d
}

// ScanGoodhart performs a Goodhart's law scan on a metric-proxy pair.
func (s *Store) ScanGoodhart(scan GoodhartScan) GoodhartScan {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan.ScannedAt = time.Now().UTC()
	// Low correlation = divergence = potential Goodhart violation
	if scan.CorrelationScore < 0.5 {
		scan.DivergenceDetected = true
	}
	s.GoodhartScans[scan.ID] = scan
	return scan
}

// GenerateAntiGamingReport produces a summary of anti-gaming state.
func (s *Store) GenerateAntiGamingReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	unresolvedDetections := 0
	criticalDetections := 0
	for _, d := range s.Detections {
		if !d.Resolved {
			unresolvedDetections++
			if d.Severity == GamingCritical {
				criticalDetections++
			}
		}
	}
	goodhartViolations := 0
	for _, gs := range s.GoodhartScans {
		if gs.DivergenceDetected {
			goodhartViolations++
		}
	}
	unfixedFindings := 0
	for _, f := range s.RedTeamFindings {
		if !f.IsFixed {
			unfixedFindings++
		}
	}
	avgAuditScore := 0.0
	completedAudits := 0
	for _, a := range s.Audits {
		if a.Status == AuditComplete {
			avgAuditScore += a.Score
			completedAudits++
		}
	}
	if completedAudits > 0 {
		avgAuditScore /= float64(completedAudits)
	}
	return map[string]interface{}{
		"total_audits":          len(s.Audits),
		"avg_audit_score":       avgAuditScore,
		"unresolved_detections": unresolvedDetections,
		"critical_detections":   criticalDetections,
		"goodhart_violations":   goodhartViolations,
		"unfixed_red_team":      unfixedFindings,
	}
}
