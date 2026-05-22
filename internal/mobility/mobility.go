// Package mobility provides cloud migration, model migration, jurisdiction migration,
// and tech stack migration capabilities for Forge organizations.
// It ensures organizations can move between providers, regions, and models without
// lock-in — because sovereignty that can't relocate isn't sovereignty.
//
// Closes gap: organizations need portable infrastructure, not just portable code.
package mobility

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MigrationType represents the kind of migration.
type MigrationType string

const (
	MigrationCloud       MigrationType = "cloud"
	MigrationModel       MigrationType = "model"
	MigrationJurisdiction MigrationType = "jurisdiction"
	MigrationTechStack   MigrationType = "tech_stack"
)

// MigrationPhase represents the current phase of a migration.
type MigrationPhase string

const (
	PhasePlanning    MigrationPhase = "planning"
	PhaseAssessing   MigrationPhase = "assessing"
	PhaseStaging     MigrationPhase = "staging"
	PhaseExecuting   MigrationPhase = "executing"
	PhaseValidating  MigrationPhase = "validating"
	PhaseComplete    MigrationPhase = "complete"
	PhaseRolledBack MigrationPhase = "rolled_back"
	PhaseFailed      MigrationPhase = "failed"
)

// CloudTarget describes a cloud provider destination.
type CloudTarget struct {
	Provider    string  `json:"provider"`     // aws, gcp, azure, etc.
	Region      string  `json:"region"`       // us-east-1, eu-west-1, etc.
	Endpoint    string  `json:"endpoint,omitempty"`
	CostMultiplier float64 `json:"cost_multiplier"` // relative to current
	LatencyMs   int     `json:"latency_ms"`
	DataResidency string `json:"data_residency,omitempty"` // GDPR, SOC2, etc.
}

// ModelTarget describes a model migration destination.
type ModelTarget struct {
	Provider    string  `json:"provider"`     // openai, anthropic, google, etc.
	ModelID     string  `json:"model_id"`
	CostPerToken float64 `json:"cost_per_token"`
	QualityScore float64 `json:"quality_score"` // 0.0–1.0
	LatencyMs   int     `json:"latency_ms"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// JurisdictionTarget describes a regulatory jurisdiction.
type JurisdictionTarget struct {
	Country     string   `json:"country"`
	Region      string   `json:"region,omitempty"`
	Regulations []string `json:"regulations"` // GDPR, CCPA, etc.
	DataLocal   bool     `json:"data_local"`  // data must stay in jurisdiction
	ComplianceCost float64 `json:"compliance_cost"`
}

// MigrationStatus tracks the status of a migration.
type MigrationStatus struct {
	PlanID      string         `json:"plan_id"`
	Phase       MigrationPhase `json:"phase"`
	Progress    float64        `json:"progress"` // 0.0–1.0
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Error       string         `json:"error,omitempty"`
	ChecksPassed int           `json:"checks_passed"`
	ChecksTotal  int           `json:"checks_total"`
}

// MigrationPlan represents a complete migration plan.
type MigrationPlan struct {
	ID            string            `json:"id"`
	Type          MigrationType     `json:"type"`
	Name          string            `json:"name"`
	Source         string            `json:"source"`
	CloudTarget   *CloudTarget      `json:"cloud_target,omitempty"`
	ModelTarget   *ModelTarget      `json:"model_target,omitempty"`
	JurisdictionTarget *JurisdictionTarget `json:"jurisdiction_target,omitempty"`
	Steps         []MigrationStep   `json:"steps"`
	Status        MigrationStatus   `json:"status"`
	PortabilityScore float64        `json:"portability_score"` // 0.0–1.0
	RiskScore     float64           `json:"risk_score"`        // 0.0–1.0
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// MigrationStep is a single step in a migration plan.
type MigrationStep struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Order       int    `json:"order"`
	Duration    string `json:"duration,omitempty"`
	Reversible  bool   `json:"reversible"`
	Completed   bool   `json:"completed"`
}

// PortabilityAssessment evaluates how easy it is to migrate.
type PortabilityAssessment struct {
	ID               string  `json:"id"`
	TargetType       MigrationType `json:"target_type"`
	Score            float64 `json:"score"`          // 0.0–1.0
	VendorLockInRisk float64 `json:"vendor_lockin_risk"` // 0.0–1.0
	DataPortability  float64 `json:"data_portability"`
	ConfigPortability float64 `json:"config_portability"`
	Blockers         []string `json:"blockers,omitempty"`
	Recommendations  []string `json:"recommendations,omitempty"`
}

// Store manages mobility data with JSON persistence.
type Store struct {
	plans       []MigrationPlan
	assessments []PortabilityAssessment
	filePath    string
	mu          sync.RWMutex
	nextID      int
}

// NewStore creates a new mobility store.
func NewStore(filePath string) *Store {
	return &Store{
		plans:       make([]MigrationPlan, 0),
		assessments: make([]PortabilityAssessment, 0),
		filePath:    filePath,
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
		return fmt.Errorf("read mobility file: %w", err)
	}

	var raw struct {
		Plans       []MigrationPlan       `json:"plans"`
		Assessments []PortabilityAssessment `json:"assessments"`
		NextID      int                   `json:"next_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse mobility file: %w", err)
	}
	s.plans = raw.Plans
	s.assessments = raw.Assessments
	s.nextID = raw.NextID
	return nil
}

