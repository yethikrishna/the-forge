// Package investor provides traction metrics collection, narrative generation,
// and investor-ready reporting. It closes the gap between raw operational data
// and the story investors need to hear: where momentum is building, what the
// numbers mean in context, and how to frame progress as a compelling arc.
package investor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// MetricPeriod represents a reporting period.
type MetricPeriod string

const (
	PeriodWeekly  MetricPeriod = "weekly"
	PeriodMonthly MetricPeriod = "monthly"
	PeriodQuarterly MetricPeriod = "quarterly"
)

// NarrativeStage represents where the company is in its story arc.
type NarrativeStage string

const (
	StageProblem     NarrativeStage = "problem"     // "Here's the pain"
	StageSolution    NarrativeStage = "solution"    // "Here's how we fix it"
	StageTraction    NarrativeStage = "traction"    // "Look at the numbers"
	StageScale       NarrativeStage = "scale"       // "Now we're growing fast"
	StageMoat        NarrativeStage = "moat"        // "Here's why we're defensible"
)

// TractionMetrics captures key business metrics.
type TractionMetrics struct {
	ID            string       `json:"id"`
	Period        MetricPeriod `json:"period"`
	PeriodStart   time.Time    `json:"period_start"`
	PeriodEnd     time.Time    `json:"period_end"`
	MRR           float64      `json:"mrr"`             // Monthly Recurring Revenue
	ARR           float64      `json:"arr"`             // Annual Recurring Revenue
	TotalUsers    int64        `json:"total_users"`
	NewUsers      int64        `json:"new_users"`
	ChurnRate     float64      `json:"churn_rate"`      // 0-1
	GrowthRate    float64      `json:"growth_rate"`     // user growth rate 0-1+
	NRR           float64      `json:"nrr"`             // Net Revenue Retention
	BurnRate      float64      `json:"burn_rate"`       // monthly burn
	RunwayMonths  float64      `json:"runway_months"`
	LTV           float64      `json:"ltv"`             // Lifetime Value
	CAC           float64      `json:"cac"`             // Customer Acquisition Cost
	CustomMetrics map[string]float64 `json:"custom_metrics,omitempty"`
	RecordedAt    time.Time    `json:"recorded_at"`
}

// Narrative represents a story arc for investors.
type Narrative struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Stage       NarrativeStage `json:"stage"`
	KeyMessage  string         `json:"key_message"`
	SupportingPoints []string  `json:"supporting_points"`
	CounterArguments []string  `json:"counter_arguments,omitempty"` // preempt objections
	MetricsRef  string         `json:"metrics_ref,omitempty"`       // ID of supporting metrics
	CreatedAt   time.Time      `json:"created_at"`
}

// PitchSection is a section of an investor pitch.
type PitchSection struct {
	ID        string `json:"id"`
	DeckID    string `json:"deck_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Order     int    `json:"order"`
	Type      string `json:"type"` // "problem", "solution", "market", "traction", "team", "ask"
	CreatedAt time.Time `json:"created_at"`
}

// InvestorReport is a complete investor-ready report.
type InvestorReport struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Summary     string    `json:"summary"`
	Highlights  []string  `json:"highlights"`
	Risks       []string  `json:"risks"`
	Ask         string    `json:"ask,omitempty"`
	MetricsID   string    `json:"metrics_id,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
}

// InvestorHub manages traction, narratives, and reporting.
type InvestorHub struct {
	mu       sync.RWMutex
	metrics  map[string]*TractionMetrics
	narratives map[string]*Narrative
	pitchSections map[string]*PitchSection
	reports  map[string]*InvestorReport
	path     string
}

// NewInvestorHub creates a new InvestorHub store.
func NewInvestorHub(persistPath string) *InvestorHub {
	ih := &InvestorHub{
		metrics:       make(map[string]*TractionMetrics),
		narratives:    make(map[string]*Narrative),
		pitchSections: make(map[string]*PitchSection),
		reports:       make(map[string]*InvestorReport),
		path:          persistPath,
	}
	ih.load()
	return ih
}

// --- Metrics ---

// CollectMetrics records a new set of traction metrics.
func (ih *InvestorHub) CollectMetrics(period MetricPeriod, periodStart, periodEnd time.Time, mrr, arr float64, totalUsers, newUsers int64, churnRate, growthRate, nrr, burnRate, runwayMonths, ltv, cac float64, custom map[string]float64) (*TractionMetrics, error) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	m := &TractionMetrics{
		ID:            genID("metrics"),
		Period:        period,
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		MRR:           mrr,
		ARR:           arr,
		TotalUsers:    totalUsers,
		NewUsers:      newUsers,
		ChurnRate:     churnRate,
		GrowthRate:    growthRate,
		NRR:           nrr,
		BurnRate:      burnRate,
		RunwayMonths:  runwayMonths,
		LTV:           ltv,
		CAC:           cac,
		CustomMetrics: custom,
		RecordedAt:    time.Now().UTC(),
	}
	ih.metrics[m.ID] = m
	ih.persist()
	return m, nil
}

