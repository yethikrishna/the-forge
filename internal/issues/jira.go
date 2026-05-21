package issues

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// JiraProvider implements the Provider interface for Atlassian Jira.
type JiraProvider struct {
	url   string
	email string
	token string
}

// NewJiraProvider creates a new Jira provider from config.
func NewJiraProvider(cfg *ProviderConfig) (*JiraProvider, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("jira: API token required")
	}
	if cfg.Email == "" {
		return nil, fmt.Errorf("jira: email required")
	}
	url := cfg.URL
	if url == "" {
		url = "https://api.atlassian.com"
	}
	return &JiraProvider{
		url:   strings.TrimRight(url, "/"),
		email: cfg.Email,
		token: cfg.Token,
	}, nil
}

func (j *JiraProvider) Type() ProviderType { return ProviderJira }

func (j *JiraProvider) client() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func (j *JiraProvider) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, j.url+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(j.email, j.token)

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

// jiraIssue represents the Jira API issue response.
type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Issuetype struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Reporter *struct {
			DisplayName string `json:"displayName"`
		} `json:"reporter"`
		Labels    []string  `json:"labels"`
		Project   struct {
			Key string `json:"key"`
			Name string `json:"name"`
		} `json:"project"`
		Created   string `json:"created"`
		Updated   string `json:"updated"`
		Duedate   string `json:"duedate"`
	} `json:"fields"`
}

func mapJiraStatus(name string) IssueStatus {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "open"), strings.Contains(lower, "to do"), strings.Contains(lower, "todo"):
		return StatusOpen
	case strings.Contains(lower, "progress"), strings.Contains(lower, "in progress"), strings.Contains(lower, "doing"):
		return StatusInProgress
	case strings.Contains(lower, "done"), strings.Contains(lower, "closed"), strings.Contains(lower, "resolved"):
		return StatusDone
	case strings.Contains(lower, "backlog"):
		return StatusBacklog
	case strings.Contains(lower, "cancel"):
		return StatusCancelled
	default:
		return StatusOpen
	}
}

func mapJiraPriority(name string) Priority {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "highest"), strings.Contains(lower, "critical"), strings.Contains(lower, "blocker"):
		return PriorityCritical
	case strings.Contains(lower, "high"):
		return PriorityHigh
	case strings.Contains(lower, "medium"):
		return PriorityMedium
	case strings.Contains(lower, "low"), strings.Contains(lower, "lowest"):
		return PriorityLow
	default:
		return PriorityNone
	}
}

func mapJiraType(name string) IssueType {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "bug"):
		return TypeBug
	case strings.Contains(lower, "story"):
		return TypeStory
	case strings.Contains(lower, "epic"):
		return TypeEpic
	case strings.Contains(lower, "sub-task"), strings.Contains(lower, "subtask"):
		return TypeSubTask
	case strings.Contains(lower, "incident"):
		return TypeIncident
	case strings.Contains(lower, "improvement"):
		return TypeImprovement
	default:
		return TypeTask
	}
}

func parseJiraTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Jira uses ISO 8601: 2024-01-15T10:30:00.000+0000
	layouts := []string{
		"2006-01-02T15:04:05.999-0700",
		"2006-01-02T15:04:05.999Z",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (j *JiraProvider) toIssue(ji *jiraIssue) *Issue {
	issue := &Issue{
		ID:          ji.ID,
		Key:         ji.Key,
		Title:       ji.Fields.Summary,
		Description: ji.Fields.Description,
		Status:      mapJiraStatus(ji.Fields.Status.Name),
		Priority:    mapJiraPriority(ji.Fields.Priority.Name),
		Type:        mapJiraType(ji.Fields.Issuetype.Name),
		Labels:      ji.Fields.Labels,
		Project:     ji.Fields.Project.Name,
		ProjectKey:  ji.Fields.Project.Key,
		Provider:    ProviderJira,
		ExternalID:  ji.Key,
		ExternalURL: fmt.Sprintf("%s/browse/%s", j.url, ji.Key),
		CreatedAt:   parseJiraTime(ji.Fields.Created),
		UpdatedAt:   parseJiraTime(ji.Fields.Updated),
	}
	if ji.Fields.Assignee != nil {
		issue.Assignee = ji.Fields.Assignee.DisplayName
	}
	if ji.Fields.Reporter != nil {
		issue.Reporter = ji.Fields.Reporter.DisplayName
	}
	if ji.Fields.Duedate != "" {
		t, err := time.Parse("2006-01-02", ji.Fields.Duedate)
		if err == nil {
			issue.DueDate = &t
		}
	}
	return issue
}

func (j *JiraProvider) ListProjects() ([]Project, error) {
	data, status, err := j.doRequest("GET", "/rest/api/2/project", nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("jira list projects: HTTP %d: %s", status, string(data))
	}

	var raw []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse projects: %w", err)
	}

	projects := make([]Project, len(raw))
	for i, r := range raw {
		projects[i] = Project{
			ID:         r.Key,
			Key:        r.Key,
			Name:       r.Name,
			Provider:   ProviderJira,
			ExternalID: r.ID,
		}
	}
	return projects, nil
}

