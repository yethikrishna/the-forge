// Package agentapi provides an HTTP API server for agent communication.
// Derived from coder/agentapi patterns — every agent exposes this API for
// workspace management, file sync, and command execution.
package agentapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ServerConfig configures the agent API server.
type ServerConfig struct {
	Addr       string `json:"addr"`
	AgentID    string `json:"agent_id"`
	Workspace  string `json:"workspace"`
	AuthToken  string `json:"auth_token"`
	DevMode    bool   `json:"dev_mode"`
}

// ServerStatus represents the agent's current status.
type ServerStatus struct {
	AgentID   string    `json:"agent_id"`
	Workspace string    `json:"workspace"`
	Status    string    `json:"status"`
	Uptime    string    `json:"uptime"`
	StartedAt time.Time `json:"started_at"`
	Version   string    `json:"version"`
	Hostname  string    `json:"hostname"`
}

// ExecRequest is a command execution request.
type ExecRequest struct {
	Command string            `json:"command"`
	Dir     string            `json:"dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout int               `json:"timeout,omitempty"` // seconds
}

// ExecResponse is the result of a command execution.
type ExecResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error,omitempty"`
}

// FileRequest is a file read/write request.
type FileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

// FileResponse is a file read response.
type FileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	Error   string `json:"error,omitempty"`
}

// Server is the agent API HTTP server.
type Server struct {
	config    ServerConfig
	server    *http.Server
	addr      string
	status    ServerStatus
	mu        sync.RWMutex
	startTime time.Time
}

// NewServer creates an agent API server.
func NewServer(config ServerConfig) *Server {
	hostname, _ := os.Hostname()
	return &Server{
		config: config,
		status: ServerStatus{
			AgentID:   config.AgentID,
			Workspace: config.Workspace,
			Status:    "initialized",
			Version:   "0.1.0",
			Hostname:  hostname,
		},
	}
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	addr := s.config.Addr
	if addr == "" {
		addr = ":0"
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.server = &http.Server{
		Addr:    addr,
		Handler: s.authMiddleware(mux),
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("agentapi listen: %w", err)
	}

	s.mu.Lock()
	s.startTime = time.Now()
	s.status.StartedAt = s.startTime
	s.status.Status = "running"
	actualAddr := ln.Addr().String()
	s.mu.Unlock()

	go func() {
		if err := s.server.Serve(ln); err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "agentapi server error: %v\n", err)
		}
	}()

	fmt.Printf("agentapi: listening on %s\n", actualAddr)

	go func() {
		<-ctx.Done()
		s.server.Close()
	}()

	return nil
}

// Addr returns the server's actual listen address.
func (s *Server) Addr() string {
	return s.addr
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.status.Status = "stopped"
	s.mu.Unlock()
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health and status
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/status", s.handleStatus)

	// Command execution
	mux.HandleFunc("/api/exec", s.handleExec)

	// File operations
	mux.HandleFunc("/api/file/read", s.handleFileRead)
	mux.HandleFunc("/api/file/write", s.handleFileWrite)
	mux.HandleFunc("/api/file/list", s.handleFileList)

	// Workspace info
	mux.HandleFunc("/api/workspace", s.handleWorkspace)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.AuthToken != "" && !s.config.DevMode {
			token := r.Header.Get("Authorization")
			if token != "Bearer "+s.config.AuthToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	st := s.status
	st.Uptime = time.Since(s.startTime).Round(time.Second).String()
	s.mu.RUnlock()
	json.NewEncoder(w).Encode(st)
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}

	if len(req.Env) > 0 {
		env := os.Environ()
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	resp := ExecResponse{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			resp.ExitCode = exitErr.ExitCode()
		} else {
			resp.ExitCode = -1
			resp.Error = err.Error()
		}
	}

	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	var req FileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Sanitize path
	cleanPath := filepath.Clean(req.Path)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile(cleanPath)
	resp := FileResponse{
		Path: cleanPath,
	}

	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Content = string(data)
		resp.Size = int64(len(data))
	}

	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req FileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cleanPath := filepath.Clean(req.Path)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	os.MkdirAll(filepath.Dir(cleanPath), 0o755)
	err := os.WriteFile(cleanPath, []byte(req.Content), 0o644)
	if err != nil {
		json.NewEncoder(w).Encode(FileResponse{Path: cleanPath, Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(FileResponse{Path: cleanPath, Size: int64(len(req.Content))})
}

func (s *Server) handleFileList(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "."
	}

	cleanDir := filepath.Clean(dir)
	if strings.Contains(cleanDir, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	type fileInfo struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size,omitempty"`
	}

	var files []fileInfo
	for _, e := range entries {
		info, _ := e.Info()
		fi := fileInfo{
			Name:  e.Name(),
			IsDir: e.IsDir(),
		}
		if info != nil {
			fi.Size = info.Size()
		}
		files = append(files, fi)
	}

	json.NewEncoder(w).Encode(files)
}

func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	cwd, _ := os.Getwd()
	json.NewEncoder(w).Encode(map[string]string{
		"workspace": s.config.Workspace,
		"cwd":       cwd,
		"agent_id":  s.config.AgentID,
	})
}

// Ensure io is used.
var _ io.Reader = nil
