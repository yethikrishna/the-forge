// Package pty provides real terminal handling for agent sessions.
// Manages PTY allocation, resize events, and I/O multiplexing.
package pty

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
)

// SessionState is the state of a PTY session.
type SessionState string

const (
	SessionStarting SessionState = "starting"
	SessionActive   SessionState = "active"
	SessionIdle     SessionState = "idle"
	SessionClosed   SessionState = "closed"
)

// Session represents an interactive terminal session.
type Session struct {
	ID          string       `json:"id"`
	AgentID     string       `json:"agent_id"`
	State       SessionState `json:"state"`
	Command     string       `json:"command"`
	WorkDir     string       `json:"work_dir"`
	StartedAt   time.Time    `json:"started_at"`
	LastActive  time.Time    `json:"last_active"`
	Cols        uint16       `json:"cols"`
	Rows        uint16       `json:"rows"`
	ExitCode    int          `json:"exit_code,omitempty"`

	ptmx       *os.File
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	mu         sync.Mutex
	onOutput   func([]byte)
}

// SessionConfig configures a new PTY session.
type SessionConfig struct {
	AgentID   string `json:"agent_id"`
	Command   string `json:"command"`
	WorkDir   string `json:"work_dir"`
	Env       []string `json:"env,omitempty"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
	Shell     string `json:"shell,omitempty"`
}

// SessionManager manages PTY sessions.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	nextID   int
}

// NewSessionManager creates a session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// Start creates and starts a new PTY session.
func (sm *SessionManager) Start(ctx context.Context, cfg SessionConfig) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	id := fmt.Sprintf("pty-%d", sm.nextID)
	sm.nextID++

	shell := cfg.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.CommandContext(ctx, shell)
	if cfg.Command != "" {
		cmd = exec.CommandContext(ctx, shell, "-c", cfg.Command)
	}

	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}

	env := os.Environ()
	env = append(env, cfg.Env...)
	env = append(env, "TERM=xterm-256color")
	cmd.Env = env

	cols := cfg.Cols
	rows := cfg.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}

	sessCtx, cancel := context.WithCancel(ctx)

	sess := &Session{
		ID:         id,
		AgentID:    cfg.AgentID,
		State:      SessionActive,
		Command:    cfg.Command,
		WorkDir:    cfg.WorkDir,
		StartedAt:  time.Now(),
		LastActive: time.Now(),
		Cols:       cols,
		Rows:       rows,
		ptmx:       ptmx,
		cmd:        cmd,
		cancel:     cancel,
	}

	sm.sessions[id] = sess

	// Monitor process exit
	go func() {
		err := cmd.Wait()
		sess.mu.Lock()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				sess.ExitCode = exitErr.ExitCode()
			}
		}
		sess.State = SessionClosed
		sess.mu.Unlock()
		cancel()
	}()

	_ = sessCtx
	return sess, nil
}

// Get retrieves a session by ID.
func (sm *SessionManager) Get(id string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sess, ok := sm.sessions[id]
	if !ok {
		return nil, fmt.Errorf("pty session %q not found", id)
	}
	return sess, nil
}

// List returns all sessions.
func (sm *SessionManager) List() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		result = append(result, s)
	}
	return result
}

// Close shuts down a session.
func (sm *SessionManager) Close(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("pty session %q not found", id)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.ptmx != nil {
		sess.ptmx.Close()
	}
	if sess.cmd.Process != nil {
		sess.cmd.Process.Kill()
	}
	sess.cancel()
	sess.State = SessionClosed
	delete(sm.sessions, id)
	return nil
}

// CloseAll shuts down all sessions.
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, sess := range sm.sessions {
		sess.mu.Lock()
		if sess.ptmx != nil {
			sess.ptmx.Close()
		}
		if sess.cmd.Process != nil {
			sess.cmd.Process.Kill()
		}
		sess.cancel()
		sess.State = SessionClosed
		sess.mu.Unlock()
		delete(sm.sessions, id)
	}
}

// Write writes data to the PTY stdin.
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != SessionActive {
		return 0, fmt.Errorf("session not active")
	}
	s.LastActive = time.Now()
	return s.ptmx.Write(data)
}

// Read reads output from the PTY.
func (s *Session) Read(buf []byte) (int, error) {
	s.mu.Lock()
	active := s.State == SessionActive
	s.mu.Unlock()

	if !active {
		return 0, io.EOF
	}
	n, err := s.ptmx.Read(buf)
	if n > 0 {
		s.mu.Lock()
		s.LastActive = time.Now()
		if s.onOutput != nil {
			s.onOutput(buf[:n])
		}
		s.mu.Unlock()
	}
	return n, err
}

// Resize changes the terminal dimensions.
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != SessionActive {
		return fmt.Errorf("session not active")
	}

	s.Cols = cols
	s.Rows = rows
	return pty.Setsize(s.ptmx, &pty.Winsize{Cols: cols, Rows: rows})
}

// OnOutput registers a callback for PTY output data.
func (s *Session) OnOutput(fn func([]byte)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onOutput = fn
}

// IsActive returns whether the session is still running.
func (s *Session) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State == SessionActive
}

// Uptime returns the session duration.
func (s *Session) Uptime() time.Duration {
	return time.Since(s.StartedAt)
}

// MarshalJSON serializes session info (without the PTY fd).
func (s *Session) MarshalJSON() ([]byte, error) {
	type Alias Session
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	})
}
