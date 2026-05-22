// Package integration provides GitHub integration with real API calls.
// This file implements the HTTP client that talks to GitHub's REST API v3.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHubClient makes authenticated calls to the GitHub API.
type GitHubClient struct {
	BaseURL   string
	Token     string
	Owner     string
	Repo      string
	HTTP      *http.Client
	UserAgent string
}

// NewGitHubClient creates a client for the GitHub API.
func NewGitHubClient(token, owner, repo string) *GitHubClient {
	return &GitHubClient{
		BaseURL:   "https://api.github.com",
		Token:     token,
		Owner:     owner,
		Repo:      repo,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: "Forge-Engine/1.0",
	}
}

// --- REST helpers ---

func (gc *GitHubClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("github: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, gc.BaseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("github: create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+gc.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", gc.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := gc.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("github: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("github: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("github: API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

func (gc *GitHubClient) repoPath(apiPath string) string {
	return fmt.Sprintf("/repos/%s/%s%s", gc.Owner, gc.Repo, apiPath)
}

// --- Pull Requests ---

// CreatePullRequest creates a PR via the GitHub API.
func (gc *GitHubClient) CreatePullRequest(ctx context.Context, title, body, head, base string) (*PullRequest, error) {
	if title == "" {
		return nil, fmt.Errorf("github: title required")
	}
	if head == "" {
		return nil, fmt.Errorf("github: head branch required")
	}
	if base == "" {
		base = "main"
	}

	payload := map[string]interface{}{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	}

	respBody, _, err := gc.doRequest(ctx, http.MethodPost, gc.repoPath("/pulls"), payload)
	if err != nil {
		return nil, err
	}

	var ghPR struct {
		ID     int    `json:"id"`
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
		Draft     bool      `json:"draft"`
		HTMLURL   string    `json:"html_url"`
	}
	if err := json.Unmarshal(respBody, &ghPR); err != nil {
		return nil, fmt.Errorf("github: parse PR response: %w", err)
	}

	var labels []string
	for _, l := range ghPR.Labels {
		labels = append(labels, l.Name)
	}

	return &PullRequest{
		ID:        ghPR.ID,
		Number:    ghPR.Number,
		Title:     ghPR.Title,
		Body:      ghPR.Body,
		State:     ghState(ghPR.State, ghPR.MergedAt),
		Branch:    ghPR.Head.Ref,
		Base:      ghPR.Base.Ref,
		Author:    ghPR.User.Login,
		Labels:    labels,
		CreatedAt: ghPR.CreatedAt,
		UpdatedAt: ghPR.UpdatedAt,
		MergedAt:  ghPR.MergedAt,
		Draft:     ghPR.Draft,
	}, nil
}

// ListPullRequests fetches open PRs from GitHub.
func (gc *GitHubClient) ListPullRequests(ctx context.Context, state string) ([]*PullRequest, error) {
	path := gc.repoPath("/pulls")
	if state != "" {
		path += "?state=" + state
	} else {
		path += "?state=open"
	}

	respBody, _, err := gc.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var ghPRs []struct {
		ID     int    `json:"id"`
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
		Draft     bool       `json:"draft"`
	}
	if err := json.Unmarshal(respBody, &ghPRs); err != nil {
		return nil, fmt.Errorf("github: parse PR list: %w", err)
	}

	var prs []*PullRequest
	for _, gh := range ghPRs {
		prs = append(prs, &PullRequest{
			ID:        gh.ID,
			Number:    gh.Number,
			Title:     gh.Title,
			State:     ghState(gh.State, gh.MergedAt),
			Branch:    gh.Head.Ref,
			Base:      gh.Base.Ref,
			Author:    gh.User.Login,
			CreatedAt: gh.CreatedAt,
			UpdatedAt: gh.UpdatedAt,
			MergedAt:  gh.MergedAt,
			Draft:     gh.Draft,
		})
	}
	return prs, nil
}

// GetPullRequest fetches a single PR.
func (gc *GitHubClient) GetPullRequest(ctx context.Context, number int) (*PullRequest, error) {
	path := gc.repoPath(fmt.Sprintf("/pulls/%d", number))
	respBody, _, err := gc.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var ghPR struct {
		ID     int    `json:"id"`
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
		Draft     bool       `json:"draft"`
		Merged    bool       `json:"merged"`
		Mergeable *bool      `json:"mergeable"`
	}
	if err := json.Unmarshal(respBody, &ghPR); err != nil {
		return nil, fmt.Errorf("github: parse PR: %w", err)
	}

	var labels []string
	for _, l := range ghPR.Labels {
		labels = append(labels, l.Name)
	}

	return &PullRequest{
		ID:        ghPR.ID,
		Number:    ghPR.Number,
		Title:     ghPR.Title,
		Body:      ghPR.Body,
		State:     ghState(ghPR.State, ghPR.MergedAt),
		Branch:    ghPR.Head.Ref,
		Base:      ghPR.Base.Ref,
		Author:    ghPR.User.Login,
		Labels:    labels,
		CreatedAt: ghPR.CreatedAt,
		UpdatedAt: ghPR.UpdatedAt,
		MergedAt:  ghPR.MergedAt,
		Draft:     ghPR.Draft,
	}, nil
}

// MergePullRequest merges a PR via the GitHub API.
func (gc *GitHubClient) MergePullRequest(ctx context.Context, number int, commitTitle string) (*PullRequest, error) {
	path := gc.repoPath(fmt.Sprintf("/pulls/%d/merge", number))
	payload := map[string]interface{}{
		"commit_title": commitTitle,
		"merge_method": "squash",
	}
	if commitTitle == "" {
		payload = map[string]interface{}{
			"merge_method": "squash",
		}
	}

	respBody, status, err := gc.doRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		// 405 = not mergeable, 409 = conflict
		if status == 405 {
			return nil, fmt.Errorf("github: PR #%d is not mergeable", number)
		}
		return nil, err
	}

	var result struct {
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
		SHA     string `json:"sha"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("github: parse merge response: %w", err)
	}
	if !result.Merged {
		return nil, fmt.Errorf("github: PR #%d not merged: %s", number, result.Message)
	}

	// Fetch updated PR
	return gc.GetPullRequest(ctx, number)
}

// --- Issues ---

// CreateIssue creates an issue via the GitHub API.
func (gc *GitHubClient) CreateIssue(ctx context.Context, title, body string, labels []string) (*Issue, error) {
	if title == "" {
		return nil, fmt.Errorf("github: title required")
	}

	payload := map[string]interface{}{
		"title":  title,
		"body":   body,
		"labels": labels,
	}

	respBody, _, err := gc.doRequest(ctx, http.MethodPost, gc.repoPath("/issues"), payload)
	if err != nil {
		return nil, err
	}

	var ghIssue struct {
		ID        int    `json:"id"`
		Number    int    `json:"number"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		State     string `json:"state"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(respBody, &ghIssue); err != nil {
		return nil, fmt.Errorf("github: parse issue response: %w", err)
	}

	var assignees []string
	for _, a := range ghIssue.Assignees {
		assignees = append(assignees, a.Login)
	}
	var issueLabels []string
	for _, l := range ghIssue.Labels {
		issueLabels = append(issueLabels, l.Name)
	}
	var milestone string
	if ghIssue.Milestone != nil {
		milestone = ghIssue.Milestone.Title
	}

	return &Issue{
		ID:        ghIssue.ID,
		Number:    ghIssue.Number,
		Title:     ghIssue.Title,
		Body:      ghIssue.Body,
		State:     mapGHState(ghIssue.State),
		Author:    ghIssue.User.Login,
		Assignees: assignees,
		Labels:    issueLabels,
		Milestone: milestone,
		CreatedAt: ghIssue.CreatedAt,
		UpdatedAt: ghIssue.UpdatedAt,
	}, nil
}

// ListIssues fetches issues from GitHub.
func (gc *GitHubClient) ListIssues(ctx context.Context, state string) ([]*Issue, error) {
	path := gc.repoPath("/issues")
	if state != "" {
		path += "?state=" + state
	}

	respBody, _, err := gc.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var ghIssues []struct {
		ID        int    `json:"id"`
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		PullRequest *struct{} `json:"pull_request,omitempty"`
	}
	if err := json.Unmarshal(respBody, &ghIssues); err != nil {
		return nil, fmt.Errorf("github: parse issues: %w", err)
	}

	var issues []*Issue
	for _, gh := range ghIssues {
		// Skip PRs (GitHub returns them in the issues endpoint)
		if gh.PullRequest != nil {
			continue
		}
		var labels []string
		for _, l := range gh.Labels {
			labels = append(labels, l.Name)
		}
		issues = append(issues, &Issue{
			ID:        gh.ID,
			Number:    gh.Number,
			Title:     gh.Title,
			State:     mapGHState(gh.State),
			Author:    gh.User.Login,
			Labels:    labels,
			CreatedAt: gh.CreatedAt,
			UpdatedAt: gh.UpdatedAt,
		})
	}
	return issues, nil
}

// CloseIssue closes an issue.
func (gc *GitHubClient) CloseIssue(ctx context.Context, number int) error {
	path := gc.repoPath(fmt.Sprintf("/issues/%d", number))
	payload := map[string]interface{}{"state": "closed"}
	_, _, err := gc.doRequest(ctx, http.MethodPatch, path, payload)
	return err
}

// --- Reviews ---

// CreateReview submits a review on a PR.
func (gc *GitHubClient) CreateReview(ctx context.Context, prNumber int, event, body string) (*Review, error) {
	path := gc.repoPath(fmt.Sprintf("/pulls/%d/reviews", prNumber))
	payload := map[string]interface{}{
		"body":  body,
		"event": event, // APPROVE, REQUEST_CHANGES, COMMENT
	}

	respBody, _, err := gc.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}

	var ghReview struct {
		ID        int       `json:"id"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		State     string    `json:"state"`
		Body      string    `json:"body"`
		SubmittedAt time.Time `json:"submitted_at"`
	}
	if err := json.Unmarshal(respBody, &ghReview); err != nil {
		return nil, fmt.Errorf("github: parse review: %w", err)
	}

	return &Review{
		ID:        ghReview.ID,
		PRNumber:  prNumber,
		Author:    ghReview.User.Login,
		State:     strings.ToLower(ghReview.State),
		Body:      ghReview.Body,
		CreatedAt: ghReview.SubmittedAt,
	}, nil
}

// ListReviews fetches reviews for a PR.
func (gc *GitHubClient) ListReviews(ctx context.Context, prNumber int) ([]*Review, error) {
	path := gc.repoPath(fmt.Sprintf("/pulls/%d/reviews", prNumber))
	respBody, _, err := gc.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var ghReviews []struct {
		ID        int       `json:"id"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		State     string    `json:"state"`
		Body      string    `json:"body"`
		SubmittedAt time.Time `json:"submitted_at"`
	}
	if err := json.Unmarshal(respBody, &ghReviews); err != nil {
		return nil, fmt.Errorf("github: parse reviews: %w", err)
	}

	var reviews []*Review
	for _, gh := range ghReviews {
		reviews = append(reviews, &Review{
			ID:        gh.ID,
			PRNumber:  prNumber,
			Author:    gh.User.Login,
			State:     strings.ToLower(gh.State),
			Body:      gh.Body,
			CreatedAt: gh.SubmittedAt,
		})
	}
	return reviews, nil
}

// --- Helpers ---

func ghState(state string, mergedAt *time.Time) GHState {
	if mergedAt != nil && !mergedAt.IsZero() {
		return GHMerged
	}
	switch state {
	case "open":
		return GHOpen
	case "closed":
		return GHClosed
	default:
		return GHState(state)
	}
}

func mapGHState(state string) GHState {
	switch state {
	case "open":
		return GHOpen
	case "closed":
		return GHClosed
	default:
		return GHState(state)
	}
}
