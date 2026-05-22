// Package tax provides tax filing, strategy optimization, and compliance
// tracking. It closes the gap in financial governance by modeling tax
// jurisdictions, obligations, deductions, and filing workflows—ensuring the
// organization meets its tax responsibilities while legally minimizing burden.
package tax

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FilingStatus represents the current status of a tax filing.
type FilingStatus string

const (
	StatusDraft      FilingStatus = "draft"
	StatusPending    FilingStatus = "pending"
	StatusFiled      FilingStatus = "filed"
	StatusAccepted   FilingStatus = "accepted"
	StatusRejected   FilingStatus = "rejected"
	StatusAmended    FilingStatus = "amended"
)

// TaxJurisdiction represents a tax authority and its rules.
type TaxJurisdiction struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Country     string    `json:"country"`
	Region      string    `json:"region"`
	Rate        float64   `json:"rate"`
	Currency    string    `json:"currency"`
	FilingDeadline time.Time `json:"filing_deadline"`
	CreatedAt   time.Time `json:"created_at"`
}

// TaxFiling represents a single tax filing for a period.
type TaxFiling struct {
	ID            string       `json:"id"`
	JurisdictionID string      `json:"jurisdiction_id"`
	PeriodStart   time.Time    `json:"period_start"`
	PeriodEnd     time.Time    `json:"period_end"`
	Status        FilingStatus `json:"status"`
	GrossIncome   float64      `json:"gross_income"`
	TotalDeductions float64    `json:"total_deductions"`
	TaxableIncome float64      `json:"taxable_income"`
	TaxOwed       float64      `json:"tax_owed"`
	FiledAt       time.Time    `json:"filed_at"`
	CreatedAt     time.Time    `json:"created_at"`
}

// TaxStrategy represents an optimization strategy.
type TaxStrategy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	EstimatedSavings float64 `json:"estimated_savings"`
	RiskLevel   float64   `json:"risk_level"` // 0.0-1.0
	JurisdictionIDs []string `json:"jurisdiction_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

// DeductionRecord represents a single deductible expense.
type DeductionRecord struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	ReceiptRef  string    `json:"receipt_ref"`
	FilingID    string    `json:"filing_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// TaxObligation represents an upcoming tax responsibility.
type TaxObligation struct {
	ID              string    `json:"id"`
	JurisdictionID  string   `json:"jurisdiction_id"`
	Type            string   `json:"type"`
	Amount          float64  `json:"amount"`
	DueDate         time.Time `json:"due_date"`
	IsPaid          bool     `json:"is_paid"`
	PaidDate        time.Time `json:"paid_date"`
	Description     string   `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
}

// Store persists tax data.
type Store struct {
	mu            sync.Mutex
	filePath      string
	Jurisdictions map[string]TaxJurisdiction `json:"jurisdictions"`
	Filings       map[string]TaxFiling       `json:"filings"`
	Strategies    map[string]TaxStrategy     `json:"strategies"`
	Deductions    map[string]DeductionRecord `json:"deductions"`
	Obligations   map[string]TaxObligation   `json:"obligations"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath:      filePath,
		Jurisdictions: make(map[string]TaxJurisdiction),
		Filings:       make(map[string]TaxFiling),
		Strategies:    make(map[string]TaxStrategy),
		Deductions:    make(map[string]DeductionRecord),
		Obligations:   make(map[string]TaxObligation),
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

// TrackObligation adds or updates a tax obligation.
func (s *Store) TrackObligation(ob TaxObligation) TaxObligation {
	s.mu.Lock()
	defer s.mu.Unlock()
	ob.CreatedAt = time.Now().UTC()
	s.Obligations[ob.ID] = ob
	return ob
}

// CalculateEstimate computes a rough tax estimate for a jurisdiction and income.
func (s *Store) CalculateEstimate(jurisdictionID string, grossIncome float64) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.Jurisdictions[jurisdictionID]
	if !ok {
		return 0
	}
	totalDeductions := 0.0
	for _, d := range s.Deductions {
		totalDeductions += d.Amount
	}
	taxable := grossIncome - totalDeductions
	if taxable < 0 {
		taxable = 0
	}
	return taxable * j.Rate
}

// OptimizeStrategy evaluates strategies and returns the best one by estimated savings minus risk.
func (s *Store) OptimizeStrategy() *TaxStrategy {
	s.mu.Lock()
	defer s.mu.Unlock()
	var best *TaxStrategy
	bestScore := -1.0
	for _, st := range s.Strategies {
		score := st.EstimatedSavings - (st.RiskLevel * st.EstimatedSavings * 0.5)
		if score > bestScore {
			copy := st
			best = &copy
			bestScore = score
		}
	}
	return best
}

// FileReturn marks a filing as filed.
func (s *Store) FileReturn(filingID string) (TaxFiling, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.Filings[filingID]
	if !ok {
		return TaxFiling{}, os.ErrNotExist
	}
	f.Status = StatusFiled
	f.FiledAt = time.Now().UTC()
	s.Filings[filingID] = f
	return f, nil
}

// GenerateTaxReport produces a summary report of all filings and obligations.
func (s *Store) GenerateTaxReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	totalOwed := 0.0
	totalPaid := 0.0
	for _, f := range s.Filings {
		totalOwed += f.TaxOwed
	}
	for _, o := range s.Obligations {
		if o.IsPaid {
			totalPaid += o.Amount
		}
	}
	pendingFilings := 0
	for _, f := range s.Filings {
		if f.Status == StatusDraft || f.Status == StatusPending {
			pendingFilings++
		}
	}
	return map[string]interface{}{
		"total_tax_owed":   totalOwed,
		"total_paid":       totalPaid,
		"pending_filings":  pendingFilings,
		"jurisdiction_count": len(s.Jurisdictions),
		"obligation_count":  len(s.Obligations),
		"deduction_count":   len(s.Deductions),
	}
}
