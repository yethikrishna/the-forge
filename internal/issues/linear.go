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

// LinearProvider implements the Provider interface for Linear.
type LinearProvider struct {
	url    string
	token  string
	teamID string
	client *http.Client
}

// NewLinearProvider creates a new Linear provider from config.
func NewLinearProvider(cfg *ProviderConfig) (*LinearProvider, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("linear: API token required")
	}
	url := cfg.URL
	if url == "" {
		url = "https://api.linear.app"
	}
	return &LinearProvider{
		url:    strings.TrimRight(url, "/"),
		token:  cfg.Token,
		teamID: cfg.TeamID,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (l *LinearProvider) Type() ProviderType { return ProviderLinear }

func (l *LinearProvider) doGraphQL(query string, variables map[string]interface{}) ([]byte, int, error) {
	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal graphql: %w", err)
	}

	req, err := http.NewRequest("POST", l.url+"/graphql", bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", l.token)

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("linear request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return body, resp.StatusCode, nil
}

type linearIssue struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Identifier  string `json:"identifier"`
	URL         string `json:"url"`
	State       struct {
		Name string `json:"name"`
	} `json:"state"`
	Priority float64 `json:"priority"`
	Assignee *struct {
		Name string `json:"name"`
	} `json:"assignee"`
	Creator *struct {
		Name string `json:"name"`
	} `json:"creator"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Team struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"team"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	DueDate   *string `json:"dueDate"`
}

func mapLinearState(name string) IssueStatus {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "backlog"):
		return StatusBacklog
	case strings.Contains(lower, "todo"), strings.Contains(lower, "unstarted"):
		return StatusTodo
	case strings.Contains(lower, "progress"), strings.Contains(lower, "started"), strings.Contains(lower, "doing"):
		return StatusInProgress
	case strings.Contains(lower, "done"), strings.Contains(lower, "completed"):
		return StatusDone
	case strings.Contains(lower, "cancel"):
		return StatusCancelled
	default:
		return StatusOpen
	}
}

func mapLinearPriority(p float64) Priority {
	// Linear: 0=no, 1=urgent, 2=high, 3=medium, 4=low
	switch int(p) {
	case 0:
		return PriorityNone
	case 1:
		return PriorityUrgent
	case 2:
		return PriorityHigh
	case 3:
		return PriorityMedium
	case 4:
		return PriorityLow
	default:
		return PriorityNone
	}
}

func parseLinearTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (l *LinearProvider) toIssue(li *linearIssue) *Issue {
	issue := &Issue{
		ID:          li.Identifier,
		Key:         li.Identifier,
		Title:       li.Title,
		Description: li.Description,
		Status:      mapLinearState(li.State.Name),
		Priority:    mapLinearPriority(li.Priority),
		Provider:    ProviderLinear,
		ExternalID:  li.ID,
		ExternalURL: li.URL,
		Project:     li.Team.Name,
		ProjectKey:  li.Team.Key,
		CreatedAt:   parseLinearTime(li.CreatedAt),
		UpdatedAt:   parseLinearTime(li.UpdatedAt),
	}
	if li.Assignee != nil {
		issue.Assignee = li.Assignee.Name
	}
	if li.Creator != nil {
		issue.Reporter = li.Creator.Name
	}
	if li.Labels.Nodes != nil {
		issue.Labels = make([]string, len(li.Labels.Nodes))
		for i, lb := range li.Labels.Nodes {
			issue.Labels[i] = lb.Name
		}
	}
	if li.DueDate != nil && *li.DueDate != "" {
		t, err := time.Parse("2006-01-02", *li.DueDate)
		if err == nil {
			issue.DueDate = &t
		}
	}
	return issue
}

const linearIssueFields = `
id, title, description, identifier, url,
state { name },
priority,
assignee { name },
creator { name },
labels { nodes { name } },
team { key, name },
createdAt, updatedAt, dueDate
`

func (l *LinearProvider) ListProjects() ([]Project, error) {
	query := `query { teams { nodes { id, key, name } } }`
	data, status, err := l.doGraphQL(query, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linear list teams: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Data struct {
			Teams struct {
				Nodes []struct {
					ID   string `json:"id"`
					Key  string `json:"key"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"teams"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse teams: %w", err)
	}

	projects := make([]Project, len(result.Data.Teams.Nodes))
	for i, t := range result.Data.Teams.Nodes {
		projects[i] = Project{
			ID:         t.Key,
			Key:        t.Key,
			Name:       t.Name,
			Provider:   ProviderLinear,
			ExternalID: t.ID,
		}
	}
	return projects, nil
}

