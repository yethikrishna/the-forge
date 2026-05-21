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

// NotionClient implements ProviderClient for Notion's REST API.
// Notion "tasks" are database pages — the database ID is stored in Config.Project.
type NotionClient struct {
	httpClient *http.Client
}

const notionAPIURL = "https://api.notion.com/v1"

func (n *NotionClient) client() *http.Client {
	if n.httpClient == nil {
		n.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return n.httpClient
}

func (n *NotionClient) notionRequest(ctx context.Context, conn *Connection, method, path string, body interface{}) ([]byte, int, error) {
	url := notionAPIURL + path

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
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Authorization", "Bearer "+conn.Config.APIToken)
	if conn.Config.Workspace != "" {
		req.Header.Set("Notion-Workspace", conn.Config.Workspace)
	}

	resp, err := n.client().Do(req)
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

func (n *NotionClient) TestConnectivity(ctx context.Context) error {
	return nil // Validated by first real request
}

// FetchTasks queries a Notion database for pages.
// Config.Project holds the database ID.
func (n *NotionClient) FetchTasks(ctx context.Context, conn *Connection, filters TaskFilters) ([]*Task, error) {
	dbID := conn.Config.Project
	if dbID == "" {
		return nil, fmt.Errorf("notion database ID required (set --project)")
	}

	// Build Notion filter
	var notionFilters []map[string]interface{}

	if filters.Status != "" {
		notionFilters = append(notionFilters, map[string]interface{}{
			"property": "Status",
			"status":   map[string]interface{}{"equals": filters.Status},
		})
	}
	if filters.Assignee != "" {
		notionFilters = append(notionFilters, map[string]interface{}{
			"property": "Assignee",
			"people":   map[string]interface{}{"contains": filters.Assignee},
		})
	}
	for _, label := range filters.Labels {
		notionFilters = append(notionFilters, map[string]interface{}{
			"property": "Tags",
			"multi_select": map[string]interface{}{"contains": label},
		})
	}

	payload := map[string]interface{}{
		"page_size": 100,
	}
	if len(notionFilters) > 0 {
		payload["filter"] = map[string]interface{}{
			"and": notionFilters,
		}
	}

	path := fmt.Sprintf("/databases/%s/query", dbID)
	body, status, err := n.notionRequest(ctx, conn, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("notion query failed (status %d): %s", status, truncate(body, 500))
	}

	var result struct {
		Results []notionPage `json:"results"`
		HasMore bool         `json:"has_more"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse notion response: %w", err)
	}

	tasks := make([]*Task, 0, len(result.Results))
	for _, page := range result.Results {
		tasks = append(tasks, notionPageToTask(page, conn))
	}
	return tasks, nil
}

// GetTask retrieves a single Notion page by ID.
func (n *NotionClient) GetTask(ctx context.Context, conn *Connection, key string) (*Task, error) {
	path := fmt.Sprintf("/pages/%s", key)
	body, status, err := n.notionRequest(ctx, conn, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("notion get page failed (status %d): %s", status, truncate(body, 500))
	}

	var page notionPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("parse notion page: %w", err)
	}
	return notionPageToTask(page, conn), nil
}

// CreateTask creates a page in the Notion database.
func (n *NotionClient) CreateTask(ctx context.Context, conn *Connection, task *Task) (string, error) {
	dbID := conn.Config.Project
	if dbID == "" {
		return "", fmt.Errorf("notion database ID required")
	}

	properties := map[string]interface{}{
		"Name": map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]string{"content": task.Title}},
			},
		},
	}

	if task.Priority != "" {
		properties["Priority"] = map[string]interface{}{
			"select": map[string]string{"name": task.Priority},
		}
	}
	if len(task.Labels) > 0 {
		properties["Tags"] = map[string]interface{}{
			"multi_select": func() []map[string]string {
				tags := make([]map[string]string, len(task.Labels))
				for i, l := range task.Labels {
					tags[i] = map[string]string{"name": l}
				}
				return tags
			}(),
		}
	}
	if task.Status != "" {
		properties["Status"] = map[string]interface{}{
			"status": map[string]string{"name": task.Status},
		}
	}
	if task.DueDate != nil {
		properties["Due Date"] = map[string]interface{}{
			"date": map[string]string{"start": task.DueDate.Format("2006-01-02")},
		}
	}

	payload := map[string]interface{}{
		"parent":     map[string]string{"database_id": dbID},
		"properties": properties,
	}

	if task.Description != "" {
		payload["children"] = []map[string]interface{}{
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{"text": map[string]string{"content": task.Description}},
					},
				},
			},
		}
	}

	body, status, err := n.notionRequest(ctx, conn, http.MethodPost, "/pages", payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return "", fmt.Errorf("notion create page failed (status %d): %s", status, truncate(body, 500))
	}

	var created notionPage
	if err := json.Unmarshal(body, &created); err != nil {
		return "", fmt.Errorf("parse notion create response: %w", err)
	}
	return created.ID, nil
}

// UpdateTask updates properties on a Notion page.
func (n *NotionClient) UpdateTask(ctx context.Context, conn *Connection, task *Task) error {
	properties := map[string]interface{}{}

	if task.Title != "" {
		properties["Name"] = map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]string{"content": task.Title}},
			},
		}
	}
	if task.Status != "" {
		properties["Status"] = map[string]interface{}{
			"status": map[string]string{"name": task.Status},
		}
	}
	if task.Priority != "" {
		properties["Priority"] = map[string]interface{}{
			"select": map[string]string{"name": task.Priority},
		}
	}
	if len(task.Labels) > 0 {
		properties["Tags"] = map[string]interface{}{
			"multi_select": func() []map[string]string {
				tags := make([]map[string]string, len(task.Labels))
				for i, l := range task.Labels {
					tags[i] = map[string]string{"name": l}
				}
				return tags
			}(),
		}
	}
	if task.DueDate != nil {
		properties["Due Date"] = map[string]interface{}{
			"date": map[string]string{"start": task.DueDate.Format("2006-01-02")},
		}
	}

	if len(properties) == 0 {
		return nil
	}

	path := fmt.Sprintf("/pages/%s", task.ProviderKey)
	_, status, err := n.notionRequest(ctx, conn, http.MethodPatch, path, map[string]interface{}{"properties": properties})
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("notion update failed (status %d)", status)
	}
	return nil
}

// AddComment adds a comment to a Notion page.
func (n *NotionClient) AddComment(ctx context.Context, conn *Connection, pageID string, author string, body string) error {
	payload := map[string]interface{}{
		"parent": map[string]string{"page_id": pageID},
		"rich_text": []map[string]interface{}{
			{"text": map[string]string{"content": fmt.Sprintf("%s: %s", author, body)}},
		},
	}

	_, status, err := n.notionRequest(ctx, conn, http.MethodPost, "/comments", payload)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("notion comment failed (status %d)", status)
	}
	return nil
}

// --- Notion API types ---

type notionPage struct {
	ID          string                 `json:"id"`
	URL         string                 `json:"url"`
	CreatedTime string                 `json:"created_time"`
	UpdatedTime string                 `json:"last_edited_time"`
	Properties  map[string]interface{} `json:"properties"`
}

func notionPageToTask(page notionPage, conn *Connection) *Task {
	t := &Task{
		ID:          page.ID,
		ProviderKey: page.ID,
		URL:         page.URL,
		Labels:      []string{},
	}

	t.CreatedAt, _ = time.Parse(time.RFC3339, page.CreatedTime)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, page.UpdatedTime)

	props := page.Properties

	// Extract title
	if nameProp, ok := props["Name"]; ok {
		t.Title = extractNotionTitle(nameProp)
	}
	if titleProp, ok := props["Title"]; ok && t.Title == "" {
		t.Title = extractNotionTitle(titleProp)
	}

	// Extract status
	if statusProp, ok := props["Status"]; ok {
		t.Status = extractNotionStatus(statusProp)
	}

	// Extract priority
	if priorityProp, ok := props["Priority"]; ok {
		t.Priority = extractNotionSelect(priorityProp)
	}

	// Extract tags/labels
	if tagsProp, ok := props["Tags"]; ok {
		t.Labels = extractNotionMultiSelect(tagsProp)
	}

	// Extract due date
	if dueProp, ok := props["Due Date"]; ok {
		if dateStr := extractNotionDate(dueProp); dateStr != "" {
			if d, err := time.Parse("2006-01-02", dateStr); err == nil {
				t.DueDate = &d
			}
		}
	}

	// Extract assignee
	if assigneeProp, ok := props["Assignee"]; ok {
		t.Assignee = extractNotionPeople(assigneeProp)
	}

	// Description is typically in page children, not properties
	// We'd need a separate API call to fetch blocks; leave empty for list views

	return t
}

func extractNotionTitle(prop interface{}) string {
	m, ok := prop.(map[string]interface{})
	if !ok {
		return ""
	}
	titles, ok := m["title"].([]interface{})
	if !ok {
		return ""
	}
	var parts []string
	for _, t := range titles {
		if tm, ok := t.(map[string]interface{}); ok {
			if plain, ok := tm["plain_text"].(string); ok {
				parts = append(parts, plain)
			}
		}
	}
	return strings.Join(parts, "")
}

func extractNotionStatus(prop interface{}) string {
	m, ok := prop.(map[string]interface{})
	if !ok {
		return ""
	}
	status, ok := m["status"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := status["name"].(string)
	return name
}

func extractNotionSelect(prop interface{}) string {
	m, ok := prop.(map[string]interface{})
	if !ok {
		return ""
	}
	sel, ok := m["select"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := sel["name"].(string)
	return normalizePriority(name)
}

func extractNotionMultiSelect(prop interface{}) []string {
	m, ok := prop.(map[string]interface{})
	if !ok {
		return nil
	}
	items, ok := m["multi_select"].([]interface{})
	if !ok {
		return nil
	}
	labels := make([]string, 0, len(items))
	for _, item := range items {
		if im, ok := item.(map[string]interface{}); ok {
			if name, ok := im["name"].(string); ok {
				labels = append(labels, name)
			}
		}
	}
	return labels
}

func extractNotionDate(prop interface{}) string {
	m, ok := prop.(map[string]interface{})
	if !ok {
		return ""
	}
	date, ok := m["date"].(map[string]interface{})
	if !ok {
		return ""
	}
	start, _ := date["start"].(string)
	return start
}

func extractNotionPeople(prop interface{}) string {
	m, ok := prop.(map[string]interface{})
	if !ok {
		return ""
	}
	people, ok := m["people"].([]interface{})
	if !ok || len(people) == 0 {
		return ""
	}
	if pm, ok := people[0].(map[string]interface{}); ok {
		if email, ok := pm["email"].(string); ok {
			return email
		}
		if name, ok := pm["name"].(string); ok {
			return name
		}
	}
	return ""
}
