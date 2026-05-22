package integration

import (
	"testing"
	"time"
)

// --- Browser ---

func TestBrowserOpenSession(t *testing.T) {
	bm := NewBrowserManager()
	sess, err := bm.OpenSession("https://example.com", "scrape data")
	if err != nil {
		t.Fatal(err)
	}
	if sess.URL != "https://example.com" {
		t.Errorf("expected URL, got %s", sess.URL)
	}
	if sess.Status != "active" {
		t.Errorf("expected active, got %s", sess.Status)
	}
}

func TestBrowserCloseSession(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")
	bm.CloseSession(sess.ID)
	if len(bm.ListSessions()) != 0 {
		t.Error("expected 0 sessions")
	}
}

func TestBrowserExecute(t *testing.T) {
	bm := NewBrowserManager()
	sess, _ := bm.OpenSession("https://example.com", "test")

	result, err := bm.Execute(nil, sess.ID, BrowserAction{Type: "navigate", URL: "https://test.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

// --- Email ---

func TestEmailSend(t *testing.T) {
	em := NewEmailManager(EmailConfig{FromAddress: "agent@forge.dev"})
	msg, err := em.Send([]string{"user@example.com"}, "Test", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Test" {
		t.Errorf("expected Test, got %s", msg.Subject)
	}
	if len(em.Outbox()) != 1 {
		t.Error("expected 1 in outbox")
	}
}

func TestEmailReceive(t *testing.T) {
	em := NewEmailManager(EmailConfig{})
	em.Receive(EmailMessage{From: "test@test.com", Subject: "Hi", Body: "Hello"})
	if len(em.Inbox()) != 1 {
		t.Error("expected 1 in inbox")
	}
}

func TestEmailUnread(t *testing.T) {
	em := NewEmailManager(EmailConfig{})
	em.Receive(EmailMessage{From: "a@b.com", Subject: "1", Read: true})
	em.Receive(EmailMessage{From: "c@d.com", Subject: "2", Read: false})
	if len(em.Unread()) != 1 {
		t.Errorf("expected 1 unread, got %d", len(em.Unread()))
	}
}

func TestEmailSearch(t *testing.T) {
	em := NewEmailManager(EmailConfig{})
	em.Receive(EmailMessage{Subject: "Deploy succeeded", Body: "v1.2.3"})
	em.Receive(EmailMessage{Subject: "Bug report", Body: "Something broke"})
	if len(em.Search("Deploy")) != 1 {
		t.Error("expected 1 result")
	}
}

func TestEmailReply(t *testing.T) {
	em := NewEmailManager(EmailConfig{FromAddress: "agent@forge.dev"})
	orig := &EmailMessage{From: "user@test.com", Subject: "Question"}
	reply, err := em.Reply(orig, "Here's the answer")
	if err != nil {
		t.Fatal(err)
	}
	if reply.Subject != "Re: Question" {
		t.Errorf("expected Re: Question, got %s", reply.Subject)
	}
}

func TestEmailNoRecipients(t *testing.T) {
	em := NewEmailManager(EmailConfig{})
	_, err := em.Send(nil, "Test", "Body")
	if err == nil {
		t.Error("expected error")
	}
}

func TestEmailMarkRead(t *testing.T) {
	em := NewEmailManager(EmailConfig{})
	em.Receive(EmailMessage{From: "a@b.com", Subject: "test"})
	msgs := em.Inbox()
	em.MarkRead(msgs[0].ID)
	if len(em.Unread()) != 0 {
		t.Error("expected 0 unread")
	}
}

// --- Payment ---

func TestPaymentChargeRuntime(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{Provider: PaymentStripe, TestMode: true})
	pay, err := pm.Charge(29.99, "Pro plan", nil)
	if err != nil {
		t.Fatal(err)
	}
	if pay.Status != PaymentCompleted {
		t.Errorf("expected completed in test mode, got %s", pay.Status)
	}
	if pm.Total() != 29.99 {
		t.Errorf("expected 29.99, got %f", pm.Total())
	}
}

func TestPaymentRefundRuntime(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{TestMode: true})
	pay, _ := pm.Charge(10, "Test", nil)
	refunded, err := pm.Refund(pay.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if refunded.Status != PaymentRefunded {
		t.Errorf("expected refunded, got %s", refunded.Status)
	}
}

func TestPaymentNegativeAmountRuntime(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{})
	_, err := pm.Charge(-5, "Bad", nil)
	if err == nil {
		t.Error("expected error for negative amount")
	}
}

func TestPaymentOverLimitRuntime(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{})
	_, err := pm.Charge(15000, "Big", nil)
	if err == nil {
		t.Error("expected error for over-limit")
	}
}

