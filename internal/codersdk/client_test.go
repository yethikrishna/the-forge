package codersdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health failed: %v", err)
	}
}

func TestClientStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			json.NewEncoder(w).Encode(StatusResponse{
				AgentID:   "agent-1",
				Workspace: "ws-1",
				Status:    "running",
				Version:   "0.1.0",
				Hostname:  "testhost",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", status.AgentID)
	}
	if status.Workspace != "ws-1" {
		t.Errorf("expected ws-1, got %s", status.Workspace)
	}
}

func TestClientExec(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/exec" && r.Method == "POST" {
			var req ExecRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(ExecResponse{
				ExitCode: 0,
				Stdout:   "output from: " + req.Command,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	resp, err := client.Exec(context.Background(), ExecRequest{Command: "echo hi"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if resp.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", resp.ExitCode)
	}
	if resp.Stdout != "output from: echo hi" {
		t.Errorf("unexpected stdout: %s", resp.Stdout)
	}
}

func TestClientReadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/file/read" {
			var req FileReadRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(FileResponse{
				Path:    req.Path,
				Content: "file contents here",
				Size:    17,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	resp, err := client.ReadFile(context.Background(), "/tmp/test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if resp.Content != "file contents here" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
}

func TestClientWriteFile(t *testing.T) {
	var receivedPath, receivedContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/file/write" {
			var req FileWriteRequest
			json.NewDecoder(r.Body).Decode(&req)
			receivedPath = req.Path
			receivedContent = req.Content
			json.NewEncoder(w).Encode(FileResponse{
				Path: req.Path,
				Size: int64(len(req.Content)),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.WriteFile(context.Background(), "/tmp/out.txt", "hello world")
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if receivedPath != "/tmp/out.txt" {
		t.Errorf("expected /tmp/out.txt, got %s", receivedPath)
	}
	if receivedContent != "hello world" {
		t.Errorf("expected 'hello world', got %s", receivedContent)
	}
}

func TestClientWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/workspace" {
			json.NewEncoder(w).Encode(WorkspaceInfo{
				Workspace: "my-ws",
				CWD:       "/home/user",
				AgentID:   "a1",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ws, err := client.Workspace(context.Background())
	if err != nil {
		t.Fatalf("Workspace failed: %v", err)
	}
	if ws.Workspace != "my-ws" {
		t.Errorf("expected my-ws, got %s", ws.Workspace)
	}
}

func TestClientAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	// Without auth
	client := NewClient(server.URL, "")
	err := client.Health(context.Background())
	if err == nil {
		t.Error("expected auth error")
	}

	// With auth
	client = NewClient(server.URL, "my-secret")
	err = client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health with auth failed: %v", err)
	}
}

func TestClientHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.Status(context.Background())
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestClientConnectionRefused(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "")
	_, err := client.Status(context.Background())
	if err == nil {
		t.Error("expected connection refused error")
	}
}

func TestClientReadFileError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileResponse{
			Path:  "/nope",
			Error: "file not found",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.ReadFile(context.Background(), "/nope")
	if err == nil {
		t.Error("expected error for file read failure")
	}
}
