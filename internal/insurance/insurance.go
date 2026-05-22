// Package insurance provides coverage management, claims tracking, and risk
// assessment. It closes the gap in organizational risk management by modeling
// policies, identifying coverage gaps, and assessing aggregate risk exposure—
// ensuring the organization maintains adequate protection against loss.
package insurance

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// PolicyType categorizes the kind of insurance policy.
type PolicyType string

const (
	PolicyGeneralLiability PolicyType = "general_liability"
	PolicyProperty         PolicyType = "property"
	PolicyCyber            PolicyType = "cyber"
	PolicyEPLI             PolicyType = "employment_practices"
	PolicyDAndO            PolicyType = "directors_and_officers"
	PolicyEandO            PolicyType = "errors_and_omissions"
	PolicyWorkersComp      PolicyType = "workers_compensation"
	PolicyKeyPerson        PolicyType = "key_person"
)

// ClaimStatus represents the current state of a claim.
type ClaimStatus string

const (
	ClaimFiled    ClaimStatus = "filed"
	ClaimReview   ClaimStatus = "under_review"
	ClaimApproved ClaimStatus = "approved"
	ClaimDenied   ClaimStatus = "denied"
	ClaimPaid     ClaimStatus = "paid"
	ClaimClosed   ClaimStatus = "closed"
)

// Policy represents an insurance policy.
type Policy struct {
	ID            string     `json:"id"`
	Type          PolicyType `json:"type"`
	CoverageLimit float64    `json:"coverage_limit"`
	Deductible    float64    `json:"deductible"`
	Premium       float64    `json:"premium"`
	Insurer       string     `json:"insurer"`
	PolicyNumber  string     `json:"policy_number"`
	StartDate     time.Time  `json:"start_date"`
	EndDate       time.Time  `json:"end_date"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Claim represents an insurance claim.
type Claim struct {
	ID          string      `json:"id"`
	PolicyID    string      `json:"policy_id"`
	Status      ClaimStatus `json:"status"`
	Amount      float64     `json:"amount"`
	Description string      `json:"description"`
	IncidentDate time.Time  `json:"incident_date"`
	FiledDate   time.Time   `json:"filed_date"`
	ResolvedDate time.Time  `json:"resolved_date"`
	CreatedAt   time.Time   `json:"created_at"`
}

// CoverageGap represents an area of inadequate coverage.
type CoverageGap struct {
	ID          string    `json:"id"`
	Type        PolicyType `json:"type"`
	Description string    `json:"description"`
	ExposureAmount float64 `json:"exposure_amount"`
	IdentifiedAt time.Time `json:"identified_at"`
}

// RiskAssessment represents an evaluation of organizational risk.
type RiskAssessment struct {
	ID              string    `json:"id"`
	OverallRisk     float64   `json:"overall_risk"` // 0.0-1.0
	CoverageScore   float64   `json:"coverage_score"` // 0.0-1.0
	Gaps            []CoverageGap `json:"gaps"`
	TotalCoverage   float64  `json:"total_coverage"`
	TotalExposure   float64  `json:"total_exposure"`
	AssessedAt      time.Time `json:"assessed_at"`
}

// Store persists insurance data.
type Store struct {
	mu          sync.Mutex
	filePath    string
	Policies    map[string]Policy       `json:"policies"`
	Claims      map[string]Claim        `json:"claims"`
	CoverageGaps map[string]CoverageGap `json:"coverage_gaps"`
	Assessments map[string]RiskAssessment `json:"assessments"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath:     filePath,
		Policies:     make(map[string]Policy),
		Claims:       make(map[string]Claim),
		CoverageGaps: make(map[string]CoverageGap),
		Assessments:  make(map[string]RiskAssessment),
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

// AddPolicy creates or updates a policy.
func (s *Store) AddPolicy(p Policy) Policy {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.CreatedAt = time.Now().UTC()
	s.Policies[p.ID] = p
	return p
}

// FileClaim creates a new claim against a policy.
func (s *Store) FileClaim(c Claim) (Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Policies[c.PolicyID]; !ok {
		return Claim{}, os.ErrNotExist
	}
	c.Status = ClaimFiled
	c.FiledDate = time.Now().UTC()
	c.CreatedAt = time.Now().UTC()
	s.Claims[c.ID] = c
	return c, nil
}

// AssessRisk evaluates the overall risk posture.
func (s *Store) AssessRisk(exposures map[PolicyType]float64) RiskAssessment {
	s.mu.Lock()
	defer s.mu.Unlock()
	coverageByType := make(map[PolicyType]float64)
	totalCoverage := 0.0
	for _, p := range s.Policies {
		if p.IsActive {
			coverageByType[p.Type] += p.CoverageLimit
			totalCoverage += p.CoverageLimit
		}
	}
	totalExposure := 0.0
	for _, exp := range exposures {
		totalExposure += exp
	}
	var gaps []CoverageGap
	i := 0
	for pType, exp := range exposures {
		cov := coverageByType[pType]
		if cov < exp {
			gaps = append(gaps, CoverageGap{
				ID:              "gap-" + string(pType),
				Type:            pType,
				Description:     "Insufficient " + string(pType) + " coverage",
				ExposureAmount:  exp - cov,
				IdentifiedAt:    time.Now().UTC(),
			})
			s.CoverageGaps["gap-"+string(pType)] = gaps[i]
			i++
		}
	}
	overallRisk := 0.0
	if totalExposure > 0 {
		overallRisk = 1.0 - (totalCoverage / totalExposure)
	}
	if overallRisk < 0 {
		overallRisk = 0
	}
	coverageScore := 1.0 - overallRisk
	ra := RiskAssessment{
		ID:            "ra-" + time.Now().Format("20060102"),
		OverallRisk:   overallRisk,
		CoverageScore: coverageScore,
		Gaps:          gaps,
		TotalCoverage: totalCoverage,
		TotalExposure: totalExposure,
		AssessedAt:    time.Now().UTC(),
	}
	s.Assessments[ra.ID] = ra
	return ra
}

// IdentifyCoverageGaps returns all known coverage gaps.
func (s *Store) IdentifyCoverageGaps() []CoverageGap {
	s.mu.Lock()
	defer s.mu.Unlock()
	gaps := make([]CoverageGap, 0, len(s.CoverageGaps))
	for _, g := range s.CoverageGaps {
		gaps = append(gaps, g)
	}
	return gaps
}

// GenerateInsuranceReport produces a summary report.
func (s *Store) GenerateInsuranceReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	activePolicies := 0
	totalPremium := 0.0
	for _, p := range s.Policies {
		if p.IsActive {
			activePolicies++
			totalPremium += p.Premium
		}
	}
	openClaims := 0
	totalClaimAmount := 0.0
	for _, c := range s.Claims {
		if c.Status != ClaimClosed && c.Status != ClaimDenied {
			openClaims++
			totalClaimAmount += c.Amount
		}
	}
	return map[string]interface{}{
		"active_policies":   activePolicies,
		"total_premium":     totalPremium,
		"open_claims":       openClaims,
		"total_claim_amount": totalClaimAmount,
		"coverage_gaps":     len(s.CoverageGaps),
	}
}
