package comm

import (
	"path/filepath"
	"testing"
)

func TestCreateChannel(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	ch, err := c.CreateChannel("eng-chat", "div-eng", "division", "Engineering team", []string{"a1", "a2"})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name != "eng-chat" {
		t.Errorf("expected eng-chat, got %s", ch.Name)
	}
	if len(ch.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(ch.Members))
	}

	channels := c.ListChannels("div-eng")
	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}
}

func TestJoinChannel(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	ch, _ := c.CreateChannel("general", "", "org", "General", []string{"a1"})
	err := c.JoinChannel(ch.ID, "a2")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := c.GetChannel(ch.ID)
	if len(got.Members) != 2 {
		t.Errorf("expected 2 members after join, got %d", len(got.Members))
	}

	// Join again should be idempotent
	c.JoinChannel(ch.ID, "a2")
	got, _ = c.GetChannel(ch.ID)
	if len(got.Members) != 2 {
		t.Errorf("expected 2 after duplicate join, got %d", len(got.Members))
	}
}

func TestSendAndRead(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	ch, _ := c.CreateChannel("eng", "div-eng", "division", "Eng", []string{"a1", "a2"})

	msg, err := c.Send("a1", ch.ID, "deployed v2", MsgChat, PrioNormal)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "deployed v2" {
		t.Errorf("unexpected body: %s", msg.Body)
	}

	msgs := c.ReadMessages(ch.ID, 10)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Body != "deployed v2" {
		t.Errorf("message body mismatch")
	}
}

func TestDM(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	msg, err := c.SendDM("a1", "a2", "can you review PR?", PrioNormal)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgDM {
		t.Errorf("expected DM type, got %s", msg.Type)
	}

	msgs := c.ReadDMs("a1", "a2", 10)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(msgs))
	}
}

func TestBroadcast(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	msg, err := c.Broadcast("ceo", "All hands", "Quarterly review Friday", PrioHigh)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgBroadcast {
		t.Errorf("expected broadcast type, got %s", msg.Type)
	}
	if msg.Subject != "All hands" {
		t.Errorf("subject mismatch")
	}
}

func TestMarkReadAndUnread(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	ch, _ := c.CreateChannel("eng", "div-eng", "division", "Eng", []string{"a1", "a2"})
	c.Send("a1", ch.ID, "msg1", MsgChat, PrioNormal)
	c.Send("a1", ch.ID, "msg2", MsgChat, PrioNormal)

	unread := c.UnreadCount("a2", ch.ID)
	if unread != 2 {
		t.Errorf("expected 2 unread, got %d", unread)
	}

	msgs := c.ReadMessages(ch.ID, 10)
	c.MarkRead(msgs[0].ID, "a2")

	unread = c.UnreadCount("a2", ch.ID)
	if unread != 1 {
		t.Errorf("expected 1 unread after marking one, got %d", unread)
	}
}

func TestActivityFeed(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	c.LogActivity("a1", "deployed", "service-foo", "rolled out v2", "engineering")
	c.LogActivity("a2", "committed", "repo-bar", "fix: memory leak", "engineering")
	c.LogActivity("a3", "completed_task", "task-123", "wrote tests", "research")

	feed := c.ActivityFeed("", 10)
	if len(feed) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(feed))
	}

	engFeed := c.ActivityFeed("engineering", 10)
	if len(engFeed) != 2 {
		t.Errorf("expected 2 engineering entries, got %d", len(engFeed))
	}

	limited := c.ActivityFeed("", 2)
	if len(limited) != 2 {
		t.Errorf("expected 2 limited entries, got %d", len(limited))
	}
}

func TestAlerts(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	alert, err := c.CreateAlert("High latency", "API latency > 500ms", PrioCritical, "monitor", []string{"eng-head", "ops"})
	if err != nil {
		t.Fatal(err)
	}

	active := c.ListActiveAlerts("")
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(active))
	}

	c.AcknowledgeAlert(alert.ID, "eng-head")
	alert, _ = c.alerts[alert.ID]
	if len(alert.AcknowledgedBy) != 1 {
		t.Error("should have 1 acknowledgment")
	}

	c.ResolveAlert(alert.ID)
	active = c.ListActiveAlerts("")
	if len(active) != 0 {
		t.Errorf("expected 0 active after resolve, got %d", len(active))
	}
}

func TestReports(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "comm.json"))

	report, err := c.CreateReport(
		"Daily Standup Summary",
		ResolutionSummary,
		"daily",
		"3 agents reported, 1 blocker",
		[]ReportSection{
			{Title: "Completed", Content: "- Feature X shipped"},
			{Title: "Blockers", Content: "- Waiting on API access"},
		},
		"a1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Resolution != ResolutionSummary {
		t.Errorf("expected summary resolution, got %s", report.Resolution)
	}

	reports := c.ListReports("daily")
	if len(reports) != 1 {
		t.Errorf("expected 1 daily report, got %d", len(reports))
	}

	got, _ := c.GetReport(report.ID)
	if got.Title != "Daily Standup Summary" {
		t.Error("report title mismatch")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comm.json")

	c1 := New(path)
	ch, _ := c1.CreateChannel("test", "", "org", "Test", []string{"a1"})
	c1.Send("a1", ch.ID, "hello", MsgChat, PrioNormal)

	c2 := New(path)
	if len(c2.channels) != 1 {
		t.Errorf("expected 1 loaded channel, got %d", len(c2.channels))
	}
	if len(c2.messages) != 1 {
		t.Errorf("expected 1 loaded message, got %d", len(c2.messages))
	}
}
