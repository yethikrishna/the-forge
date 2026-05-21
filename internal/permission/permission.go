// Package permission provides per-session permission scoping for agents.
// Restricts what agents can do within a session: read-only, source-only,
// sandbox, or full access.
//
// Boundaries protect. Scopes enforce.
package permission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Scope represents the permission level of a session.
type Scope string

const (
	ScopeReadOnly Scope = "read-only" // Can only read files, no writes/execution
	ScopeSrcOnly  Scope = "src-only"  // Can only access src/ directory
	ScopeSandbox  Scope = "sandbox"   // Can only write to sandbox directory
	ScopeFull     Scope = "full"      // Full access (default)
)

// Action represents an agent action type.
type Action string

const (
	ActionRead    Action = "read"
	ActionWrite   Action = "write"
	ActionExecute Action = "execute"
	ActionDelete  Action = "delete"
	ActionNetwork Action = "network"
	ActionEnv     Action = "env"
)

// Policy is the permission policy for a session.
type Policy struct {
	SessionID      string    `json:"session_id"`
	Scope          Scope     `json:"scope"`
	AllowedDirs    []string  `json:"allowed_dirs"`
	BlockedDirs    []string  `json:"blocked_dirs"`
	AllowedActions []Action  `json:"allowed_actions"`
	BlockedActions []Action  `json:"blocked_actions"`
	MaxFileSize    int64     `json:"max_file_size"` // max file size in bytes (0 = unlimited)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Violation represents a permission violation.
type Violation struct {
	SessionID string    `json:"session_id"`
	Action    Action    `json:"action"`
	Target    string    `json:"target"`
	Reason    string    `json:"reason"`
	Blocked   bool      `json:"blocked"`
	Timestamp time.Time `json:"timestamp"`
}

// Enforcer enforces per-session permissions.
type Enforcer struct {
	policies   map[string]*Policy
	violations map[string][]Violation // sessionID -> violations
	storeDir   string
	mu         sync.RWMutex
}

// NewEnforcer creates a permission enforcer.
func NewEnforcer(storeDir string) *Enforcer {
	e := &Enforcer{
		policies:   make(map[string]*Policy),
		violations: make(map[string][]Violation),
		storeDir:   storeDir,
	}
	e.load()
	return e
}

// SetPolicy sets the permission policy for a session.
func (e *Enforcer) SetPolicy(policy Policy) error {
	if policy.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	now := time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	// Apply scope defaults if no explicit actions set
	if len(policy.AllowedActions) == 0 {
		policy.AllowedActions = scopeDefaults(policy.Scope)
	}

	e.mu.Lock()
	e.policies[policy.SessionID] = &policy
	e.mu.Unlock()
	e.save()
	return nil
}

// Check checks if an action is allowed for a session.
func (e *Enforcer) Check(sessionID string, action Action, target string) error {
	e.mu.RLock()
	policy, ok := e.policies[sessionID]
	e.mu.RUnlock()

	if !ok {
		// No policy = full access by default
		return nil
	}

	// Check blocked actions
	for _, blocked := range policy.BlockedActions {
		if blocked == action {
			e.recordViolation(sessionID, action, target, fmt.Sprintf("Action %s is blocked", action), true)
			return fmt.Errorf("permission denied: action %s is blocked for session %s", action, sessionID)
		}
	}

	// Check allowed actions
	allowed := false
	for _, a := range policy.AllowedActions {
		if a == action {
			allowed = true
			break
		}
	}
	if !allowed {
		e.recordViolation(sessionID, action, target, fmt.Sprintf("Action %s not in allowed list", action), true)
		return fmt.Errorf("permission denied: action %s not allowed in scope %s", action, policy.Scope)
	}

	// Check directory restrictions
	if target != "" && (len(policy.AllowedDirs) > 0 || len(policy.BlockedDirs) > 0) {
		if err := e.checkPath(policy, target); err != nil {
			e.recordViolation(sessionID, action, target, err.Error(), true)
			return err
		}
	}

	// Check file size for writes
	if action == ActionWrite && policy.MaxFileSize > 0 {
		if info, err := os.Stat(target); err == nil && info.Size() > policy.MaxFileSize {
			e.recordViolation(sessionID, action, target, "file exceeds max size", true)
			return fmt.Errorf("permission denied: file %s exceeds max size %d bytes", target, policy.MaxFileSize)
		}
	}

	return nil
}

// GetPolicy returns the policy for a session.
func (e *Enforcer) GetPolicy(sessionID string) (*Policy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.policies[sessionID]
	if !ok {
		return nil, false
	}
	copy := *p
	return &copy, true
}

// ListSessions returns all sessions with policies.
func (e *Enforcer) ListSessions() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ids := make([]string, 0, len(e.policies))
	for id := range e.policies {
		ids = append(ids, id)
	}
	return ids
}

// GetViolations returns violations for a session.
func (e *Enforcer) GetViolations(sessionID string) []Violation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.violations[sessionID]
}

