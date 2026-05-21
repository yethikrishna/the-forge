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

// LinearClient implements ProviderClient for Linear's GraphQL API.
type LinearClient struct {
	httpClient *http.Client
}

const linearAPIURL = "https://api.linear.app/graphql"

func (l *LinearClient) client() *http.Client {
	if l.httpClient == nil {
		l.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return l.httpClient
}

func (l *LinearClient) linearRequest(ctx context.Context, conn *Connection, query string, variables map[string]interface{}) ([]byte, error) {
	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal graphql: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearAPIURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", conn.Config.APIToken)

	resp, err := l.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("linear request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check for GraphQL errors
	var gqlErr struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &gqlErr); err == nil && len(gqlErr.Errors) > 0 {
		msgs := make([]string, len(gqlErr.Errors))
		for i, e := range gqlErr.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("linear graphql error: %s", strings.Join(msgs, "; "))
	}

	return body, nil
}

func (l *LinearClient) TestConnectivity(ctx context.Context) error {
	return nil // Validated by first real request
}

// FetchTasks retrieves issues from Linear using GraphQL.
func (l *LinearClient) FetchTasks(ctx context.Context, conn *Connection, filters TaskFilters) ([]*Task, error) {
	filterClauses := []string{}

	project := filters.Project
	if project == "" {
		project = conn.Config.Project
	}
	if project != "" {
		filterClauses = append(filterClauses, fmt.Sprintf(`project: {key: {eq: "%s"}}`, project))
	}
	if filters.Status != "" {
		filterClauses = append(filterClauses, fmt.Sprintf(`state: {name: {eq: "%s"}}`, filters.Status))
	}
	if filters.Assignee != "" {
		filterClauses = append(filterClauses, fmt.Sprintf(`assignee: {email: {eq: "%s"}}`, filters.Assignee))
	}
	for _, label := range filters.Labels {
		filterClauses = append(filterClauses, fmt.Sprintf(`labels: {some: {name: {eq: "%s"}}}`, label))
	}

	limit := 50
	if filters.Limit > 0 {
		limit = filters.Limit
	}

	filterStr := ""
	if len(filterClauses) > 0 {
		filterStr = "filter: {" + strings.Join(filterClauses, ", ") + "}"
	}

	query := fmt.Sprintf(`query {
  issues(%s, first: %d, orderBy: updatedAt) {
    nodes {
      id identifier title description
      state { name }
      priority
      assignee { email displayName }
      labels { nodes { name } }
      project { key name }
      parent { identifier }
      team { key }
      url createdAt updatedAt dueDate
    }
  }
}`, filterStr, limit)

	body, err := l.linearRequest(ctx, conn, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Issues struct {
				Nodes []linearIssue `json:"nodes"`
			} `json:"issues"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse linear response: %w", err)
	}

	tasks := make([]*Task, 0, len(result.Data.Issues.Nodes))
	for _, issue := range result.Data.Issues.Nodes {
		tasks = append(tasks, linearIssueToTask(issue))
	}
	return tasks, nil
}

// GetTask retrieves a single issue by identifier.
func (l *LinearClient) GetTask(ctx context.Context, conn *Connection, key string) (*Task, error) {
	query := `query($id: String!) {
  issue(id: $id) {
    id identifier title description
    state { name }
    priority
    assignee { email displayName }
    labels { nodes { name } }
    project { key name }
    parent { identifier }
    team { key }
    url createdAt updatedAt dueDate
  }
}`

	body, err := l.linearRequest(ctx, conn, query, map[string]interface{}{"id": key})
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Issue *linearIssue `json:"issue"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse linear issue: %w", err)
	}
	if result.Data.Issue == nil {
		return nil, fmt.Errorf("linear issue %q not found", key)
	}
	return linearIssueToTask(*result.Data.Issue), nil
}

// CreateTask creates a Linear issue.
func (l *LinearClient) CreateTask(ctx context.Context, conn *Connection, task *Task) (string, error) {
	input := map[string]interface{}{
		"title":       task.Title,
		"description": task.Description,
	}

	project := conn.Config.Project
	if task.Project != "" {
		project = task.Project
	}
	if project != "" {
		input["projectId"] = project
	}
	if task.Priority != "" {
		input["priority"] = linearPriority(task.Priority)
	}
	if task.Assignee != "" {
		input["assigneeId"] = task.Assignee
	}
	if len(task.Labels) > 0 {
		input["labelIds"] = task.Labels
	}
	if task.DueDate != nil {
		input["dueDate"] = task.DueDate.Format("2006-01-02")
	}

	query := `mutation($input: IssueCreateInput!) {
  issueCreate(input: $input) {
    success
    issue { id identifier }
  }
}`

	body, err := l.linearRequest(ctx, conn, query, map[string]interface{}{"input": input})
	if err != nil {
		return "", err
	}

	var result struct {
		Data struct {
			IssueCreate struct {
				Success bool `json:"success"`
				Issue   *struct {
					ID         string `json:"id"`
					Identifier string `json:"identifier"`
				} `json:"issue"`
			} `json:"issueCreate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse linear create response: %w", err)
	}
	if !result.Data.IssueCreate.Success || result.Data.IssueCreate.Issue == nil {
		return "", fmt.Errorf("linear create failed")
	}
	return result.Data.IssueCreate.Issue.Identifier, nil
}

// UpdateTask updates a Linear issue.
func (l *LinearClient) UpdateTask(ctx context.Context, conn *Connection, task *Task) error {
	input := map[string]interface{}{}
	if task.Title != "" {
		input["title"] = task.Title
	}
	if task.Description != "" {
		input["description"] = task.Description
	}
	if task.Priority != "" {
		input["priority"] = linearPriority(task.Priority)
	}
	if task.Assignee != "" {
		input["assigneeId"] = task.Assignee
	}
	if len(task.Labels) > 0 {
		input["labelIds"] = task.Labels
	}

	// Map status to state ID — we pass the status name directly
	if task.Status != "" {
		input["stateId"] = task.Status // Linear expects a state UUID; caller resolves via status name
	}

	if len(input) == 0 {
		return nil
	}

	query := `mutation($id: String!, $input: IssueUpdateInput!) {
  issueUpdate(id: $id, input: $input) {
    success
  }
}`

	body, err := l.linearRequest(ctx, conn, query, map[string]interface{}{"id": task.ProviderKey, "input": input})
	if err != nil {
		return err
	}

	var result struct {
		Data struct {
			IssueUpdate struct {
				Success bool `json:"success"`
			} `json:"issueUpdate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse linear update response: %w", err)
	}
	if !result.Data.IssueUpdate.Success {
		return fmt.Errorf("linear update failed")
	}
	return nil
}

// AddComment creates a comment on a Linear issue.
func (l *LinearClient) AddComment(ctx context.Context, conn *Connection, issueID string, author string, body string) error {
	query := `mutation($input: CommentCreateInput!) {
  commentCreate(input: $input) {
    success
  }
}`

	input := map[string]interface{}{
		"issueId": issueID,
		"body":    body,
	}

	respBody, err := l.linearRequest(ctx, conn, query, map[string]interface{}{"input": input})
	if err != nil {
		return err
	}

	var result struct {
		Data struct {
			CommentCreate struct {
				Success bool `json:"success"`
			} `json:"commentCreate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse linear comment response: %w", err)
	}
	if !result.Data.CommentCreate.Success {
		return fmt.Errorf("linear comment failed")
	}
	return nil
}

// --- Linear API types ---

type linearIssue struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       struct {
		Name string `json:"name"`
	} `json:"state"`
	Priority int `json:"priority"`
	Assignee *struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
	} `json:"assignee"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Project *struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"project"`
	Parent *struct {
		Identifier string `json:"identifier"`
	} `json:"parent"`
	Team struct {
		Key string `json:"key"`
	} `json:"team"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	DueDate   string `json:"dueDate"`
}

func linearIssueToTask(issue linearIssue) *Task {
	t := &Task{
		ID:          issue.ID,
		ProviderKey: issue.Identifier,
		Title:       issue.Title,
		Description: issue.Description,
		Status:      issue.State.Name,
		Priority:    linearPriorityFromInt(issue.Priority),
		URL:         issue.URL,
	}

	if issue.Assignee != nil {
		t.Assignee = issue.Assignee.Email
	}

	labels := make([]string, 0, len(issue.Labels.Nodes))
	for _, l := range issue.Labels.Nodes {
		labels = append(labels, l.Name)
	}
	t.Labels = labels

	if issue.Project != nil {
		t.Project = issue.Project.Key
	}
	if issue.Parent != nil {
		t.ParentID = issue.Parent.Identifier
	}

	t.CreatedAt, _ = time.Parse(time.RFC3339, issue.CreatedAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, issue.UpdatedAt)

	if issue.DueDate != "" {
		if d, err := time.Parse("2006-01-02", issue.DueDate); err == nil {
			t.DueDate = &d
		}
	}

	return t
}

// Linear priority: 0=No priority, 1=Urgent, 2=High, 3=Medium, 4=Low
func linearPriority(p string) int {
	switch strings.ToLower(p) {
	case "critical", "urgent":
		return 1
	case "high":
		return 2
	case "medium":
		return 3
	case "low":
		return 4
	default:
		return 0
	}
}

func linearPriorityFromInt(p int) string {
	switch p {
	case 1:
		return "critical"
	case 2:
		return "high"
	case 3:
		return "medium"
	case 4:
		return "low"
	default:
		return "medium"
	}
}
