package relay

import (
	"testing"
	"time"
)

func TestNewRelay(t *testing.T) {
	r := NewRelay(t.TempDir())
	if r == nil {
		t.Fatal("NewRelay should return a relay")
	}
}

func TestSendDirect(t *testing.T) {
	r := NewRelay(t.TempDir())
	msg, err := r.Send(Message{From: "agent-1", To: "agent-2", Subject: "hello", Body: "world"})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if msg.ID == "" {
		t.Error("Message should have an ID")
	}
	if msg.State != StateQueued {
		t.Errorf("State = %q, want %q", msg.State, StateQueued)
	}
}

func TestReceive(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{From: "agent-1", To: "agent-2", Subject: "hello"})

	msg, err := r.Receive("agent-2")
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	if msg == nil {
		t.Fatal("Should receive a message")
	}
	if msg.Subject != "hello" {
		t.Errorf("Subject = %q, want %q", msg.Subject, "hello")
	}
	if msg.State != StateDelivered {
		t.Errorf("State = %q, want %q", msg.State, StateDelivered)
	}
}

func TestReceiveEmpty(t *testing.T) {
	r := NewRelay(t.TempDir())
	msg, _ := r.Receive("agent-1")
	if msg != nil {
		t.Error("Should return nil for empty queue")
	}
}

func TestAck(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{From: "a", To: "b", Subject: "test"})
	msg, _ := r.Receive("b")

	err := r.Ack(msg.ID)
	if err != nil {
		t.Fatalf("Ack error: %v", err)
	}

	m, _ := r.Message(msg.ID)
	if m.State != StateAcked {
		t.Errorf("State = %q, want %q", m.State, StateAcked)
	}
}

func TestAckNotFound(t *testing.T) {
	r := NewRelay(t.TempDir())
	err := r.Ack("nonexistent")
	if err == nil {
		t.Error("Should error for nonexistent message")
	}
}

func TestNackRetry(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{From: "a", To: "b", Subject: "test", MaxRetries: 3})
	msg, _ := r.Receive("b")

	r.Nack(msg.ID)

	m, _ := r.Message(msg.ID)
	if m.Retries != 1 {
		t.Errorf("Retries = %d, want 1", m.Retries)
	}
}

func TestNackDeadLetter(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{From: "a", To: "b", Subject: "test", MaxRetries: 1})
	msg, _ := r.Receive("b")

	r.Nack(msg.ID)

	dl := r.DeadLetters()
	if len(dl) != 1 {
		t.Fatalf("DeadLetters = %d, want 1", len(dl))
	}
	if dl[0].State != StateDeadLetter {
		t.Errorf("State = %q, want %q", dl[0].State, StateDeadLetter)
	}
}

func TestSubscribe(t *testing.T) {
	r := NewRelay(t.TempDir())
	sub, err := r.Subscribe("agent-1", "alerts", "")
	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}
	if sub.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", sub.AgentID, "agent-1")
	}
}

func TestUnsubscribe(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Subscribe("agent-1", "alerts", "")
	r.Unsubscribe("agent-1", "alerts")

	subs := r.Subscriptions("alerts")
	if len(subs) != 0 {
		t.Error("Should have no subscriptions after unsubscribe")
	}
}

func TestPubSub(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Subscribe("agent-1", "events", "")
	r.Subscribe("agent-2", "events", "")

	r.Send(Message{From: "publisher", Channel: "events", Subject: "deploy", Body: "deployed v2"})

	msg1, _ := r.Receive("agent-1")
	if msg1 == nil || msg1.Subject != "deploy" {
		t.Error("agent-1 should receive the event")
	}

	msg2, _ := r.Receive("agent-2")
	if msg2 == nil || msg2.Subject != "deploy" {
		t.Error("agent-2 should receive the event")
	}
}

func TestBroadcast(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Subscribe("agent-1", "broadcast", "")
	r.Subscribe("agent-2", "broadcast", "")

	r.Send(Message{From: "system", To: "broadcast", Subject: "shutdown", Body: "shutting down in 5 min"})

	msg1, _ := r.Receive("agent-1")
	if msg1 == nil {
		t.Error("agent-1 should receive broadcast")
	}

	msg2, _ := r.Receive("agent-2")
	if msg2 == nil {
		t.Error("agent-2 should receive broadcast")
	}
}

func TestPendingCount(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{From: "a", To: "b", Subject: "msg1"})
	r.Send(Message{From: "a", To: "b", Subject: "msg2"})

	if r.PendingCount("b") != 2 {
		t.Errorf("PendingCount = %d, want 2", r.PendingCount("b"))
	}
}

func TestChannelStats(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Subscribe("agent-1", "alerts", "")
	r.Subscribe("agent-2", "alerts", "")
	r.Subscribe("agent-3", "events", "")

	stats := r.ChannelStats()
	if len(stats) != 2 {
		t.Errorf("ChannelStats = %d channels, want 2", len(stats))
	}
}

func TestStats(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{From: "a", To: "b", Subject: "test"})

	stats := r.Stats()
	if stats.TotalSent != 1 {
		t.Errorf("TotalSent = %d, want 1", stats.TotalSent)
	}
}

func TestExpiredMessages(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{
		From:      "a",
		To:        "b",
		Subject:   "expires",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // already expired
	})

	count := r.Expire()
	if count != 1 {
		t.Errorf("Expired = %d, want 1", count)
	}
}

func TestExportMarkdown(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Subscribe("agent-1", "alerts", "")
	r.Send(Message{From: "a", To: "agent-1", Subject: "test"})

	md := r.ExportMarkdown()
	if md == "" {
		t.Error("ExportMarkdown should not be empty")
	}
}

func TestMessageFIFO(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{From: "a", To: "b", Subject: "first"})
	r.Send(Message{From: "a", To: "b", Subject: "second"})

	msg1, _ := r.Receive("b")
	if msg1.Subject != "first" {
		t.Errorf("First message = %q, want %q", msg1.Subject, "first")
	}

	msg2, _ := r.Receive("b")
	if msg2.Subject != "second" {
		t.Errorf("Second message = %q, want %q", msg2.Subject, "second")
	}
}

func TestCorrelationID(t *testing.T) {
	r := NewRelay(t.TempDir())
	r.Send(Message{
		From:          "requester",
		To:            "responder",
		Subject:       "request",
		CorrelationID: "corr-123",
		ReplyTo:       "responses",
	})

	msg, _ := r.Receive("responder")
	if msg.CorrelationID != "corr-123" {
		t.Errorf("CorrelationID = %q, want %q", msg.CorrelationID, "corr-123")
	}
	if msg.ReplyTo != "responses" {
		t.Errorf("ReplyTo = %q, want %q", msg.ReplyTo, "responses")
	}
}