func (l *LinearProvider) ListIssues(filter SearchFilter) ([]Issue, error) {
	var filterParts []string
	if l.teamID != "" {
		filterParts = append(filterParts, fmt.Sprintf(`team: {id: {eq: "%s"}}`, l.teamID))
	}
	if filter.Query != "" {
		filterParts = append(filterParts, fmt.Sprintf(`or: [{title: {contains: "%s"}}, {description: {contains: "%s"}}]`, filter.Query, filter.Query))
	}
	if filter.Assignee != "" {
		filterParts = append(filterParts, fmt.Sprintf(`assignee: {name: {eq: "%s"}}`, filter.Assignee))
	}
	if len(filter.Status) > 0 {
		states := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			states[i] = string(s)
		}
		filterParts = append(filterParts, fmt.Sprintf(`state: {name: {in: [%s]}}`, fmt.Sprintf(`"%s"`, strings.Join(states, `", "`))))
	}

	limit := 50
	if filter.Limit > 0 && filter.Limit < 50 {
		limit = filter.Limit
	}

	filterStr := ""
	if len(filterParts) > 0 {
		filterStr = fmt.Sprintf(`(filter: {%s})`, strings.Join(filterParts, ", "))
	}

	query := fmt.Sprintf(`query { issues %s { nodes { %s } } }`,
		fmt.Sprintf(`(first: %d)%s`, limit, func() string {
			if filterStr != "" {
				return " " + filterStr
			}
			return ""
		}()),
		linearIssueFields,
	)

	// Clean query construction
	if filterStr != "" {
		query = fmt.Sprintf(`query { issues(first: %d, filter: {%s}) { nodes { %s } } }`,
			limit, strings.Join(filterParts, ", "), linearIssueFields)
	} else {
		query = fmt.Sprintf(`query { issues(first: %d) { nodes { %s } } }`, limit, linearIssueFields)
	}

	data, status, err := l.doGraphQL(query, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linear list issues: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Data struct {
			Issues struct {
				Nodes []linearIssue `json:"nodes"`
			} `json:"issues"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse issues: %w", err)
	}

	issues := make([]Issue, len(result.Data.Issues.Nodes))
	for i, li := range result.Data.Issues.Nodes {
		issues[i] = *l.toIssue(&li)
	}
	return issues, nil
}

func (l *LinearProvider) GetIssue(id string) (*Issue, error) {
	query := fmt.Sprintf(`query { issue(id: "%s") { %s } }`, id, linearIssueFields)
	data, status, err := l.doGraphQL(query, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linear get issue: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Data struct {
			Issue linearIssue `json:"issue"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse issue: %w", err)
	}
	return l.toIssue(&result.Data.Issue), nil
}