// GetLatestMetrics returns the most recent metrics for a period type.
func (ih *InvestorHub) GetLatestMetrics(period MetricPeriod) (*TractionMetrics, error) {
	ih.mu.RLock()
	defer ih.mu.RUnlock()

	var latest *TractionMetrics
	for _, m := range ih.metrics {
		if m.Period != period {
			continue
		}
		if latest == nil || m.RecordedAt.After(latest.RecordedAt) {
			latest = m
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no %s metrics found", period)
	}
	return latest, nil
}

// --- Narrative ---

// GenerateNarrative creates a narrative based on current metrics.
func (ih *InvestorHub) GenerateNarrative(title string, metricsID string) (*Narrative, error) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	var m *TractionMetrics
	if metricsID != "" {
		m = ih.metrics[metricsID]
	}

	stage := StageProblem
	keyMessage := "We've identified a significant pain point in the market."
	var supportingPoints, counterArguments []string

	if m != nil {
		switch {
		case m.GrowthRate >= 0.2 && m.NRR >= 1.2:
			stage = StageScale
			keyMessage = "We're scaling rapidly with strong retention."
			supportingPoints = append(supportingPoints,
				fmt.Sprintf("%.0f%% MoM user growth", m.GrowthRate*100),
				fmt.Sprintf("%.0f%% Net Revenue Retention", m.NRR*100),
			)
			counterArguments = append(counterArguments, "Can this growth rate be sustained?")
		case m.GrowthRate >= 0.1 && m.MRR > 0:
			stage = StageTraction
			keyMessage = "We have strong traction and growing revenue."
			supportingPoints = append(supportingPoints,
				fmt.Sprintf("$%.0f MRR", m.MRR),
				fmt.Sprintf("%.0f%% growth rate", m.GrowthRate*100),
				fmt.Sprintf("%d total users", m.TotalUsers),
			)
			counterArguments = append(counterArguments, "Is unit economics sustainable?")
		case m.TotalUsers > 0:
			stage = StageSolution
			keyMessage = "Our solution is resonating with early users."
			supportingPoints = append(supportingPoints,
				fmt.Sprintf("%d total users", m.TotalUsers),
				fmt.Sprintf("%d new this period", m.NewUsers),
			)
			counterArguments = append(counterArguments, "Are users engaged or just signed up?")
		}

		if m.ChurnRate > 0.1 {
			counterArguments = append(counterArguments, fmt.Sprintf("Churn at %.0f%% is concerning", m.ChurnRate*100))
		}
		if m.LTV > 0 && m.CAC > 0 {
			ratio := m.LTV / m.CAC
			supportingPoints = append(supportingPoints, fmt.Sprintf("LTV:CAC ratio of %.1f:1", ratio))
		}
	}

	n := &Narrative{
		ID:               genID("narr"),
		Title:            title,
		Stage:            stage,
		KeyMessage:       keyMessage,
		SupportingPoints: supportingPoints,
		CounterArguments: counterArguments,
		MetricsRef:       metricsID,
		CreatedAt:        time.Now().UTC(),
	}
	ih.narratives[n.ID] = n
	ih.persist()
	return n, nil
}

// --- Pitch Deck ---

// BuildPitchDeck generates pitch deck sections based on current data.
func (ih *InvestorHub) BuildPitchDeck(metricsID string) ([]*PitchSection, error) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	deckID := genID("deck")
	var sections []*PitchSection

	// Problem section
	sections = append(sections, &PitchSection{
		ID: genID("sec"), DeckID: deckID, Title: "The Problem",
		Content: "Current solutions are fragmented, slow, and don't leverage AI-native approaches.",
		Order: 1, Type: "problem", CreatedAt: time.Now().UTC(),
	})

	// Solution section
	sections = append(sections, &PitchSection{
		ID: genID("sec"), DeckID: deckID, Title: "Our Solution",
		Content: "An AI-native platform that automates complex workflows end-to-end.",
		Order: 2, Type: "solution", CreatedAt: time.Now().UTC(),
	})

	// Traction section (with real data if available)
	tractionContent := "Early stage — collecting metrics."
	if m, ok := ih.metrics[metricsID]; ok {
		tractionContent = fmt.Sprintf("MRR: $%.0f | Users: %d | Growth: %.0f%% | NRR: %.0f%%",
			m.MRR, m.TotalUsers, m.GrowthRate*100, m.NRR*100)
	}
	sections = append(sections, &PitchSection{
		ID: genID("sec"), DeckID: deckID, Title: "Traction",
		Content: tractionContent, Order: 3, Type: "traction", CreatedAt: time.Now().UTC(),
	})

	// Market section
	sections = append(sections, &PitchSection{
		ID: genID("sec"), DeckID: deckID, Title: "Market Opportunity",
		Content: "Large and growing market with significant pain points that existing solutions fail to address.",
		Order: 4, Type: "market", CreatedAt: time.Now().UTC(),
	})

	// Team section
	sections = append(sections, &PitchSection{
		ID: genID("sec"), DeckID: deckID, Title: "Team",
		Content: "Experienced founding team with deep domain expertise.",
		Order: 5, Type: "team", CreatedAt: time.Now().UTC(),
	})

	for _, s := range sections {
		ih.pitchSections[s.ID] = s
	}
	ih.persist()
	return sections, nil
}

