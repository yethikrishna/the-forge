package pubsub_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forge/sword/internal/pubsub"
)

func TestPublishSubscribe(t *testing.T) {
	bus := pubsub.NewBus()
	var received atomic.Int32

	bus.Subscribe("test.topic", func(_ context.Context, msg pubsub.Message) error {
		received.Add(1)
		return nil
	})

	err := bus.Publish(context.Background(), "test.topic", []byte("hello"))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if received.Load() != 1 {
		t.Errorf("expected 1, got %d", received.Load())
	}
}

func TestMultipleSubscribers(t *testing.T) {
	bus := pubsub.NewBus()
	var received atomic.Int32

	for i := 0; i < 3; i++ {
		bus.Subscribe("test.topic", func(_ context.Context, msg pubsub.Message) error {
			received.Add(1)
			return nil
		})
	}

	bus.Publish(context.Background(), "test.topic", []byte("hello"))

	if received.Load() != 3 {
		t.Errorf("expected 3, got %d", received.Load())
	}
}

func TestWildcard(t *testing.T) {
	bus := pubsub.NewBus()
	var received atomic.Int32

	bus.Subscribe("*", func(_ context.Context, msg pubsub.Message) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "any.topic", []byte("hello"))
	bus.Publish(context.Background(), "other.topic", []byte("world"))

	if received.Load() != 2 {
		t.Errorf("expected 2, got %d", received.Load())
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := pubsub.NewBus()
	var received atomic.Int32

	subID := bus.Subscribe("test.topic", func(_ context.Context, msg pubsub.Message) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "test.topic", []byte("first"))
	if received.Load() != 1 {
		t.Errorf("expected 1, got %d", received.Load())
	}

	removed := bus.Unsubscribe(subID)
	if !removed {
		t.Error("should find subscription")
	}

	bus.Publish(context.Background(), "test.topic", []byte("second"))
	if received.Load() != 1 {
		t.Errorf("should not receive after unsubscribe, got %d", received.Load())
	}
}

func TestUnsubscribeNotFound(t *testing.T) {
	bus := pubsub.NewBus()
	removed := bus.Unsubscribe("nonexistent")
	if removed {
		t.Error("should not find nonexistent subscription")
	}
}

func TestHistory(t *testing.T) {
	bus := pubsub.NewBus()

	bus.Subscribe("test.topic", func(_ context.Context, _ pubsub.Message) error { return nil })

	bus.Publish(context.Background(), "test.topic", []byte("msg1"))
	bus.Publish(context.Background(), "test.topic", []byte("msg2"))
	bus.Publish(context.Background(), "test.topic", []byte("msg3"))

	history := bus.History("test.topic", 10)
	if len(history) != 3 {
		t.Errorf("expected 3, got %d", len(history))
	}
}

func TestHistoryLimit(t *testing.T) {
	bus := pubsub.NewBus()

	bus.Subscribe("test.topic", func(_ context.Context, _ pubsub.Message) error { return nil })

	for i := 0; i < 10; i++ {
		bus.Publish(context.Background(), "test.topic", []byte("msg"))
	}

	history := bus.History("test.topic", 5)
	if len(history) != 5 {
		t.Errorf("expected 5, got %d", len(history))
	}
}

func TestTopics(t *testing.T) {
	bus := pubsub.NewBus()

	bus.Subscribe("topic1", func(_ context.Context, _ pubsub.Message) error { return nil })
	bus.Subscribe("topic2", func(_ context.Context, _ pubsub.Message) error { return nil })

	topics := bus.Topics()
	if len(topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(topics))
	}
}

func TestSubscribers(t *testing.T) {
	bus := pubsub.NewBus()

	bus.Subscribe("topic1", func(_ context.Context, _ pubsub.Message) error { return nil })
	bus.Subscribe("topic1", func(_ context.Context, _ pubsub.Message) error { return nil })

	if bus.Subscribers("topic1") != 2 {
		t.Errorf("expected 2, got %d", bus.Subscribers("topic1"))
	}
}

func TestWithPublisher(t *testing.T) {
	bus := pubsub.NewBus()

	var publisher string
	bus.Subscribe("test", func(_ context.Context, msg pubsub.Message) error {
		publisher = msg.Publisher
		return nil
	})

	bus.Publish(context.Background(), "test", []byte("hello"),
		pubsub.WithPublisher("agent-1"),
	)

	if publisher != "agent-1" {
		t.Errorf("expected 'agent-1', got %s", publisher)
	}
}

func TestWithHeaders(t *testing.T) {
	bus := pubsub.NewBus()

	var headerVal string
	bus.Subscribe("test", func(_ context.Context, msg pubsub.Message) error {
		headerVal = msg.Headers["x-test"]
		return nil
	})

	bus.Publish(context.Background(), "test", []byte("hello"),
		pubsub.WithHeaders(map[string]string{"x-test": "value"}),
	)

	if headerVal != "value" {
		t.Errorf("expected 'value', got %s", headerVal)
	}
}

func TestRequestResponse(t *testing.T) {
	bus := pubsub.NewBus()
	rr := pubsub.NewRequestResponse(bus, "test")

	// Register a responder
	rr.Respond("echo", func(_ context.Context, req pubsub.Message) ([]byte, error) {
		return append([]byte("echo:"), req.Payload...), nil
	})

	// Send a request
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := rr.Request(ctx, "echo", []byte("hello"), time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if string(resp.Payload) != "echo:hello" {
		t.Errorf("expected 'echo:hello', got %s", string(resp.Payload))
	}
}

func TestRequestTimeout(t *testing.T) {
	bus := pubsub.NewBus()
	rr := pubsub.NewRequestResponse(bus, "timeout")

	// No responder registered
	ctx := context.Background()
	_, err := rr.Request(ctx, "slow", []byte("hello"), 50*time.Millisecond)
	if err == nil {
		t.Error("should timeout")
	}
}

func TestIsolatedTopics(t *testing.T) {
	bus := pubsub.NewBus()
	var topic1Count, topic2Count atomic.Int32

	bus.Subscribe("topic1", func(_ context.Context, _ pubsub.Message) error {
		topic1Count.Add(1)
		return nil
	})
	bus.Subscribe("topic2", func(_ context.Context, _ pubsub.Message) error {
		topic2Count.Add(1)
		return nil
	})

	bus.Publish(context.Background(), "topic1", []byte("msg"))

	if topic1Count.Load() != 1 {
		t.Errorf("topic1 should receive 1, got %d", topic1Count.Load())
	}
	if topic2Count.Load() != 0 {
		t.Errorf("topic2 should receive 0, got %d", topic2Count.Load())
	}
}