func (j *JiraProvider) ListIssues(filter SearchFilter) ([]Issue, error) {
	jql := j.buildJQL(filter)
	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": 50,
		"fields":     []string{"summary", "description", "status", "priority", "issuetype", "assignee", "reporter", "labels", "project", "created", "updated", "duedate"},
	}
	if filter.Limit > 0 && filter.Limit < 50 {
		payload["maxResults"] = filter.Limit
	}

	data, status, err := j.doRequest("POST", "/rest/api/2/search", payload)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("jira search: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Issues []jiraIssue `json:"issues"`
		Total  int         `json:"total"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse issues: %w", err)
	}

	issues := make([]Issue, len(result.Issues))
	for i, ji := range result.Issues {
		issues[i] = *j.toIssue(&ji)
	}
	return issues, nil
}

func (j *JiraProvider) buildJQL(filter SearchFilter) string {
	var parts []string

	if filter.Query != "" {
		parts = append(parts, fmt.Sprintf("summary ~ %q OR description ~ %q", filter.Query, filter.Query))
	}
	if filter.Project != "" {
		parts = append(parts, fmt.Sprintf("project = %q", filter.Project))
	}
	if filter.Assignee != "" {
		parts = append(parts, fmt.Sprintf("assignee = %q", filter.Assignee))
	}
	if len(filter.Status) > 0 {
		statuses := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			statuses[i] = string(s)
		}
		parts = append(parts, fmt.Sprintf("status in (%s)", strings.Join(statuses, ", ")))
	}
	if len(filter.Labels) > 0 {
		parts = append(parts, fmt.Sprintf("labels in (%s)", strings.Join(filter.Labels, ", ")))
	}
	if filter.Since != nil {
		parts = append(parts, fmt.Sprintf("updated >= %q", filter.Since.Format("2006-01-02")))
	}

	if len(parts) == 0 {
		return "ORDER BY updated DESC"
	}
	return strings.Join(parts, " AND ") + " ORDER BY updated DESC"
}

func (j *JiraProvider) GetIssue(id string) (*Issue, error) {
	data, status, err := j.doRequest("GET", fmt.Sprintf("/rest/api/2/issue/%s", id), nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("jira get issue: HTTP %d: %s", status, string(data))
	}

	var ji jiraIssue
	if err := json.Unmarshal(data, &ji); err != nil {
		return nil, fmt.Errorf("parse issue: %w", err)
	}
	return j.toIssue(&ji), nil
}

func (j *JiraProvider) CreateIssue(req CreateIssueRequest) (*Issue, error) {
	fields := map[string]interface{}{
		"summary": req.Title,
		"project": map[string]string{"key": req.ProjectKey},
		"issuetype": map[string]string{"name": string(req.Type)},
	}
	if req.Description != "" {
		fields["description"] = req.Description
	}
	if req.Priority != "" {
		fields["priority"] = map[string]string{"name": string(req.Priority)}
	}
	if req.Assignee != "" {
		fields["assignee"] = map[string]string{"name": req.Assignee}
	}
	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}
	if req.DueDate != nil {
		fields["duedate"] = req.DueDate.Format("2006-01-02")
	}

	payload := map[string]interface{}{"fields": fields}
	data, status, err := j.doRequest("POST", "/rest/api/2/issue", payload)
	if err != nil {
		return nil, err
	}
	if status != 201 {
		return nil, fmt.Errorf("jira create issue: HTTP %d: %s", status, string(data))
	}

	var result struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse created issue: %w", err)
	}

	return j.GetIssue(result.Key)
}

func (j *JiraProvider) UpdateIssue(id string, req UpdateIssueRequest) (*Issue, error) {
	fields := map[string]interface{}{}
	if req.Title != nil {
		fields["summary"] = *req.Title
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.Priority != nil {
		fields["priority"] = map[string]string{"name": string(*req.Priority)}
	}
	if req.Assignee != nil {
		fields["assignee"] = map[string]string{"name": *req.Assignee}
	}
	if req.Labels != nil {
		fields["labels"] = *req.Labels
	}
	if req.DueDate != nil {
		fields["duedate"] = req.DueDate.Format("2006-01-02")
	}

	payload := map[string]interface{}{"fields": fields}
	data, status, err := j.doRequest("PUT", fmt.Sprintf("/rest/api/2/issue/%s", id), payload)
	if err != nil {
		return nil, err
	}
	if status != 204 && status != 200 {
		return nil, fmt.Errorf("jira update issue: HTTP %d: %s", status, string(data))
	}

	// Handle status transition separately
	if req.Status != nil {
		transitionID, err := j.findTransitionID(id, string(*req.Status))
		if err == nil {
			transPayload := map[string]interface{}{
				"transition": map[string]string{"id": transitionID},
			}
			_, _, _ = j.doRequest("POST", fmt.Sprintf("/rest/api/2/issue/%s/transitions", id), transPayload)
		}
	}

	return j.GetIssue(id)
}

func (j *JiraProvider) findTransitionID(issueID, targetStatus string) (string, error) {
	data, status, err := j.doRequest("GET", fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueID), nil)
	if err != nil {
		return "", err
	}
	if status != 200 {
		return "", fmt.Errorf("HTTP %d", status)
	}

	var result struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	target := strings.ToLower(targetStatus)
	for _, t := range result.Transitions {
		if strings.ToLower(t.To.Name) == target || strings.ToLower(t.Name) == target {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("no transition to %q", targetStatus)
}

func (j *JiraProvider) AddComment(id string, body string) (*Comment, error) {
	payload := map[string]interface{}{
		"body": body,
	}
	data, status, err := j.doRequest("POST", fmt.Sprintf("/rest/api/2/issue/%s/comment", id), payload)
	if err != nil {
		return nil, err
	}
	if status != 201 {
		return nil, fmt.Errorf("jira add comment: HTTP %d: %s", status, string(data))
	}

	var result struct {
		ID        string `json:"id"`
		Body      string `json:"body"`
		CreatedAt string `json:"created"`
		Author    struct {
			DisplayName string `json:"displayName"`
		} `json:"author"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse comment: %w", err)
	}

	return &Comment{
		ID:        result.ID,
		IssueID:   id,
		Author:    result.Author.DisplayName,
		Body:      result.Body,
		CreatedAt: parseJiraTime(result.CreatedAt),
	}, nil
}

func (j *JiraProvider) ListComments(id string) ([]Comment, error) {
	data, status, err := j.doRequest("GET", fmt.Sprintf("/rest/api/2/issue/%s/comment", id), nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("jira list comments: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Comments []struct {
			ID        string `json:"id"`
			Body      string `json:"body"`
			CreatedAt string `json:"created"`
			Author    struct {
				DisplayName string `json:"displayName"`
			} `json:"author"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse comments: %w", err)
	}

	comments := make([]Comment, len(result.Comments))
	for i, c := range result.Comments {
		comments[i] = Comment{
			ID:        c.ID,
			IssueID:   id,
			Author:    c.Author.DisplayName,
			Body:      c.Body,
			CreatedAt: parseJiraTime(c.CreatedAt),
		}
	}
	return comments, nil
}

func (j *JiraProvider) Search(filter SearchFilter) ([]Issue, error) {
	return j.ListIssues(filter)
}
