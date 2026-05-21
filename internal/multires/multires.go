// Package multires generates multi-resolution views of the same data.
// Executive summary, technical detail, financial impact — all from the same source.
package multires

import (
	"fmt"
	"sync"
)

// Resolution defines the view detail level.
type Resolution int

const (
	ResolutionExecutive Resolution = iota // CEO-level: 1 paragraph
	ResolutionTechnical                   // Engineer-level: full details
	ResolutionFinancial                   // CFO-level: cost/revenue/ROI
	ResolutionOperational                 // Ops-level: actions/timeline/risks
	ResolutionLegal                       // Legal-level: compliance/risks/obligations
)

func (r Resolution) String() string {
	return [...]string{"executive", "technical", "financial", "operational", "legal"}[r]
}

// DetailLevel controls depth within a resolution.
type DetailLevel int

const (
	DetailSummary DetailLevel = iota  // 1-2 sentences
	DetailStandard                     // paragraph
	DetailDetailed                     // multiple paragraphs
	DetailRaw                          // raw data
)

func (d DetailLevel) String() string {
	return [...]string{"summary", "standard", "detailed", "raw"}[d]
}

// DataBlock is raw data with metadata.
type DataBlock struct {
	ID         string
	Source     string
	Confidence float64   // 0-1
	Timestamp  string
	Data       map[string]interface{}
	Tags       []string
}

// Audience describes who's reading the output.
type Audience struct {
	Role      string   // "ceo", "engineer", "cfo", "lawyer", "ops"
	Expertise []string // domains they understand
	Interests []string // what they care about
	Detail    DetailLevel
	Format    string // "text", "table", "chart", "slides"
}

// View is a rendered output at a specific resolution.
type View struct {
	Resolution Resolution
	Detail     DetailLevel
	Content    string
	Highlights []string
	Warnings   []string
	Format     string
}

// ViewTemplate defines how to transform data for a resolution.
type ViewTemplate struct {
	Resolution Resolution
	Extractors map[string]func(DataBlock) string
	Formatter  func(map[string]string) string
}

// MultiResolution is the main view generation engine.
type MultiResolution struct {
	templates map[Resolution]ViewTemplate
	mu        sync.RWMutex
}

// New creates a new multi-resolution engine.
func New() *MultiResolution {
	mr := &MultiResolution{
		templates: make(map[Resolution]ViewTemplate),
	}
	mr.registerDefaults()
	return mr
}

// View generates a view of data at a specific resolution.
func (mr *MultiResolution) View(data DataBlock, resolution Resolution) (*View, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	view := &View{
		Resolution: resolution,
		Format:     "text",
	}

	switch resolution {
	case ResolutionExecutive:
		view.Detail = DetailSummary
		view.Content = mr.executiveView(data)
		view.Highlights = mr.executiveHighlights(data)
	case ResolutionTechnical:
		view.Detail = DetailDetailed
		view.Content = mr.technicalView(data)
		view.Highlights = mr.techHighlights(data)
	case ResolutionFinancial:
		view.Detail = DetailStandard
		view.Content = mr.financialView(data)
		view.Highlights = mr.financialHighlights(data)
	case ResolutionOperational:
		view.Detail = DetailStandard
		view.Content = mr.operationalView(data)
		view.Highlights = mr.opsHighlights(data)
	case ResolutionLegal:
		view.Detail = DetailDetailed
		view.Content = mr.legalView(data)
		view.Warnings = mr.legalWarnings(data)
	default:
		view.Content = fmt.Sprintf("Data: %+v", data.Data)
	}

	return view, nil
}

// ViewFor generates a view tailored to an audience.
func (mr *MultiResolution) ViewFor(data DataBlock, audience Audience) (*View, error) {
	resolution := mr.inferResolution(audience)
	view, err := mr.View(data, resolution)
	if err != nil {
		return nil, err
	}
	view.Detail = audience.Detail
	return view, nil
}

// Levels generates all 5 progressive disclosure levels.
func (mr *MultiResolution) Levels(data DataBlock) ([]View, error) {
	levels := make([]View, 5)
	resolutions := []Resolution{
		ResolutionExecutive, ResolutionTechnical, ResolutionFinancial,
		ResolutionOperational, ResolutionLegal,
	}
	for i, res := range resolutions {
		v, err := mr.View(data, res)
		if err != nil {
			return nil, err
		}
		levels[i] = *v
	}
	return levels, nil
}

// RegisterTemplate adds a custom view template.
func (mr *MultiResolution) RegisterTemplate(resolution Resolution, template ViewTemplate) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.templates[resolution] = template
	return nil
}

