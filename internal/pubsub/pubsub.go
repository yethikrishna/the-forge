// Package pubsub provides an internal publish/subscribe message bus
// for inter-agent coordination within the forge.
package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Message is a published message.
type Message struct {
	Topic     string            `json:"topic"`
	Payload   []byte            `json:"payload"`
	Headers   map[string]string `json:"headers,omitempty"`
	Publisher string            `json:"publisher"`
	Timestamp time.Time         `json:"timestamp"`
	ID        string            `json:"id"`
}

// Handler processes a message.
type Handler func(ctx context.Context, msg Message) error

// Subscription is a topic subscription.
type Subscription struct {
	ID      string
	Topic   string
	Handler Handler
}

// Bus is the in-process message bus.
type Bus struct {
	subscriptions map[string][]Subscription // topic -> subscriptions
	allSubs       []Subscription            // wildcard subscriptions
	msgCounter    int64
	mu            sync.RWMutex
	history       map[string][]Message // topic -> recent messages
	historySize   int
}

// NewBus creates a new message bus.
func NewBus() *Bus {
	return &Bus{
		subscriptions: make(map[string][]Subscription),
		allSubs:       nil,
		history:       make(map[string][]Message),
		historySize:   100,
	}
}

// Publish sends a message to all subscribers of a topic.
func (b *Bus) Publish(ctx context.Context, topic string, payload []byte, opts ...PublishOption) error {
	b.mu.Lock()
	b.msgCounter++
	msg := Message{
		Topic:     topic,
		Payload:   payload,
		Headers:   make(map[string]string),
		Timestamp: time.Now().UTC(),
		ID:        fmt.Sprintf("msg-%d", b.msgCounter),
	}
	for _, o := range opts {
		o(&msg)
	}

	// Store in history
	b.history[topic] = append(b.history[topic], msg)
	if len(b.history[topic]) > b.historySize {
		b.history[topic] = b.history[topic][len(b.history[topic])-b.historySize:]
	}

	// Get handlers
	subs := make([]Subscription, len(b.subscriptions[topic]))
	copy(subs, b.subscriptions[topic])
	allSubs := make([]Subscription, len(b.allSubs))
	copy(allSubs, b.allSubs)
	b.mu.Unlock()

	// Deliver to topic subscribers
	var errs []error
	for _, sub := range subs {
		if err := sub.Handler(ctx, msg); err != nil {
			errs = append(errs, fmt.Errorf("handler %s: %w", sub.ID, err))
		}
	}

	// Deliver to wildcard subscribers
	for _, sub := range allSubs {
		if err := sub.Handler(ctx, msg); err != nil {
			errs = append(errs, fmt.Errorf("wildcard handler %s: %w", sub.ID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("publish errors: %v", errs)
	}
	return nil
}

// Subscribe registers a handler for a topic.
func (b *Bus) Subscribe(topic string, handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := fmt.Sprintf("sub-%d", b.msgCounter+1)
	b.msgCounter++

	sub := Subscription{
		ID:      id,
		Topic:   topic,
		Handler: handler,
	}

	if topic == "*" {
		b.allSubs = append(b.allSubs, sub)
	} else {
		b.subscriptions[topic] = append(b.subscriptions[topic], sub)
	}

	return id
}

// Unsubscribe removes a subscription.
func (b *Bus) Unsubscribe(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check topic subscriptions
	for topic, subs := range b.subscriptions {
		for i, sub := range subs {
			if sub.ID == id {
				b.subscriptions[topic] = append(subs[:i], subs[i+1:]...)
				return true
			}
		}
	}

	// Check wildcard
	for i, sub := range b.allSubs {
		if sub.ID == id {
			b.allSubs = append(b.allSubs[:i], b.allSubs[i+1:]...)
			return true
		}
	}

	return false
}

// History returns recent messages for a topic.
func (b *Bus) History(topic string, limit int) []Message {
	b.mu.RLock()
	defer b.mu.RUnlock()

	msgs := b.history[topic]
	if len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}

	result := make([]Message, len(msgs))
	copy(result, msgs)
	return result
}

// Topics returns all active topics.
func (b *Bus) Topics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.subscriptions))
	for topic := range b.subscriptions {
		if len(b.subscriptions[topic]) > 0 {
			topics = append(topics, topic)
		}
	}
	return topics
}

// Subscribers returns subscriber count for a topic.
func (b *Bus) Subscribers(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if topic == "*" {
		return len(b.allSubs)
	}
	return len(b.subscriptions[topic])
}

// PublishOption customizes message publishing.
type PublishOption func(*Message)

// WithPublisher sets the message publisher.
func WithPublisher(publisher string) PublishOption {
	return func(m *Message) { m.Publisher = publisher }
}

// WithHeaders sets message headers.
func WithHeaders(headers map[string]string) PublishOption {
	return func(m *Message) {
		for k, v := range headers {
			m.Headers[k] = v
		}
	}
}

// RequestResponse implements a request-response pattern on top of pub/sub.
type RequestResponse struct {
	bus    *Bus
	prefix string
}

// NewRequestResponse creates a request-response layer.
func NewRequestResponse(bus *Bus, prefix string) *RequestResponse {
	return &RequestResponse{bus: bus, prefix: prefix}
}

// Request sends a request and waits for a response.
func (rr *RequestResponse) Request(ctx context.Context, channel string, payload []byte, timeout time.Duration) (Message, error) {
	responseTopic := fmt.Sprintf("%s.%s.response", rr.prefix, channel)
	requestTopic := fmt.Sprintf("%s.%s.request", rr.prefix, channel)

	respCh := make(chan Message, 1)
	subID := rr.bus.Subscribe(responseTopic, func(_ context.Context, msg Message) error {
		select {
		case respCh <- msg:
		default:
		}
		return nil
	})
	defer rr.bus.Unsubscribe(subID)

	// Publish request
	if err := rr.bus.Publish(ctx, requestTopic, payload, WithHeaders(map[string]string{
		"response-topic": responseTopic,
	})); err != nil {
		return Message{}, err
	}

	// Wait for response
	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		return Message{}, fmt.Errorf("request timeout after %v", timeout)
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

// Respond registers a handler for requests on a channel.
func (rr *RequestResponse) Respond(channel string, handler func(ctx context.Context, req Message) ([]byte, error)) string {
	requestTopic := fmt.Sprintf("%s.%s.request", rr.prefix, channel)

	return rr.bus.Subscribe(requestTopic, func(ctx context.Context, msg Message) error {
		respTopic := msg.Headers["response-topic"]
		if respTopic == "" {
			return nil
		}

		respPayload, err := handler(ctx, msg)
		if err != nil {
			return err
		}

		return rr.bus.Publish(ctx, respTopic, respPayload)
	})
}
