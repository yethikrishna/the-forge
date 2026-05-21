// Package comm provides inter-agent communication infrastructure.
// Division channels, DMs, broadcasts, activity feed, multi-resolution
// reports, and alerts — the organizational nervous system.
package comm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// MessageType categorizes a communication.
type MessageType string

const (
	MsgChat      MessageType = "chat"
	MsgDM        MessageType = "dm"
	MsgBroadcast MessageType = "broadcast"
	MsgAlert     MessageType = "alert"
	MsgReport    MessageType = "report"
	MsgHandoff   MessageType = "handoff"
	MsgStandup   MessageType = "standup"
	MsgSystem    MessageType = "system"
)

// Priority levels for messages.
type MsgPriority string

const (
	PrioLow      MsgPriority = "low"
	PrioNormal   MsgPriority = "normal"
	PrioHigh     MsgPriority = "high"
	PrioCritical MsgPriority = "critical"
)

// Channel represents a communication channel (division-wide or cross-org).
type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DivisionID  string    `json:"division_id,omitempty"` // empty = org-wide
	Type        string    `json:"type"`                  // division, org, project, adhoc
	Members     []string  `json:"members"`               // agent IDs
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
}

// Message is a single communication.
type Message struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	ChannelID string      `json:"channel_id,omitempty"`
	From      string      `json:"from"` // agent ID
	To        []string    `json:"to,omitempty"` // recipient agent IDs (empty = channel)
	Priority  MsgPriority `json:"priority"`
	Subject   string      `json:"subject,omitempty"`
	Body      string      `json:"body"`
	Timestamp time.Time   `json:"timestamp"`
	ReadBy    []string    `json:"read_by,omitempty"`
	ThreadID  string      `json:"thread_id,omitempty"` // for threaded conversations
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ActivityEntry represents an entry in the org-wide activity feed.
type ActivityEntry struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Action    string    `json:"action"` // e.g., "deployed", "committed", "completed_task"
	Target    string    `json:"target,omitempty"`
	Details   string    `json:"details,omitempty"`
	Division  string    `json:"division,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Alert represents a time-sensitive notification.
type Alert struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Body       string      `json:"body"`
	Severity   MsgPriority `json:"severity"`
	Source     string      `json:"source"` // agent or system
	Targets    []string    `json:"targets"` // agent/division IDs
	AcknowledgedBy []string `json:"acknowledged_by,omitempty"`
	Resolved   bool        `json:"resolved"`
	CreatedAt  time.Time   `json:"created_at"`
	ResolvedAt *time.Time  `json:"resolved_at,omitempty"`
}

// ReportResolution defines report detail level.
type ReportResolution string

const (
	ResolutionExecutive ReportResolution = "executive" // 1-2 sentences
	ResolutionSummary   ReportResolution = "summary"   // bullet points
	ResolutionDetailed  ReportResolution = "detailed"  // full context
)

// Report is a generated communication report.
type Report struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Resolution ReportResolution  `json:"resolution"`
	Period     string            `json:"period"` // "daily", "weekly", "adhoc"
	Summary    string            `json:"summary"`
	Sections   []ReportSection   `json:"sections,omitempty"`
	GeneratedAt time.Time        `json:"generated_at"`
	GeneratedBy string           `json:"generated_by"`
}

// ReportSection is a section within a report.
type ReportSection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Comm is the main communication hub.
type Comm struct {
	mu       sync.RWMutex
	channels map[string]*Channel
	messages map[string]*Message
	activity []ActivityEntry
	alerts   map[string]*Alert
	reports  map[string]*Report
	path     string
}

// New creates a new communication hub with optional persistence.
func New(persistPath string) *Comm {
	c := &Comm{
		channels: make(map[string]*Channel),
		messages: make(map[string]*Message),
		activity: make([]ActivityEntry, 0, 1000),
		alerts:   make(map[string]*Alert),
		reports:  make(map[string]*Report),
		path:     persistPath,
	}
	c.load()
	return c
}

// --- Channel Management ---

// CreateChannel creates a new communication channel.
func (c *Comm) CreateChannel(name, divisionID, channelType, description string, members []string) (*Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch := &Channel{
		ID:          genID("ch"),
		Name:        name,
		DivisionID:  divisionID,
		Type:        channelType,
		Members:     members,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		Active:      true,
	}

	c.channels[ch.ID] = ch
	c.persist()
	return ch, nil
}

// GetChannel returns a channel by ID.
func (c *Comm) GetChannel(id string) (*Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ch, ok := c.channels[id]
	if !ok {
		return nil, fmt.Errorf("channel %s not found", id)
	}
	return ch, nil
}

// ListChannels returns channels, optionally filtered by division.
func (c *Comm) ListChannels(divisionID string) []*Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Channel
	for _, ch := range c.channels {
		if (divisionID == "" || ch.DivisionID == divisionID) && ch.Active {
			result = append(result, ch)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// JoinChannel adds an agent to a channel.
func (c *Comm) JoinChannel(channelID, agentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, ok := c.channels[channelID]
	if !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}
	for _, m := range ch.Members {
		if m == agentID {
			return nil // already member
		}
	}
	ch.Members = append(ch.Members, agentID)
	c.persist()
	return nil
}

// --- Messaging ---

// Send sends a message to a channel.
func (c *Comm) Send(from, channelID, body string, msgType MessageType, priority MsgPriority) (*Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := &Message{
		ID:        genID("msg"),
		Type:      msgType,
		ChannelID: channelID,
		From:      from,
		Priority:  priority,
		Body:      body,
		Timestamp: time.Now().UTC(),
		ReadBy:    []string{from},
		Metadata:  make(map[string]string),
	}

	c.messages[msg.ID] = msg

	// Log to activity feed
	c.activity = append(c.activity, ActivityEntry{
		ID:        genID("act"),
		AgentID:   from,
		Action:    "sent_" + string(msgType),
		Target:    channelID,
		Details:   truncate(body, 200),
		Timestamp: time.Now().UTC(),
	})

	c.persist()
	return msg, nil
}

// SendDM sends a direct message between agents.
func (c *Comm) SendDM(from, to, body string, priority MsgPriority) (*Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := &Message{
		ID:        genID("msg"),
		Type:      MsgDM,
		From:      from,
		To:        []string{to},
		Priority:  priority,
		Body:      body,
		Timestamp: time.Now().UTC(),
		ReadBy:    []string{from},
		Metadata:  make(map[string]string),
	}

	c.messages[msg.ID] = msg
	c.persist()
	return msg, nil
}

// Broadcast sends a message to the entire org.
func (c *Comm) Broadcast(from, subject, body string, priority MsgPriority) (*Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := &Message{
		ID:        genID("msg"),
		Type:      MsgBroadcast,
		From:      from,
		Priority:  priority,
		Subject:   subject,
		Body:      body,
		Timestamp: time.Now().UTC(),
		ReadBy:    []string{from},
		Metadata:  make(map[string]string),
	}

	c.messages[msg.ID] = msg

	c.activity = append(c.activity, ActivityEntry{
		ID:        genID("act"),
		AgentID:   from,
		Action:    "broadcast",
		Details:   truncate(subject+": "+body, 200),
		Timestamp: time.Now().UTC(),
	})

	c.persist()
	return msg, nil
}

// ReadMessages returns messages for a channel, ordered by time.
func (c *Comm) ReadMessages(channelID string, limit int) []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Message
	for _, m := range c.messages {
		if m.ChannelID == channelID {
			result = append(result, m)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// ReadDMs returns DMs between two agents.
func (c *Comm) ReadDMs(agent1, agent2 string, limit int) []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Message
	for _, m := range c.messages {
		if m.Type == MsgDM {
			for _, to := range m.To {
				if (m.From == agent1 && to == agent2) || (m.From == agent2 && to == agent1) {
					result = append(result, m)
					break
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// MarkRead marks a message as read by an agent.
func (c *Comm) MarkRead(messageID, agentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	m, ok := c.messages[messageID]
	if !ok {
		return fmt.Errorf("message %s not found", messageID)
	}
	for _, r := range m.ReadBy {
		if r == agentID {
			return nil
		}
	}
	m.ReadBy = append(m.ReadBy, agentID)
	c.persist()
	return nil
}

// UnreadCount returns unread message count for an agent in a channel.
func (c *Comm) UnreadCount(agentID, channelID string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, m := range c.messages {
		if m.ChannelID != channelID {
			continue
		}
		read := false
		for _, r := range m.ReadBy {
			if r == agentID {
				read = true
				break
			}
		}
		if !read {
			count++
		}
	}
	return count
}

// --- Activity Feed ---

// LogActivity adds an entry to the org-wide activity feed.
func (c *Comm) LogActivity(agentID, action, target, details, division string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.activity = append(c.activity, ActivityEntry{
		ID:        genID("act"),
		AgentID:   agentID,
		Action:    action,
		Target:    target,
		Details:   details,
		Division:  division,
		Timestamp: time.Now().UTC(),
	})

	// Keep activity log bounded
	if len(c.activity) > 10000 {
		c.activity = c.activity[len(c.activity)-5000:]
	}
	c.persist()
}

// ActivityFeed returns recent activity entries.
func (c *Comm) ActivityFeed(division string, limit int) []ActivityEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []ActivityEntry
	for i := len(c.activity) - 1; i >= 0; i-- {
		entry := c.activity[i]
		if division == "" || entry.Division == division {
			result = append(result, entry)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// --- Alerts ---

// CreateAlert creates a new alert.
func (c *Comm) CreateAlert(title, body string, severity MsgPriority, source string, targets []string) (*Alert, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	alert := &Alert{
		ID:        genID("alert"),
		Title:     title,
		Body:      body,
		Severity:  severity,
		Source:    source,
		Targets:   targets,
		CreatedAt: time.Now().UTC(),
	}

	c.alerts[alert.ID] = alert
	c.persist()
	return alert, nil
}

// AcknowledgeAlert records that an agent acknowledged an alert.
func (c *Comm) AcknowledgeAlert(alertID, agentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	a, ok := c.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %s not found", alertID)
	}
	for _, ack := range a.AcknowledgedBy {
		if ack == agentID {
			return nil
		}
	}
	a.AcknowledgedBy = append(a.AcknowledgedBy, agentID)
	c.persist()
	return nil
}

// ResolveAlert marks an alert as resolved.
func (c *Comm) ResolveAlert(alertID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	a, ok := c.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %s not found", alertID)
	}
	a.Resolved = true
	now := time.Now().UTC()
	a.ResolvedAt = &now
	c.persist()
	return nil
}

// ListActiveAlerts returns unresolved alerts.
func (c *Comm) ListActiveAlerts(target string) []*Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Alert
	for _, a := range c.alerts {
		if a.Resolved {
			continue
		}
		if target == "" {
			result = append(result, a)
			continue
		}
		for _, t := range a.Targets {
			if t == target {
				result = append(result, a)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// --- Reports ---

// CreateReport generates a communication report.
func (c *Comm) CreateReport(title string, resolution ReportResolution, period, summary string, sections []ReportSection, generatedBy string) (*Report, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	report := &Report{
		ID:          genID("rpt"),
		Title:       title,
		Resolution:  resolution,
		Period:      period,
		Summary:     summary,
		Sections:    sections,
		GeneratedAt: time.Now().UTC(),
		GeneratedBy: generatedBy,
	}

	c.reports[report.ID] = report
	c.persist()
	return report, nil
}

// GetReport returns a report by ID.
func (c *Comm) GetReport(id string) (*Report, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.reports[id]
	if !ok {
		return nil, fmt.Errorf("report %s not found", id)
	}
	return r, nil
}

// ListReports returns reports, optionally filtered by period.
func (c *Comm) ListReports(period string) []*Report {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Report
	for _, r := range c.reports {
		if period == "" || r.Period == period {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].GeneratedAt.After(result[j].GeneratedAt)
	})
	return result
}

// --- Persistence ---

type commPersist struct {
	Channels map[string]*Channel  `json:"channels"`
	Messages map[string]*Message  `json:"messages"`
	Activity []ActivityEntry      `json:"activity"`
	Alerts   map[string]*Alert    `json:"alerts"`
	Reports  map[string]*Report   `json:"reports"`
}

func (c *Comm) persist() {
	if c.path == "" {
		return
	}
	data := commPersist{
		Channels: c.channels,
		Messages: c.messages,
		Activity: c.activity,
		Alerts:   c.alerts,
		Reports:  c.reports,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(c.path), 0755)
	os.WriteFile(c.path, raw, 0644)
}

func (c *Comm) load() {
	if c.path == "" {
		return
	}
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return
	}
	var data commPersist
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	if data.Channels != nil {
		c.channels = data.Channels
	}
	if data.Messages != nil {
		c.messages = data.Messages
	}
	if data.Activity != nil {
		c.activity = data.Activity
	}
	if data.Alerts != nil {
		c.alerts = data.Alerts
	}
	if data.Reports != nil {
		c.reports = data.Reports
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