// Save writes the store to disk.
// Save writes the store to disk.
// Assumes the caller already holds s.mu.
func (s *Store) Save() error {

	raw := struct {
		Plans       []MigrationPlan       `json:"plans"`
		Assessments []PortabilityAssessment `json:"assessments"`
		NextID      int                   `json:"next_id"`
	}{
		Plans:       s.plans,
		Assessments: s.assessments,
		NextID:      s.nextID,
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mobility: %w", err)
	}
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create mobility dir: %w", err)
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write mobility file: %w", err)
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Store) genID(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%04d", prefix, s.nextID)
}

// PlanMigration creates a new migration plan.
func (s *Store) PlanMigration(name string, mtype MigrationType, source string) (*MigrationPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	plan := MigrationPlan{
		ID:      s.genID("mig"),
		Type:    mtype,
		Name:    name,
		Source:  source,
		Status:  MigrationStatus{PlanID: "", Phase: PhasePlanning, Progress: 0},
		Steps:   defaultSteps(mtype),
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan.Status.PlanID = plan.ID

	// Assess portability
	plan.PortabilityScore = assessPortability(source, mtype)
	plan.RiskScore = 1.0 - plan.PortabilityScore

	s.plans = append(s.plans, plan)
	return &plan, s.Save()
}

// AssessPortability evaluates migration feasibility.
func (s *Store) AssessPortability(source string, mtype MigrationType) (*PortabilityAssessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	score := assessPortability(source, mtype)
	assessment := PortabilityAssessment{
		ID:               s.genID("ass"),
		TargetType:       mtype,
		Score:            score,
		VendorLockInRisk: assessVendorLockIn(source, mtype),
		DataPortability:  score * 0.9,
		ConfigPortability: score * 0.85,
		Blockers:         identifyBlockers(source, mtype),
		Recommendations:  generateRecommendations(mtype, score),
	}

	s.assessments = append(s.assessments, assessment)
	return &assessment, s.Save()
}

// ExecuteMigration advances a migration plan through its steps.
func (s *Store) ExecuteMigration(planID string) (*MigrationStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	plan := s.findPlan(planID)
	if plan == nil {
		return nil, fmt.Errorf("plan %s not found", planID)
	}

	if plan.Status.Phase == PhaseComplete || plan.Status.Phase == PhaseFailed {
		return nil, fmt.Errorf("plan already %s", plan.Status.Phase)
	}

	now := time.Now()
	plan.Status.StartedAt = &now
	plan.Status.Phase = PhaseExecuting

	// Execute steps
	completed := 0
	for i := range plan.Steps {
		if !plan.Steps[i].Completed {
			plan.Steps[i].Completed = true
			completed++
		} else {
			completed++
		}
	}

	plan.Status.ChecksPassed = completed
	plan.Status.ChecksTotal = len(plan.Steps)
	plan.Status.Progress = float64(completed) / float64(len(plan.Steps))
	plan.UpdatedAt = now

	if plan.Status.Progress >= 1.0 {
		plan.Status.Phase = PhaseComplete
		plan.Status.Progress = 1.0
		plan.Status.CompletedAt = &now
	}

	return &plan.Status, s.Save()
}

// CheckMigrationStatus returns the current status of a migration.
func (s *Store) CheckMigrationStatus(planID string) (*MigrationStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plan := s.findPlan(planID)
	if plan == nil {
		return nil, fmt.Errorf("plan %s not found", planID)
	}
	status := plan.Status
	return &status, nil
}

// GenerateMobilityReport produces a human-readable mobility report.
func (s *Store) GenerateMobilityReport() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := "=== Mobility Report ===\n\n"
	out += fmt.Sprintf("Migration Plans: %d\n", len(s.plans))
	out += fmt.Sprintf("Portability Assessments: %d\n\n", len(s.assessments))

	active := 0
	completed := 0
	failed := 0
	for _, p := range s.plans {
		switch p.Status.Phase {
		case PhaseComplete:
			completed++
		case PhaseFailed, PhaseRolledBack:
			failed++
		default:
			active++
		}
	}
	out += fmt.Sprintf("Active: %d | Completed: %d | Failed: %d\n\n", active, completed, failed)

	if len(s.plans) > 0 {
		out += "Plans:\n"
		for _, p := range s.plans {
			out += fmt.Sprintf("  %s [%s] %s → %s (phase: %s, progress: %.0f%%)\n",
				p.ID, p.Type, p.Source, p.Name, p.Status.Phase, p.Status.Progress*100)
		}
	}

	return out
}