// --- Reports ---

// GenerateInvestorReport creates a comprehensive investor report.
func (ih *InvestorHub) GenerateInvestorReport(title string, periodStart, periodEnd time.Time) (*InvestorReport, error) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	var latest *TractionMetrics
	for _, m := range ih.metrics {
		if m.RecordedAt.After(periodStart) && m.RecordedAt.Before(periodEnd.Add(time.Minute)) {
			if latest == nil || m.RecordedAt.After(latest.RecordedAt) {
				latest = m
			}
		}
	}

	summary := "Operational period completed. See highlights and risks below."
	var highlights, risks []string
	var metricsID string

	if latest != nil {
		metricsID = latest.ID
		highlights = append(highlights,
			fmt.Sprintf("MRR: $%.0f", latest.MRR),
			fmt.Sprintf("Total Users: %d", latest.TotalUsers),
			fmt.Sprintf("Growth Rate: %.0f%%", latest.GrowthRate*100),
		)
		if latest.NRR >= 1.0 {
			highlights = append(highlights, fmt.Sprintf("NRR: %.0f%% (net expansion)", latest.NRR*100))
		}
		if latest.ChurnRate > 0.1 {
			risks = append(risks, fmt.Sprintf("Churn at %.0f%% needs attention", latest.ChurnRate*100))
		}
		if latest.RunwayMonths < 6 {
			risks = append(risks, fmt.Sprintf("Only %.0f months of runway remaining", latest.RunwayMonths))
		}
		if latest.LTV > 0 && latest.CAC > 0 && latest.LTV/latest.CAC < 3 {
			risks = append(risks, "LTV:CAC ratio below 3:1 target")
		}
	} else {
		risks = append(risks, "No metrics recorded for this period")
	}

	report := &InvestorReport{
		ID:          genID("report"),
		Title:       title,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Summary:     summary,
		Highlights:  highlights,
		Risks:       risks,
		MetricsID:   metricsID,
		GeneratedAt: time.Now().UTC(),
	}
	ih.reports[report.ID] = report
	ih.persist()
	return report, nil
}

// ListReports returns reports sorted by generation time.
func (ih *InvestorHub) ListReports() []*InvestorReport {
	ih.mu.RLock()
	defer ih.mu.RUnlock()
	var result []*InvestorReport
	for _, r := range ih.reports {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GeneratedAt.After(result[j].GeneratedAt) })
	return result
}

// --- Persistence ---

func (ih *InvestorHub) persist() {
	if ih.path == "" {
		return
	}
	data := struct {
		Metrics       map[string]*TractionMetrics `json:"metrics"`
		Narratives    map[string]*Narrative        `json:"narratives"`
		PitchSections map[string]*PitchSection     `json:"pitch_sections"`
		Reports       map[string]*InvestorReport   `json:"reports"`
	}{ih.metrics, ih.narratives, ih.pitchSections, ih.reports}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(ih.path), 0755)
	os.WriteFile(ih.path, raw, 0644)
}

func (ih *InvestorHub) load() {
	if ih.path == "" {
		return
	}
	raw, err := os.ReadFile(ih.path)
	if err != nil {
		return
	}
	var data struct {
		Metrics       map[string]*TractionMetrics `json:"metrics"`
		Narratives    map[string]*Narrative        `json:"narratives"`
		PitchSections map[string]*PitchSection     `json:"pitch_sections"`
		Reports       map[string]*InvestorReport   `json:"reports"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Metrics != nil {
			ih.metrics = data.Metrics
		}
		if data.Narratives != nil {
			ih.narratives = data.Narratives
		}
		if data.PitchSections != nil {
			ih.pitchSections = data.PitchSections
		}
		if data.Reports != nil {
			ih.reports = data.Reports
		}
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
