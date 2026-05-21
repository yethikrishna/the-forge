// Package replay provides session replay and time-travel debugging
// for agent conversations. It captures the full lifecycle of agent
// interactions — prompts, responses, tool calls, errors — and allows
// stepping through them forward and backward, branching from any
// point, and comparing alternative execution paths.
//
// "Those who cannot remember the past are condemned to repeat it."
package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// EventType represents the type of recorded event.
type EventType int

const (
	EventPrompt      EventType = iota
	EventResponse
	EventToolCall
	EventToolResult
	EventError
	EventRetry
	EventRedirect
	EventCacheHit
	EventCacheMiss
	EventCancel
	EventTimeout
)

func (e EventType) String() string {
	switch e {
	case EventPrompt:
		return "prompt"
	case EventResponse:
		return "response"
	case EventToolCall:
		return "tool_call"
	case EventToolResult:
		return "tool_result"
	case EventError:
		return "error"
	case EventRetry:
		return "retry"
	case EventRedirect:
		return "redirect"
	case EventCacheHit:
		return "cache_hit"
	case EventCacheMiss:
		return "cache_miss"
	case EventCancel:
		return "cancel"
	case EventTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// Event represents a single recorded event in a session.
type Event struct {
	ID        string                 `json:"id"`
	Sequence  int                    `json:"seq"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"ts"`
	AgentID   string                 `json:"agent_id,omitempty"`
	Model     string                 `json:"model,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	TokensIn  int                    `json:"tokens_in,omitempty"`
	TokensOut int                    `json:"tokens_out,omitempty"`
	CostUSD   float64                `json:"cost_usd,omitempty"`
	ParentID  string                 `json:"parent_id,omitempty"` // for tool calls linked to prompts
	Tags      []string               `json:"tags,omitempty"`
}

// Session represents a recorded agent session.
type Session struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	AgentID     string    `json:"agent_id"`
	Events      []Event   `json:"events"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at,omitempty"`
	TotalTokens int       `json:"total_tokens"`
	TotalCost   float64   `json:"total_cost"`
	BranchFrom  string    `json:"branch_from,omitempty"` // session ID this branched from
	BranchAt    int       `json:"branch_at,omitempty"`   // sequence number where branch happened
	Status      string    `json:"status"`                // "recording", "completed", "error"
}

// Checkpoint is a named point in a session that can be restored.
type Checkpoint struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Name      string    `json:"name"`
	Sequence  int       `json:"seq"` // event sequence number
	CreatedAt time.Time `json:"created_at"`
	Note      string    `json:"note,omitempty"`
}

// Recorder captures events into a session.
type Recorder struct {
	mu          sync.Mutex
	session     *Session
	checkpoints []*Checkpoint
	nextSeq     int
	storeDir    string
}

// NewRecorder creates a new session recorder.
func NewRecorder(name, agentID, storeDir string) *Recorder {
	now := time.Now()
	return &Recorder{
		session: &Session{
			ID:        fmt.Sprintf("sess-%d", now.UnixMilli()),
			Name:      name,
			AgentID:   agentID,
			Events:    make([]Event, 0),
			StartedAt: now,
			Status:    "recording",
		},
		nextSeq:     1,
		storeDir:    storeDir,
		checkpoints: make([]*Checkpoint, 0),
	}
}

// Record adds an event to the session.
func (r *Recorder) Record(eventType EventType, data map[string]interface{}) Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	event := Event{
		ID:        fmt.Sprintf("evt-%s-%d", r.session.ID[len(r.session.ID)-6:], r.nextSeq),
		Sequence:  r.nextSeq,
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	r.nextSeq++
	r.session.Events = append(r.session.Events, event)
	r.updateTotals(&event)

	return event
}

// RecordPrompt records a prompt event.
func (r *Recorder) RecordPrompt(agentID, model, prompt string, tokensIn int) Event {
	return r.Record(EventPrompt, map[string]interface{}{
		"agent_id":  agentID,
		"model":     model,
		"prompt":    prompt,
		"tokens_in": tokensIn,
	})
}

// RecordResponse records a response event.
func (r *Recorder) RecordResponse(agentID, model, response string, tokensOut int, costUSD float64, duration time.Duration) Event {
	event := r.Record(EventResponse, map[string]interface{}{
		"agent_id":   agentID,
		"model":      model,
		"response":   response,
		"tokens_out": tokensOut,
	})
	event.TokensOut = tokensOut
	event.CostUSD = costUSD
	event.Duration = duration
	r.mu.Lock()
	r.session.TotalCost += costUSD
	r.session.TotalTokens += tokensOut
	r.mu.Unlock()
	return event
}

// RecordToolCall records a tool call event.
func (r *Recorder) RecordToolCall(toolName string, params map[string]interface{}, parentID string) Event {
	event := r.Record(EventToolCall, map[string]interface{}{
		"tool":   toolName,
		"params": params,
	})
	event.ParentID = parentID
	return event
}

// RecordToolResult records a tool result event.
func (r *Recorder) RecordToolResult(toolName string, result interface{}, duration time.Duration) Event {
	event := r.Record(EventToolResult, map[string]interface{}{
		"tool":   toolName,
		"result": result,
	})
	event.Duration = duration
	return event
}

// RecordError records an error event.
func (r *Recorder) RecordError(err error, recoverable bool) Event {
	return r.Record(EventError, map[string]interface{}{
		"error":        err.Error(),
		"recoverable":  recoverable,
	})
}

// Checkpoint creates a named checkpoint at the current position.
func (r *Recorder) Checkpoint(name, note string) *Checkpoint {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := &Checkpoint{
		ID:        fmt.Sprintf("cp-%d", len(r.checkpoints)+1),
		SessionID: r.session.ID,
		Name:      name,
		Sequence:  r.nextSeq - 1,
		CreatedAt: time.Now(),
		Note:      note,
	}
	r.checkpoints = append(r.checkpoints, cp)
	return cp
}

// Stop stops recording and saves the session.
func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.session.EndedAt = time.Now()
	r.session.Status = "completed"
	return r.save()
}

