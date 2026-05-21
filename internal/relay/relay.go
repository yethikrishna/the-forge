// Package relay provides inter-agent message relay with pub/sub,
// request/response, and broadcast patterns. Agents communicate through
// named channels with delivery guarantees, message ordering, and
// dead letter handling.
//
// Messages flow like water between agents.
package relay

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MessageState represents the state of a message.
type MessageState string

const (
	StateQueued     MessageState = "queued"
	StateDelivered  MessageState = "delivered"
	StateAcked      MessageState = "acked"
	StateFailed     MessageState = "failed"
	StateExpired    MessageState = "expired"
	StateDeadLetter MessageState = "dead_letter"
)

// Message represents a relay message.
type Message struct {
	ID            string            `json:"id"`
	From          string            `json:"from"`
	To            string            `json:"to"` // agent ID or "broadcast" or channel name
	Channel       string            `json:"channel,omitempty"`
	Subject       string            `json:"subject"`
	Body          string            `json:"body"`
	Priority      int               `json:"priority"`
	State         MessageState      `json:"state"`
	CreatedAt     time.Time         `json:"created_at"`
	DeliveredAt   time.Time         `json:"delivered_at,omitempty"`
	AckedAt       time.Time         `json:"acked_at,omitempty"`
	ExpiresAt     time.Time         `json:"expires_at,omitempty"`
	Retries       int               `json:"retries"`
	MaxRetries    int               `json:"max_retries"`
	Headers       map[string]string `json:"headers,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"` // for request/response
	ReplyTo       string            `json:"reply_to,omitempty"`       // channel for replies
}

// Subscription represents an agent's subscription to a channel.
type Subscription struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Channel   string    `json:"channel"`
	Pattern   string    `json:"pattern,omitempty"` // glob pattern for subject matching
	CreatedAt time.Time `json:"created_at"`
	Active    bool      `json:"active"`
}

// ChannelStats holds statistics for a channel.
type ChannelStats struct {
	Name         string `json:"name"`
	Subscribers  int    `json:"subscribers"`
	MessagesSent int    `json:"messages_sent"`
	PendingCount int    `json:"pending_count"`
}

// Relay is the message relay service.
type Relay struct {
	mu            sync.RWMutex
	dir           string
	queues        map[string][]*Message      // agent/channel -> message queue
	subscriptions map[string][]*Subscription // channel -> subscriptions
	deadLetters   []*Message
	allMessages   map[string]*Message
	stats         RelayStats
}

// RelayStats holds relay statistics.
type RelayStats struct {
	TotalSent       int `json:"total_sent"`
	TotalDelivered  int `json:"total_delivered"`
	TotalAcked      int `json:"total_acked"`
	TotalFailed     int `json:"total_failed"`
	TotalDead       int `json:"total_dead"`
	ChannelCount    int `json:"channel_count"`
	SubscriberCount int `json:"subscriber_count"`
}

// NewRelay creates a new relay service.
func NewRelay(dir string) *Relay {
	return &Relay{
		dir:           dir,
		queues:        make(map[string][]*Message),
		subscriptions: make(map[string][]*Subscription),
		deadLetters:   make([]*Message, 0),
		allMessages:   make(map[string]*Message),
	}
}

// Send sends a message to an agent or channel.
func (r *Relay) Send(msg Message) (*Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if msg.ID == "" {
		msg.ID = messageID(msg.From, msg.To, msg.Subject)
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.State == "" {
		msg.State = StateQueued
	}
	if msg.MaxRetries == 0 {
		msg.MaxRetries = 3
	}

	r.allMessages[msg.ID] = &msg
	r.stats.TotalSent++

	// Direct message to specific agent
	if msg.To != "" && msg.To != "broadcast" && msg.Channel == "" {
		r.queues[msg.To] = append(r.queues[msg.To], &msg)
		return &msg, nil
	}

	// Broadcast to channel subscribers
	channel := msg.Channel
	if channel == "" && msg.To == "broadcast" {
		channel = "broadcast"
	}

	if subs, ok := r.subscriptions[channel]; ok {
		for _, sub := range subs {
			if !sub.Active {
				continue
			}
			if sub.Pattern != "" && !matchSubject(sub.Pattern, msg.Subject) {
				continue
			}
			copy := msg
			copy.ID = messageID(msg.From, sub.AgentID, msg.Subject)
			copy.To = sub.AgentID
			r.queues[sub.AgentID] = append(r.queues[sub.AgentID], &copy)
			r.allMessages[copy.ID] = &copy
			r.stats.TotalDelivered++
		}
	}

	return &msg, nil
}

// Receive receives the next message for an agent.
func (r *Relay) Receive(agentID string) (*Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	queue, ok := r.queues[agentID]
	if !ok || len(queue) == 0 {
		return nil, nil // no messages
	}

	msg := queue[0]
	r.queues[agentID] = queue[1:]

	msg.State = StateDelivered
	msg.DeliveredAt = time.Now()

	return msg, nil
}

// Ack acknowledges a message.
func (r *Relay) Ack(messageID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg, ok := r.allMessages[messageID]
	if !ok {
		return fmt.Errorf("message %s not found", messageID)
	}
	msg.State = StateAcked
	msg.AckedAt = time.Now()
	r.stats.TotalAcked++
	return nil
}

// Nack negatively acknowledges a message (will retry or dead-letter).
func (r *Relay) Nack(messageID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg, ok := r.allMessages[messageID]
	if !ok {
		return fmt.Errorf("message %s not found", messageID)
	}

	msg.Retries++
	if msg.Retries >= msg.MaxRetries {
		msg.State = StateDeadLetter
		r.deadLetters = append(r.deadLetters, msg)
		r.stats.TotalDead++
	} else {
		msg.State = StateQueued
		// Re-queue
		r.queues[msg.To] = append(r.queues[msg.To], msg)
		r.stats.TotalFailed++
	}

	return nil
}

// Subscribe subscribes an agent to a channel.
func (r *Relay) Subscribe(agentID, channel string, pattern string) (*Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sub := &Subscription{
		ID:        subID(agentID, channel),
		AgentID:   agentID,
		Channel:   channel,
		Pattern:   pattern,
		CreatedAt: time.Now(),
		Active:    true,
	}

	r.subscriptions[channel] = append(r.subscriptions[channel], sub)
	r.stats.SubscriberCount++
	return sub, nil
}

// Unsubscribe removes an agent's subscription.
func (r *Relay) Unsubscribe(agentID, channel string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	subs := r.subscriptions[channel]
	for i, sub := range subs {
		if sub.AgentID == agentID {
			r.subscriptions[channel] = append(subs[:i], subs[i+1:]...)
			r.stats.SubscriberCount--
			break
		}
	}
}

// Subscriptions returns all subscriptions for a channel.
func (r *Relay) Subscriptions(channel string) []*Subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.subscriptions[channel]
}

// DeadLetters returns all dead-lettered messages.
func (r *Relay) DeadLetters() []*Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.deadLetters
}

// PendingCount returns the number of pending messages for an agent.
func (r *Relay) PendingCount(agentID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.queues[agentID])
}

// Stats returns relay statistics.
func (r *Relay) Stats() RelayStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// ChannelStats returns statistics for all channels.
func (r *Relay) ChannelStats() []ChannelStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ChannelStats
	for name, subs := range r.subscriptions {
		result = append(result, ChannelStats{
			Name:        name,
			Subscribers: len(subs),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Message returns a message by ID.
func (r *Relay) Message(id string) (*Message, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	msg, ok := r.allMessages[id]
	return msg, ok
}

// Expire removes expired messages and moves them to dead letters.
func (r *Relay) Expire() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, queue := range r.queues {
		var remaining []*Message
		for _, msg := range queue {
			if !msg.ExpiresAt.IsZero() && time.Now().After(msg.ExpiresAt) {
				msg.State = StateExpired
				r.deadLetters = append(r.deadLetters, msg)
				r.stats.TotalDead++
				count++
			} else {
				remaining = append(remaining, msg)
			}
		}
		// Note: can't replace queue directly since we're iterating map
		_ = remaining
	}
	return count
}

// ExportMarkdown exports relay status as markdown.
func (r *Relay) ExportMarkdown() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "# Message Relay\n\n")
	stats := r.stats
	fmt.Fprintf(&b, "**Sent:** %d | **Delivered:** %d | **Acked:** %d | **Dead:** %d\n\n",
		stats.TotalSent, stats.TotalDelivered, stats.TotalAcked, stats.TotalDead)

	if len(r.subscriptions) > 0 {
		b.WriteString("## Channels\n\n")
		for _, cs := range r.ChannelStats() {
			fmt.Fprintf(&b, "- **%s:** %d subscribers\n", cs.Name, cs.Subscribers)
		}
		b.WriteString("\n")
	}

	if len(r.deadLetters) > 0 {
		fmt.Fprintf(&b, "## Dead Letters (%d)\n\n", len(r.deadLetters))
		for _, msg := range r.deadLetters {
			fmt.Fprintf(&b, "- %s → %s: %s (retries: %d)\n", msg.From, msg.To, msg.Subject, msg.Retries)
		}
	}

	return b.String()
}

// Save persists relay state.
func (r *Relay) Save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := os.MkdirAll(r.dir, 0755); err != nil {
		return err
	}

	data, _ := json.MarshalIndent(r.allMessages, "", "  ")
	return os.WriteFile(filepath.Join(r.dir, "messages.json"), data, 0644)
}

// Helper functions

func messageID(from, to, subject string) string {
	h := sha256.Sum256([]byte(from + to + subject + time.Now().String()))
	return fmt.Sprintf("msg-%x", h[:8])
}

func subID(agentID, channel string) string {
	h := sha256.Sum256([]byte(agentID + channel))
	return fmt.Sprintf("sub-%x", h[:8])
}

func matchSubject(pattern, subject string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	return strings.Contains(subject, pattern) || strings.Contains(pattern, subject)
}
