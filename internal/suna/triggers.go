// Package suna provides trigger mapping between Suna's cron/webhook system
// and Forge org events. Suna triggers are mapped to Forge division events,
// allowing the org to react to external schedules, webhooks, and timers.
package suna

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TriggerType represents the type of trigger.
type TriggerType string

const (
	TriggerCron    TriggerType = "cron"
	TriggerWebhook TriggerType = "webhook"
	TriggerEv      TriggerType = "event"
	TriggerManual  TriggerType = "manual"
)

// TriggerState represents the state of a trigger.
type TriggerState string

const (
	TriggerActive   TriggerState = "active"
	TriggerPaused   TriggerState = "paused"
	TriggerError    TriggerState = "error"
	TriggerDisabled TriggerState = "disabled"
)

// Trigger represents a mapped trigger from Suna to Forge.
type Trigger struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        TriggerType       `json:"type"`
	State       TriggerState      `json:"state"`
	Division    string            `json:"division"`
	AgentID     string            `json:"agent_id"`
	Schedule    string            `json:"schedule,omitempty"` // cron expression for cron type
	WebhookPath string            `json:"webhook_path,omitempty"`
	EventTopic  string            `json:"event_topic,omitempty"`
	Task        string            `json:"task"` // what to do when triggered
	LastFired   time.Time         `json:"last_fired"`
	FireCount   int64             `json:"fire_count"`
	ErrorCount  int64             `json:"error_count"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
}

// TriggerConfig is the input for creating a trigger.
type TriggerConfig struct {
	Name        string            `json:"name"`
	Type        TriggerType       `json:"type"`
	Division    string            `json:"division"`
	AgentID     string            `json:"agent_id"`
	Schedule    string            `json:"schedule,omitempty"`
	WebhookPath string            `json:"webhook_path,omitempty"`
	EventTopic  string            `json:"event_topic,omitempty"`
	Task        string            `json:"task"`
	Labels      map[string]string `json:"labels"`
}

// TriggerEvent represents an event fired by a trigger.
type TriggerEvent struct {
	TriggerID  string                 `json:"trigger_id"`
	TriggerName string               `json:"trigger_name"`
	FiredAt    time.Time              `json:"fired_at"`
	Payload    map[string]interface{} `json:"payload"`
	Result     string                 `json:"result,omitempty"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
}

// TriggerHandler is called when a trigger fires.
type TriggerHandler func(ctx context.Context, event TriggerEvent) error

// TriggerManager manages triggers mapped from Suna to Forge.
type TriggerManager struct {
	bridge   *Bridge
	mu       sync.RWMutex
	triggers map[string]*Trigger
	handlers map[string]TriggerHandler
}

// NewTriggerManager creates a new trigger manager.
func NewTriggerManager(bridge *Bridge) *TriggerManager {
	return &TriggerManager{
		bridge:   bridge,
		triggers: make(map[string]*Trigger),
		handlers: make(map[string]TriggerHandler),
	}
}

// Create sets up a new trigger.
func (tm *TriggerManager) Create(ctx context.Context, cfg TriggerConfig) (*Trigger, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("trigger name is required")
	}
	if cfg.Task == "" {
		return nil, fmt.Errorf("trigger task is required")
	}
	if cfg.Type == TriggerCron && cfg.Schedule == "" {
		return nil, fmt.Errorf("cron trigger requires a schedule")
	}

	trigger := &Trigger{
		ID:          fmt.Sprintf("trig-%d", time.Now().UnixNano()),
		Name:        cfg.Name,
		Type:        cfg.Type,
		State:       TriggerActive,
		Division:    cfg.Division,
		AgentID:     cfg.AgentID,
		Schedule:    cfg.Schedule,
		WebhookPath: cfg.WebhookPath,
		EventTopic:  cfg.EventTopic,
		Task:        cfg.Task,
		Labels:      cfg.Labels,
		CreatedAt:   time.Now(),
	}

	// Register with Suna API
	payload := map[string]interface{}{
		"name":        trigger.Name,
		"type":        trigger.Type,
		"division":    trigger.Division,
		"agentId":     trigger.AgentID,
		"schedule":    trigger.Schedule,
		"webhookPath": trigger.WebhookPath,
		"eventTopic":  trigger.EventTopic,
		"task":        trigger.Task,
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := tm.bridge.PostJSON(ctx, "/api/triggers", payload, &result); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}
	if result.ID != "" {
		trigger.ID = result.ID
	}

	tm.mu.Lock()
	tm.triggers[trigger.ID] = trigger
	tm.mu.Unlock()

	return trigger, nil
}

// Get retrieves a trigger by ID.
func (tm *TriggerManager) Get(id string) (*Trigger, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.triggers[id]
	if !ok {
		return nil, fmt.Errorf("trigger %s not found", id)
	}
	return t, nil
}

// List returns all triggers, optionally filtered by type or division.
func (tm *TriggerManager) List(trigType TriggerType, division string) []*Trigger {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []*Trigger
	for _, t := range tm.triggers {
		if trigType != "" && t.Type != trigType {
			continue
		}
		if division != "" && t.Division != "" && t.Division != division {
			continue
		}
		result = append(result, t)
	}
	return result
}

// RegisterHandler registers a handler for when a trigger fires.
func (tm *TriggerManager) RegisterHandler(triggerID string, handler TriggerHandler) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.handlers[triggerID] = handler
}

// Fire handles a trigger event, executing the registered handler.
func (tm *TriggerManager) Fire(ctx context.Context, event TriggerEvent) error {
	tm.mu.RLock()
	handler, ok := tm.handlers[event.TriggerID]
	trigger, tok := tm.triggers[event.TriggerID]
	tm.mu.RUnlock()

	if !tok {
		return fmt.Errorf("trigger %s not found", event.TriggerID)
	}

	start := time.Now()
	event.FiredAt = start

	if ok && handler != nil {
		if err := handler(ctx, event); err != nil {
			trigger.ErrorCount++
			event.Error = err.Error()
			event.Success = false
			return err
		}
	}

	event.Success = true
	event.Duration = time.Since(start)
	trigger.LastFired = start
	trigger.FireCount++

	return nil
}

// Pause temporarily disables a trigger.
func (tm *TriggerManager) Pause(ctx context.Context, id string) error {
	tm.mu.RLock()
	t, ok := tm.triggers[id]
	tm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("trigger %s not found", id)
	}
	t.State = TriggerPaused
	return tm.bridge.PatchJSON(ctx, "/api/triggers/"+id, map[string]interface{}{"state": "paused"})
}

// Resume re-enables a paused trigger.
func (tm *TriggerManager) Resume(ctx context.Context, id string) error {
	tm.mu.RLock()
	t, ok := tm.triggers[id]
	tm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("trigger %s not found", id)
	}
	t.State = TriggerActive
	return tm.bridge.PatchJSON(ctx, "/api/triggers/"+id, map[string]interface{}{"state": "active"})
}

// Delete removes a trigger.
func (tm *TriggerManager) Delete(ctx context.Context, id string) error {
	if err := tm.bridge.DeleteJSON(ctx, "/api/triggers/"+id); err != nil {
		return fmt.Errorf("delete trigger %s: %w", id, err)
	}
	tm.mu.Lock()
	delete(tm.triggers, id)
	delete(tm.handlers, id)
	tm.mu.Unlock()
	return nil
}

// PatchJSON performs a PATCH request against the Suna API.
func (b *Bridge) PatchJSON(ctx context.Context, path string, body interface{}) error {
	resp, err := b.doRequest(ctx, "PATCH", path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("suna API %s: HTTP %d", path, resp.StatusCode)
	}
	return nil
}
