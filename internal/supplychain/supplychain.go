// Package supplychain provides supplier risk monitoring, diversification,
// and business continuity planning. Prevents single points of failure.
package supplychain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// RiskLevel represents supplier risk.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// SupplierStatus tracks supplier state.
type SupplierStatus string

const (
	SupplierActive     SupplierStatus = "active"
	SupplierDegraded   SupplierStatus = "degraded"
	SupplierAtRisk     SupplierStatus = "at_risk"
	SupplierFailed     SupplierStatus = "failed"
	SupplierSuspended  SupplierStatus = "suspended"
	SupplierTerminated SupplierStatus = "terminated"
)

// Supplier represents an external supplier.
type Supplier struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Service      string         `json:"service"`
	Category     string         `json:"category"` // cloud, api, data, infra, saas
	Status       SupplierStatus `json:"status"`
	RiskLevel    RiskLevel      `json:"risk_level"`
	Rating       float64        `json:"rating"` // 0-5
	SpendMonthly float64        `json:"spend_monthly"`
	Criticality  string         `json:"criticality"` // low, medium, high, critical
	ContractEnd  *time.Time     `json:"contract_end,omitempty"`
	Alternatives []string       `json:"alternatives,omitempty"` // alternative supplier IDs
	LastAudit    *time.Time     `json:"last_audit,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	Notes        string         `json:"notes,omitempty"`
}

// RiskAssessment evaluates supplier risk.
type RiskAssessment struct {
	ID          string    `json:"id"`
	SupplierID  string    `json:"supplier_id"`
	OverallRisk RiskLevel `json:"overall_risk"`
	Categories  map[string]RiskLevel `json:"categories"` // financial, operational, compliance, geopolitical
	Findings    []string  `json:"findings,omitempty"`
	Mitigations []string  `json:"mitigations,omitempty"`
	AssessedAt  time.Time `json:"assessed_at"`
	AssessedBy  string    `json:"assessed_by"`
}

// DiversificationPlan tracks multi-vendor strategy.
type DiversificationPlan struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`
	Primary     string    `json:"primary"`    // primary supplier ID
	Secondaries []string  `json:"secondaries"` // backup supplier IDs
	FailoverAuto bool     `json:"failover_auto"`
	LastTested  *time.Time `json:"last_tested,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ContinuityPlan is a disaster recovery plan.
type ContinuityPlan struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Trigger      string    `json:"trigger"` // what activates this plan
	Fallbacks    []Fallback `json:"fallbacks"`
	RecoveryTime string    `json:"recovery_time"` // expected RTO
	TestedAt     *time.Time `json:"tested_at,omitempty"`
	Status       string    `json:"status"` // draft, active, tested, obsolete
	CreatedAt    time.Time `json:"created_at"`
}

// Fallback is a single fallback action in a continuity plan.
type Fallback struct {
	Order    int    `json:"order"`
	Action   string `json:"action"`
	Supplier string `json:"supplier,omitempty"`
	Service  string `json:"service,omitempty"`
}

// Manager manages supply chain operations.
type Manager struct {
	mu          sync.RWMutex
	suppliers   map[string]*Supplier
	assessments map[string]*RiskAssessment
	divPlans    map[string]*DiversificationPlan
	contPlans   map[string]*ContinuityPlan
	path        string
}

// NewManager creates a new supply chain manager.
func NewManager(persistPath string) *Manager {
	m := &Manager{
		suppliers:   make(map[string]*Supplier),
		assessments: make(map[string]*RiskAssessment),
		divPlans:    make(map[string]*DiversificationPlan),
		contPlans:   make(map[string]*ContinuityPlan),
		path:        persistPath,
	}
	m.load()
	return m
}

// --- Suppliers ---

// AddSupplier registers a new supplier.
func (m *Manager) AddSupplier(name, service, category, criticality string, spendMonthly float64) (*Supplier, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := &Supplier{
		ID:           genID("sup"),
		Name:         name,
		Service:      service,
		Category:     category,
		Status:       SupplierActive,
		RiskLevel:    RiskLow,
		Criticality:  criticality,
		SpendMonthly: spendMonthly,
		Alternatives: []string{},
		CreatedAt:    time.Now().UTC(),
	}

	m.suppliers[s.ID] = s
	m.persist()
	return s, nil
}

// UpdateSupplierStatus updates a supplier's operational status.
func (m *Manager) UpdateSupplierStatus(id string, status SupplierStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.suppliers[id]
	if !ok {
		return fmt.Errorf("supplier %s not found", id)
	}
	s.Status = status
	m.persist()
	return nil
}

// RateSupplier updates a supplier's rating.
func (m *Manager) RateSupplier(id string, rating float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.suppliers[id]
	if !ok {
		return fmt.Errorf("supplier %s not found", id)
	}
	s.Rating = rating
	// Auto-adjust risk based on rating
	if rating < 2 {
		s.RiskLevel = RiskHigh
	} else if rating < 3 {
		s.RiskLevel = RiskMedium
	} else {
		s.RiskLevel = RiskLow
	}
	m.persist()
	return nil
}

// ListSuppliers returns suppliers filtered by category or status.
func (m *Manager) ListSuppliers(category string, status SupplierStatus) []*Supplier {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Supplier
	for _, s := range m.suppliers {
		if (category == "" || s.Category == category) && (status == "" || s.Status == status) {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

// GetHighRisk returns suppliers with elevated risk.
func (m *Manager) GetHighRisk() []*Supplier {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Supplier
	for _, s := range m.suppliers {
		if s.RiskLevel == RiskHigh || s.RiskLevel == RiskCritical ||
			s.Status == SupplierDegraded || s.Status == SupplierAtRisk {
			result = append(result, s)
		}
	}
	return result
}

// --- Risk Assessment ---

// AssessRisk creates a risk assessment for a supplier.
func (m *Manager) AssessRisk(supplierID string, categories map[string]RiskLevel, findings, mitigations []string, assessedBy string) (*RiskAssessment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.suppliers[supplierID]; !ok {
		return nil, fmt.Errorf("supplier %s not found", supplierID)
	}

	// Determine overall risk (worst category)
	overall := RiskLow
	for _, r := range categories {
		if r == RiskCritical {
			overall = RiskCritical
			break
		}
		if r == RiskHigh && overall != RiskCritical {
			overall = RiskHigh
		}
		if r == RiskMedium && overall != RiskHigh && overall != RiskCritical {
			overall = RiskMedium
		}
	}

	assess := &RiskAssessment{
		ID:          genID("risk"),
		SupplierID:  supplierID,
		OverallRisk: overall,
		Categories:  categories,
		Findings:    findings,
		Mitigations: mitigations,
		AssessedAt:  time.Now().UTC(),
		AssessedBy:  assessedBy,
	}

	m.assessments[assess.ID] = assess

	// Update supplier risk level
	m.suppliers[supplierID].RiskLevel = overall

	m.persist()
	return assess, nil
}

// --- Diversification ---

// CreateDiversificationPlan creates a multi-vendor strategy.
func (m *Manager) CreateDiversificationPlan(category, primary string, secondaries []string, autoFailover bool) (*DiversificationPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan := &DiversificationPlan{
		ID:           genID("div"),
		Category:     category,
		Primary:      primary,
		Secondaries:  secondaries,
		FailoverAuto: autoFailover,
		CreatedAt:    time.Now().UTC(),
	}

	// Link alternatives on supplier
	if s, ok := m.suppliers[primary]; ok {
		s.Alternatives = secondaries
	}

	m.divPlans[plan.ID] = plan
	m.persist()
	return plan, nil
}

// --- Business Continuity ---

// CreateContinuityPlan creates a disaster recovery plan.
func (m *Manager) CreateContinuityPlan(name, trigger, recoveryTime string, fallbacks []Fallback) (*ContinuityPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan := &ContinuityPlan{
		ID:           genID("cont"),
		Name:         name,
		Trigger:      trigger,
		Fallbacks:    fallbacks,
		RecoveryTime: recoveryTime,
		Status:       "draft",
		CreatedAt:    time.Now().UTC(),
	}

	m.contPlans[plan.ID] = plan
	m.persist()
	return plan, nil
}

// TestContinuityPlan records a test of the continuity plan.
func (m *Manager) TestContinuityPlan(planID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.contPlans[planID]
	if !ok {
		return fmt.Errorf("plan %s not found", planID)
	}
	plan.Status = "tested"
	now := time.Now().UTC()
	plan.TestedAt = &now
	m.persist()
	return nil
}

// ListContinuityPlans returns all continuity plans.
func (m *Manager) ListContinuityPlans() []*ContinuityPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ContinuityPlan
	for _, p := range m.contPlans {
		result = append(result, p)
	}
	return result
}

func (m *Manager) persist() {
	if m.path == "" { return }
	data := struct {
		Suppliers   map[string]*Supplier        `json:"suppliers"`
		Assessments map[string]*RiskAssessment   `json:"assessments"`
		DivPlans    map[string]*DiversificationPlan `json:"diversification_plans"`
		ContPlans   map[string]*ContinuityPlan   `json:"continuity_plans"`
	}{m.suppliers, m.assessments, m.divPlans, m.contPlans}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(m.path), 0755)
	os.WriteFile(m.path, raw, 0644)
}

func (m *Manager) load() {
	if m.path == "" { return }
	raw, err := os.ReadFile(m.path)
	if err != nil { return }
	var data struct {
		Suppliers   map[string]*Supplier        `json:"suppliers"`
		Assessments map[string]*RiskAssessment   `json:"assessments"`
		DivPlans    map[string]*DiversificationPlan `json:"diversification_plans"`
		ContPlans   map[string]*ContinuityPlan   `json:"continuity_plans"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Suppliers != nil { m.suppliers = data.Suppliers }
		if data.Assessments != nil { m.assessments = data.Assessments }
		if data.DivPlans != nil { m.divPlans = data.DivPlans }
		if data.ContPlans != nil { m.contPlans = data.ContPlans }
	}
}

func genID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }
