package eventbus

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublishSubscribe(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var received atomic.Int32
	bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "test.topic", "hello")
	time.Sleep(10 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("Expected 1 received, got %d", received.Load())
	}
}

func TestMultipleSubscribers(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var received atomic.Int32
	for i := 0; i < 3; i++ {
		bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
			received.Add(1)
			return nil
		})
	}

	bus.Publish(context.Background(), "test.topic", "hello")
	time.Sleep(10 * time.Millisecond)

	if received.Load() != 3 {
		t.Errorf("Expected 3 received, got %d", received.Load())
	}
}

func TestTopicIsolation(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var received atomic.Int32
	bus.Subscribe("topic.a", func(ctx context.Context, event Event) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "topic.b", "hello")
	time.Sleep(10 * time.Millisecond)

	if received.Load() != 0 {
		t.Errorf("Expected 0 received for different topic, got %d", received.Load())
	}
}

func TestSubscribeAll(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var received atomic.Int32
	bus.SubscribeAll(func(ctx context.Context, event Event) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "topic.a", "hello")
	bus.Publish(context.Background(), "topic.b", "world")
	time.Sleep(10 * time.Millisecond)

	if received.Load() != 2 {
		t.Errorf("Expected 2 received, got %d", received.Load())
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var received atomic.Int32
	subID := bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "test.topic", "before")
	time.Sleep(10 * time.Millisecond)

	bus.Unsubscribe(subID)

	bus.Publish(context.Background(), "test.topic", "after")
	time.Sleep(10 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("Expected 1 received after unsubscribe, got %d", received.Load())
	}
}

func TestFilter(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var received atomic.Int32
	bus.SubscribeWithFilter("test.topic", func(ctx context.Context, event Event) error {
		received.Add(1)
		return nil
	}, func(event Event) bool {
		return event.Source == "allowed"
	})

	bus.PublishWithSource(context.Background(), "test.topic", "allowed", "hello")
	bus.PublishWithSource(context.Background(), "test.topic", "blocked", "world")
	time.Sleep(10 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("Expected 1 received (filtered), got %d", received.Load())
	}
}

func TestDeadLetter(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		return fmt.Errorf("handler error")
	})

	bus.Publish(context.Background(), "test.topic", "hello")
	time.Sleep(10 * time.Millisecond)

	dls := bus.GetDeadLetters()
	if len(dls) != 1 {
		t.Fatalf("Expected 1 dead letter, got %d", len(dls))
	}
	if dls[0].Error != "handler error" {
		t.Errorf("Expected 'handler error', got %q", dls[0].Error)
	}
}

func TestPurgeDeadLetters(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		return fmt.Errorf("error")
	})

	bus.Publish(context.Background(), "test.topic", "hello")
	time.Sleep(10 * time.Millisecond)

	bus.PurgeDeadLetters()
	dls := bus.GetDeadLetters()
	if len(dls) != 0 {
		t.Errorf("Expected 0 dead letters after purge, got %d", len(dls))
	}
}

func TestStats(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		return nil
	})

	bus.Publish(context.Background(), "test.topic", "hello")
	time.Sleep(10 * time.Millisecond)

	stats := bus.Stats()
	if stats.TotalPublished != 1 {
		t.Errorf("Expected 1 published, got %d", stats.TotalPublished)
	}
	if stats.TotalDelivered != 1 {
		t.Errorf("Expected 1 delivered, got %d", stats.TotalDelivered)
	}
	if stats.Subscribers != 1 {
		t.Errorf("Expected 1 subscriber, got %d", stats.Subscribers)
	}
}

func TestTopics(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	bus.Subscribe("topic.a", func(ctx context.Context, event Event) error { return nil })
	bus.Subscribe("topic.b", func(ctx context.Context, event Event) error { return nil })

	topics := bus.Topics()
	if len(topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(topics))
	}
}

func TestCloseBus(t *testing.T) {
	bus := New(100)
	bus.Close()

	err := bus.Publish(context.Background(), "test.topic", "hello")
	if err == nil {
		t.Error("Expected error when publishing to closed bus")
	}
}

func TestPublishWithSource(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var source string
	bus.Subscribe("test.topic", func(ctx context.Context, event Event) error {
		source = event.Source
		return nil
	})

	bus.PublishWithSource(context.Background(), "test.topic", "my-agent", "hello")
	time.Sleep(10 * time.Millisecond)

	if source != "my-agent" {
		t.Errorf("Expected source 'my-agent', got %q", source)
	}
}

func TestEventPayload(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	var payload AgentEventPayload
	bus.Subscribe(TopicAgentStarted, func(ctx context.Context, event Event) error {
		if p, ok := event.Payload.(AgentEventPayload); ok {
			payload = p
		}
		return nil
	})

	bus.Publish(context.Background(), TopicAgentStarted, AgentEventPayload{
		AgentID: "agent-1",
		Model:   "claude-sonnet-4",
		Task:    "code review",
	})
	time.Sleep(10 * time.Millisecond)

	if payload.AgentID != "agent-1" {
		t.Errorf("Expected agent-1, got %s", payload.AgentID)
	}
	if payload.Model != "claude-sonnet-4" {
		t.Errorf("Expected claude-sonnet-4, got %s", payload.Model)
	}
}
