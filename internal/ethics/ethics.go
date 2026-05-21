// Package ethics provides external ethics review and whistleblower reporting.
// Independent oversight ensures the org does the right thing, even when nobody's watching.
package ethics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ReviewStatus tracks ethics review lifecycle.
type ReviewStatus string

const (
	ReviewSubmitted  ReviewStatus = "submitted"
	ReviewInProgress ReviewStatus = "in_progress"
	ReviewApproved   ReviewStatus = "approved"
	ReviewFlagged    ReviewStatus = "flagged"
	ReviewRejected   ReviewStatus = "rejected"
)

// ReviewCategory classifies what's being reviewed.
type ReviewCategory string

const (
	CategoryDecision    ReviewCategory = "decision"
	CategoryProduct     ReviewCategory = "product"
	CategoryDataUse     ReviewCategory = "data_use"
	CategoryAutomation  ReviewCategory = "automation"
	CategoryEmployment  ReviewCategory = "employment"
	CategoryEnvironmental ReviewCategory = "environmental"
)

// ReviewRequest is a request for ethics review.
type ReviewRequest struct {
	ID          string         `json:"id"`
	Category    ReviewCategory `json:"category"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Requester   string         `json:"requester"` // agent or division ID
	Impact      string         `json:"impact"`    // who/what is affected
	Status      ReviewStatus   `json:"status"`
	Reviewer    string         `json:"reviewer,omitempty"`
	Decision    string         `json:"decision,omitempty"`
	Conditions  []string       `json:"conditions,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	ReviewedAt  *time.Time     `json:"reviewed_at,omitempty"`
}

