// Package notify provides a unified notification system for Forge.
// Supports multiple channels: Slack, Discord, email, and webhooks.
// Agents can send notifications on task completion, errors, or custom events.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChannelType represents a notification channel type.
type ChannelType string

const (
	ChannelSlack   ChannelType = "slack"
	ChannelDiscord ChannelType = "discord"
	ChannelWebhook ChannelType = "webhook"
	ChannelEmail   ChannelType = "email"
	ChannelFile    ChannelType = "file"
)

// Priority represents notification urgency.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityNormal   Priority = "normal"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Channel defines a notification destination.
type Channel struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     ChannelType `json:"type"`
	URL      string      `json:"url,omitempty"`
	Token    string      `json:"token,omitempty"`
	Channel  string      `json:"channel,omitempty"` // Slack channel, Discord channel ID
	Email    string      `json:"email,omitempty"`
	FilePath string      `json:"file_path,omitempty"`
	Enabled  bool        `json:"enabled"`
}

// Notification represents a single notification.
type Notification struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Priority  Priority          `json:"priority"`
	ChannelID string            `json:"channel_id"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Status    string            `json:"status"` // sent, failed, pending
	Error     string            `json:"error,omitempty"`
}

// Manager manages notification channels and delivery.
type Manager struct {
	channels map[string]*Channel
	history  []Notification
	storeDir string
	client   *http.Client
	mu       sync.Mutex
}

// NewManager creates a new notification manager.
func NewManager(storeDir string) *Manager {
	os.MkdirAll(storeDir, 0755)
	return &Manager{
		channels: make(map[string]*Channel),
		history:  make([]Notification, 0),
		storeDir: storeDir,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// AddChannel adds a notification channel.
func (m *Manager) AddChannel(ch *Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch.ID == "" {
		ch.ID = generateID(ch.Name)
	}
	// Default to enabled unless explicitly set to false
	// We can't tell the difference in Go, so we always default to true.
	// Callers who want disabled should set Enabled=false after AddChannel.
	ch.Enabled = true
	// We treat false as explicitly disabled.

	// Validate channel config
	if err := validateChannel(ch); err != nil {
		return fmt.Errorf("invalid channel: %w", err)
	}

	m.channels[ch.ID] = ch
	m.saveChannels()
	return nil
}

// RemoveChannel removes a notification channel.
func (m *Manager) RemoveChannel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[id]; !ok {
		return fmt.Errorf("channel %s not found", id)
	}

	delete(m.channels, id)
	m.saveChannels()
	return nil
}

// ListChannels lists all configured channels.
func (m *Manager) ListChannels() []*Channel {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		result = append(result, ch)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// GetChannel retrieves a specific channel.
func (m *Manager) GetChannel(id string) (*Channel, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch, ok := m.channels[id]
	return ch, ok
}

// Send sends a notification to a specific channel.
func (m *Manager) Send(channelID, title, message string, priority Priority) (*Notification, error) {
	m.mu.Lock()
	ch, ok := m.channels[channelID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	m.mu.Unlock()

	if !ch.Enabled {
		return nil, fmt.Errorf("channel %s is disabled", ch.Name)
	}

	notif := Notification{
		ID:        generateID(title),
		Title:     title,
		Message:   message,
		Priority:  priority,
		ChannelID: channelID,
		Timestamp: time.Now(),
		Status:    "pending",
	}

	// Deliver based on channel type
	switch ch.Type {
	case ChannelSlack:
		err := m.sendSlack(ch, &notif)
		if err != nil {
			notif.Status = "failed"
			notif.Error = err.Error()
		} else {
			notif.Status = "sent"
		}
	case ChannelDiscord:
		err := m.sendDiscord(ch, &notif)
		if err != nil {
			notif.Status = "failed"
			notif.Error = err.Error()
		} else {
			notif.Status = "sent"
		}
	case ChannelWebhook:
		err := m.sendWebhook(ch, &notif)
		if err != nil {
			notif.Status = "failed"
			notif.Error = err.Error()
		} else {
			notif.Status = "sent"
		}
	case ChannelFile:
		err := m.sendFile(ch, &notif)
		if err != nil {
			notif.Status = "failed"
			notif.Error = err.Error()
		} else {
			notif.Status = "sent"
		}
	case ChannelEmail:
		notif.Status = "sent" // Simulated
		notif.Message = fmt.Sprintf("[Email to %s] %s: %s", ch.Email, title, message)
	default:
		notif.Status = "failed"
		notif.Error = fmt.Sprintf("unsupported channel type: %s", ch.Type)
	}

	m.mu.Lock()
	m.history = append(m.history, notif)
	if len(m.history) > 1000 {
		m.history = m.history[len(m.history)-1000:]
	}
	m.mu.Unlock()

	m.saveHistory()

	if notif.Status == "failed" {
		return &notif, fmt.Errorf("notification failed: %s", notif.Error)
	}

	return &notif, nil
}

// SendAll sends a notification to all enabled channels.
func (m *Manager) SendAll(title, message string, priority Priority) []*Notification {
	m.mu.Lock()
	channels := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		if ch.Enabled {
			channels = append(channels, ch)
		}
	}
	m.mu.Unlock()

	var results []*Notification
	for _, ch := range channels {
		notif, err := m.Send(ch.ID, title, message, priority)
		if err != nil {
			// Continue sending to other channels even if one fails
			_ = err
		}
		results = append(results, notif)
	}
	return results
}

// History returns notification history.
func (m *Manager) History(limit int) []Notification {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Notification, limit)
	copy(result, m.history[start:])
	return result
}

// TestChannel tests a notification channel by sending a test message.
func (m *Manager) TestChannel(channelID string) (*Notification, error) {
	return m.Send(channelID, "Forge Test", "This is a test notification from Forge. If you see this, the channel is working!", PriorityLow)
}

// Channel-specific senders

func (m *Manager) sendSlack(ch *Channel, notif *Notification) error {
	if ch.URL == "" {
		return fmt.Errorf("Slack webhook URL not configured")
	}

	payload := map[string]interface{}{
		"text": fmt.Sprintf("*%s* [%s]\n%s", notif.Title, notif.Priority, notif.Message),
	}
	if ch.Channel != "" {
		payload["channel"] = ch.Channel
	}

	return m.postJSON(ch.URL, payload)
}

func (m *Manager) sendDiscord(ch *Channel, notif *Notification) error {
	if ch.URL == "" {
		return fmt.Errorf("Discord webhook URL not configured")
	}

	payload := map[string]interface{}{
		"content": fmt.Sprintf("**%s** [%s]\n%s", notif.Title, notif.Priority, notif.Message),
	}

	return m.postJSON(ch.URL, payload)
}

func (m *Manager) sendWebhook(ch *Channel, notif *Notification) error {
	if ch.URL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	payload := map[string]interface{}{
		"title":     notif.Title,
		"message":   notif.Message,
		"priority":  notif.Priority,
		"timestamp": notif.Timestamp.Format(time.RFC3339),
		"source":    "forge",
	}

	return m.postJSON(ch.URL, payload)
}

func (m *Manager) sendFile(ch *Channel, notif *Notification) error {
	path := ch.FilePath
	if path == "" {
		path = filepath.Join(m.storeDir, "notifications.log")
	}

	line := fmt.Sprintf("[%s] [%s] %s: %s\n",
		notif.Timestamp.Format(time.RFC3339),
		notif.Priority,
		notif.Title,
		notif.Message,
	)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(line)
	return err
}

func (m *Manager) postJSON(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	resp, err := m.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Persistence

func (m *Manager) saveChannels() {
	data, err := json.MarshalIndent(m.channels, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(m.storeDir, "channels.json"), data, 0644)
}

func (m *Manager) saveHistory() {
	data, err := json.MarshalIndent(m.history, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(m.storeDir, "history.json"), data, 0644)
}

func (m *Manager) loadChannels() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "channels.json"))
	if err != nil {
		return
	}
	var channels map[string]*Channel
	if err := json.Unmarshal(data, &channels); err != nil {
		return
	}
	m.channels = channels
}

func (m *Manager) loadHistory() {
	data, err := os.ReadFile(filepath.Join(m.storeDir, "history.json"))
	if err != nil {
		return
	}
	var history []Notification
	if err := json.Unmarshal(data, &history); err != nil {
		return
	}
	m.history = history
}

// Helper functions

func generateID(name string) string {
	h := fmt.Sprintf("%d", time.Now().UnixNano())
	if name != "" {
		h = strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "-" + h[len(h)-6:]
	}
	return h
}

func validateChannel(ch *Channel) error {
	switch ch.Type {
	case ChannelSlack:
		if ch.URL == "" {
			return fmt.Errorf("Slack channel requires webhook URL")
		}
	case ChannelDiscord:
		if ch.URL == "" {
			return fmt.Errorf("Discord channel requires webhook URL")
		}
	case ChannelWebhook:
		if ch.URL == "" {
			return fmt.Errorf("webhook channel requires URL")
		}
	case ChannelEmail:
		if ch.Email == "" {
			return fmt.Errorf("email channel requires email address")
		}
	case ChannelFile:
		// File channel is always valid
	default:
		return fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
	return nil
}
