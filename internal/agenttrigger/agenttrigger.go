// Package agenttrigger provides event-driven agent triggering.
// It connects file system changes, webhook events, and PR events
// to agent pipeline execution, enabling automated workflows like
// "run tests on .go file changes" or "review on PR open".
//
// When the anvil rings, the agent strikes.
package agenttrigger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// TriggerType defines what kind of event activates a trigger.
type TriggerType string

const (
	TriggerFileChange  TriggerType = "file_change"
	TriggerWebhook     TriggerType = "webhook"
	TriggerPR          TriggerType = "pr"
	TriggerCron        TriggerType = "cron"
	TriggerManual      TriggerType = "manual"
)

// FileChangeCondition defines which file changes activate the trigger.
type FileChangeCondition struct {
	// Paths to watch (relative to project root).
	Paths []string `json:"paths,omitempty"`

	// Extensions to match (e.g. [".go", ".tsx"]).
	Extensions []string `json:"extensions,omitempty"`

	// Event types: "create", "modify", "delete". Empty = all.
	Events []string `json:"events,omitempty"`

	// Ignore patterns.
	Ignore []string `json:"ignore,omitempty"`
}

// PRCondition defines which PR events activate the trigger.
type PRCondition struct {
	// Events: "opened", "synchronize", "closed", "labeled".
	Events []string `json:"events,omitempty"`

	// Branches to match (glob patterns). Empty = all.
	Branches []string `json:"branches,omitempty"`

	// Labels that must be present.
	Labels []string `json:"labels,omitempty"`
}

// WebhookCondition defines webhook trigger conditions.
type WebhookCondition struct {
	// HTTP path to listen on (e.g. "/hooks/deploy").
	Path string `json:"path"`

	// Allowed HTTP methods.
	Methods []string `json:"methods,omitempty"`

	// Secret for HMAC verification (optional).
	Secret string `json:"secret,omitempty"`
}

// Condition is the trigger condition (only one field should be set).
type Condition struct {
	FileChange  *FileChangeCondition `json:"file_change,omitempty"`
	PR          *PRCondition         `json:"pr,omitempty"`
	Webhook     *WebhookCondition    `json:"webhook,omitempty"`
	CronExpr    string               `json:"cron_expr,omitempty"`
}

// Action defines what happens when the trigger fires.
type Action struct {
	// Pipeline to run (references a forge.yaml pipeline name).
	Pipeline string `json:"pipeline"`

	// Agent to use (overrides pipeline default).
	Agent string `json:"agent,omitempty"`

	// Model to use.
	Model string `json:"model,omitempty"`

	// Extra arguments to pass to the pipeline.
	Args map[string]string `json:"args,omitempty"`

	// Environment variables.
	Env map[string]string `json:"env,omitempty"`

	// Timeout for the pipeline run.
	Timeout string `json:"timeout,omitempty"`
}

// Trigger is a complete trigger definition.
type Trigger struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        TriggerType `json:"type"`
	Enabled     bool      `json:"enabled"`
	Condition   Condition `json:"condition"`
	Action      Action    `json:"action"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	FireCount   int       `json:"fire_count"`
	LastFiredAt *time.Time `json:"last_fired_at,omitempty"`
}

// TriggerEvent is an event that may fire triggers.
type TriggerEvent struct {
	ID        string                 `json:"id"`
	Type      TriggerType            `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	Source    string                 `json:"source"`
}

// ExecutionRecord tracks a trigger execution.
type ExecutionRecord struct {
	ID         string     `json:"id"`
	TriggerID  string     `json:"trigger_id"`
	TriggerName string   `json:"trigger_name"`
	Event      TriggerEvent `json:"event"`
	Status     string     `json:"status"` // "started", "completed", "failed", "timeout"
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	PipelineOutput string `json:"pipeline_output,omitempty"`
}

// PipelineRunner is the interface for executing pipelines.
// The caller provides the implementation (connects to forge pipeline system).
type PipelineRunner interface {
	Run(ctx context.Context, pipeline, agent, model string, args, env map[string]string) (string, error)
}

// Manager manages event-driven agent triggers.
type Manager struct {
	mu         sync.RWMutex
	dir        string
	triggers   map[string]*Trigger
	history    []ExecutionRecord
	runner     PipelineRunner
	maxHistory int
}

// NewManager creates a new trigger manager.
func NewManager(dir string, runner PipelineRunner) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create trigger dir: %w", err)
	}
	m := &Manager{
		dir:        dir,
		triggers:   make(map[string]*Trigger),
		history:    make([]ExecutionRecord, 0),
		runner:     runner,
		maxHistory: 100,
	}
	m.load()
	return m, nil
}

// Create creates a new trigger.
func (m *Manager) Create(name string, triggerType TriggerType, condition Condition, action Action, description string) (*Trigger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("trigger-%d", time.Now().UnixNano())
	now := time.Now().UTC()

	t := &Trigger{
		ID:          id,
		Name:        name,
		Type:        triggerType,
		Enabled:     true,
		Condition:   condition,
		Action:      action,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.triggers[id] = t
	m.save()
	return t, nil
}

// Get returns a trigger by ID.
func (m *Manager) Get(id string) (*Trigger, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.triggers[id]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// List returns all triggers.
func (m *Manager) List() []Trigger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Trigger
	for _, t := range m.triggers {
		result = append(result, *t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Update updates an existing trigger.
func (m *Manager) Update(id string, updates ...UpdateOption) (*Trigger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.triggers[id]
	if !ok {
		return nil, fmt.Errorf("trigger %q not found", id)
	}

	for _, opt := range updates {
		opt(t)
	}
	t.UpdatedAt = time.Now().UTC()
	m.save()

	cp := *t
	return &cp, nil
}

// Delete removes a trigger.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.triggers[id]; !ok {
		return fmt.Errorf("trigger %q not found", id)
	}
	delete(m.triggers, id)
	m.save()
	return nil
}

// Enable enables a trigger.
func (m *Manager) Enable(id string) error {
	_, err := m.Update(id, WithEnabled(true))
	return err
}

// Disable disables a trigger.
func (m *Manager) Disable(id string) error {
	_, err := m.Update(id, WithEnabled(false))
	return err
}

// ProcessEvent processes an incoming event and fires matching triggers.
func (m *Manager) ProcessEvent(ctx context.Context, event TriggerEvent) ([]ExecutionRecord, error) {
	m.mu.RLock()
	var matching []*Trigger
	for _, t := range m.triggers {
		if !t.Enabled {
			continue
		}
		if m.matches(t, event) {
			matching = append(matching, t)
		}
	}
	m.mu.RUnlock()

	if len(matching) == 0 {
		return nil, nil
	}

	var records []ExecutionRecord
	var errs []string

	for _, t := range matching {
		rec, err := m.execute(ctx, t, event)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", t.Name, err))
		}
		records = append(records, rec)
	}

	if len(errs) > 0 {
		return records, fmt.Errorf("errors: %s", strings.Join(errs, "; "))
	}
	return records, nil
}

// matches checks if a trigger matches an event.
func (m *Manager) matches(t *Trigger, event TriggerEvent) bool {
	if t.Type != event.Type {
		return false
	}

	switch t.Type {
	case TriggerFileChange:
		return m.matchFileChange(t.Condition.FileChange, event)
	case TriggerPR:
		return m.matchPR(t.Condition.PR, event)
	case TriggerWebhook:
		return m.matchWebhook(t.Condition.Webhook, event)
	case TriggerCron:
		return true // cron triggers match their own schedule
	case TriggerManual:
		return true
	default:
		return false
	}
}

func (m *Manager) matchFileChange(cond *FileChangeCondition, event TriggerEvent) bool {
	if cond == nil {
		return true
	}

	// Check event type
	if len(cond.Events) > 0 {
		eventType, _ := event.Payload["event_type"].(string)
		found := false
		for _, e := range cond.Events {
			if strings.EqualFold(e, eventType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check extension
	filePath, _ := event.Payload["path"].(string)
	if len(cond.Extensions) > 0 && filePath != "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		found := false
		for _, allowed := range cond.Extensions {
			if strings.EqualFold(ext, allowed) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check path prefix
	if len(cond.Paths) > 0 && filePath != "" {
		found := false
		for _, p := range cond.Paths {
			if strings.HasPrefix(filePath, p) || matchGlob(filePath, p) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check ignore patterns
	if len(cond.Ignore) > 0 && filePath != "" {
		for _, pattern := range cond.Ignore {
			if matchGlob(filepath.Base(filePath), pattern) {
				return false
			}
		}
	}

	return true
}

func (m *Manager) matchPR(cond *PRCondition, event TriggerEvent) bool {
	if cond == nil {
		return true
	}

	// Check PR event type
	if len(cond.Events) > 0 {
		prEvent, _ := event.Payload["pr_event"].(string)
		found := false
		for _, e := range cond.Events {
			if strings.EqualFold(e, prEvent) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check branch
	if len(cond.Branches) > 0 {
		branch, _ := event.Payload["branch"].(string)
		found := false
		for _, b := range cond.Branches {
			if matchGlob(branch, b) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check labels
	if len(cond.Labels) > 0 {
		rawLabels, _ := event.Payload["labels"].([]interface{})
		eventLabels := make(map[string]bool)
		for _, l := range rawLabels {
			if s, ok := l.(string); ok {
				eventLabels[s] = true
			}
		}
		for _, required := range cond.Labels {
			if !eventLabels[required] {
				return false
			}
		}
	}

	return true
}

func (m *Manager) matchWebhook(cond *WebhookCondition, event TriggerEvent) bool {
	if cond == nil {
		return true
	}

	path, _ := event.Payload["path"].(string)
	if cond.Path != "" && path != cond.Path {
		return false
	}

	if len(cond.Methods) > 0 {
		method, _ := event.Payload["method"].(string)
		found := false
		for _, m := range cond.Methods {
			if strings.EqualFold(m, method) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// execute runs a trigger's action and records the execution.
func (m *Manager) execute(ctx context.Context, t *Trigger, event TriggerEvent) (ExecutionRecord, error) {
	rec := ExecutionRecord{
		ID:          fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		TriggerID:   t.ID,
		TriggerName: t.Name,
		Event:       event,
		Status:      "started",
		StartedAt:   time.Now().UTC(),
	}

	var execErr error
	var output string

	if m.runner != nil {
		// Merge trigger args with event context
		args := make(map[string]string)
		for k, v := range t.Action.Args {
			args[k] = v
		}
		// Add event context
		args["_trigger_id"] = t.ID
		args["_trigger_name"] = t.Name
		args["_event_type"] = string(event.Type)

		output, execErr = m.runner.Run(ctx, t.Action.Pipeline, t.Action.Agent, t.Action.Model, args, t.Action.Env)
	} else {
		execErr = fmt.Errorf("no pipeline runner configured")
	}

	now := time.Now().UTC()
	rec.FinishedAt = &now
	rec.PipelineOutput = output

	if execErr != nil {
		rec.Status = "failed"
		rec.Error = execErr.Error()
	} else {
		rec.Status = "completed"
	}

	// Update trigger stats
	m.mu.Lock()
	t.FireCount++
	t.LastFiredAt = &now
	m.history = append(m.history, rec)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
	m.save()
	m.mu.Unlock()

	return rec, execErr
}

// History returns execution history, optionally filtered by trigger ID.
func (m *Manager) History(triggerID string, limit int) []ExecutionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ExecutionRecord
	for i := len(m.history) - 1; i >= 0; i-- {
		rec := m.history[i]
		if triggerID != "" && rec.TriggerID != triggerID {
			continue
		}
		result = append(result, rec)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// UpdateOption is a functional option for updating triggers.
type UpdateOption func(*Trigger)

// WithEnabled sets the enabled state.
func WithEnabled(enabled bool) UpdateOption {
	return func(t *Trigger) { t.Enabled = enabled }
}

// WithName updates the name.
func WithName(name string) UpdateOption {
	return func(t *Trigger) { t.Name = name }
}

// WithDescription updates the description.
func WithDescription(desc string) UpdateOption {
	return func(t *Trigger) { t.Description = desc }
}

// WithCondition updates the condition.
func WithCondition(cond Condition) UpdateOption {
	return func(t *Trigger) { t.Condition = cond }
}

// WithAction updates the action.
func WithAction(action Action) UpdateOption {
	return func(t *Trigger) { t.Action = action }
}

// Stats returns trigger statistics.
type Stats struct {
	TotalTriggers   int            `json:"total_triggers"`
	EnabledTriggers int            `json:"enabled_triggers"`
	ByType          map[TriggerType]int `json:"by_type"`
	TotalExecutions int            `json:"total_executions"`
	FailedExecutions int           `json:"failed_executions"`
	LastExecution   *time.Time     `json:"last_execution,omitempty"`
}

// Stats returns aggregate statistics.
func (m *Manager) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s := Stats{
		ByType: make(map[TriggerType]int),
	}

	for _, t := range m.triggers {
		s.TotalTriggers++
		if t.Enabled {
			s.EnabledTriggers++
		}
		s.ByType[t.Type]++
	}

	s.TotalExecutions = len(m.history)
	for _, rec := range m.history {
		if rec.Status == "failed" {
			s.FailedExecutions++
		}
	}
	if len(m.history) > 0 {
		last := m.history[len(m.history)-1].StartedAt
		s.LastExecution = &last
	}

	return s
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.dir, "triggers.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.triggers)

	hdata, err := os.ReadFile(filepath.Join(m.dir, "history.json"))
	if err == nil {
		json.Unmarshal(hdata, &m.history)
	}
}

func (m *Manager) save() {
	data, _ := json.MarshalIndent(m.triggers, "", "  ")
	os.WriteFile(filepath.Join(m.dir, "triggers.json"), data, 0644)

	hdata, _ := json.MarshalIndent(m.history, "", "  ")
	os.WriteFile(filepath.Join(m.dir, "history.json"), hdata, 0644)
}

// matchGlob does simple glob matching (only supports * wildcard).
func matchGlob(name, pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return name == pattern
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		return strings.HasPrefix(name, parts[0]) && strings.HasSuffix(name, parts[1])
	}

	// Simple wildcard: just check prefix and suffix
	return strings.HasPrefix(name, parts[0])
}

// FormatTrigger formats a trigger for display.
func FormatTrigger(t *Trigger) string {
	var out string
	status := "enabled"
	if !t.Enabled {
		status = "disabled"
	}
	out += fmt.Sprintf("  %s [%s] (%s)\n", t.Name, t.Type, status)
	if t.Description != "" {
		out += fmt.Sprintf("    %s\n", t.Description)
	}
	out += fmt.Sprintf("    Action: pipeline=%s", t.Action.Pipeline)
	if t.Action.Agent != "" {
		out += fmt.Sprintf(" agent=%s", t.Action.Agent)
	}
	out += "\n"
	out += fmt.Sprintf("    Fires: %d | Last: ", t.FireCount)
	if t.LastFiredAt != nil {
		out += t.LastFiredAt.Format("2006-01-02 15:04:05")
	} else {
		out += "never"
	}
	out += "\n"
	return out
}

// FormatHistory formats an execution record for display.
func FormatHistory(rec ExecutionRecord) string {
	var out string
	out += fmt.Sprintf("  [%s] %s → %s", rec.Status, rec.TriggerName, rec.Event.Type)
	if rec.FinishedAt != nil {
		out += fmt.Sprintf(" (%s)", rec.FinishedAt.Sub(rec.StartedAt).Round(time.Millisecond))
	}
	if rec.Error != "" {
		out += fmt.Sprintf("\n    Error: %s", rec.Error)
	}
	out += "\n"
	return out
}
