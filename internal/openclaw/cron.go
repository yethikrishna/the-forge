// Package openclaw provides cron scheduling backed by the OpenClaw cron system.
// Forge org uses OpenClaw cron for agent scheduling — periodic checks, reminders,
// heartbeat polls, and division standups.
package openclaw

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CronSchedule represents a cron expression and its metadata.
type CronSchedule struct {
	ID          string            `json:"id"`
	Expression  string            `json:"expression"`  // cron expression like "*/5 * * * *"
	Task        string            `json:"task"`        // what to run
	Label       string            `json:"label"`       // human-readable name
	Division    string            `json:"division"`    // which division owns this cron
	AgentID     string            `json:"agent_id"`    // which agent runs this
	Enabled     bool              `json:"enabled"`
	LastRun     time.Time         `json:"last_run"`
	NextRun     time.Time         `json:"next_run"`
	RunCount    int64             `json:"run_count"`
	Metadata    map[string]string `json:"metadata"`
}

// CronEntry is a cron job to create or update.
type CronEntry struct {
	Expression string `json:"expression"`
	Task       string `json:"task"`
	Label      string `json:"label"`
	Division   string `json:"division"`
	AgentID    string `json:"agent_id"`
}

// CronManager manages cron jobs via the OpenClaw runtime.
type CronManager struct {
	bridge *Bridge
	mu     sync.RWMutex
	jobs   map[string]*CronSchedule
}

// NewCronManager creates a new cron manager backed by the bridge.
func NewCronManager(bridge *Bridge) *CronManager {
	return &CronManager{
		bridge: bridge,
		jobs:   make(map[string]*CronSchedule),
	}
}

// Create schedules a new cron job in the OpenClaw runtime.
func (cm *CronManager) Create(ctx context.Context, entry CronEntry) (*CronSchedule, error) {
	if entry.Expression == "" {
		return nil, fmt.Errorf("cron expression is required")
	}
	if entry.Task == "" {
		return nil, fmt.Errorf("cron task is required")
	}

	// Generate a stable ID from label or expression
	id := entry.Label
	if id == "" {
		id = fmt.Sprintf("cron-%d", time.Now().UnixNano())
	}

	schedule := &CronSchedule{
		ID:         id,
		Expression: entry.Expression,
		Task:       entry.Task,
		Label:      entry.Label,
		Division:   entry.Division,
		AgentID:    entry.AgentID,
		Enabled:    true,
		Metadata:   make(map[string]string),
	}

	// Register with OpenClaw gateway
	payload := map[string]interface{}{
		"id":        schedule.ID,
		"expression": schedule.Expression,
		"task":      schedule.Task,
		"label":     schedule.Label,
		"division":  schedule.Division,
		"agentId":   schedule.AgentID,
		"enabled":   true,
	}
	var result map[string]interface{}
	if err := cm.bridge.PostJSON(ctx, "/api/cron", payload, &result); err != nil {
		// Store locally even if gateway is down — will sync when available
		schedule.Metadata["sync_pending"] = "true"
	}

	// Parse next run time from result if available
	if nextStr, ok := result["next_run"].(string); ok {
		if t, err := time.Parse(time.RFC3339, nextStr); err == nil {
			schedule.NextRun = t
		}
	}

	cm.mu.Lock()
	cm.jobs[schedule.ID] = schedule
	cm.mu.Unlock()

	return schedule, nil
}

// Get retrieves a cron job by ID.
func (cm *CronManager) Get(ctx context.Context, id string) (*CronSchedule, error) {
	cm.mu.RLock()
	if job, ok := cm.jobs[id]; ok {
		cm.mu.RUnlock()
		return job, nil
	}
	cm.mu.RUnlock()

	// Try gateway
	var schedule CronSchedule
	if err := cm.bridge.GetJSON(ctx, "/api/cron/"+id, &schedule); err != nil {
		return nil, fmt.Errorf("cron job %s not found: %w", id, err)
	}
	cm.mu.Lock()
	cm.jobs[schedule.ID] = &schedule
	cm.mu.Unlock()
	return &schedule, nil
}

// List returns all cron jobs, optionally filtered by division.
func (cm *CronManager) List(ctx context.Context, division string) ([]*CronSchedule, error) {
	path := "/api/cron"
	if division != "" {
		path = fmt.Sprintf("/api/cron?division=%s", division)
	}

	var schedules []*CronSchedule
	if err := cm.bridge.GetJSON(ctx, path, &schedules); err != nil {
		// Fall back to local cache
		cm.mu.RLock()
		for _, job := range cm.jobs {
			if division == "" || job.Division == division {
				schedules = append(schedules, job)
			}
		}
		cm.mu.RUnlock()
		return schedules, nil
	}

	cm.mu.Lock()
	for _, s := range schedules {
		cm.jobs[s.ID] = s
	}
	cm.mu.Unlock()

	return schedules, nil
}

// Update modifies an existing cron job.
func (cm *CronManager) Update(ctx context.Context, id string, entry CronEntry) (*CronSchedule, error) {
	cm.mu.Lock()
	job, ok := cm.jobs[id]
	if !ok {
		cm.mu.Unlock()
		return nil, fmt.Errorf("cron job %s not found", id)
	}
	cm.mu.Unlock()

	if entry.Expression != "" {
		job.Expression = entry.Expression
	}
	if entry.Task != "" {
		job.Task = entry.Task
	}
	if entry.Label != "" {
		job.Label = entry.Label
	}
	if entry.Division != "" {
		job.Division = entry.Division
	}
	if entry.AgentID != "" {
		job.AgentID = entry.AgentID
	}

	payload := map[string]interface{}{
		"expression": job.Expression,
		"task":       job.Task,
		"label":      job.Label,
		"division":   job.Division,
		"agentId":    job.AgentID,
	}
	if err := cm.bridge.PatchJSON(ctx, "/api/cron/"+id, payload); err != nil {
		job.Metadata["sync_pending"] = "true"
	}

	return job, nil
}

// Delete removes a cron job.
func (cm *CronManager) Delete(ctx context.Context, id string) error {
	if err := cm.bridge.Delete(ctx, "/api/cron/"+id); err != nil {
		return fmt.Errorf("delete cron job %s: %w", id, err)
	}
	cm.mu.Lock()
	delete(cm.jobs, id)
	cm.mu.Unlock()
	return nil
}

// Disable pauses a cron job without removing it.
func (cm *CronManager) Disable(ctx context.Context, id string) error {
	cm.mu.Lock()
	job, ok := cm.jobs[id]
	if !ok {
		cm.mu.Unlock()
		return fmt.Errorf("cron job %s not found", id)
	}
	job.Enabled = false
	cm.mu.Unlock()

	return cm.bridge.PatchJSON(ctx, "/api/cron/"+id, map[string]interface{}{"enabled": false})
}

// Enable resumes a paused cron job.
func (cm *CronManager) Enable(ctx context.Context, id string) error {
	cm.mu.Lock()
	job, ok := cm.jobs[id]
	if !ok {
		cm.mu.Unlock()
		return fmt.Errorf("cron job %s not found", id)
	}
	job.Enabled = true
	cm.mu.Unlock()

	return cm.bridge.PatchJSON(ctx, "/api/cron/"+id, map[string]interface{}{"enabled": true})
}

// RecordRun updates the last run time for a cron job.
func (cm *CronManager) RecordRun(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if job, ok := cm.jobs[id]; ok {
		job.LastRun = time.Now()
		job.RunCount++
	}
}
