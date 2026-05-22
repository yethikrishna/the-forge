// Package orghealth provides organizational wellness checks, internal politics
// detection, bloat/garbage-collection identification, and effectiveness metrics.
// It closes the gap in organizational health intelligence — enabling the Forge
// to diagnose and treat organizational ailments before they become fatal.
package orghealth

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// HealthCheck represents a wellness assessment of an organization.
type HealthCheck struct {
	ID          string             `json:"id"`
	OrgID       string             `json:"org_id"`
	Category    string             `json:"category"` // culture, process, tech, people, financial
	Status      string             `json:"status"`   // healthy, warning, critical
	Score       float64            `json:"score"`    // 0-1
	Findings    []string           `json:"findings"`
	CheckedAt   time.Time          `json:"checked_at"`
}

// PoliticsSignal detects internal political dynamics.
type PoliticsSignal struct {
	ID          string    `json:"id"`
	SignalType  string    `json:"signal_type"` // empire_building, information_hoarding, credit_theft, faction_forming
	Strength    float64   `json:"strength"`    // 0-1
	Description string    `json:"description"`
	Actor       string    `json:"actor"`
	DetectedAt  time.Time `json:"detected_at"`
	Severity    string    `json:"severity"` // low, medium, high
}

// BloatIndicator identifies organizational bloat.
type BloatIndicator struct {
	ID            string    `json:"id"`
	Area          string    `json:"area"` // meetings, processes, layers, tools, reports
	Indicator     string    `json:"indicator"`
	BloatScore    float64   `json:"bloat_score"` // 0-1
	Impact        string    `json:"impact"`
	Recommendation string   `json:"recommendation"`
	DetectedAt    time.Time `json:"detected_at"`
}

// EffectivenessMetric measures organizational effectiveness.
type EffectivenessMetric struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Metric       string    `json:"metric"`
	Value        float64   `json:"value"`
	Target       float64   `json:"target"`
	Unit         string    `json:"unit"`
	Trend        string    `json:"trend"` // improving, stable, declining
	MeasuredAt   time.Time `json:"measured_at"`
}

// OrgHealthScore is a composite organizational health score.
type OrgHealthScore struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	CultureScore    float64   `json:"culture_score"`   // 0-1
	ProcessScore    float64   `json:"process_score"`
	PeopleScore     float64   `json:"people_score"`
	FinancialScore  float64   `json:"financial_score"`
	CompositeScore  float64   `json:"composite_score"`
	Status          string    `json:"status"` // thriving, healthy, strained, distressed
	AssessedAt      time.Time `json:"assessed_at"`
}

// HealthReport is a consolidated organizational health report.
type HealthReport struct {
	GeneratedAt       time.Time          `json:"generated_at"`
	HealthChecks      []HealthCheck      `json:"health_checks"`
	PoliticsSignals   []PoliticsSignal   `json:"politics_signals"`
	BloatIndicators   []BloatIndicator   `json:"bloat_indicators"`
	EffectivenessMetrics []EffectivenessMetric `json:"effectiveness_metrics"`
	OrgHealthScores   []OrgHealthScore   `json:"org_health_scores"`
}

// Store persists org health data to a JSON file with thread safety.
type Store struct {
	mu                  sync.Mutex
	filePath            string
	HealthChecks        []HealthCheck        `json:"health_checks"`
	PoliticsSignals     []PoliticsSignal     `json:"politics_signals"`
	BloatIndicators     []BloatIndicator     `json:"bloat_indicators"`
	EffectivenessMetrics []EffectivenessMetric `json:"effectiveness_metrics"`
	OrgHealthScores     []OrgHealthScore     `json:"org_health_scores"`
}

// NewStore creates a new Store backed by the given file path.
func NewStore(filePath string) *Store {
	return &Store{filePath: filePath}
}

// Load reads data from the backing file.
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

// Save writes data to the backing file.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// RunWellnessCheck performs a health check on a category.
func RunWellnessCheck(orgID, category string, metrics map[string]float64) HealthCheck {
	score := 0.0
	count := 0
	for _, v := range metrics {
		score += v
		count++
	}
	if count > 0 {
		score /= float64(count)
	}

	status := "healthy"
	switch {
	case score >= 0.7:
		status = "healthy"
	case score >= 0.4:
		status = "warning"
	default:
		status = "critical"
	}

	var findings []string
	if score < 0.5 {
		findings = append(findings, "Score below acceptable threshold")
	}
	for k, v := range metrics {
		if v < 0.3 {
			findings = append(findings, k+" is critically low")
		}
	}

	return HealthCheck{
		ID:        genID("hc"),
		OrgID:     orgID,
		Category:  category,
		Status:    status,
		Score:     score,
		Findings:  findings,
		CheckedAt: time.Now(),
	}
}

// DetectPolitics identifies political dynamics from behavioral signals.
func DetectPolitics(signals map[string]float64) []PoliticsSignal {
	var results []PoliticsSignal

	type threshold struct {
		key       string
		sigType   string
		desc      string
		threshold float64
	}

	checks := []threshold{
		{"empire_building", "empire_building", "Team growing without clear need", 0.4},
		{"info_hoarding", "information_hoarding", "Information silos detected", 0.3},
		{"credit_theft", "credit_theft", "Credit misattribution patterns", 0.2},
		{"faction_forming", "faction_forming", "Faction formation detected", 0.35},
	}

	for _, c := range checks {
		if val, ok := signals[c.key]; ok && val >= c.threshold {
			severity := "low"
			if val > 0.7 {
				severity = "high"
			} else if val > 0.5 {
				severity = "medium"
			}
			results = append(results, PoliticsSignal{
				ID:          genID("ps"),
				SignalType:  c.sigType,
				Strength:    val,
				Description: c.desc,
				DetectedAt:  time.Now(),
				Severity:    severity,
			})
		}
	}

	return results
}

// IdentifyBloat detects organizational bloat indicators.
func IdentifyBloat(area, indicator, impact, recommendation string, score float64) BloatIndicator {
	return BloatIndicator{
		ID:              genID("bi"),
		Area:            area,
		Indicator:       indicator,
		BloatScore:      score,
		Impact:          impact,
		Recommendation:  recommendation,
		DetectedAt:      time.Now(),
	}
}

// MeasureEffectiveness records an effectiveness metric.
func MeasureEffectiveness(orgID, metric, unit, trend string, value, target float64) EffectivenessMetric {
	return EffectivenessMetric{
		ID:         genID("em"),
		OrgID:      orgID,
		Metric:     metric,
		Value:      value,
		Target:     target,
		Unit:       unit,
		Trend:      trend,
		MeasuredAt: time.Now(),
	}
}

// CollectGarbage identifies bloat indicators above a threshold and returns cleanup recommendations.
func CollectGarbage(indicators []BloatIndicator, threshold float64) []BloatIndicator {
	var garbage []BloatIndicator
	for _, bi := range indicators {
		if bi.BloatScore >= threshold {
			garbage = append(garbage, bi)
		}
	}
	return garbage
}

// GenerateHealthReport produces a consolidated health report.
func GenerateHealthReport(s *Store) HealthReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return HealthReport{
		GeneratedAt:        time.Now(),
		HealthChecks:       s.HealthChecks,
		PoliticsSignals:    s.PoliticsSignals,
		BloatIndicators:    s.BloatIndicators,
		EffectivenessMetrics: s.EffectivenessMetrics,
		OrgHealthScores:    s.OrgHealthScores,
	}
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
