// Package openclaw provides session management backed by OpenClaw sessions.
// Forge sessions are persistent, resumable, branchable conversations that
// survive restarts and can be transferred between devices.
package openclaw

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/forge/sword/internal/comm"
	"github.com/forge/sword/internal/cost"
	"github.com/forge/sword/internal/ledger"
)

// SessionState represents the current state of a session.
type SessionState string

const (
	SessionActive   SessionState = "active"
	SessionIdle     SessionState = "idle"
	SessionClosed   SessionState = "closed"
	SessionBranched SessionState = "branched"
)

// Session represents a Forge agent session backed by an OpenClaw session.
type Session struct {
	ID          string            `json:"id"`
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	AgentID     string            `json:"agent_id"`
	Division    string            `json:"division"`
	State       SessionState      `json:"state"`
	Model       string            `json:"model"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastActive  time.Time         `json:"last_active"`
	ParentID    string            `json:"parent_id,omitempty"`  // for branched sessions
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	TokenCount  int64             `json:"token_count"`
	CostUSD     float64           `json:"cost_usd"`
}

// SessionCreate is the input for creating a new session.
type SessionCreate struct {
	Label    string   `json:"label"`
	AgentID  string   `json:"agent_id"`
	Division string   `json:"division"`
	Model    string   `json:"model"`
	Tags     []string `json:"tags"`
}

// SessionManager manages sessions via the OpenClaw runtime.
type SessionManager struct {
	bridge   *Bridge
	mu       sync.RWMutex
	sessions map[string]*Session
	// Cost guard wiring
	costTracker *cost.Tracker
	costLedger  *ledger.Ledger
	divBudget   float64      // division cost budget in USD
	downgradeModel string    // fallback model for 80% soft cap
	comm        *comm.Comm
	divHeadChannelID string  // channel to notify on hard cap
}

// NewSessionManager creates a new session manager.
func NewSessionManager(bridge *Bridge) *SessionManager {
	return &SessionManager{
		bridge:   bridge,
		sessions: make(map[string]*Session),
	}
}

// WithCostGuard wires a cost tracker and ledger into the session manager.
// divBudget is the total USD budget for this division/session pool.
// downgradeModel is the cheaper model to switch to at 80% usage.
// comm and divHeadChannelID are used to notify on hard cap.
func (sm *SessionManager) WithCostGuard(tracker *cost.Tracker, l *ledger.Ledger, divBudget float64, downgradeModel string, c *comm.Comm, divHeadChannelID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.costTracker = tracker
	sm.costLedger = l
	sm.divBudget = divBudget
	sm.downgradeModel = downgradeModel
	sm.comm = c
	sm.divHeadChannelID = divHeadChannelID
}

// Create starts a new session.
func (sm *SessionManager) Create(ctx context.Context, input SessionCreate) (*Session, error) {
	if input.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	payload := map[string]interface{}{
		"label":    input.Label,
		"agentId":  input.AgentID,
		"division": input.Division,
		"model":    input.Model,
		"tags":     input.Tags,
	}

	var session Session
	if err := sm.bridge.PostJSON(ctx, "/api/sessions", payload, &session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	sm.mu.Lock()
	sm.sessions[session.ID] = &session
	sm.mu.Unlock()

	return &session, nil
}

// Get retrieves a session by ID.
func (sm *SessionManager) Get(ctx context.Context, id string) (*Session, error) {
	sm.mu.RLock()
	if s, ok := sm.sessions[id]; ok {
		sm.mu.RUnlock()
		return s, nil
	}
	sm.mu.RUnlock()

	var session Session
	if err := sm.bridge.GetJSON(ctx, "/api/sessions/"+id, &session); err != nil {
		return nil, fmt.Errorf("get session %s: %w", id, err)
	}
	sm.mu.Lock()
	sm.sessions[session.ID] = &session
	sm.mu.Unlock()
	return &session, nil
}

// List returns sessions, optionally filtered by division, agent, or state.
func (sm *SessionManager) List(ctx context.Context, opts SessionListOpts) ([]*Session, error) {
	path := "/api/sessions"
	params := []string{}
	if opts.Division != "" {
		params = append(params, "division="+opts.Division)
	}
	if opts.AgentID != "" {
		params = append(params, "agentId="+opts.AgentID)
	}
	if opts.State != "" {
		params = append(params, "state="+string(opts.State))
	}
	if opts.Limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", opts.Limit))
	}
	if len(params) > 0 {
		path += "?" + params[0]
		for _, p := range params[1:] {
			path += "&" + p
		}
	}

	var sessions []*Session
	if err := sm.bridge.GetJSON(ctx, path, &sessions); err != nil {
		// Fall back to local cache
		sm.mu.RLock()
		for _, s := range sm.sessions {
			if opts.matches(s) {
				sessions = append(sessions, s)
			}
		}
		sm.mu.RUnlock()
		return sessions, nil
	}

	sm.mu.Lock()
	for _, s := range sessions {
		sm.sessions[s.ID] = s
	}
	sm.mu.Unlock()

	return sessions, nil
}

// SessionListOpts filters for listing sessions.
type SessionListOpts struct {
	Division string       `json:"division"`
	AgentID  string       `json:"agent_id"`
	State    SessionState `json:"state"`
	Limit    int          `json:"limit"`
}

func (o SessionListOpts) matches(s *Session) bool {
	if o.Division != "" && s.Division != o.Division {
		return false
	}
	if o.AgentID != "" && s.AgentID != o.AgentID {
		return false
	}
	if o.State != "" && s.State != o.State {
		return false
	}
	return true
}

// Send sends a message into a session and waits for the agent response.
// Before sending, it enforces the cost budget:
//   - 80% of budget: downgrade to cheaper model (soft cap)
//   - 100% of budget: stop + notify division head (hard cap)
func (sm *SessionManager) Send(ctx context.Context, sessionID, message string) (string, error) {
	// W03: Cost guard — check budget before every Send()
	if sm.costTracker != nil && sm.divBudget > 0 {
		status := sm.costTracker.CheckBudget("daily", sm.divBudget, 0)

		// 100% hard cap: stop session + notify division head
		if status.OverBudget {
			// Immutable ledger entry
			if sm.costLedger != nil {
				sm.costLedger.RecordAction(sessionID, sessionID, "hard_cap_enforced",
					"send_blocked", 0)
			}
			// Notify division head
			if sm.comm != nil && sm.divHeadChannelID != "" {
				notice := fmt.Sprintf("[HARD CAP] Session %s blocked: budget $%.2f exhausted (spent $%.2f)",
					sessionID, status.Budget, status.Spent)
				sm.comm.Send(sessionID, sm.divHeadChannelID, notice,
					comm.MsgSystem, comm.PrioCritical)
			}
			return "", fmt.Errorf("session %s blocked: budget hard cap reached ($%.2f spent of $%.2f)",
				sessionID, status.Spent, status.Budget)
		}

		// 80% soft cap: downgrade model
		if status.ShouldWarn && sm.downgradeModel != "" {
			sm.mu.RLock()
			s, ok := sm.sessions[sessionID]
			sm.mu.RUnlock()
			if ok && s.Model != sm.downgradeModel {
				// Downgrade the model for this session
				_ = sm.SetModel(ctx, sessionID, sm.downgradeModel)
				// Immutable ledger entry
				if sm.costLedger != nil {
					sm.costLedger.RecordAction(sessionID, sessionID, "soft_cap_downgrade",
						"model_downgraded_to_"+sm.downgradeModel, 0)
				}
			}
		}
	}

	payload := map[string]interface{}{
		"message": message,
	}
	var result struct {
		Reply string `json:"reply"`
	}
	if err := sm.bridge.PostJSON(ctx, "/api/sessions/"+sessionID+"/send", payload, &result); err != nil {
		return "", fmt.Errorf("send to session %s: %w", sessionID, err)
	}
	return result.Reply, nil
}

// Branch creates a new session that continues from an existing one.
func (sm *SessionManager) Branch(ctx context.Context, parentID string, label string) (*Session, error) {
	payload := map[string]interface{}{
		"label":    label,
		"parentId": parentID,
	}
	var session Session
	if err := sm.bridge.PostJSON(ctx, "/api/sessions/"+parentID+"/branch", payload, &session); err != nil {
		return nil, fmt.Errorf("branch session %s: %w", parentID, err)
	}

	// Mark parent as branched
	sm.mu.Lock()
	if parent, ok := sm.sessions[parentID]; ok {
		parent.State = SessionBranched
	}
	sm.sessions[session.ID] = &session
	sm.mu.Unlock()

	return &session, nil
}

// Close ends a session.
func (sm *SessionManager) Close(ctx context.Context, id string) error {
	if err := sm.bridge.PostJSON(ctx, "/api/sessions/"+id+"/close", nil, nil); err != nil {
		return fmt.Errorf("close session %s: %w", id, err)
	}
	sm.mu.Lock()
	if s, ok := sm.sessions[id]; ok {
		s.State = SessionClosed
	}
	sm.mu.Unlock()
	return nil
}

// SetModel overrides the model for a specific session.
func (sm *SessionManager) SetModel(ctx context.Context, id, model string) error {
	if err := sm.bridge.PatchJSON(ctx, "/api/sessions/"+id, map[string]interface{}{"model": model}); err != nil {
		return fmt.Errorf("set model for session %s: %w", id, err)
	}
	sm.mu.Lock()
	if s, ok := sm.sessions[id]; ok {
		s.Model = model
	}
	sm.mu.Unlock()
	return nil
}

// Touch updates the last-active timestamp for a session.
func (sm *SessionManager) Touch(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[id]; ok {
		s.LastActive = time.Now()
		s.UpdatedAt = time.Now()
	}
}
