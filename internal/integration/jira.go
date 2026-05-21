package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// JiraClient implements ProviderClient for Jira Cloud REST API v3.
type JiraClient struct {
	httpClient *http.Client
}

func (j *JiraClient) client() *http.Client {
	if j.httpClient == nil {
		j.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return j.httpClient
}

// jiraRequest makes an authenticated request to the Jira API.
func (j *JiraClient) jiraRequest(ctx context.Context, conn *Connection, method, path string, body interface{}) ([]byte, int, error) {
	baseURL := strings.TrimRight(conn.Config.BaseURL, "/")
	url := baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Jira Cloud uses Basic auth with email:token or just Bearer token
	if conn.Config.Email != "" && conn.Config.APIToken != "" {
		req.SetBasicAuth(conn.Config.Email, conn.Config.APIToken)
	} else if conn.Config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.Config.APIToken)
	}

	resp, err := j.client().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("jira request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

func (j *JiraClient) TestConnectivity(ctx context.Context) error {
	return nil // Validated by first real request
}

// FetchTasks queries Jira using JQL.
func (j *JiraClient) FetchTasks(ctx context.Context, conn *Connection, filters TaskFilters) ([]*Task, error) {
	jqlParts := []string{}
	project := filters.Project
	if project == "" {
		project = conn.Config.Project
	}
	if project != "" {
		jqlParts = append(jqlParts, fmt.Sprintf("project=%s", project))
	}
	if filters.Status != "" {
		jqlParts = append(jqlParts, fmt.Sprintf("status=\"%s\"", filters.Status))
	}
	if filters.Assignee != "" {
		jqlParts = append(jqlParts, fmt.Sprintf("assignee=\"%s\"", filters.Assignee))
	}
	for _, label := range filters.Labels {
		jqlParts = append(jqlParts, fmt.Sprintf("labels=\"%s\"", label))
	}
	jql := strings.Join(jqlParts, " AND ")
	if jql == "" {
		jql = "assignee=currentUser() ORDER BY updated DESC"
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}

	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": limit,
		"fields":     []string{"summary", "status", "priority", "assignee", "labels", "description", "created", "updated", "duedate", "parent", "issuetype", "project"},
	}

	body, status, err := j.jiraRequest(ctx, conn, http.MethodPost, "/rest/api/3/search", payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("jira search failed (status %d): %s", status, truncate(body, 500))
	}

	var result struct {
		Issues []jiraIssue `json:"issues"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse jira response: %w", err)
	}

	tasks := make([]*Task, 0, len(result.Issues))
	for _, issue := range result.Issues {
		tasks = append(tasks, jiraIssueToTask(issue, conn))
	}
	return tasks, nil
}

// GetTask retrieves a single issue by key.
func (j *JiraClient) GetTask(ctx context.Context, conn *Connection, key string) (*Task, error) {
	body, status, err := j.jiraRequest(ctx, conn, http.MethodGet, "/rest/api/3/issue/"+key, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("jira get issue failed (status %d): %s", status, truncate(body, 500))
	}

	var issue jiraIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("parse jira issue: %w", err)
	}
	return jiraIssueToTask(issue, conn), nil
}

// CreateTask creates a Jira issue.
func (j *JiraClient) CreateTask(ctx context.Context, conn *Connection, task *Task) (string, error) {
	project := conn.Config.Project
	if task.Project != "" {
		project = task.Project
	}
	if project == "" {
		return "", fmt.Errorf("jira project key required")
	}

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":     map[string]string{"key": project},
			"summary":     task.Title,
			"description": map[string]interface{}{"type": "doc", "version": 1, "content": []map[string]interface{}{{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": task.Description}}}}},
			"issuetype":   map[string]string{"name": "Task"},
		},
	}

	if task.Priority != "" {
		payload["fields"].(map[string]interface{})["priority"] = map[string]string{"name": jiraPriority(task.Priority)}
	}
	if len(task.Labels) > 0 {
		payload["fields"].(map[string]interface{})["labels"] = task.Labels
	}
	if task.Assignee != "" {
		payload["fields"].(map[string]interface{})["assignee"] = map[string]string{"emailAddress": task.Assignee}
	}
	if task.DueDate != nil {
		payload["fields"].(map[string]interface{})["duedate"] = task.DueDate.Format("2006-01-02")
	}

	body, status, err := j.jiraRequest(ctx, conn, http.MethodPost, "/rest/api/3/issue", payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("jira create failed (status %d): %s", status, truncate(body, 500))
	}

	var created struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return "", fmt.Errorf("parse jira create response: %w", err)
	}
	return created.Key, nil
}

// UpdateTask transitions a Jira issue and updates fields.
func (j *JiraClient) UpdateTask(ctx context.Context, conn *Connection, task *Task) error {
	// Update fields
	fields := map[string]interface{}{}
	if task.Title != "" {
		fields["summary"] = task.Title
	}
	if task.Description != "" {
		fields["description"] = map[string]interface{}{"type": "doc", "version": 1, "content": []map[string]interface{}{{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": task.Description}}}}}
	}
	if task.Priority != "" {
		fields["priority"] = map[string]string{"name": jiraPriority(task.Priority)}
	}
	if len(task.Labels) > 0 {
		fields["labels"] = task.Labels
	}
	if task.Assignee != "" {
		fields["assignee"] = map[string]string{"emailAddress": task.Assignee}
	}

	if len(fields) > 0 {
		_, status, err := j.jiraRequest(ctx, conn, http.MethodPut, "/rest/api/3/issue/"+task.ProviderKey, map[string]interface{}{"fields": fields})
		if err != nil {
			return err
		}
		if status != http.StatusNoContent && status != http.StatusOK {
			return fmt.Errorf("jira update failed (status %d)", status)
		}
	}

	// Transition status if changed
	if task.Status != "" {
		if err := j.transitionStatus(ctx, conn, task.ProviderKey, task.Status); err != nil {
			return fmt.Errorf("transition: %w", err)
		}
	}

	return nil
}

// AddComment adds a comment to a Jira issue.
func (j *JiraClient) AddComment(ctx context.Context, conn *Connection, issueKey string, author string, body string) error {
	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": body}}},
			},
		},
	}

	_, status, err := j.jiraRequest(ctx, conn, http.MethodPost, "/rest/api/3/issue/"+issueKey+"/comment", payload)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("jira comment failed (status %d)", status)
	}
	return nil
}

// transitionStatus looks up the transition ID for a target status name and performs the transition.
func (j *JiraClient) transitionStatus(ctx context.Context, conn *Connection, issueKey, targetStatus string) error {
	body, status, err := j.jiraRequest(ctx, conn, http.MethodGet, "/rest/api/3/issue/"+issueKey+"/transitions", nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("get transitions failed (status %d)", status)
	}

	var transitions struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(body, &transitions); err != nil {
		return fmt.Errorf("parse transitions: %w", err)
	}

	target := strings.ToLower(targetStatus)
	for _, tr := range transitions.Transitions {
		if strings.ToLower(tr.To.Name) == target || strings.ToLower(tr.Name) == target {
			_, status, err := j.jiraRequest(ctx, conn, http.MethodPost, "/rest/api/3/issue/"+issueKey+"/transitions", map[string]string{"transition": tr.ID})
			if err != nil {
				return err
			}
			if status != http.StatusNoContent && status != http.StatusOK {
				return fmt.Errorf("transition failed (status %d)", status)
			}
			return nil
		}
	}

	return fmt.Errorf("no transition found to status %q", targetStatus)
}

// --- Jira API types ---

type jiraIssue struct {
	Key    string `json:"key"`
	ID     string `json:"id"`
	Fields struct {
		Summary     string `json:"summary"`
		Description interface{} `json:"description"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Assignee *struct {
			EmailAddress string `json:"emailAddress"`
			DisplayName  string `json:"displayName"`
		} `json:"assignee"`
		Labels    []string  `json:"labels"`
		Project   struct {
			Key string `json:"key"`
		} `json:"project"`
		Parent *struct {
			Key string `json:"key"`
		} `json:"parent"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Created   string  `json:"created"`
		Updated   string  `json:"updated"`
		DueDate   string  `json:"duedate"`
	} `json:"fields"`
}

func jiraIssueToTask(issue jiraIssue, conn *Connection) *Task {
	t := &Task{
		ID:          issue.ID,
		ProviderKey: issue.Key,
		Title:       issue.Fields.Summary,
		Status:      issue.Fields.Status.Name,
		Priority:    normalizePriority(issue.Fields.Priority.Name),
		Labels:      issue.Fields.Labels,
		Project:     issue.Fields.Project.Key,
		URL:         fmt.Sprintf("%s/browse/%s", strings.TrimRight(conn.Config.BaseURL, "/"), issue.Key),
		CustomFields: map[string]string{
			"issue_type": issue.Fields.IssueType.Name,
		},
	}

	if issue.Fields.Assignee != nil {
		t.Assignee = issue.Fields.Assignee.EmailAddress
	}

	if issue.Fields.Parent != nil {
		t.ParentID = issue.Fields.Parent.Key
	}

	t.CreatedAt, _ = parseJiraTime(issue.Fields.Created)
	t.UpdatedAt, _ = parseJiraTime(issue.Fields.Updated)

	if issue.Fields.DueDate != "" {
		if d, err := time.Parse("2006-01-02", issue.Fields.DueDate); err == nil {
			t.DueDate = &d
		}
	}

	// Extract description text from Atlassian Document Format
	t.Description = extractADFText(issue.Fields.Description)

	return t
}

func parseJiraTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	// Jira returns ISO 8601 like 2024-01-15T10:30:00.000+0000
	layouts := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05-0700",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}

// extractADFText extracts plain text from Atlassian Document Format.
func extractADFText(doc interface{}) string {
	if doc == nil {
		return ""
	}
	switch v := doc.(type) {
	case string:
		return v
	case map[string]interface{}:
		content, ok := v["content"].([]interface{})
		if !ok {
			return ""
		}
		var parts []string
		for _, item := range content {
			parts = append(parts, extractADFText(item))
		}
		return strings.Join(parts, "\n")
	case []interface{}:
		var parts []string
		for _, item := range v {
			parts = append(parts, extractADFText(item))
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func jiraPriority(p string) string {
	switch strings.ToLower(p) {
	case "critical":
		return "Highest"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return "Medium"
	}
}

func normalizePriority(p string) string {
	switch strings.ToLower(p) {
	case "highest", "critical", "blocker":
		return "critical"
	case "high":
		return "high"
	case "medium", "normal":
		return "medium"
	case "low", "lowest", "trivial":
		return "low"
	default:
		return "medium"
	}
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// atoi is a helper for safe string-to-int conversion.
func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