// WhistleblowerReport is an anonymous internal report of wrongdoing.
type WhistleblowerReport struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"` // fraud, abuse, safety, discrimination, other
	Description string    `json:"description"`
	Severity    string    `json:"severity"` // low, medium, high, critical
	Status      string    `json:"status"`   // submitted, investigating, resolved
	Reporter    string    `json:"reporter,omitempty"` // optional, can be anonymous
	Investigator string   `json:"investigator,omitempty"`
	Resolution  string    `json:"resolution,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Anonymous   bool      `json:"anonymous"`
}

// BoardMember is a member of the external ethics board.
type BoardMember struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Expertise string    `json:"expertise"`
	Active    bool      `json:"active"`
	AppointedAt time.Time `json:"appointed_at"`
}

// Board manages ethics reviews and whistleblower reports.
type Board struct {
	mu       sync.RWMutex
	members  map[string]*BoardMember
	reviews  map[string]*ReviewRequest
	reports  map[string]*WhistleblowerReport
	path     string
}

// NewBoard creates a new ethics board.
func NewBoard(persistPath string) *Board {
	b := &Board{
		members: make(map[string]*BoardMember),
		reviews: make(map[string]*ReviewRequest),
		reports: make(map[string]*WhistleblowerReport),
		path:    persistPath,
	}
	b.load()
	return b
}

// --- Board Management ---

// AppointMember adds a member to the ethics board.
func (b *Board) AppointMember(name, expertise string) (*BoardMember, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	m := &BoardMember{
		ID:          genID("board"),
		Name:        name,
		Expertise:   expertise,
		Active:      true,
		AppointedAt: time.Now().UTC(),
	}
	b.members[m.ID] = m
	b.persist()
	return m, nil
}

// ListMembers returns active board members.
func (b *Board) ListMembers() []*BoardMember {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var result []*BoardMember
	for _, m := range b.members {
		if m.Active {
			result = append(result, m)
		}
	}
	return result
}

// --- Ethics Reviews ---

// RequestReview submits a decision for ethics review.
func (b *Board) RequestReview(category ReviewCategory, title, description, requester, impact string) (*ReviewRequest, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	req := &ReviewRequest{
		ID:          genID("review"),
		Category:    category,
		Title:       title,
		Description: description,
		Requester:   requester,
		Impact:      impact,
		Status:      ReviewSubmitted,
		CreatedAt:   time.Now().UTC(),
	}

	b.reviews[req.ID] = req
	b.persist()
	return req, nil
}

// StartReview assigns a reviewer and begins the review.
func (b *Board) StartReview(reviewID, reviewerID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reviews[reviewID]
	if !ok {
		return fmt.Errorf("review %s not found", reviewID)
	}
	r.Status = ReviewInProgress
	r.Reviewer = reviewerID
	r.ReviewedAt = nil
	b.persist()
	return nil
}

// ApproveReview approves the reviewed action, optionally with conditions.
func (b *Board) ApproveReview(reviewID, decision string, conditions []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reviews[reviewID]
	if !ok {
		return fmt.Errorf("review %s not found", reviewID)
	}
	r.Status = ReviewApproved
	r.Decision = decision
	r.Conditions = conditions
	now := time.Now().UTC()
	r.ReviewedAt = &now
	b.persist()
	return nil
}

// FlagReview flags a review as ethically concerning.
func (b *Board) FlagReview(reviewID, decision string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reviews[reviewID]
	if !ok {
		return fmt.Errorf("review %s not found", reviewID)
	}
	r.Status = ReviewFlagged
	r.Decision = decision
	now := time.Now().UTC()
	r.ReviewedAt = &now
	b.persist()
	return nil
}

// ListReviews returns reviews filtered by status.
func (b *Board) ListReviews(status ReviewStatus) []*ReviewRequest {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*ReviewRequest
	for _, r := range b.reviews {
		if status == "" || r.Status == status {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// --- Whistleblower ---

// SubmitReport submits a whistleblower report (can be anonymous).
func (b *Board) SubmitReport(category, description, severity, reporter string, anonymous bool) (*WhistleblowerReport, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().UTC()
	wr := &WhistleblowerReport{
		ID:          genID("wb"),
		Category:    category,
		Description: description,
		Severity:    severity,
		Reporter:    reporter,
		Status:      "submitted",
		CreatedAt:   now,
		UpdatedAt:   now,
		Anonymous:   anonymous,
	}

	if anonymous {
		wr.Reporter = ""
	}

	b.reports[wr.ID] = wr
	b.persist()
	return wr, nil
}

// InvestigateReport assigns an investigator and starts investigation.
func (b *Board) InvestigateReport(reportID, investigator string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reports[reportID]
	if !ok {
		return fmt.Errorf("report %s not found", reportID)
	}
	r.Status = "investigating"
	r.Investigator = investigator
	r.UpdatedAt = time.Now().UTC()
	b.persist()
	return nil
}

// ResolveReport resolves a whistleblower report.
func (b *Board) ResolveReport(reportID, resolution string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reports[reportID]
	if !ok {
		return fmt.Errorf("report %s not found", reportID)
	}
	r.Status = "resolved"
	r.Resolution = resolution
	now := time.Now().UTC()
	r.ResolvedAt = &now
	r.UpdatedAt = now
	b.persist()
	return nil
}

// ListOpenReports returns unresolved whistleblower reports.
func (b *Board) ListOpenReports() []*WhistleblowerReport {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*WhistleblowerReport
	for _, r := range b.reports {
		if r.Status != "resolved" {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

func (b *Board) persist() {
	if b.path == "" { return }
	data := struct {
		Members map[string]*BoardMember       `json:"members"`
		Reviews map[string]*ReviewRequest     `json:"reviews"`
		Reports map[string]*WhistleblowerReport `json:"reports"`
	}{b.members, b.reviews, b.reports}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(b.path), 0755)
	os.WriteFile(b.path, raw, 0644)
}

func (b *Board) load() {
	if b.path == "" { return }
	raw, err := os.ReadFile(b.path)
	if err != nil { return }
	var data struct {
		Members map[string]*BoardMember       `json:"members"`
		Reviews map[string]*ReviewRequest     `json:"reviews"`
		Reports map[string]*WhistleblowerReport `json:"reports"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Members != nil { b.members = data.Members }
		if data.Reviews != nil { b.reviews = data.Reviews }
		if data.Reports != nil { b.reports = data.Reports }
	}
}

func genID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }
