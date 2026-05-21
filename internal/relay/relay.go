// Package relay provides inter-agent message relay with pub/sub semantics.
// Agents subscribe to topics, publish messages, and relay coordinates
// delivery. Supports filtering, replay, and dead-letter handling.
//
// Messages flow. Agents listen.
package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MessageState represents the state of a relayed message.
type MessageState string

const (
	StateDelivered MessageState = "delivered"
	StatePending   MessageState = "pending"
	StateFailed    MessageState = "failed"
	StateExpired   MessageState = "expired"
)

// Message represents a relayed message.
type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	From      string            `json:"from"`
	Payload   string            `json:"payload"`
	Headers   map[string]string `json:"headers,omitempty"`
	State     MessageState      `json:"state"`
	CreatedAt time.Time         `json:"created_at"`
}

// Subscription represents an agent's subscription to a topic.
type Subscription struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Topic     string    `json:"topic"`
	Filter    string    `json:"filter,omitempty"` // Content filter
	CreatedAt time.Time `json:"created_at"`
}

// Delivery represents a message delivery to a subscriber.
type Delivery struct {
	ID        string       `json:"id"`
	MessageID string       `json:"message_id"`
	AgentID   string       `json:"agent_id"`
	Topic     string       `json:"topic"`
	State     MessageState `json:"state"`
	DeliveredAt time.Time  `json:"delivered_at,omitempty"`
}

// Relay coordinates message delivery between agents.
type Relay struct {
	dir           string
	subscriptions map[string][]Subscription // topic -> subs
	messages      []Message
	deliveries    []Delivery
	deadLetters   []Message
	mu            sync.RWMutex
}

// NewRelay creates a new message relay.
func NewRelay(dir string) *Relay {
	os.MkdirAll(dir, 0755)
	r := &Relay{
		dir:           dir,
		subscriptions: make(map[string][]Subscription),
	}
	r.load()
	return r
}

// Subscribe subscribes an agent to a topic.
func (r *Relay) Subscribe(agentID, topic, filter string) *Subscription {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := Subscription{
		ID:        fmt.Sprintf("sub-%d", time.Now().UnixNano()),
		AgentID:   agentID,
		Topic:     topic,
		Filter:    filter,
		CreatedAt: time.Now(),
	}

	r.subscriptions[topic] = append(r.subscriptions[topic], s)
	r.save()
	return &s
}

// Unsubscribe removes an agent's subscription.
func (r *Relay) Unsubscribe(subscriptionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for topic, subs := range r.subscriptions {
		for i, s := range subs {
			if s.ID == subscriptionID {
				r.subscriptions[topic] = append(subs[:i], subs[i+1:]...)
				r.save()
				return nil
			}
		}
	}
	return fmt.Errorf("subscription %q not found", subscriptionID)
}

// Publish publishes a message to a topic.
func (r *Relay) Publish(from, topic, payload string, headers map[string]string) *Message {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg := Message{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Topic:     topic,
		From:      from,
		Payload:   payload,
		Headers:   headers,
		State:     StateDelivered,
		CreatedAt: time.Now(),
	}

	r.messages = append(r.messages, msg)

	// Deliver to subscribers
	subs := r.subscriptions[topic]
	if len(subs) == 0 {
		// No subscribers — dead letter
		msg.State = StatePending
		r.deadLetters = append(r.deadLetters, msg)
		r.save()
		return &msg
	}

	for _, sub := range subs {
		// Apply filter if present
		if sub.Filter != "" && !strings.Contains(strings.ToLower(payload), strings.ToLower(sub.Filter)) {
			continue
		}

		delivery := Delivery{
			ID:        fmt.Sprintf("dlv-%d", time.Now().UnixNano()),
			MessageID: msg.ID,
			AgentID:   sub.AgentID,
			Topic:     topic,
			State:     StateDelivered,
			DeliveredAt: time.Now(),
		}
		r.deliveries = append(r.deliveries, delivery)
	}

	r.save()
	return &msg
}

// Subscriptions returns subscriptions for a topic or agent.
func (r *Relay) Subscriptions(topic, agentID string) []Subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Subscription
	if topic != "" {
		result = append(result, r.subscriptions[topic]...)
	} else {
		for _, subs := range r.subscriptions {
			result = append(result, subs...)
		}
	}

	if agentID != "" {
		var filtered []Subscription
		for _, s := range result {
			if s.AgentID == agentID {
				filtered = append(filtered, s)
			}
		}
		result = filtered
	}

	return result
}

// Messages returns messages for a topic.
func (r *Relay) Messages(topic string, limit int) []Message {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Message
	for i := len(r.messages) - 1; i >= 0; i-- {
		if topic == "" || r.messages[i].Topic == topic {
			result = append(result, r.messages[i])
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// Deliveries returns deliveries for an agent.
func (r *Relay) Deliveries(agentID string, limit int) []Delivery {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Delivery
	for i := len(r.deliveries) - 1; i >= 0; i-- {
		if agentID == "" || r.deliveries[i].AgentID == agentID {
			result = append(result, r.deliveries[i])
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// DeadLetters returns undelivered messages.
func (r *Relay) DeadLetters(limit int) []Message {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit > 0 && len(r.deadLetters) > limit {
		return r.deadLetters[:limit]
	}
	return r.deadLetters
}

// Redeliver attempts to redeliver dead letters.
func (r *Relay) Redeliver() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	var remaining []Message
	for i := range r.deadLetters {
		msg := &r.deadLetters[i]
		subs := r.subscriptions[msg.Topic]
		if len(subs) > 0 {
			for _, sub := range subs {
				delivery := Delivery{
					ID:          fmt.Sprintf("dlv-%d", time.Now().UnixNano()),
					MessageID:   msg.ID,
					AgentID:     sub.AgentID,
					Topic:       msg.Topic,
					State:       StateDelivered,
					DeliveredAt: time.Now(),
				}
				r.deliveries = append(r.deliveries, delivery)
			}
			msg.State = StateDelivered
			count++
		} else {
			remaining = append(remaining, *msg)
		}
	}

	r.deadLetters = remaining
	r.save()

	return count
}

// Stats returns relay statistics.
func (r *Relay) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalSubs := 0
	for _, subs := range r.subscriptions {
		totalSubs += len(subs)
	}

	topics := make([]string, 0, len(r.subscriptions))
	for t := range r.subscriptions {
		topics = append(topics, t)
	}
	sort.Strings(topics)

	return map[string]interface{}{
		"topics":        len(r.subscriptions),
		"subscriptions": totalSubs,
		"messages":      len(r.messages),
		"deliveries":    len(r.deliveries),
		"dead_letters":  len(r.deadLetters),
	}
}

func (r *Relay) save() {
	if r.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(r.subscriptions, "", "  ")
	os.WriteFile(filepath.Join(r.dir, "subscriptions.json"), data, 0644)

	msgData, _ := json.MarshalIndent(r.messages, "", "  ")
	os.WriteFile(filepath.Join(r.dir, "messages.json"), msgData, 0644)

	dlvData, _ := json.MarshalIndent(r.deliveries, "", "  ")
	os.WriteFile(filepath.Join(r.dir, "deliveries.json"), dlvData, 0644)

	dlData, _ := json.MarshalIndent(r.deadLetters, "", "  ")
	os.WriteFile(filepath.Join(r.dir, "dead_letters.json"), dlData, 0644)
}

func (r *Relay) load() {
	if r.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(r.dir, "subscriptions.json"))
	if err == nil {
		json.Unmarshal(data, &r.subscriptions)
	}
	msgData, err := os.ReadFile(filepath.Join(r.dir, "messages.json"))
	if err == nil {
		json.Unmarshal(msgData, &r.messages)
	}
	dlvData, err := os.ReadFile(filepath.Join(r.dir, "deliveries.json"))
	if err == nil {
		json.Unmarshal(dlvData, &r.deliveries)
	}
	dlData, err := os.ReadFile(filepath.Join(r.dir, "dead_letters.json"))
	if err == nil {
		json.Unmarshal(dlData, &r.deadLetters)
	}
}