func (mr *MultiResolution) inferResolution(a Audience) Resolution {
	switch a.Role {
	case "ceo", "executive", "founder":
		return ResolutionExecutive
	case "engineer", "developer", "devops":
		return ResolutionTechnical
	case "cfo", "finance", "accountant":
		return ResolutionFinancial
	case "ops", "operations", "sre":
		return ResolutionOperational
	case "lawyer", "legal", "compliance":
		return ResolutionLegal
	default:
		return ResolutionExecutive
	}
}

func (mr *MultiResolution) registerDefaults() {
	// Default templates are handled in the View() method directly
}

// --- Resolution generators ---

func (mr *MultiResolution) executiveView(data DataBlock) string {
	status, _ := data.Data["status"].(string)
	tasks, _ := data.Data["tasks_completed"].(float64)
	total, _ := data.Data["tasks_total"].(float64)
	cost, _ := data.Data["cost_usd"].(float64)
	revenue, _ := data.Data["revenue_usd"].(float64)

	roi := 0.0
	if cost > 0 {
		roi = (revenue - cost) / cost * 100
	}

	return fmt.Sprintf("Status: %s. %.0f/%.0f tasks complete. Cost: $%.2f. Revenue: $%.2f. ROI: %.1f%%.",
		status, tasks, total, cost, revenue, roi)
}

func (mr *MultiResolution) executiveHighlights(data DataBlock) []string {
	var highlights []string
	if blocking, ok := data.Data["blocking"].(float64); ok && blocking > 0 {
		highlights = append(highlights, fmt.Sprintf("%.0f items blocked", blocking))
	}
	if atRisk, ok := data.Data["at_risk"].(float64); ok && atRisk > 0 {
		highlights = append(highlights, fmt.Sprintf("%.0f items at risk", atRisk))
	}
	return highlights
}

func (mr *MultiResolution) technicalView(data DataBlock) string {
	var parts []string
	for k, v := range data.Data {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	return fmt.Sprintf("Technical Report\nSource: %s | Confidence: %.0f%%\n%s",
		data.Source, data.Confidence*100, joinParts(parts))
}

func (mr *MultiResolution) techHighlights(data DataBlock) []string {
	var highlights []string
	if errors, ok := data.Data["errors"].([]interface{}); ok {
		highlights = append(highlights, fmt.Sprintf("%d errors detected", len(errors)))
	}
	return highlights
}

func (mr *MultiResolution) financialView(data DataBlock) string {
	cost, _ := data.Data["cost_usd"].(float64)
	revenue, _ := data.Data["revenue_usd"].(float64)
	burn, _ := data.Data["burn_rate"].(float64)

	return fmt.Sprintf("Financial Impact\nCost: $%.2f | Revenue: $%.2f | Burn Rate: $%.2f/day | Net: $%.2f",
		cost, revenue, burn, revenue-cost)
}

func (mr *MultiResolution) financialHighlights(data DataBlock) []string {
	var highlights []string
	cost, _ := data.Data["cost_usd"].(float64)
	budget, _ := data.Data["budget_usd"].(float64)
	if budget > 0 && cost > budget*0.8 {
		highlights = append(highlights, fmt.Sprintf("approaching budget limit (%.0f%% used)", cost/budget*100))
	}
	return highlights
}

func (mr *MultiResolution) operationalView(data DataBlock) string {
	status, _ := data.Data["status"].(string)
	deploy, _ := data.Data["next_deploy"].(string)
	downtime, _ := data.Data["est_downtime"].(string)

	return fmt.Sprintf("Operational Status: %s\nNext Deploy: %s\nEstimated Downtime: %s",
		status, deploy, downtime)
}

func (mr *MultiResolution) opsHighlights(data DataBlock) []string {
	var highlights []string
	if incidents, ok := data.Data["active_incidents"].(float64); ok && incidents > 0 {
		highlights = append(highlights, fmt.Sprintf("%.0f active incidents", incidents))
	}
	return highlights
}

func (mr *MultiResolution) legalView(data DataBlock) string {
	compliant, _ := data.Data["compliant"].(bool)
	risks, _ := data.Data["legal_risks"].([]interface{})
	obligations, _ := data.Data["obligations"].([]interface{})

	return fmt.Sprintf("Legal Assessment\nCompliant: %v | Risks: %d | Obligations: %d",
		compliant, len(toSlice(risks)), len(toSlice(obligations)))
}

func (mr *MultiResolution) legalWarnings(data DataBlock) []string {
	var warnings []string
	if risks, ok := data.Data["legal_risks"].([]interface{}); ok {
		for _, r := range risks {
			if s, ok := r.(string); ok {
				warnings = append(warnings, s)
			}
		}
	}
	return warnings
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "\n"
		}
		result += p
	}
	return result
}

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if s, ok := v.([]interface{}); ok {
		return s
	}
	return nil
}
