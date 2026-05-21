// Package schedule provides cron-like scheduling for forge agents.
// The forge strikes at appointed hours, unbidden.
package schedule

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

// Schedule defines when and what to run.
type Schedule struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Cron      string            `json:"cron"`  // cron expression
	Agent     string            `json:"agent"` // agent name or ID
	Task      string            `json:"task"`  // task to execute
	Args      map[string]string `json:"args,omitempty"`
	Enabled   bool              `json:"enabled"`
	LastRun   time.Time         `json:"last_run,omitempty"`
	NextRun   time.Time         `json:"next_run,omitempty"`
	RunCount  int               `json:"run_count"`
	CreatedAt time.Time         `json:"created_at"`
	Tags      []string          `json:"tags,omitempty"`
}

// RunLog records a schedule execution.
type RunLog struct {
	ScheduleID string    `json:"schedule_id"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Status     string    `json:"status"` // success, error, timeout
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// Scheduler manages scheduled tasks.
type Scheduler struct {
	schedules map[string]*Schedule
	runLogs   []RunLog
	store     string
	mu        sync.RWMutex
	running   bool
	cancel    context.CancelFunc
	onRun     func(ctx context.Context, sched *Schedule) (string, error)
}

// NewScheduler creates a new scheduler.
func NewScheduler(storePath string, onRun func(ctx context.Context, sched *Schedule) (string, error)) *Scheduler {
	s := &Scheduler{
		schedules: make(map[string]*Schedule),
		runLogs:   make([]RunLog, 0),
		store:     storePath,
		onRun:     onRun,
	}
	s.load()
	return s
}

// Add creates a new schedule.
func (s *Scheduler) Add(name, cronExpr, agent, task string, opts ...ScheduleOption) (*Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextRun, err := ParseCron(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron: %w", err)
	}

	sched := &Schedule{
		ID:        fmt.Sprintf("sched-%d", time.Now().UnixNano()),
		Name:      name,
		Cron:      cronExpr,
		Agent:     agent,
		Task:      task,
		Enabled:   true,
		NextRun:   nextRun,
		CreatedAt: time.Now().UTC(),
	}

	for _, o := range opts {
		o(sched)
	}

	s.schedules[sched.ID] = sched
	s.save()

	return sched, nil
}

// Get retrieves a schedule by ID.
func (s *Scheduler) Get(id string) (*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sched, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	return sched, nil
}

// List returns all schedules.
func (s *Scheduler) List() []*Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Schedule, 0, len(s.schedules))
	for _, sched := range s.schedules {
		result = append(result, sched)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NextRun.Before(result[j].NextRun)
	})
	return result
}

// Update modifies a schedule.
func (s *Scheduler) Update(id string, opts ...ScheduleOption) (*Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sched, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	for _, o := range opts {
		o(sched)
	}

	if sched.Cron != "" {
		nextRun, err := ParseCron(sched.Cron)
		if err != nil {
			return nil, fmt.Errorf("invalid cron: %w", err)
		}
		sched.NextRun = nextRun
	}

	s.save()
	return sched, nil
}

// Delete removes a schedule.
func (s *Scheduler) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	delete(s.schedules, id)
	s.save()
	return nil
}

// Enable enables a schedule.
func (s *Scheduler) Enable(id string) error {
	_, err := s.Update(id, func(sc *Schedule) { sc.Enabled = true })
	return err
}

// Disable disables a schedule.
func (s *Scheduler) Disable(id string) error {
	_, err := s.Update(id, func(sc *Schedule) { sc.Enabled = false })
	return err
}

// RunNow executes a schedule immediately.
func (s *Scheduler) RunNow(ctx context.Context, id string) (*RunLog, error) {
	s.mu.RLock()
	sched, ok := s.schedules[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	return s.execute(ctx, sched)
}

// Start begins the scheduling loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	ctx, s.cancel = context.WithCancel(ctx)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// IsRunning returns whether the scheduler is active.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Logs returns recent run logs.
func (s *Scheduler) Logs(limit int) []RunLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.runLogs) > limit {
		return s.runLogs[len(s.runLogs)-limit:]
	}
	return s.runLogs
}

func (s *Scheduler) tick(ctx context.Context) {
	s.mu.RLock()
	now := time.Now().UTC()
	var due []*Schedule
	for _, sched := range s.schedules {
		if sched.Enabled && !sched.NextRun.IsZero() && !sched.NextRun.After(now) {
			due = append(due, sched)
		}
	}
	s.mu.RUnlock()

	for _, sched := range due {
		s.execute(ctx, sched)
	}
}

func (s *Scheduler) execute(ctx context.Context, sched *Schedule) (*RunLog, error) {
	log := RunLog{
		ScheduleID: sched.ID,
		StartedAt:  time.Now().UTC(),
	}

	var output string
	var err error

	if s.onRun != nil {
		output, err = s.onRun(ctx, sched)
	}

	log.FinishedAt = time.Now().UTC()
	if err != nil {
		log.Status = "error"
		log.Error = err.Error()
	} else {
		log.Status = "success"
		log.Output = output
	}

	s.mu.Lock()
	sched.LastRun = log.StartedAt
	sched.RunCount++
	nextRun, _ := ParseCron(sched.Cron)
	sched.NextRun = nextRun
	s.runLogs = append(s.runLogs, log)
	if len(s.runLogs) > 1000 {
		s.runLogs = s.runLogs[len(s.runLogs)-1000:]
	}
	s.save()
	s.mu.Unlock()

	return &log, err
}

func (s *Scheduler) load() {
	if s.store == "" {
		return
	}
	data, err := os.ReadFile(s.store)
	if err != nil {
		return
	}

	var store struct {
		Schedules map[string]*Schedule `json:"schedules"`
		RunLogs   []RunLog             `json:"run_logs"`
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return
	}

	if store.Schedules != nil {
		s.schedules = store.Schedules
	}
	if store.RunLogs != nil {
		s.runLogs = store.RunLogs
	}
}

func (s *Scheduler) save() {
	if s.store == "" {
		return
	}

	store := struct {
		Schedules map[string]*Schedule `json:"schedules"`
		RunLogs   []RunLog             `json:"run_logs"`
	}{
		Schedules: s.schedules,
		RunLogs:   s.runLogs,
	}

	data, _ := json.MarshalIndent(store, "", "  ")
	dir := filepath.Dir(s.store)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(s.store, data, 0o644)
}

// ParseCron parses a simplified cron expression and returns the next run time.
// Supports: @every <duration>, @hourly, @daily, @weekly, and basic 5-field cron.
func ParseCron(expr string) (time.Time, error) {
	expr = strings.TrimSpace(expr)
	now := time.Now().UTC()

	// Named schedules
	switch expr {
	case "@hourly":
		return now.Truncate(time.Hour).Add(time.Hour), nil
	case "@daily":
		return now.AddDate(0, 0, 1).Truncate(24 * time.Hour), nil
	case "@weekly":
		return now.AddDate(0, 0, 7).Truncate(24 * time.Hour), nil
	case "@monthly":
		return now.AddDate(0, 1, 0).Truncate(24 * time.Hour), nil
	case "@yearly", "@annually":
		return now.AddDate(1, 0, 0).Truncate(24 * time.Hour), nil
	}

	// @every duration
	if strings.HasPrefix(expr, "@every ") {
		dur, err := time.ParseDuration(strings.TrimPrefix(expr, "@every "))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration: %w", err)
		}
		return now.Add(dur), nil
	}

	// Simple 5-field cron (minute hour day month weekday)
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("expected 5 cron fields, got %d", len(fields))
	}

	// Simplified: just compute next occurrence
	// For now, support basic patterns
	minute := fields[0]
	hour := fields[1]

	// Handle specific minute/hour
	if minute == "*" && hour == "*" {
		return now.Add(time.Minute), nil
	}

	// Parse hour and minute
	targetHour := now.Hour()
	targetMinute := now.Minute()

	if hour != "*" {
		fmt.Sscanf(hour, "%d", &targetHour)
	}
	if minute != "*" {
		fmt.Sscanf(minute, "%d", &targetMinute)
	}

	next := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	return next, nil
}

// ScheduleOption customizes a schedule.
type ScheduleOption func(*Schedule)

// WithArgs sets schedule arguments.
func WithArgs(args map[string]string) ScheduleOption {
	return func(s *Schedule) { s.Args = args }
}

// WithTags sets schedule tags.
func WithTags(tags ...string) ScheduleOption {
	return func(s *Schedule) { s.Tags = tags }
}

// WithEnabled sets enabled state.
func WithEnabled(enabled bool) ScheduleOption {
	return func(s *Schedule) { s.Enabled = enabled }
}

// FormatSchedule formats a schedule for display.
func FormatSchedule(s *Schedule) string {
	status := "enabled"
	if !s.Enabled {
		status = "disabled"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %-15s %s\n", "ID:", s.ID))
	b.WriteString(fmt.Sprintf("  %-15s %s\n", "Name:", s.Name))
	b.WriteString(fmt.Sprintf("  %-15s %s\n", "Cron:", s.Cron))
	b.WriteString(fmt.Sprintf("  %-15s %s\n", "Agent:", s.Agent))
	b.WriteString(fmt.Sprintf("  %-15s %s\n", "Task:", s.Task))
	b.WriteString(fmt.Sprintf("  %-15s %s\n", "Status:", status))
	b.WriteString(fmt.Sprintf("  %-15s %s\n", "Next Run:", s.NextRun.Format("2006-01-02 15:04:05")))
	if !s.LastRun.IsZero() {
		b.WriteString(fmt.Sprintf("  %-15s %s\n", "Last Run:", s.LastRun.Format("2006-01-02 15:04:05")))
	}
	b.WriteString(fmt.Sprintf("  %-15s %d\n", "Run Count:", s.RunCount))
	return b.String()
}