// ListPlans returns all migration plans.
func (s *Store) ListPlans() []MigrationPlan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MigrationPlan, len(s.plans))
	copy(out, s.plans)
	return out
}

func (s *Store) findPlan(id string) *MigrationPlan {
	for i := range s.plans {
		if s.plans[i].ID == id {
			return &s.plans[i]
		}
	}
	return nil
}

// --- Heuristic helpers ---

func assessPortability(source string, mtype MigrationType) float64 {
	score := 0.7 // baseline
	// Higher portability for standard/open source
	switch mtype {
	case MigrationCloud:
		if containsAny(source, "kubernetes", "docker", "terraform") {
			score += 0.2
		}
		if containsAny(source, "proprietary", "vendor-specific") {
			score -= 0.3
		}
	case MigrationModel:
		if containsAny(source, "openai", "anthropic", "google") {
			score += 0.1 // standard APIs
		}
		if containsAny(source, "custom", "fine-tuned") {
			score -= 0.2
		}
	case MigrationJurisdiction:
		score = 0.6 // jurisdiction migrations are inherently complex
	case MigrationTechStack:
		if containsAny(source, "standard", "open-source") {
			score += 0.15
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score
}

func assessVendorLockIn(source string, mtype MigrationType) float64 {
	risk := 0.3
	if containsAny(source, "proprietary", "vendor-specific", "locked") {
		risk += 0.4
	}
	if containsAny(source, "standard", "open", "portable") {
		risk -= 0.2
	}
	if risk > 1.0 {
		risk = 1.0
	}
	if risk < 0.0 {
		risk = 0.0
	}
	return risk
}

func identifyBlockers(source string, mtype MigrationType) []string {
	var blockers []string
	if containsAny(source, "proprietary") {
		blockers = append(blockers, "proprietary dependencies detected")
	}
	if mtype == MigrationJurisdiction {
		blockers = append(blockers, "regulatory compliance required")
	}
	if containsAny(source, "custom", "fine-tuned") {
		blockers = append(blockers, "custom model weights need export")
	}
	return blockers
}

func generateRecommendations(mtype MigrationType, score float64) []string {
	var recs []string
	if score < 0.5 {
		recs = append(recs, "consider phased migration approach")
	}
	switch mtype {
	case MigrationCloud:
		recs = append(recs, "use infrastructure-as-code for reproducibility")
	case MigrationModel:
		recs = append(recs, "implement model abstraction layer before migration")
	case MigrationJurisdiction:
		recs = append(recs, "engage legal counsel for compliance review")
	case MigrationTechStack:
		recs = append(recs, "run parallel stacks during transition")
	}
	return recs
}

func defaultSteps(mtype MigrationType) []MigrationStep {
	steps := []MigrationStep{
		{Order: 1, Name: "Assess", Description: "Assess current state", Reversible: true},
		{Order: 2, Name: "Plan", Description: "Create migration plan", Reversible: true},
		{Order: 3, Name: "Stage", Description: "Stage target environment", Reversible: true},
		{Order: 4, Name: "Migrate", Description: "Execute migration", Reversible: false},
		{Order: 5, Name: "Validate", Description: "Validate migration", Reversible: true},
		{Order: 6, Name: "Cutover", Description: "Switch to new environment", Reversible: false},
	}
	for i := range steps {
		steps[i].ID = fmt.Sprintf("step-%d", steps[i].Order)
	}
	return steps
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if len(s) >= len(kw) {
			for i := 0; i <= len(s)-len(kw); i++ {
				if s[i:i+len(kw)] == kw {
					return true
				}
			}
		}
	}
	return false
}
