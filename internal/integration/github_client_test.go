package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGitHubClientCreatePR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/pulls" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("missing auth header")
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["title"] != "Test PR" {
			t.Errorf("expected title 'Test PR', got %v", payload["title"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     42,
			"number": 7,
			"title":  "Test PR",
			"body":   "PR description",
			"state":  "open",
			"head":   map[string]string{"ref": "feature-branch"},
			"base":   map[string]string{"ref": "main"},
			"user":   map[string]string{"login": "dev"},
			"labels": []map[string]string{{"name": "enhancement"}},
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
			"draft":  false,
			"html_url": "https://github.com/owner/repo/pull/7",
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("test-token", "owner", "repo")
	gc.BaseURL = srv.URL

	pr, err := gc.CreatePullRequest(context.Background(), "Test PR", "PR description", "feature-branch", "main")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if pr.Number != 7 {
		t.Errorf("expected PR #7, got %d", pr.Number)
	}
	if pr.State != GHOpen {
		t.Errorf("expected open, got %s", pr.State)
	}
	if pr.Branch != "feature-branch" {
		t.Errorf("expected feature-branch, got %s", pr.Branch)
	}
	if pr.Author != "dev" {
		t.Errorf("expected dev, got %s", pr.Author)
	}
	if len(pr.Labels) != 1 || pr.Labels[0] != "enhancement" {
		t.Errorf("expected [enhancement], got %v", pr.Labels)
	}
}

func TestGitHubClientCreatePRValidation(t *testing.T) {
	gc := NewGitHubClient("token", "owner", "repo")

	_, err := gc.CreatePullRequest(context.Background(), "", "body", "head", "main")
	if err == nil {
		t.Error("expected error for empty title")
	}

	_, err = gc.CreatePullRequest(context.Background(), "Title", "body", "", "main")
	if err == nil {
		t.Error("expected error for empty head")
	}
}

func TestGitHubClientListPRs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "open" {
			t.Errorf("expected state=open, got %s", r.URL.Query().Get("state"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id":     1,
				"number": 1,
				"title":  "First PR",
				"state":  "open",
				"head":   map[string]string{"ref": "feat-1"},
				"base":   map[string]string{"ref": "main"},
				"user":   map[string]string{"login": "alice"},
				"created_at": time.Now().Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
			},
			{
				"id":     2,
				"number": 2,
				"title":  "Second PR",
				"state":  "open",
				"head":   map[string]string{"ref": "feat-2"},
				"base":   map[string]string{"ref": "main"},
				"user":   map[string]string{"login": "bob"},
				"created_at": time.Now().Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
			},
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	prs, err := gc.ListPullRequests(context.Background(), "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}
	if prs[0].Title != "First PR" {
		t.Errorf("expected 'First PR', got %s", prs[0].Title)
	}
	if prs[1].Author != "bob" {
		t.Errorf("expected bob, got %s", prs[1].Author)
	}
}

func TestGitHubClientMergePR(t *testing.T) {
	var requestNum int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNum++
		if requestNum == 1 {
			// Merge request
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT for merge, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"merged":  true,
				"message": "Pull Request successfully merged",
				"sha":     "abc123",
			})
		} else {
			// Get updated PR
			now := time.Now().Format(time.RFC3339)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     5,
				"number": 5,
				"title":  "Merged PR",
				"state":  "closed",
				"head":   map[string]string{"ref": "feat"},
				"base":   map[string]string{"ref": "main"},
				"user":   map[string]string{"login": "dev"},
				"created_at": now,
				"updated_at": now,
				"merged_at":  now,
				"draft":  false,
			})
		}
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	pr, err := gc.MergePullRequest(context.Background(), 5, "Merge PR #5")
	if err != nil {
		t.Fatal(err)
	}
	if pr.State != GHMerged {
		t.Errorf("expected merged state, got %s", pr.State)
	}
}

func TestGitHubClientMergeNotMergeable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Pull Request is not mergeable",
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	_, err := gc.MergePullRequest(context.Background(), 5, "")
	if err == nil {
		t.Error("expected error for non-mergeable PR")
	}
}

func TestGitHubClientCreateIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/issues" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["title"] != "Bug report" {
			t.Errorf("expected 'Bug report', got %v", payload["title"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":        10,
			"number":    3,
			"title":     "Bug report",
			"body":      "Something broke",
			"state":     "open",
			"user":      map[string]string{"login": "dev"},
			"assignees": []map[string]string{{"login": "alice"}},
			"labels":    []map[string]string{{"name": "bug"}},
			"milestone": map[string]string{"title": "v1.0"},
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	issue, err := gc.CreateIssue(context.Background(), "Bug report", "Something broke", []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Number != 3 {
		t.Errorf("expected issue #3, got %d", issue.Number)
	}
	if issue.State != GHOpen {
		t.Errorf("expected open, got %s", issue.State)
	}
	if len(issue.Assignees) != 1 || issue.Assignees[0] != "alice" {
		t.Errorf("expected [alice], got %v", issue.Assignees)
	}
	if issue.Milestone != "v1.0" {
		t.Errorf("expected v1.0, got %s", issue.Milestone)
	}
}

func TestGitHubClientCreateIssueValidation(t *testing.T) {
	gc := NewGitHubClient("token", "owner", "repo")
	_, err := gc.CreateIssue(context.Background(), "", "body", nil)
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestGitHubClientListIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id":     1,
				"number": 1,
				"title":  "Real issue",
				"state":  "open",
				"user":   map[string]string{"login": "dev"},
				"labels": []map[string]string{{"name": "bug"}},
				"created_at": time.Now().Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
			},
			{
				"id":     2,
				"number": 2,
				"title":  "PR disguised as issue",
				"state":  "open",
				"user":   map[string]string{"login": "dev"},
				"labels": []map[string]string{},
				"created_at": time.Now().Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
				"pull_request": map[string]string{"url": "https://api.github.com/repos/owner/repo/pulls/2"},
			},
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	issues, err := gc.ListIssues(context.Background(), "open")
	if err != nil {
		t.Fatal(err)
	}
	// Should skip the PR
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue (PRs filtered), got %d", len(issues))
	}
	if issues[0].Title != "Real issue" {
		t.Errorf("expected 'Real issue', got %s", issues[0].Title)
	}
}

func TestGitHubClientCloseIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["state"] != "closed" {
			t.Errorf("expected state=closed, got %v", payload["state"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"state": "closed"})
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	if err := gc.CloseIssue(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubClientCreateReview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["event"] != "APPROVE" {
			t.Errorf("expected APPROVE, got %v", payload["event"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    99,
			"user":  map[string]string{"login": "reviewer"},
			"state": "APPROVED",
			"body":  "LGTM",
			"submitted_at": time.Now().Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	review, err := gc.CreateReview(context.Background(), 7, "APPROVE", "LGTM")
	if err != nil {
		t.Fatal(err)
	}
	if review.ID != 99 {
		t.Errorf("expected review ID 99, got %d", review.ID)
	}
	if review.State != "approved" {
		t.Errorf("expected approved, got %s", review.State)
	}
	if review.Author != "reviewer" {
		t.Errorf("expected reviewer, got %s", review.Author)
	}
}

func TestGitHubClientAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("bad-token", "owner", "repo")
	gc.BaseURL = srv.URL

	_, err := gc.ListPullRequests(context.Background(), "open")
	if err == nil {
		t.Error("expected error for 401")
	}
}

func TestGitHubClientDefaultBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify default base branch
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["base"] != "main" {
			t.Errorf("expected default base 'main', got %v", payload["base"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     1,
			"number": 1,
			"title":  "Test",
			"state":  "open",
			"head":   map[string]string{"ref": "feat"},
			"base":   map[string]string{"ref": "main"},
			"user":   map[string]string{"login": "dev"},
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	gc := NewGitHubClient("token", "owner", "repo")
	gc.BaseURL = srv.URL

	pr, err := gc.CreatePullRequest(context.Background(), "Test", "", "feat", "")
	if err != nil {
		t.Fatal(err)
	}
	if pr.Base != "main" {
		t.Errorf("expected base=main, got %s", pr.Base)
	}
}