// Session returns the recorded session.
func (r *Recorder) Session() *Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.session
}

// Branch creates a new recorder that branches from the current session at a given sequence.
func (r *Recorder) Branch(fromSeq int) *Recorder {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	newSession := &Session{
		ID:          fmt.Sprintf("sess-%d", now.UnixMilli()),
		Name:        r.session.Name + " (branch)",
		AgentID:     r.session.AgentID,
		Events:      make([]Event, fromSeq),
		StartedAt:   now,
		Status:      "recording",
		BranchFrom:  r.session.ID,
		BranchAt:    fromSeq,
	}

	copy(newSession.Events, r.session.Events[:fromSeq])

	// Recalculate totals
	for _, e := range newSession.Events {
		r.updateTotals(&e)
	}

	return &Recorder{
		session:     newSession,
		nextSeq:     fromSeq + 1,
		storeDir:    r.storeDir,
		checkpoints: make([]*Checkpoint, 0),
	}
}

// Player replays a recorded session with time-travel capabilities.
type Player struct {
	mu      sync.RWMutex
	session *Session
	pos     int // current position (0-based index into Events)
}

// NewPlayer creates a player for a session.
func NewPlayer(session *Session) *Player {
	return &Player{
		session: session,
		pos:     -1, // before first event
	}
}

// Current returns the current event, or nil if not positioned.
func (p *Player) Current() *Event {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.getEvent(p.pos)
}

// Next advances to the next event and returns it.
func (p *Player) Next() *Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pos++
	return p.getEvent(p.pos)
}

// Prev goes back to the previous event and returns it.
func (p *Player) Prev() *Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pos > 0 {
		p.pos--
	}
	return p.getEvent(p.pos)
}

// SeekTo moves to a specific sequence number.
func (p *Player) SeekTo(seq int) *Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, e := range p.session.Events {
		if e.Sequence == seq {
			p.pos = i
			return &e
		}
	}
	return nil
}

// SeekToTime moves to the event closest to the given time.
func (p *Player) SeekToTime(t time.Time) *Event {
	p.mu.Lock()
	defer p.mu.Unlock()

	best := -1
	bestDiff := time.Duration(1<<63 - 1)

	for i, e := range p.session.Events {
		diff := e.Timestamp.Sub(t)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			best = i
		}
	}

	if best >= 0 {
		p.pos = best
		return &p.session.Events[best]
	}
	return nil
}

// JumpToType moves to the next event of a given type.
func (p *Player) JumpToType(eventType EventType) *Event {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := p.pos + 1; i < len(p.session.Events); i++ {
		if p.session.Events[i].Type == eventType {
			p.pos = i
			return &p.session.Events[i]
		}
	}
	return nil
}

// Rewind goes back to the beginning.
func (p *Player) Rewind() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pos = -1
}

// Position returns the current position (0-based) and total events.
func (p *Player) Position() (int, int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pos, len(p.session.Events)
}

// HasNext returns true if there are more events.
func (p *Player) HasNext() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pos < len(p.session.Events)-1
}

// HasPrev returns true if we can go back.
func (p *Player) HasPrev() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pos > 0
}

// Filter returns events matching a filter function.
func (p *Player) Filter(fn func(Event) bool) []Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []Event
	for _, e := range p.session.Events {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

// EventsBetween returns events in a sequence range.
func (p *Player) EventsBetween(fromSeq, toSeq int) []Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []Event
	for _, e := range p.session.Events {
		if e.Sequence >= fromSeq && e.Sequence <= toSeq {
			result = append(result, e)
		}
	}
	return result
}

