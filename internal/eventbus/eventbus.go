package eventbus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Event represents a typed event on the bus.
type Event struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Payload   interface{}       `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Handler processes events.
type Handler func(ctx context.Context, event Event) error

// Filter determines if an event should be delivered.
type Filter func(event Event) bool

// DeadLetter represents an event that could not be delivered.
type DeadLetter struct {
	Event     Event     `json:"event"`
	Error     string    `json:"error"`
	HandlerID string    `json:"handler_id"`
	Timestamp time.Time `json:"timestamp"`
	Retries   int       `json:"retries"`
}

// Stats holds event bus statistics.
type Stats struct {
	TotalPublished   int64          `json:"total_published"`
	TotalDelivered   int64          `json:"total_delivered"`
	TotalFailed      int64          `json:"total_failed"`
	TotalDeadLetters int64          `json:"total_dead_letters"`
	Topics           map[string]int `json:"topics"`
	Subscribers      int            `json:"subscribers"`
}

// Bus is the internal event bus.
type Bus struct {
	mu          sync.RWMutex
	handlers    map[string][]subscriber
	allHandlers []subscriber
	deadMu      sync.Mutex
	deadLetters []DeadLetter
	published   atomic.Int64
	delivered   atomic.Int64
	failed      atomic.Int64
	deadCount   atomic.Int64
	closed      atomic.Bool
	closeCh     chan struct{}
}

type subscriber struct {
	id      string
	handler Handler
	filter  Filter
	async   bool
}

// New creates a new event bus.
func New(bufferSize int) *Bus {
	_ = bufferSize
	return &Bus{
		handlers: make(map[string][]subscriber),
		closeCh:  make(chan struct{}),
	}
}

// Subscribe registers a handler for a specific topic.
func (b *Bus) Subscribe(topic string, handler Handler) string {
	return b.subscribe(topic, handler, nil, false)
}

// SubscribeAsync registers an async handler for a specific topic.
func (b *Bus) SubscribeAsync(topic string, handler Handler) string {
	return b.subscribe(topic, handler, nil, true)
}

// SubscribeWithFilter registers a handler with an event filter.
func (b *Bus) SubscribeWithFilter(topic string, handler Handler, filter Filter) string {
	return b.subscribe(topic, handler, filter, false)
}

// SubscribeAll registers a handler for all topics.
func (b *Bus) SubscribeAll(handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := fmt.Sprintf("sub-all-%d", time.Now().UnixNano())
	b.allHandlers = append(b.allHandlers, subscriber{id: id, handler: handler})
	return id
}

func (b *Bus) subscribe(topic string, handler Handler, filter Filter, async bool) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := fmt.Sprintf("sub-%s-%d", topic, time.Now().UnixNano())
	b.handlers[topic] = append(b.handlers[topic], subscriber{
		id: id, handler: handler, filter: filter, async: async,
	})
	return id
}

// Unsubscribe removes a subscription.
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for topic, subs := range b.handlers {
		for i, sub := range subs {
			if sub.id == id {
				b.handlers[topic] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
	for i, sub := range b.allHandlers {
		if sub.id == id {
			b.allHandlers = append(b.allHandlers[:i], b.allHandlers[i+1:]...)
			return
		}
	}
}

// Publish publishes an event to all subscribers of the topic.
func (b *Bus) Publish(ctx context.Context, topic string, payload interface{}) error {
	if b.closed.Load() {
		return fmt.Errorf("event bus is closed")
	}

	event := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	b.published.Add(1)

	// Snapshot subscribers under read lock
	b.mu.RLock()
	subs := make([]subscriber, len(b.handlers[topic]))
	copy(subs, b.handlers[topic])
	allSubs := make([]subscriber, len(b.allHandlers))
	copy(allSubs, b.allHandlers)
	b.mu.RUnlock()

	for _, sub := range subs {
		if sub.filter != nil && !sub.filter(event) {
			continue
		}
		b.deliver(ctx, event, sub)
	}
	for _, sub := range allSubs {
		if sub.filter != nil && !sub.filter(event) {
			continue
		}
		b.deliver(ctx, event, sub)
	}

	return nil
}

// PublishWithSource publishes an event with a source identifier.
func (b *Bus) PublishWithSource(ctx context.Context, topic, source string, payload interface{}) error {
	if b.closed.Load() {
		return fmt.Errorf("event bus is closed")
	}

	event := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
		Source:    source,
	}

	b.published.Add(1)

	b.mu.RLock()
	subs := make([]subscriber, len(b.handlers[topic]))
	copy(subs, b.handlers[topic])
	allSubs := make([]subscriber, len(b.allHandlers))
	copy(allSubs, b.allHandlers)
	b.mu.RUnlock()

	for _, sub := range subs {
		if sub.filter != nil && !sub.filter(event) {
			continue
		}
		b.deliver(ctx, event, sub)
	}
	for _, sub := range allSubs {
		if sub.filter != nil && !sub.filter(event) {
			continue
		}
		b.deliver(ctx, event, sub)
	}

	return nil
}

func (b *Bus) deliver(ctx context.Context, event Event, sub subscriber) {
	if sub.async {
		go func() {
			if err := sub.handler(ctx, event); err != nil {
				b.recordDeadLetter(event, sub.id, err.Error())
			} else {
				b.delivered.Add(1)
			}
		}()
	} else {
		if err := sub.handler(ctx, event); err != nil {
			b.recordDeadLetter(event, sub.id, err.Error())
		} else {
			b.delivered.Add(1)
		}
	}
}

func (b *Bus) recordDeadLetter(event Event, handlerID, errMsg string) {
	b.deadMu.Lock()
	defer b.deadMu.Unlock()

	b.deadLetters = append(b.deadLetters, DeadLetter{
		Event: event, Error: errMsg, HandlerID: handlerID, Timestamp: time.Now(),
	})
	b.failed.Add(1)
	b.deadCount.Add(1)

	if len(b.deadLetters) > 1000 {
		b.deadLetters = b.deadLetters[len(b.deadLetters)-1000:]
	}
}

// GetDeadLetters returns all dead letters.
func (b *Bus) GetDeadLetters() []DeadLetter {
	b.deadMu.Lock()
	defer b.deadMu.Unlock()
	result := make([]DeadLetter, len(b.deadLetters))
	copy(result, b.deadLetters)
	return result
}

// PurgeDeadLetters removes all dead letters.
func (b *Bus) PurgeDeadLetters() {
	b.deadMu.Lock()
	defer b.deadMu.Unlock()
	b.deadLetters = nil
	b.deadCount.Store(0)
}

// Stats returns event bus statistics.
func (b *Bus) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	s := Stats{
		TotalPublished:   b.published.Load(),
		TotalDelivered:   b.delivered.Load(),
		TotalFailed:      b.failed.Load(),
		TotalDeadLetters: b.deadCount.Load(),
		Topics:           make(map[string]int),
	}
	for topic, subs := range b.handlers {
		s.Topics[topic] = len(subs)
	}
	s.Subscribers = len(b.allHandlers)
	for _, subs := range b.handlers {
		s.Subscribers += len(subs)
	}
	return s
}

// Topics returns all active topics.
func (b *Bus) Topics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	topics := make([]string, 0, len(b.handlers))
	for topic, subs := range b.handlers {
		if len(subs) > 0 {
			topics = append(topics, topic)
		}
	}
	return topics
}

// Close shuts down the event bus.
func (b *Bus) Close() {
	b.closed.Store(true)
	close(b.closeCh)
}

// IsClosed returns whether the bus is closed.
func (b *Bus) IsClosed() bool {
	return b.closed.Load()
}

// Predefined event topics
const (
	TopicAgentStarted   = "agent.started"
	TopicAgentCompleted = "agent.completed"
	TopicAgentFailed    = "agent.failed"
	TopicToolCalled     = "tool.called"
	TopicCostUpdated    = "cost.updated"
	TopicFileChanged    = "file.changed"
	TopicSessionStarted = "session.started"
	TopicSessionEnded   = "session.ended"
	TopicPipelineStep   = "pipeline.step"
	TopicHealthCheck    = "health.check"
	TopicConfigChanged  = "config.changed"
	TopicError          = "error.occurred"
)

// AgentEventPayload is a structured payload for agent events.
type AgentEventPayload struct {
	AgentID  string  `json:"agent_id"`
	Model    string  `json:"model,omitempty"`
	Task     string  `json:"task,omitempty"`
	Cost     float64 `json:"cost,omitempty"`
	Duration string  `json:"duration,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// CostEventPayload is a structured payload for cost events.
type CostEventPayload struct {
	AgentID   string  `json:"agent_id"`
	Provider  string  `json:"provider"`
	Model     string  `json:"model"`
	Tokens    int     `json:"tokens"`
	Cost      float64 `json:"cost"`
	Remaining float64 `json:"remaining,omitempty"`
}
