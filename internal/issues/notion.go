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

// NotionProvider implements the Provider interface for Notion.
// Uses Notion databases API to model issues as database entries.
type NotionProvider struct {
	url        string
	token      string
	databaseID string
	client     *http.Client
}

// NewNotionProvider creates a new Notion provider from config.
func NewNotionProvider(cfg *ProviderConfig) (*NotionProvider, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("notion: API token required")
	}
	url := cfg.URL
	if url == "" {
		url = "https://api.notion.com"
	}
	return &NotionProvider{
		url:        strings.TrimRight(url, "/"),
		token:      cfg.Token,
		databaseID: cfg.DatabaseID,
		client:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (n *NotionProvider) Type() ProviderType { return ProviderNotion }

func (n *NotionProvider) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, n.url+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("notion request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

type notionPage struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	CreatedTime string `json:"created_time"`
	Properties  map[string]struct {
		Type  string `json:"type"`
		Title []struct {
			PlainText string `json:"plain_text"`
		} `json:"title,omitempty"`
		RichText []struct {
			PlainText string `json:"plain_text"`
		} `json:"rich_text,omitempty"`
		Select *struct {
			Name string `json:"name"`
		} `json:"select,omitempty"`
		MultiSelect []struct {
			Name string `json:"name"`
		} `json:"multi_select,omitempty"`
		People []struct {
			Name string `json:"name"`
		} `json:"people,omitempty"`
		Date *struct {
			Start string `json:"start"`
		} `json:"date,omitempty"`
		Checkbox *bool `json:"checkbox,omitempty"`
		Number   *float64 `json:"number,omitempty"`
	} `json:"properties"`
}

func mapNotionStatus(name string) IssueStatus {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "backlog"):
		return StatusBacklog
	case strings.Contains(lower, "todo"), strings.Contains(lower, "to do"), strings.Contains(lower, "not started"):
		return StatusTodo
	case strings.Contains(lower, "progress"), strings.Contains(lower, "doing"), strings.Contains(lower, "in progress"):
		return StatusInProgress
	case strings.Contains(lower, "done"), strings.Contains(lower, "complete"):
		return StatusDone
	case strings.Contains(lower, "cancel"):
		return StatusCancelled
	default:
		return StatusOpen
	}
}

func mapNotionPriority(name string) Priority {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "urgent"), strings.Contains(lower, "critical"):
		return PriorityCritical
	case strings.Contains(lower, "high"):
		return PriorityHigh
	case strings.Contains(lower, "medium"):
		return PriorityMedium
	case strings.Contains(lower, "low"):
		return PriorityLow
	default:
		return PriorityNone
	}
}

func mapNotionType(name string) IssueType {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "bug"):
		return TypeBug
	case strings.Contains(lower, "story"):
		return TypeStory
	case strings.Contains(lower, "epic"):
		return TypeEpic
	case strings.Contains(lower, "incident"):
		return TypeIncident
	case strings.Contains(lower, "improvement"), strings.Contains(lower, "enhancement"):
		return TypeImprovement
	default:
		return TypeTask
	}
}

func parseNotionTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05-07:00",
		time.RFC3339,
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (n *NotionProvider) getPropText(page *notionPage, key string) string {
	prop, ok := page.Properties[key]
	if !ok {
		return ""
	}
	switch prop.Type {
	case "title":
		if len(prop.Title) > 0 {
			return prop.Title[0].PlainText
		}
	case "rich_text":
		if len(prop.RichText) > 0 {
			return prop.RichText[0].PlainText
		}
	}
	return ""
}

func (n *NotionProvider) getPropSelect(page *notionPage, key string) string {
	prop, ok := page.Properties[key]
	if !ok || prop.Select == nil {
		return ""
	}
	return prop.Select.Name
}

func (n *NotionProvider) getPropMultiSelect(page *notionPage, key string) []string {
	prop, ok := page.Properties[key]
	if !ok {
		return nil
	}
	result := make([]string, len(prop.MultiSelect))
	for i, s := range prop.MultiSelect {
		result[i] = s.Name
	}
	return result
}

func (n *NotionProvider) getPropPeople(page *notionPage, key string) string {
	prop, ok := page.Properties[key]
	if !ok || len(prop.People) == 0 {
		return ""
	}
	return prop.People[0].Name
}

func (n *NotionProvider) getPropDate(page *notionPage, key string) *time.Time {
	prop, ok := page.Properties[key]
	if !ok || prop.Date == nil {
		return nil
	}
	t := parseNotionTime(prop.Date.Start)
	if t.IsZero() {
		return nil
	}
	return &t
}