// RemovePolicy removes a session's policy.
func (e *Enforcer) RemovePolicy(sessionID string) {
	e.mu.Lock()
	delete(e.policies, sessionID)
	e.mu.Unlock()
	e.save()
}

// QuickScope creates a policy from a scope string for a session.
func (e *Enforcer) QuickScope(sessionID string, scope Scope) error {
	policy := Policy{
		SessionID: sessionID,
		Scope:     scope,
	}

	switch scope {
	case ScopeReadOnly:
		policy.AllowedActions = []Action{ActionRead}
		policy.BlockedActions = []Action{ActionWrite, ActionExecute, ActionDelete}
		policy.BlockedDirs = []string{"/etc", "/var", "/sys", "/proc"}
	case ScopeSrcOnly:
		policy.AllowedActions = []Action{ActionRead, ActionWrite}
		policy.AllowedDirs = []string{"src/", "./src/", "internal/", "./internal/"}
		policy.BlockedActions = []Action{ActionExecute, ActionDelete}
	case ScopeSandbox:
		policy.AllowedActions = []Action{ActionRead, ActionWrite, ActionExecute}
		policy.AllowedDirs = []string{"/tmp/sandbox/", "./sandbox/", "/tmp/"}
		policy.BlockedDirs = []string{"/etc", "/var", "/sys", "/proc", "/home"}
	case ScopeFull:
		policy.AllowedActions = []Action{ActionRead, ActionWrite, ActionExecute, ActionDelete, ActionNetwork, ActionEnv}
	}

	return e.SetPolicy(policy)
}

func (e *Enforcer) checkPath(policy *Policy, target string) error {
	// Normalize
	target = filepath.Clean(target)

	// Check blocked dirs first
	for _, blocked := range policy.BlockedDirs {
		clean := filepath.Clean(blocked)
		if strings.HasPrefix(target, clean) {
			return fmt.Errorf("permission denied: path %s is in blocked directory %s", target, clean)
		}
	}

	// If allowed dirs specified, target must be in one
	if len(policy.AllowedDirs) > 0 {
		inAllowed := false
		for _, allowed := range policy.AllowedDirs {
			clean := filepath.Clean(allowed)
			if strings.HasPrefix(target, clean) || strings.HasPrefix(target, "./"+clean) {
				inAllowed = true
				break
			}
		}
		if !inAllowed {
			return fmt.Errorf("permission denied: path %s not in allowed directories", target)
		}
	}

	return nil
}

func (e *Enforcer) recordViolation(sessionID string, action Action, target, reason string, blocked bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	v := Violation{
		SessionID: sessionID,
		Action:    action,
		Target:    target,
		Reason:    reason,
		Blocked:   blocked,
		Timestamp: time.Now(),
	}

	e.violations[sessionID] = append(e.violations[sessionID], v)

	// Keep last 50 violations per session
	if len(e.violations[sessionID]) > 50 {
		e.violations[sessionID] = e.violations[sessionID][len(e.violations[sessionID])-50:]
	}
}

func scopeDefaults(scope Scope) []Action {
	switch scope {
	case ScopeReadOnly:
		return []Action{ActionRead}
	case ScopeSrcOnly:
		return []Action{ActionRead, ActionWrite}
	case ScopeSandbox:
		return []Action{ActionRead, ActionWrite, ActionExecute}
	case ScopeFull:
		return []Action{ActionRead, ActionWrite, ActionExecute, ActionDelete, ActionNetwork, ActionEnv}
	default:
		return []Action{ActionRead}
	}
}

func (e *Enforcer) save() {
	if e.storeDir == "" {
		return
	}
	os.MkdirAll(e.storeDir, 0755)
	data, _ := json.MarshalIndent(e.policies, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "permissions.json"), data, 0644)
}

func (e *Enforcer) load() {
	if e.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(e.storeDir, "permissions.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &e.policies)
}

// FormatPolicy formats a policy for display.
func FormatPolicy(p *Policy) string {
	s := fmt.Sprintf("Session:  %s\n", p.SessionID)
	s += fmt.Sprintf("Scope:    %s\n", p.Scope)
	s += fmt.Sprintf("Actions:  %v\n", p.AllowedActions)
	if len(p.BlockedActions) > 0 {
		s += fmt.Sprintf("Blocked:  %v\n", p.BlockedActions)
	}
	if len(p.AllowedDirs) > 0 {
		s += fmt.Sprintf("Allowed dirs: %v\n", p.AllowedDirs)
	}
	if len(p.BlockedDirs) > 0 {
		s += fmt.Sprintf("Blocked dirs: %v\n", p.BlockedDirs)
	}
	return s
}

// FormatViolation formats a violation for display.
func FormatViolation(v *Violation) string {
	status := "BLOCKED"
	if !v.Blocked {
		status = "LOGGED"
	}
	return fmt.Sprintf("[%s] %s → %s: %s (%s)", status, v.Action, v.Target, v.Reason, v.Timestamp.Format("15:04:05"))
}
