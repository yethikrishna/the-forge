// Package integration provides calendar integration for scheduling.
package integration

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CalendarEvent represents a calendar event.
type CalendarEvent struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Location    string            `json:"location,omitempty"`
	Start       time.Time         `json:"start"`
	End         time.Time         `json:"end"`
	AllDay      bool              `json:"all_day"`
	Attendees   []string          `json:"attendees,omitempty"`
	Calendar    string            `json:"calendar,omitempty"`
	Color       string            `json:"color,omitempty"`
	Reminders   []time.Duration   `json:"reminders,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CalendarProvider identifies the calendar service.
type CalendarProvider string

const (
	CalendarGoogle    CalendarProvider = "google"
	CalendarOutlook   CalendarProvider = "outlook"
	CalendarCalDAV    CalendarProvider = "caldav"
	CalendarLocal     CalendarProvider = "local"
)

// CalendarConfig configures calendar integration.
type CalendarConfig struct {
	Provider     CalendarProvider `json:"provider"`
	DefaultCal   string           `json:"default_calendar"`
	Timezone     string           `json:"timezone"`
	ReminderMin  int              `json:"reminder_min"`
}

// CalendarManager handles calendar operations.
type CalendarManager struct {
	config CalendarConfig
	events map[string]*CalendarEvent
	mu     sync.RWMutex
}

// NewCalendarManager creates a calendar manager.
func NewCalendarManager(config CalendarConfig) *CalendarManager {
	if config.DefaultCal == "" {
		config.DefaultCal = "primary"
	}
	if config.Timezone == "" {
		config.Timezone = "UTC"
	}
	if config.ReminderMin == 0 {
		config.ReminderMin = 15
	}
	return &CalendarManager{
		config: config,
		events: make(map[string]*CalendarEvent),
	}
}

// CreateEvent creates a new calendar event.
func (cm *CalendarManager) CreateEvent(title string, start, end time.Time) (*CalendarEvent, error) {
	if title == "" {
		return nil, fmt.Errorf("calendar: title required")
	}
	if end.Before(start) {
		return nil, fmt.Errorf("calendar: end must be after start")
	}

	event := &CalendarEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Title:     title,
		Start:     start,
		End:       end,
		Calendar:  cm.config.DefaultCal,
		Reminders: []time.Duration{time.Duration(cm.config.ReminderMin) * time.Minute},
	}

	cm.mu.Lock()
	cm.events[event.ID] = event
	cm.mu.Unlock()

	return event, nil
}

// CreateAllDay creates an all-day event.
func (cm *CalendarManager) CreateAllDay(title string, date time.Time) (*CalendarEvent, error) {
	event, err := cm.CreateEvent(title, date, date.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	event.AllDay = true
	return event, nil
}

// UpdateEvent updates an existing event.
func (cm *CalendarManager) UpdateEvent(id string, updates CalendarEvent) (*CalendarEvent, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	event, ok := cm.events[id]
	if !ok {
		return nil, fmt.Errorf("calendar: event %s not found", id)
	}

	if updates.Title != "" {
		event.Title = updates.Title
	}
	if updates.Description != "" {
		event.Description = updates.Description
	}
	if updates.Location != "" {
		event.Location = updates.Location
	}
	if !updates.Start.IsZero() {
		event.Start = updates.Start
	}
	if !updates.End.IsZero() {
		event.End = updates.End
	}
	if updates.Attendees != nil {
		event.Attendees = updates.Attendees
	}

	return event, nil
}

// DeleteEvent removes an event.
func (cm *CalendarManager) DeleteEvent(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, ok := cm.events[id]; !ok {
		return fmt.Errorf("calendar: event %s not found", id)
	}
	delete(cm.events, id)
	return nil
}

// GetEvent retrieves an event by ID.
func (cm *CalendarManager) GetEvent(id string) (*CalendarEvent, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	event, ok := cm.events[id]
	if !ok {
		return nil, fmt.Errorf("calendar: event %s not found", id)
	}
	return event, nil
}

// ListEvents returns events in a time range.
func (cm *CalendarManager) ListEvents(from, to time.Time) []*CalendarEvent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*CalendarEvent
	for _, e := range cm.events {
		if (from.IsZero() || !e.End.Before(from)) && (to.IsZero() || !e.Start.After(to)) {
			result = append(result, e)
		}
	}
	return result
}

// Upcoming returns events in the next N hours.
func (cm *CalendarManager) Upcoming(hours int) []*CalendarEvent {
	now := time.Now()
	return cm.ListEvents(now, now.Add(time.Duration(hours)*time.Hour))
}

// Conflicts returns events that overlap with the given time range.
func (cm *CalendarManager) Conflicts(start, end time.Time) []*CalendarEvent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var conflicts []*CalendarEvent
	for _, e := range cm.events {
		if e.Start.Before(end) && e.End.After(start) {
			conflicts = append(conflicts, e)
		}
	}
	return conflicts
}

// FreeSlots returns available time slots in a range.
func (cm *CalendarManager) FreeSlots(from, to time.Time, duration time.Duration) []time.Time {
	events := cm.ListEvents(from, to)

	// Sort events by start time
	sorted := make([]*CalendarEvent, len(events))
	copy(sorted, events)

	var slots []time.Time
	cursor := from
	for _, e := range sorted {
		if cursor.Add(duration).Before(e.Start) || cursor.Add(duration).Equal(e.Start) {
			slots = append(slots, cursor)
		}
		if e.End.After(cursor) {
			cursor = e.End
		}
	}
	// Check remaining time
	if cursor.Add(duration).Before(to) || cursor.Add(duration).Equal(to) {
		slots = append(slots, cursor)
	}

	return slots
}

var _ = json.Marshal
