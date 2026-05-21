package relay_test

import (
	"testing"

	"github.com/forge/sword/internal/relay"
)

func TestSubscribe(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	s := r.Subscribe("agent-1", "build-events", "")

	if s.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", s.AgentID)
	}
	if s.Topic != "build-events" {
		t.Errorf("expected build-events, got %s", s.Topic)
	}
}

func TestUnsubscribe(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	s := r.Subscribe("agent-1", "events", "")

	err := r.Unsubscribe(s.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	subs := r.Subscriptions("events", "")
	if len(subs) != 0 {
		t.Error("expected no subscriptions after unsubscribe")
	}
}

func TestPublish(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "")

	msg := r.Publish("coordinator", "events", "build started", nil)
	if msg.State != relay.StateDelivered {
		t.Errorf("expected delivered, got %s", msg.State)
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	msg := r.Publish("coordinator", "events", "build started", nil)

	// Should go to dead letters
	dl := r.DeadLetters(10)
	if len(dl) == 0 {
		t.Error("expected dead letter for no subscribers")
	}
	if msg.State != relay.StatePending {
		t.Errorf("expected pending, got %s", msg.State)
	}
}

func TestPublishWithFilter(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "error")
	r.Subscribe("agent-2", "events", "")

	r.Publish("coordinator", "events", "build started", nil)
	r.Publish("coordinator", "events", "build error occurred", nil)

	// agent-1 should only get the error message
	d1 := r.Deliveries("agent-1", 0)
	if len(d1) != 1 {
		t.Errorf("expected 1 delivery for agent-1, got %d", len(d1))
	}

	// agent-2 should get both
	d2 := r.Deliveries("agent-2", 0)
	if len(d2) != 2 {
		t.Errorf("expected 2 deliveries for agent-2, got %d", len(d2))
	}
}

func TestDeliveries(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "")
	r.Publish("coordinator", "events", "msg1", nil)
	r.Publish("coordinator", "events", "msg2", nil)

	d := r.Deliveries("agent-1", 0)
	if len(d) != 2 {
		t.Errorf("expected 2 deliveries, got %d", len(d))
	}
}

func TestMessages(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "")
	r.Publish("coordinator", "events", "msg1", nil)

	msgs := r.Messages("events", 0)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}

func TestMessagesLimit(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "")
	for i := 0; i < 10; i++ {
		r.Publish("coordinator", "events", "msg", nil)
	}

	msgs := r.Messages("events", 5)
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages, got %d", len(msgs))
	}
}

func TestRedeliver(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Publish("coordinator", "events", "orphan msg", nil)

	// Now add subscriber
	r.Subscribe("agent-1", "events", "")

	count := r.Redeliver()
	if count != 1 {
		t.Errorf("expected 1 redelivered, got %d", count)
	}

	dl := r.DeadLetters(10)
	if len(dl) != 0 {
		t.Error("expected no dead letters after redeliver")
	}
}

func TestStats(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "")
	r.Publish("coordinator", "events", "msg", nil)

	stats := r.Stats()
	if stats["messages"].(int) != 1 {
		t.Errorf("expected 1 message, got %v", stats["messages"])
	}
}

func TestSubscriptionsFilter(t *testing.T) {
	r := relay.NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "")
	r.Subscribe("agent-2", "events", "")
	r.Subscribe("agent-1", "builds", "")

	subs := r.Subscriptions("", "agent-1")
	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions for agent-1, got %d", len(subs))
	}
}