func (l *LinearProvider) CreateIssue(req CreateIssueRequest) (*Issue, error) {
	input := map[string]interface{}{
		"title": req.Title,
	}
	if l.teamID != "" {
		input["teamId"] = l.teamID
	}
	if req.Description != "" {
		input["description"] = req.Description
	}
	if req.Priority != "" {
		input["priority"] = mapLinearPriorityToNum(req.Priority)
	}
	if req.Assignee != "" {
		input["assigneeId"] = req.Assignee
	}
	if len(req.Labels) > 0 {
		input["labelIds"] = req.Labels
	}
	if req.DueDate != nil {
		input["dueDate"] = req.DueDate.Format("2006-01-02")
	}

	query := `mutation CreateIssue($input: IssueCreateInput!) { issueCreate(input: $input) { issue { id } success } }`
	data, status, err := l.doGraphQL(query, map[string]interface{}{"input": input})
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linear create issue: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Data struct {
			IssueCreate struct {
				Issue struct {
					ID string `json:"id"`
				} `json:"issue"`
				Success bool `json:"success"`
			} `json:"issueCreate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse created issue: %w", err)
	}
	if !result.Data.IssueCreate.Success {
		return nil, fmt.Errorf("linear create issue: not successful")
	}

	return l.GetIssue(result.Data.IssueCreate.Issue.ID)
}

func mapLinearPriorityToNum(p Priority) float64 {
	switch p {
	case PriorityUrgent:
		return 1
	case PriorityHigh:
		return 2
	case PriorityMedium:
		return 3
	case PriorityLow:
		return 4
	default:
		return 0
	}
}

func (l *LinearProvider) UpdateIssue(id string, req UpdateIssueRequest) (*Issue, error) {
	input := map[string]interface{}{}
	if req.Title != nil {
		input["title"] = *req.Title
	}
	if req.Description != nil {
		input["description"] = *req.Description
	}
	if req.Priority != nil {
		input["priority"] = mapLinearPriorityToNum(*req.Priority)
	}
	if req.Assignee != nil {
		input["assigneeId"] = *req.Assignee
	}
	if req.DueDate != nil {
		input["dueDate"] = req.DueDate.Format("2006-01-02")
	}

	query := `mutation UpdateIssue($id: String!, $input: IssueUpdateInput!) { issueUpdate(id: $id, input: $input) { issue { id } success } }`
	data, status, err := l.doGraphQL(query, map[string]interface{}{"id": id, "input": input})
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linear update issue: HTTP %d: %s", status, string(data))
	}

	// Handle status change separately
	if req.Status != nil {
		stateInput := map[string]interface{}{
			"stateName": string(*req.Status),
		}
		stateQuery := `mutation UpdateIssueState($id: String!, $input: IssueUpdateInput!) { issueUpdate(id: $id, input: $input) { success } }`
		_, _, _ = l.doGraphQL(stateQuery, map[string]interface{}{"id": id, "input": stateInput})
	}

	return l.GetIssue(id)
}

func (l *LinearProvider) AddComment(id string, body string) (*Comment, error) {
	input := map[string]interface{}{
		"issueId": id,
		"body":    body,
	}
	query := `mutation CreateComment($input: CommentCreateInput!) { commentCreate(input: $input) { comment { id body createdAt user { name } } success } }`
	data, status, err := l.doGraphQL(query, map[string]interface{}{"input": input})
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linear add comment: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Data struct {
			CommentCreate struct {
				Comment struct {
					ID        string `json:"id"`
					Body      string `json:"body"`
					CreatedAt string `json:"createdAt"`
					User      struct {
						Name string `json:"name"`
					} `json:"user"`
				} `json:"comment"`
				Success bool `json:"success"`
			} `json:"commentCreate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse comment: %w", err)
	}

	return &Comment{
		ID:        result.Data.CommentCreate.Comment.ID,
		IssueID:   id,
		Author:    result.Data.CommentCreate.Comment.User.Name,
		Body:      result.Data.CommentCreate.Comment.Body,
		CreatedAt: parseLinearTime(result.Data.CommentCreate.Comment.CreatedAt),
	}, nil
}

func (l *LinearProvider) ListComments(id string) ([]Comment, error) {
	query := fmt.Sprintf(`query { issueComments(issueId: "%s", first: 50) { nodes { id body createdAt user { name } } } }`, id)
	data, status, err := l.doGraphQL(query, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linear list comments: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Data struct {
			IssueComments struct {
				Nodes []struct {
					ID        string `json:"id"`
					Body      string `json:"body"`
					CreatedAt string `json:"createdAt"`
					User      struct {
						Name string `json:"name"`
					} `json:"user"`
				} `json:"nodes"`
			} `json:"issueComments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse comments: %w", err)
	}

	comments := make([]Comment, len(result.Data.IssueComments.Nodes))
	for i, c := range result.Data.IssueComments.Nodes {
		comments[i] = Comment{
			ID:        c.ID,
			IssueID:   id,
			Author:    c.User.Name,
			Body:      c.Body,
			CreatedAt: parseLinearTime(c.CreatedAt),
		}
	}
	return comments, nil
}

func (l *LinearProvider) Search(filter SearchFilter) ([]Issue, error) {
	return l.ListIssues(filter)
}
