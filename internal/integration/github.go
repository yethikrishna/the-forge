// Package integration provides GitHub integration for PRs, issues, and reviews.
package integration

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// GHState is the state of a PR or issue.
type GHState string

const (
	GHOpen   GHState = "open"
	GHClosed GHState = "closed"
	GHMerged GHState = "merged"
)

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	ID        int       `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	State     GHState   `json:"state"`
	Branch    string    `json:"branch"`
	Base      string    `json:"base"`
	Author    string    `json:"author"`
	Labels    []string  `json:"labels,omitempty"`
	Reviewers []string  `json:"reviewers,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`
	Draft     bool      `json:"draft"`
}

// Issue represents a GitHub issue.
type Issue struct {
	ID        int       `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	State     GHState   `json:"state"`
	Author    string    `json:"author"`
	Assignees []string  `json:"assignees,omitempty"`
	Labels    []string  `json:"labels,omitempty"`
	Milestone string    `json:"milestone,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Review represents a PR review.
type Review struct {
	ID        int       `json:"id"`
	PRNumber  int       `json:"pr_number"`
	Author    string    `json:"author"`
	State     string    `json:"state"` // approved, changes_requested, commented
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// GitHubConfig configures GitHub integration.
type GitHubConfig struct {
	Token    string `json:"token"`
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	BaseURL  string `json:"base_url,omitempty"` // for GitHub Enterprise
}

// GitHubManager handles GitHub operations.
type GitHubManager struct {
	config GitHubConfig
	prs    map[int]*PullRequest
	issues map[int]*Issue
	reviews map[int][]*Review
	mu     sync.RWMutex
	nextPR  int
	nextIssue int
}

// NewGitHubManager creates a GitHub manager.
func NewGitHubManager(config GitHubConfig) *GitHubManager {
	return &GitHubManager{
		config:   config,
		prs:      make(map[int]*PullRequest),
		issues:   make(map[int]*Issue),
		reviews:  make(map[int][]*Review),
		nextPR:   1,
		nextIssue: 1,
	}
}

// CreatePR creates a pull request.
func (gm *GitHubManager) CreatePR(title, body, branch, base string) (*PullRequest, error) {
	if title == "" {
		return nil, fmt.Errorf("github: title required")
	}
	if branch == "" {
		return nil, fmt.Errorf("github: branch required")
	}
	if base == "" {
		base = "main"
	}

	gm.mu.Lock()
	num := gm.nextPR
	gm.nextPR++
	pr := &PullRequest{
		Number:    num,
		Title:     title,
		Body:      body,
		State:     GHOpen,
		Branch:    branch,
		Base:      base,
		Author:    "forge-agent",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	gm.prs[num] = pr
	gm.mu.Unlock()

	return pr, nil
}

// ListPRs returns all PRs.
func (gm *GitHubManager) ListPRs(state GHState) []*PullRequest {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*PullRequest
	for _, pr := range gm.prs {
		if state == "" || pr.State == state {
			result = append(result, pr)
		}
	}
	return result
}

// GetPR retrieves a PR by number.
func (gm *GitHubManager) GetPR(num int) (*PullRequest, error) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	pr, ok := gm.prs[num]
	if !ok {
		return nil, fmt.Errorf("github: PR #%d not found", num)
	}
	return pr, nil
}

// MergePR merges a PR.
func (gm *GitHubManager) MergePR(num int) (*PullRequest, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	pr, ok := gm.prs[num]
	if !ok {
		return nil, fmt.Errorf("github: PR #%d not found", num)
	}
	if pr.State != GHOpen {
		return nil, fmt.Errorf("github: PR #%d is not open", num)
	}

	pr.State = GHMerged
	now := time.Now()
	pr.MergedAt = &now
	pr.UpdatedAt = now
	return pr, nil
}

// ClosePR closes a PR.
func (gm *GitHubManager) ClosePR(num int) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	pr, ok := gm.prs[num]
	if !ok {
		return fmt.Errorf("github: PR #%d not found", num)
	}
	pr.State = GHClosed
	pr.UpdatedAt = time.Now()
	return nil
}

// CreateIssue creates an issue.
func (gm *GitHubManager) CreateIssue(title, body string, labels []string) (*Issue, error) {
	if title == "" {
		return nil, fmt.Errorf("github: title required")
	}

	gm.mu.Lock()
	num := gm.nextIssue
	gm.nextIssue++
	issue := &Issue{
		Number:    num,
		Title:     title,
		Body:      body,
		State:     GHOpen,
		Author:    "forge-agent",
		Labels:    labels,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	gm.issues[num] = issue
	gm.mu.Unlock()

	return issue, nil
}

// ListIssues returns issues.
func (gm *GitHubManager) ListIssues(state GHState) []*Issue {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*Issue
	for _, issue := range gm.issues {
		if state == "" || issue.State == state {
			result = append(result, issue)
		}
	}
	return result
}

// CloseIssue closes an issue.
func (gm *GitHubManager) CloseIssue(num int) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	issue, ok := gm.issues[num]
	if !ok {
		return fmt.Errorf("github: issue #%d not found", num)
	}
	issue.State = GHClosed
	issue.UpdatedAt = time.Now()
	return nil
}

// AddReview adds a review to a PR.
func (gm *GitHubManager) AddReview(prNum int, state, body, author string) (*Review, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if _, ok := gm.prs[prNum]; !ok {
		return nil, fmt.Errorf("github: PR #%d not found", prNum)
	}

	review := &Review{
		ID:        len(gm.reviews[prNum]) + 1,
		PRNumber:  prNum,
		Author:    author,
		State:     state,
		Body:      body,
		CreatedAt: time.Now(),
	}
	gm.reviews[prNum] = append(gm.reviews[prNum], review)
	return review, nil
}

// GetReviews returns reviews for a PR.
func (gm *GitHubManager) GetReviews(prNum int) ([]*Review, error) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	if _, ok := gm.prs[prNum]; !ok {
		return nil, fmt.Errorf("github: PR #%d not found", prNum)
	}
	reviews := gm.reviews[prNum]
	if reviews == nil {
		return []*Review{}, nil
	}
	return reviews, nil
}

var _ = json.Marshal
