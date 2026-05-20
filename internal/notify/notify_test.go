package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddChannel(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.AddChannel(&Channel{
		Name:    "test-slack",
		Type:    ChannelSlack,
		URL:     "https://hooks.slack.com/services/test",
		Channel: "#forge",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	channels := m.ListChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "test-slack" {
		t.Errorf("expected test-slack, got %s", channels[0].Name)
	}
	if !channels[0].Enabled {
		t.Error("expected channel to be enabled by default")
	}
}

func TestAddChannelValidation(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	tests := []struct {
		channel *Channel
		wantErr bool
	}{
		{&Channel{Name: "slack-no-url", Type: ChannelSlack}, true},
		{&Channel{Name: "discord-no-url", Type: ChannelDiscord}, true},
		{&Channel{Name: "webhook-no-url", Type: ChannelWebhook}, true},
		{&Channel{Name: "email-no-addr", Type: ChannelEmail}, true},
		{&Channel{Name: "file-ok", Type: ChannelFile}, false},
		{&Channel{Name: "slack-ok", Type: ChannelSlack, URL: "https://hooks.slack.com/test"}, false},
	}

	for _, tt := range tests {
		err := m.AddChannel(tt.channel)
		if tt.wantErr && err == nil {
			t.Errorf("expected error for %s", tt.channel.Name)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("unexpected error for %s: %v", tt.channel.Name, err)
		}
	}
}

func TestRemoveChannel(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddChannel(&Channel{
		Name: "to-remove",
		Type: ChannelSlack,
		URL:  "https://hooks.slack.com/test",
	})

	channels := m.ListChannels()
	id := channels[0].ID

	err := m.RemoveChannel(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.ListChannels()) != 0 {
		t.Error("expected channel to be removed")
	}
}

func TestRemoveChannelNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.RemoveChannel("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent channel")
	}
}

func TestGetChannel(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddChannel(&Channel{
		Name: "test-channel",
		Type: ChannelFile,
	})

	channels := m.ListChannels()
	id := channels[0].ID

	ch, ok := m.GetChannel(id)
	if !ok {
		t.Fatal("expected to find channel")
	}
	if ch.Name != "test-channel" {
		t.Errorf("expected test-channel, got %s", ch.Name)
	}
}

func TestSendFile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddChannel(&Channel{
		Name:     "file-notif",
		Type:     ChannelFile,
		FilePath: filepath.Join(dir, "notifications.log"),
	})

	channels := m.ListChannels()
	id := channels[0].ID

	notif, err := m.Send(id, "Test Title", "Test message body", PriorityNormal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notif.Status != "sent" {
		t.Errorf("expected sent, got %s", notif.Status)
	}

	// Verify file was written
	data, err := os.ReadFile(filepath.Join(dir, "notifications.log"))
	if err != nil {
		t.Fatalf("failed to read notification file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Test Title") {
		t.Errorf("expected notification in file, got: %s", content)
	}
}

func TestSendNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.Send("nonexistent", "Test", "Message", PriorityLow)
	if err == nil {
		t.Error("expected error for nonexistent channel")
	}
}

func TestSendDisabled(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddChannel(&Channel{
		Name:    "disabled-ch",
		Type:    ChannelFile,
		Enabled: false,
	})

	channels := m.ListChannels()
	id := channels[0].ID

	_, err := m.Send(id, "Test", "Message", PriorityLow)
	if err == nil {
		t.Error("expected error for disabled channel")
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddChannel(&Channel{
		Name: "file-ch",
		Type: ChannelFile,
	})

	channels := m.ListChannels()
	id := channels[0].ID

	m.Send(id, "First", "Message 1", PriorityLow)
	m.Send(id, "Second", "Message 2", PriorityNormal)
	m.Send(id, "Third", "Message 3", PriorityHigh)

	history := m.History(0)
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}
}

func TestHistoryLimit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddChannel(&Channel{
		Name: "file-ch",
		Type: ChannelFile,
	})

	channels := m.ListChannels()
	id := channels[0].ID

	for i := 0; i < 5; i++ {
		m.Send(id, "Test", "Message", PriorityLow)
	}

	history := m.History(2)
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
}

func TestTestChannel(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	m.AddChannel(&Channel{
		Name:     "test-file",
		Type:     ChannelFile,
		FilePath: filepath.Join(dir, "test.log"),
	})

	channels := m.ListChannels()
	id := channels[0].ID

	notif, err := m.TestChannel(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notif.Status != "sent" {
		t.Errorf("expected sent, got %s", notif.Status)
	}
	if !strings.Contains(notif.Message, "test notification") {
		t.Errorf("expected test message, got: %s", notif.Message)
	}
}

func TestChannelPersistence(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	m1.AddChannel(&Channel{
		Name:    "persistent-slack",
		Type:    ChannelSlack,
		URL:     "https://hooks.slack.com/test",
		Channel: "#test",
	})

	// New manager should load from file
	m2 := NewManager(dir)
	m2.loadChannels()

	channels := m2.ListChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel after reload, got %d", len(channels))
	}
	if channels[0].Name != "persistent-slack" {
		t.Errorf("expected persistent-slack, got %s", channels[0].Name)
	}
}

func TestNotificationJSON(t *testing.T) {
	notif := Notification{
		ID:        "test-123",
		Title:     "Test",
		Message:   "Hello",
		Priority:  PriorityHigh,
		ChannelID: "ch-1",
		Timestamp: time.Now(),
		Status:    "sent",
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Notification
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Title != "Test" {
		t.Errorf("expected Test, got %s", decoded.Title)
	}
	if decoded.Priority != PriorityHigh {
		t.Errorf("expected high priority, got %s", decoded.Priority)
	}
}