// Summary returns a summary of the session.
func (p *Player) Summary() SessionSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()

	summary := SessionSummary{
		SessionID:   p.session.ID,
		Name:        p.session.Name,
		TotalEvents: len(p.session.Events),
		StartedAt:   p.session.StartedAt,
		EndedAt:     p.session.EndedAt,
		TotalTokens: p.session.TotalTokens,
		TotalCost:   p.session.TotalCost,
		ByType:      make(map[string]int),
	}

	for _, e := range p.session.Events {
		summary.ByType[e.Type.String()]++
	}

	if !p.session.EndedAt.IsZero() {
		summary.Duration = p.session.EndedAt.Sub(p.session.StartedAt)
	}

	return summary
}

// SessionSummary holds summary statistics.
type SessionSummary struct {
	SessionID   string            `json:"session_id"`
	Name        string            `json:"name"`
	TotalEvents int               `json:"total_events"`
	StartedAt   time.Time         `json:"started_at"`
	EndedAt     time.Time         `json:"ended_at"`
	Duration    time.Duration     `json:"duration"`
	TotalTokens int               `json:"total_tokens"`
	TotalCost   float64           `json:"total_cost"`
	ByType      map[string]int    `json:"by_type"`
}

// Store manages replay sessions.
type Store struct {
	mu       sync.RWMutex
	dir      string
	sessions map[string]*Session
}

// NewStore creates a new replay store.
func NewStore(dir string) *Store {
	s := &Store{
		dir:      dir,
		sessions: make(map[string]*Session),
	}
	s.load()
	return s
}

// Save saves a session to the store.
func (s *Store) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = session
	return s.saveSession(session)
}

// Get retrieves a session by ID.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return session, nil
}

// List returns all sessions sorted by start time.
func (s *Store) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Session, 0, len(s.sessions))
	for _, s := range s.sessions {
		result = append(result, s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	return result
}

// Delete removes a session.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, id)
	return os.Remove(filepath.Join(s.dir, id+".json"))
}

// Compare compares two sessions and returns differences.
func (s *Store) Compare(id1, id2 string) (*Comparison, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s1, ok1 := s.sessions[id1]
	s2, ok2 := s.sessions[id2]

	if !ok1 {
		return nil, fmt.Errorf("session %s not found", id1)
	}
	if !ok2 {
		return nil, fmt.Errorf("session %s not found", id2)
	}

	cmp := &Comparison{
		SessionA: id1,
		SessionB: id2,
	}

	cmp.EventsA = len(s1.Events)
	cmp.EventsB = len(s2.Events)
	cmp.TokensA = s1.TotalTokens
	cmp.TokensB = s2.TotalTokens
	cmp.CostA = s1.TotalCost
	cmp.CostB = s2.TotalCost

	if s1.TotalTokens > 0 {
		cmp.TokenDiff = float64(s2.TotalTokens-s1.TotalTokens) / float64(s1.TotalTokens) * 100
	}
	if s1.TotalCost > 0 {
		cmp.CostDiff = (s2.TotalCost - s1.TotalCost) / s1.TotalCost * 100
	}

	// Find common prefix
	cmp.CommonPrefix = 0
	minLen := len(s1.Events)
	if len(s2.Events) < minLen {
		minLen = len(s2.Events)
	}
	for i := 0; i < minLen; i++ {
		if s1.Events[i].Type == s2.Events[i].Type {
			cmp.CommonPrefix++
		} else {
			break
		}
	}

	return cmp, nil
}

// Comparison holds the result of comparing two sessions.
type Comparison struct {
	SessionA    string  `json:"session_a"`
	SessionB    string  `json:"session_b"`
	EventsA     int     `json:"events_a"`
	EventsB     int     `json:"events_b"`
	TokensA     int     `json:"tokens_a"`
	TokensB     int     `json:"tokens_b"`
	CostA       float64 `json:"cost_a"`
	CostB       float64 `json:"cost_b"`
	TokenDiff   float64 `json:"token_diff_pct"`
	CostDiff    float64 `json:"cost_diff_pct"`
	CommonPrefix int    `json:"common_prefix"`
}

func (p *Player) getEvent(pos int) *Event {
	if pos < 0 || pos >= len(p.session.Events) {
		return nil
	}
	e := p.session.Events[pos]
	return &e
}

func (r *Recorder) updateTotals(event *Event) {
	r.session.TotalTokens += event.TokensIn + event.TokensOut
	r.session.TotalCost += event.CostUSD
}

func (r *Recorder) save() error {
	if r.storeDir == "" {
		return nil
	}
	os.MkdirAll(r.storeDir, 0755)
	data, _ := json.MarshalIndent(r.session, "", "  ")
	return os.WriteFile(filepath.Join(r.storeDir, r.session.ID+".json"), data, 0644)
}

func (s *Store) saveSession(session *Session) error {
	if s.dir == "" {
		return nil
	}
	os.MkdirAll(s.dir, 0755)
	data, _ := json.MarshalIndent(session, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, session.ID+".json"), data, 0644)
}

func (s *Store) load() {
	if s.dir == "" {
		return
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		s.sessions[session.ID] = &session
	}
}