// --- Calendar ---

func TestCalendarCreateEvent(t *testing.T) {
	cm := NewCalendarManager(CalendarConfig{})
	now := time.Now()
	event, err := cm.CreateEvent("Standup", now, now.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if event.Title != "Standup" {
		t.Errorf("expected Standup, got %s", event.Title)
	}
}

func TestCalendarConflicts(t *testing.T) {
	cm := NewCalendarManager(CalendarConfig{})
	now := time.Now()
	cm.CreateEvent("Meeting A", now, now.Add(time.Hour))
	cm.CreateEvent("Meeting B", now.Add(2*time.Hour), now.Add(3*time.Hour))

	conflicts := cm.Conflicts(now.Add(30*time.Minute), now.Add(90*time.Minute))
	if len(conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(conflicts))
	}
}

func TestCalendarUpcoming(t *testing.T) {
	cm := NewCalendarManager(CalendarConfig{})
	cm.CreateEvent("Soon", time.Now().Add(1*time.Hour), time.Now().Add(2*time.Hour))
	upcoming := cm.Upcoming(3)
	if len(upcoming) != 1 {
		t.Errorf("expected 1 upcoming, got %d", len(upcoming))
	}
}

func TestCalendarDeleteEvent(t *testing.T) {
	cm := NewCalendarManager(CalendarConfig{})
	event, _ := cm.CreateEvent("Delete me", time.Now(), time.Now().Add(time.Hour))
	cm.DeleteEvent(event.ID)
	if len(cm.ListEvents(time.Time{}, time.Time{})) != 0 {
		t.Error("expected 0 events")
	}
}

// --- GitHub ---

func TestGitHubCreatePR(t *testing.T) {
	gh := NewGitHubManager(GitHubConfig{Owner: "forge", Repo: "sword"})
	pr, err := gh.CreatePR("Add feature", "Description", "feature-branch", "main")
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 1 {
		t.Errorf("expected PR #1, got #%d", pr.Number)
	}
	if pr.State != GHOpen {
		t.Errorf("expected open, got %s", pr.State)
	}
}

func TestGitHubMergePR(t *testing.T) {
	gh := NewGitHubManager(GitHubConfig{})
	pr, _ := gh.CreatePR("Merge me", "", "branch", "main")
	merged, err := gh.MergePR(pr.Number)
	if err != nil {
		t.Fatal(err)
	}
	if merged.State != GHMerged {
		t.Errorf("expected merged, got %s", merged.State)
	}
}

func TestGitHubCreateIssue(t *testing.T) {
	gh := NewGitHubManager(GitHubConfig{})
	issue, err := gh.CreateIssue("Bug found", "Something broke", []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Number != 1 {
		t.Errorf("expected #1, got #%d", issue.Number)
	}
	if len(issue.Labels) != 1 {
		t.Errorf("expected 1 label, got %d", len(issue.Labels))
	}
}

func TestGitHubReview(t *testing.T) {
	gh := NewGitHubManager(GitHubConfig{})
	pr, _ := gh.CreatePR("Review me", "", "branch", "main")
	review, err := gh.AddReview(pr.Number, "approved", "LGTM!", "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if review.State != "approved" {
		t.Errorf("expected approved, got %s", review.State)
	}
	reviews, _ := gh.GetReviews(pr.Number)
	if len(reviews) != 1 {
		t.Errorf("expected 1 review, got %d", len(reviews))
	}
}