func (n *NotionProvider) toIssue(page *notionPage) *Issue {
	now := time.Now()
	createdAt := parseNotionTime(page.CreatedTime)
	if createdAt.IsZero() {
		createdAt = now
	}

	title := n.getPropText(page, "Name")
	if title == "" {
		title = n.getPropText(page, "Title")
	}
	description := n.getPropText(page, "Description")
	if description == "" {
		description = n.getPropText(page, "Details")
	}

	status := mapNotionStatus(n.getPropSelect(page, "Status"))
	priority := mapNotionPriority(n.getPropSelect(page, "Priority"))
	issueType := mapNotionType(n.getPropSelect(page, "Type"))
	assignee := n.getPropPeople(page, "Assignee")
	reporter := n.getPropPeople(page, "Reporter")
	labels := n.getPropMultiSelect(page, "Labels")
	project := n.getPropSelect(page, "Project")
	dueDate := n.getPropDate(page, "Due Date")

	return &Issue{
		ID:          page.ID,
		Key:         page.ID[:8], // Short key from Notion page ID
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		Type:        issueType,
		Assignee:    assignee,
		Reporter:    reporter,
		Labels:      labels,
		Project:     project,
		Provider:    ProviderNotion,
		ExternalID:  page.ID,
		ExternalURL: page.URL,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		DueDate:     dueDate,
	}
}

func (n *NotionProvider) ListProjects() ([]Project, error) {
	if n.databaseID == "" {
		return nil, fmt.Errorf("notion: database ID required")
	}
	data, status, err := n.doRequest("GET", fmt.Sprintf("/v1/databases/%s", n.databaseID), nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("notion get database: HTTP %d: %s", status, string(data))
	}

	var result struct {
		ID    string `json:"id"`
		Title []struct {
			PlainText string `json:"plain_text"`
		} `json:"title"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse database: %w", err)
	}

	title := "Untitled"
	if len(result.Title) > 0 {
		title = result.Title[0].PlainText
	}

	return []Project{{
		ID:         n.databaseID,
		Name:       title,
		Provider:   ProviderNotion,
		ExternalID: result.ID,
	}}, nil
}

func (n *NotionProvider) ListIssues(filter SearchFilter) ([]Issue, error) {
	if n.databaseID == "" {
		return nil, fmt.Errorf("notion: database ID required")
	}

	payload := map[string]interface{}{
		"page_size": 50,
	}

	// Build Notion filter
	var filterParts []map[string]interface{}
	if filter.Query != "" {
		filterParts = append(filterParts, map[string]interface{}{
			"property": "Name",
			"title": map[string]interface{}{
				"contains": filter.Query,
			},
		})
	}
	if len(filter.Status) > 0 {
		options := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			options[i] = string(s)
		}
		filterParts = append(filterParts, map[string]interface{}{
			"property": "Status",
			"select": map[string]interface{}{
				"equals": options[0],
			},
		})
	}
	if len(filterParts) > 0 {
		payload["filter"] = map[string]interface{}{
			"and": filterParts,
		}
	}

	if filter.Limit > 0 && filter.Limit < 50 {
		payload["page_size"] = filter.Limit
	}

	data, status, err := n.doRequest("POST", fmt.Sprintf("/v1/databases/%s/query", n.databaseID), payload)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("notion query: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Results []notionPage `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse pages: %w", err)
	}

	issues := make([]Issue, len(result.Results))
	for i, page := range result.Results {
		issues[i] = *n.toIssue(&page)
	}
	return issues, nil
}

func (n *NotionProvider) GetIssue(id string) (*Issue, error) {
	data, status, err := n.doRequest("GET", fmt.Sprintf("/v1/pages/%s", id), nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("notion get page: HTTP %d: %s", status, string(data))
	}

	var page notionPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}
	return n.toIssue(&page), nil
}

