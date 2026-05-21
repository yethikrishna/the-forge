package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Jira Client Tests ---

func TestJiraClient_FetchTasks(t *testing.T) {
	response := map[string]interface{}{
		"issues": []map[string]interface{}{
			{
				"key": "PROJ-1",
				"id":  "10001",
				"fields": map[string]interface{}{
					"summary":  "Fix login bug",
					"status":   map[string]string{"name": "In Progress"},
					"priority": map[string]string{"name": "High"},
					"labels":   []string{"bug", "backend"},
					"project":  map[string]string{"key": "PROJ"},
					"issuetype": map[string]string{"name": "Bug"},
					"assignee": map[string]string{
						"emailAddress": "alice@example.com",
						"displayName": "Alice",
					},
					"created": "2024-01-15T10:30:00.000+0000",
					"updated": "2024-01-16T14:00:00.000+0000",
					"duedate": "2024-02-01",
				},
			},
			{
				"key": "PROJ-2",
				"id":  "10002",
				"fields": map[string]interface{}{
					"summary":  "Add feature",
					"status":   map[string]string{"name": "To Do"},
					"priority": map[string]string{"name": "Medium"},
					"labels":   []string{},
					"project":  map[string]string{"key": "PROJ"},
					"issuetype": map[string]string{"name": "Task"},
					"created": "2024-01-10T08:00:00.000+0000",
					"updated": "2024-01-12T09:00:00.000+0000",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search" {
			t.Errorf("expected /rest/api/3/search, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user@example.com" || pass != "test-token" {
			t.Errorf("expected basic auth user@example.com:test-token, got %s:%s", user, pass)
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &JiraClient{}
	conn := &Connection{
		Config: Config{
			Provider: ProviderJira,
			BaseURL:  server.URL,
			APIToken: "test-token",
			Email:    "user@example.com",
			Project:  "PROJ",
		},
	}

	tasks, err := client.FetchTasks(context.Background(), conn, TaskFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	if tasks[0].ProviderKey != "PROJ-1" {
		t.Errorf("expected PROJ-1, got %s", tasks[0].ProviderKey)
	}
	if tasks[0].Title != "Fix login bug" {
		t.Errorf("expected 'Fix login bug', got %s", tasks[0].Title)
	}
	if tasks[0].Status != "In Progress" {
		t.Errorf("expected 'In Progress', got %s", tasks[0].Status)
	}
	if tasks[0].Priority != "high" {
		t.Errorf("expected 'high', got %s", tasks[0].Priority)
	}
	if tasks[0].Assignee != "alice@example.com" {
		t.Errorf("expected 'alice@example.com', got %s", tasks[0].Assignee)
	}
	if tasks[0].Project != "PROJ" {
		t.Errorf("expected 'PROJ', got %s", tasks[0].Project)
	}
	if len(tasks[0].Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(tasks[0].Labels))
	}
	if tasks[0].DueDate == nil {
		t.Error("expected due date")
	}
}

func TestJiraClient_GetTask(t *testing.T) {
	response := map[string]interface{}{
		"key": "PROJ-42",
		"id":  "10042",
		"fields": map[string]interface{}{
			"summary":  "Single issue",
			"status":   map[string]string{"name": "Done"},
			"priority": map[string]string{"name": "Low"},
			"labels":   []string{"cleanup"},
			"project":  map[string]string{"key": "PROJ"},
			"issuetype": map[string]string{"name": "Task"},
			"created":  "2024-03-01T10:00:00.000+0000",
			"updated":  "2024-03-02T15:00:00.000+0000",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-42" {
			t.Errorf("expected /rest/api/3/issue/PROJ-42, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &JiraClient{}
	conn := &Connection{
		Config: Config{
			Provider: ProviderJira,
			BaseURL:  server.URL,
			APIToken: "test-token",
		},
	}

	task, err := client.GetTask(context.Background(), conn, "PROJ-42")
	if err != nil {
		t.Fatal(err)
	}
	if task.ProviderKey != "PROJ-42" {
		t.Errorf("expected PROJ-42, got %s", task.ProviderKey)
	}
	if task.Title != "Single issue" {
		t.Errorf("expected 'Single issue', got %s", task.Title)
	}
}

func TestJiraClient_CreateTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			t.Errorf("expected /rest/api/3/issue, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify payload
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		fields := payload["fields"].(map[string]interface{})
		if fields["summary"] != "New task" {
			t.Errorf("expected summary 'New task', got %v", fields["summary"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"key": "PROJ-99",
			"id":  "10099",
		})
	}))
	defer server.Close()

	client := &JiraClient{}
	conn := &Connection{
		Config: Config{
			Provider: ProviderJira,
			BaseURL:  server.URL,
			APIToken: "test-token",
			Project:  "PROJ",
		},
	}

	key, err := client.CreateTask(context.Background(), conn, &Task{
		Title:       "New task",
		Description: "Task body",
		Priority:    "high",
		Labels:      []string{"feature"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if key != "PROJ-99" {
		t.Errorf("expected PROJ-99, got %s", key)
	}
}

func TestJiraClient_UpdateTask(t *testing.T) {
	updateCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/PROJ-1" {
			updateCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Transitions endpoint
		if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/PROJ-1/transitions" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []map[string]interface{}{
					{"id": "31", "name": "Done", "to": map[string]string{"name": "Done"}},
				},
			})
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue/PROJ-1/transitions" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &JiraClient{}
	conn := &Connection{
		Config: Config{
			Provider: ProviderJira,
			BaseURL:  server.URL,
			APIToken: "test-token",
		},
	}

	err := client.UpdateTask(context.Background(), conn, &Task{
		ProviderKey: "PROJ-1",
		Title:       "Updated title",
		Status:      "Done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updateCalled {
		t.Error("expected update to be called")
	}
}

func TestJiraClient_AddComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/comment" {
			t.Errorf("expected comment endpoint, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "comment-1"})
	}))
	defer server.Close()

	client := &JiraClient{}
	conn := &Connection{
		Config: Config{
			Provider: ProviderJira,
			BaseURL:  server.URL,
			APIToken: "test-token",
		},
	}

	err := client.AddComment(context.Background(), conn, "PROJ-1", "forge", "Looking at this")
	if err != nil {
		t.Fatal(err)
	}
}

func TestJiraClient_FetchTasksWithFilters(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"issues": []interface{}{},
		})
	}))
	defer server.Close()

	client := &JiraClient{}
	conn := &Connection{
		Config: Config{
			Provider: ProviderJira,
			BaseURL:  server.URL,
			APIToken: "test-token",
			Project:  "ENG",
		},
	}

	_, err := client.FetchTasks(context.Background(), conn, TaskFilters{
		Status:   "In Progress",
		Assignee: "bob@example.com",
		Labels:   []string{"backend"},
		Limit:    25,
	})
	if err != nil {
		t.Fatal(err)
	}

	jql, ok := receivedPayload["jql"].(string)
	if !ok {
		t.Fatal("expected jql in payload")
	}
	if !containsStr(jql, "project=ENG") {
		t.Errorf("expected project filter in JQL: %s", jql)
	}
	if !containsStr(jql, "status=\"In Progress\"") {
		t.Errorf("expected status filter in JQL: %s", jql)
	}
	if !containsStr(jql, "assignee=\"bob@example.com\"") {
		t.Errorf("expected assignee filter in JQL: %s", jql)
	}
	if !containsStr(jql, "labels=\"backend\"") {
		t.Errorf("expected label filter in JQL: %s", jql)
	}
}

func TestJiraClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorMessages": []string{"Unauthorized"},
		})
	}))
	defer server.Close()

	client := &JiraClient{}
	conn := &Connection{
		Config: Config{
			Provider: ProviderJira,
			BaseURL:  server.URL,
			APIToken: "bad-token",
		},
	}

	_, err := client.FetchTasks(context.Background(), conn, TaskFilters{})
	if err == nil {
		t.Error("expected error for 401 response")
	}
}

// --- Helper ---

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
