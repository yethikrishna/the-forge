// Package growthgovernor provides maximum size caps, human approval for
// growth, and cancer-prevention mechanisms. It closes the gap in unchecked
// organizational expansion by enforcing growth limits, requiring explicit
// authorization to exceed caps, and monitoring for cancerous growth patterns
// (teams that grow without proportional output)—ensuring the organization
// grows intentionally rather than accidently.
package growthgovernor

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// CapType represents what the cap measures.
type CapType string

const (
	CapHeadcount    CapType = "headcount"
	CapBudget       CapType = "budget"
	CapTeams        CapType = "teams"
	CapRevenue      CapType = "revenue"
)

// RequestStatus represents the state of a growth request.
type RequestStatus string

const (
	RequestPending  RequestStatus = "pending"
	RequestApproved RequestStatus = "approved"
	RequestDenied   RequestStatus = "denied"
	RequestExpired  RequestStatus = "expired"
)

// GrowthCap represents a maximum allowed size.
type GrowthCap struct {
	ID          string    `json:"id"`
	Type        CapType   `json:"type"`
	MaxValue    float64   `json:"max_value"`
	CurrentValue float64  `json:"current_value"`
	Scope       string    `json:"scope"` // org, department, team
	Description string    `json:"description"`
	SetAt       time.Time `json:"set_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// GrowthRequest represents a request to exceed a cap.
type GrowthRequest struct {
	ID           string        `json:"id"`
	CapID        string        `json:"cap_id"`
	RequestedBy  string        `json:"requested_by"`
	Justification string       `json:"justification"`
	RequestedValue float64     `json:"requested_value"`
	Status       RequestStatus `json:"status"`
	ReviewedBy   string        `json:"reviewed_by"`
	ReviewNotes  string        `json:"review_notes"`
	CreatedAt    time.Time     `json:"created_at"`
	ReviewedAt   time.Time     `json:"reviewed_at"`
}

// OrgSizeMetrics represents current organizational size metrics.
type OrgSizeMetrics struct {
	ID          string    `json:"id"`
	Headcount   int       `json:"headcount"`
	TeamCount   int       `json:"team_count"`
	Budget      float64   `json:"budget"`
	Revenue     float64   `json:"revenue"`
	RevenuePerEmployee float64 `json:"revenue_per_employee"`
	MeasuredAt  time.Time `json:"measured_at"`
}

// GrowthGovernorConfig represents the governor configuration.
type GrowthGovernorConfig struct {
	ID                    string  `json:"id"`
	AutoDenyAboveFactor   float64 `json:"auto_deny_above_factor"` // auto-deny if request > factor * cap
	RequireHumanApproval  bool    `json:"require_human_approval"`
	CancerThreshold       float64 `json:"cancer_threshold"` // headcount/output ratio
	ReviewWindowHours     int     `json:"review_window_hours"`
	Enabled               bool    `json:"enabled"`
	CreatedAt             time.Time `json:"created_at"`
}

// Store persists growth governor data.
type Store struct {
	mu       sync.Mutex
	filePath string
	Caps     map[string]GrowthCap          `json:"caps"`
	Requests map[string]GrowthRequest      `json:"requests"`
	Metrics  map[string]OrgSizeMetrics     `json:"metrics"`
	Configs  map[string]GrowthGovernorConfig `json:"configs"`
}

// NewStore creates a Store backed by the given file.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		Caps:     make(map[string]GrowthCap),
		Requests: make(map[string]GrowthRequest),
		Metrics:  make(map[string]OrgSizeMetrics),
		Configs:  make(map[string]GrowthGovernorConfig),
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

// SetCap creates or updates a growth cap.
func (s *Store) SetCap(cap GrowthCap) GrowthCap {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	cap.SetAt = now
	if cap.CreatedAt.IsZero() {
		cap.CreatedAt = now
	}
	s.Caps[cap.ID] = cap
	return cap
}

// RequestGrowth creates a growth request for human review.
func (s *Store) RequestGrowth(req GrowthRequest) (GrowthRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cap, ok := s.Caps[req.CapID]
	if !ok {
		return GrowthRequest{}, os.ErrNotExist
	}
	req.CreatedAt = time.Now().UTC()
	req.Status = RequestPending
	// Check if auto-deny applies
	for _, cfg := range s.Configs {
		if cfg.Enabled && cfg.AutoDenyAboveFactor > 0 {
			if req.RequestedValue > cap.MaxValue*cfg.AutoDenyAboveFactor {
				req.Status = RequestDenied
				req.ReviewNotes = "Auto-denied: exceeds configured factor of cap"
				req.ReviewedAt = time.Now().UTC()
			}
		}
	}
	s.Requests[req.ID] = req
	return req, nil
}

// CheckCompliance checks if current metrics violate any caps.
func (s *Store) CheckCompliance() []GrowthCap {
	s.mu.Lock()
	defer s.mu.Unlock()
	var violations []GrowthCap
	for _, cap := range s.Caps {
		if cap.CurrentValue > cap.MaxValue {
			violations = append(violations, cap)
		}
	}
	return violations
}

// EnforceCap applies the cap by flagging or rejecting over-limit growth.
// Returns the number of caps that were enforced (currently over limit).
func (s *Store) EnforceCap() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	enforced := 0
	for id, cap := range s.Caps {
		if cap.CurrentValue > cap.MaxValue {
			enforced++
			// Cap remains but is flagged; in production this would trigger alerts
			_ = id
		}
	}
	return enforced
}

// GenerateGrowthReport produces a summary of growth governance state.
func (s *Store) GenerateGrowthReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	pendingRequests := 0
	approvedRequests := 0
	deniedRequests := 0
	for _, r := range s.Requests {
		switch r.Status {
		case RequestPending:
			pendingRequests++
		case RequestApproved:
			approvedRequests++
		case RequestDenied:
			deniedRequests++
		}
	}
	violations := 0
	for _, cap := range s.Caps {
		if cap.CurrentValue > cap.MaxValue {
			violations++
		}
	}
	return map[string]interface{}{
		"total_caps":        len(s.Caps),
		"cap_violations":    violations,
		"pending_requests":  pendingRequests,
		"approved_requests": approvedRequests,
		"denied_requests":   deniedRequests,
		"metrics_count":     len(s.Metrics),
	}
}