func (n *NotionProvider) CreateIssue(req CreateIssueRequest) (*Issue, error) {
	if n.databaseID == "" {
		return nil, fmt.Errorf("notion: database ID required")
	}

	properties := map[string]interface{}{
		"Name": map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]interface{}{"content": req.Title}},
			},
		},
	}
	if req.Description != "" {
		properties["Description"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": req.Description}},
			},
		}
	}
	if req.Status != "" {
		properties["Status"] = map[string]interface{}{
			"select": map[string]interface{}{"name": string(req.Status)},
		}
	}
	if req.Priority != "" {
		properties["Priority"] = map[string]interface{}{
			"select": map[string]interface{}{"name": string(req.Priority)},
		}
	}
	if req.Type != "" {
		properties["Type"] = map[string]interface{}{
			"select": map[string]interface{}{"name": string(req.Type)},
		}
	}
	if req.DueDate != nil {
		properties["Due Date"] = map[string]interface{}{
			"date": map[string]interface{}{"start": req.DueDate.Format("2006-01-02")},
		}
	}
	if len(req.Labels) > 0 {
		labels := make([]map[string]interface{}, len(req.Labels))
		for i, l := range req.Labels {
			labels[i] = map[string]interface{}{"name": l}
		}
		properties["Labels"] = map[string]interface{}{"multi_select": labels}
	}

	payload := map[string]interface{}{
		"parent":     map[string]interface{}{"database_id": n.databaseID},
		"properties": properties,
	}

	data, status, err := n.doRequest("POST", "/v1/pages", payload)
	if err != nil {
		return nil, err
	}
	if status != 200 && status != 201 {
		return nil, fmt.Errorf("notion create page: HTTP %d: %s", status, string(data))
	}

	var page notionPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parse created page: %w", err)
	}
	return n.toIssue(&page), nil
}

func (n *NotionProvider) UpdateIssue(id string, req UpdateIssueRequest) (*Issue, error) {
	properties := map[string]interface{}{}
	if req.Title != nil {
		properties["Name"] = map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]interface{}{"content": *req.Title}},
			},
		}
	}
	if req.Description != nil {
		properties["Description"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]interface{}{"content": *req.Description}},
			},
		}
	}
	if req.Status != nil {
		properties["Status"] = map[string]interface{}{
			"select": map[string]interface{}{"name": string(*req.Status)},
		}
	}
	if req.Priority != nil {
		properties["Priority"] = map[string]interface{}{
			"select": map[string]interface{}{"name": string(*req.Priority)},
		}
	}
	if req.DueDate != nil {
		properties["Due Date"] = map[string]interface{}{
			"date": map[string]interface{}{"start": req.DueDate.Format("2006-01-02")},
		}
	}

	payload := map[string]interface{}{
		"properties": properties,
	}

	data, status, err := n.doRequest("PATCH", fmt.Sprintf("/v1/pages/%s", id), payload)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("notion update page: HTTP %d: %s", status, string(data))
	}

	var page notionPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parse updated page: %w", err)
	}
	return n.toIssue(&page), nil
}

func (n *NotionProvider) AddComment(id string, body string) (*Comment, error) {
	payload := map[string]interface{}{
		"parent": map[string]interface{}{"page_id": id},
		"rich_text": []map[string]interface{}{
			{"text": map[string]interface{}{"content": body}},
		},
	}

	data, status, err := n.doRequest("POST", "/v1/comments", payload)
	if err != nil {
		return nil, err
	}
	if status != 200 && status != 201 {
		return nil, fmt.Errorf("notion add comment: HTTP %d: %s", status, string(data))
	}

	var result struct {
		ID          string `json:"id"`
		CreatedTime string `json:"created_time"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse comment: %w", err)
	}

	return &Comment{
		ID:        result.ID,
		IssueID:   id,
		Body:      body,
		CreatedAt: parseNotionTime(result.CreatedTime),
	}, nil
}

func (n *NotionProvider) ListComments(id string) ([]Comment, error) {
	data, status, err := n.doRequest("GET", fmt.Sprintf("/v1/comments?block_id=%s", id), nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("notion list comments: HTTP %d: %s", status, string(data))
	}

	var result struct {
		Results []struct {
			ID          string `json:"id"`
			CreatedTime string `json:"created_time"`
			RichText    []struct {
				PlainText string `json:"plain_text"`
			} `json:"rich_text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse comments: %w", err)
	}

	comments := make([]Comment, len(result.Results))
	for i, c := range result.Results {
		body := ""
		if len(c.RichText) > 0 {
			body = c.RichText[0].PlainText
		}
		comments[i] = Comment{
			ID:        c.ID,
			IssueID:   id,
			Body:      body,
			CreatedAt: parseNotionTime(c.CreatedTime),
		}
	}
	return comments, nil
}

func (n *NotionProvider) Search(filter SearchFilter) ([]Issue, error) {
	return n.ListIssues(filter)
}
