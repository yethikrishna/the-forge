package agentapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServerHealth(t *testing.T) {
	srv := NewServer(ServerConfig{DevMode: true})

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.registerRoutes(http.NewServeMux())
	// Test directly
	handler := http.NewServeMux()
	srv.registerRoutes(handler)
	srv.authMiddleware(handler).ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected ok, got %s", resp["status"])
	}
}

func TestServerStatus(t *testing.T) {
	srv := NewServer(ServerConfig{
		AgentID:   "agent-1",
		Workspace: "ws-1",
		DevMode:   true,
	})
	srv.startTime = time.Now()

	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	var status ServerStatus
	json.NewDecoder(w.Body).Decode(&status)
	if status.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", status.AgentID)
	}
	if status.Workspace != "ws-1" {
		t.Errorf("expected ws-1, got %s", status.Workspace)
	}
}

func TestServerExec(t *testing.T) {
	srv := NewServer(ServerConfig{DevMode: true})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	body := `{"command": "echo hello world"}`
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	var resp ExecResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", resp.ExitCode)
	}
	if !strings.Contains(resp.Stdout, "hello world") {
		t.Errorf("expected 'hello world' in stdout, got %s", resp.Stdout)
	}
}

func TestServerExecWithDir(t *testing.T) {
	srv := NewServer(ServerConfig{DevMode: true})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	body := `{"command": "pwd", "dir": "/tmp"}`
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	var resp ExecResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", resp.ExitCode)
	}
	if !strings.Contains(resp.Stdout, "/tmp") {
		t.Errorf("expected /tmp in output, got %s", resp.Stdout)
	}
}

func TestServerExecFailure(t *testing.T) {
	srv := NewServer(ServerConfig{DevMode: true})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	body := `{"command": "false"}`
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	var resp ExecResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ExitCode == 0 {
		t.Error("expected non-zero exit code for 'false'")
	}
}

func TestServerFileWriteRead(t *testing.T) {
	srv := NewServer(ServerConfig{DevMode: true})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	tmpDir := t.TempDir()
	path := tmpDir + "/test.txt"

	// Write
	writeBody := `{"path": "` + path + `", "content": "hello forge"}`
	req := httptest.NewRequest("POST", "/api/file/write", strings.NewReader(writeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	var writeResp FileResponse
	json.NewDecoder(w.Body).Decode(&writeResp)
	if writeResp.Error != "" {
		t.Fatalf("write error: %s", writeResp.Error)
	}

	// Read
	readBody := `{"path": "` + path + `"}`
	req = httptest.NewRequest("POST", "/api/file/read", strings.NewReader(readBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	var readResp FileResponse
	json.NewDecoder(w.Body).Decode(&readResp)
	if readResp.Content != "hello forge" {
		t.Errorf("expected 'hello forge', got %s", readResp.Content)
	}
}

func TestServerFilePathTraversal(t *testing.T) {
	srv := NewServer(ServerConfig{DevMode: true})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	body := `{"path": "../../../etc/passwd"}`
	req := httptest.NewRequest("POST", "/api/file/read", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d", w.Code)
	}
}

func TestServerFileList(t *testing.T) {
	srv := NewServer(ServerConfig{DevMode: true})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	req := httptest.NewRequest("GET", "/api/file/list?dir=/tmp", nil)
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServerAuth(t *testing.T) {
	srv := NewServer(ServerConfig{
		AuthToken: "secret-token",
	})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	// No auth header
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}

	// Wrong token
	req = httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong token, got %d", w.Code)
	}

	// Correct token
	req = httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w = httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with correct token, got %d", w.Code)
	}
}

func TestServerWorkspace(t *testing.T) {
	srv := NewServer(ServerConfig{
		AgentID:   "a1",
		Workspace: "my-ws",
		DevMode:   true,
	})
	handler := http.NewServeMux()
	srv.registerRoutes(handler)

	req := httptest.NewRequest("GET", "/api/workspace", nil)
	w := httptest.NewRecorder()
	srv.authMiddleware(handler).ServeHTTP(w, req)

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["agent_id"] != "a1" {
		t.Errorf("expected a1, got %s", resp["agent_id"])
	}
	if resp["workspace"] != "my-ws" {
		t.Errorf("expected my-ws, got %s", resp["workspace"])
	}
}

func TestServerStartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv := NewServer(ServerConfig{
		Addr:    "127.0.0.1:0",
		DevMode: true,
	})

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if srv.Addr() == "" {
		t.Error("expected non-empty address after start")
	}

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
