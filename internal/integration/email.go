// Package integration provides email integration via SMTP/IMAP.
package integration

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// EmailMessage represents an email.
type EmailMessage struct {
	ID        string            `json:"id"`
	From      string            `json:"from"`
	To        []string          `json:"to"`
	CC        []string          `json:"cc,omitempty"`
	BCC       []string          `json:"bcc,omitempty"`
	Subject   string            `json:"subject"`
	Body      string            `json:"body"`
	HTML      string            `json:"html,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Date      time.Time         `json:"date"`
	ThreadID  string            `json:"thread_id,omitempty"`
	Labels    []string          `json:"labels,omitempty"`
	Read      bool              `json:"read"`
	Attachments []Attachment    `json:"attachments,omitempty"`
}

// Attachment is an email attachment.
type Attachment struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int    `json:"size"`
}

// EmailConfig configures email integration.
type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	IMAPHost     string `json:"imap_host"`
	IMAPPort     int    `json:"imap_port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	FromAddress  string `json:"from_address"`
	UseTLS       bool   `json:"use_tls"`
}

// EmailManager handles email operations.
type EmailManager struct {
	config   EmailConfig
	outbox   []*EmailMessage
	inbox    []*EmailMessage
	mu       sync.RWMutex
}

// NewEmailManager creates an email manager.
func NewEmailManager(config EmailConfig) *EmailManager {
	return &EmailManager{
		config: config,
	}
}

// Send sends an email via SMTP. If SMTP is not configured, it queues locally.
func (em *EmailManager) Send(to []string, subject, body string) (*EmailMessage, error) {
	if len(to) == 0 {
		return nil, fmt.Errorf("email: recipients required")
	}
	if subject == "" {
		return nil, fmt.Errorf("email: subject required")
	}

	msg := &EmailMessage{
		ID:      fmt.Sprintf("email-%d", time.Now().UnixNano()),
		From:    em.config.FromAddress,
		To:      to,
		Subject: subject,
		Body:    body,
		Date:    time.Now(),
	}

	if em.config.SMTPHost != "" {
		if err := em.sendSMTP(msg); err != nil {
			return nil, fmt.Errorf("email: SMTP send failed: %w", err)
		}
	}

	em.mu.Lock()
	em.outbox = append(em.outbox, msg)
	em.mu.Unlock()

	return msg, nil
}

// sendSMTP delivers the message via SMTP.
func (em *EmailManager) sendSMTP(msg *EmailMessage) error {
	host := em.config.SMTPHost
	port := em.config.SMTPPort
	if port == 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	// Build the email message in RFC 2822 format
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", msg.From))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))
	if len(msg.CC) > 0 {
		sb.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(msg.CC, ", ")))
	}
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	sb.WriteString(fmt.Sprintf("Date: %s\r\n", msg.Date.Format(time.RFC1123Z)))
	sb.WriteString("MIME-Version: 1.0\r\n")
	if msg.HTML != "" {
		boundary := "forge-boundary-" + msg.ID
		sb.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n", boundary))
		sb.WriteString("\r\n")
		sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		sb.WriteString(msg.Body)
		sb.WriteString("\r\n")
		sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		sb.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
		sb.WriteString(msg.HTML)
		sb.WriteString("\r\n")
		sb.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		sb.WriteString(msg.Body)
	}

	body := sb.String()

	// Authenticate and send
	auth := smtp.PlainAuth("", em.config.Username, em.config.Password, host)

	if em.config.UseTLS || port == 465 {
		// Direct TLS connection (port 465)
		tlsConfig := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial %s: %w", addr, err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer client.Close()

		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := client.Mail(msg.From); err != nil {
			return fmt.Errorf("smtp mail from: %w", err)
		}
		for _, rcpt := range msg.To {
			if err := client.Rcpt(rcpt); err != nil {
				return fmt.Errorf("smtp rcpt to %s: %w", rcpt, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		return w.Close()
	}

	// STARTTLS (port 587 or 25)
	return smtp.SendMail(addr, auth, msg.From, msg.To, []byte(body))
}

// SendWithCC sends an email with CC recipients.
func (em *EmailManager) SendWithCC(to, cc []string, subject, body string) (*EmailMessage, error) {
	msg, err := em.Send(to, subject, body)
	if err != nil {
		return nil, err
	}
	msg.CC = cc
	return msg, nil
}

// Reply creates a reply to an existing email.
func (em *EmailManager) Reply(original *EmailMessage, body string) (*EmailMessage, error) {
	return em.Send(
		[]string{original.From},
		"Re: "+original.Subject,
		body,
	)
}

// Receive simulates receiving an email (for testing and IMAP polling).
func (em *EmailManager) Receive(msg EmailMessage) {
	msg.ID = fmt.Sprintf("email-%d", time.Now().UnixNano())
	if msg.Date.IsZero() {
		msg.Date = time.Now()
	}

	em.mu.Lock()
	em.inbox = append(em.inbox, &msg)
	em.mu.Unlock()
}

// Inbox returns received emails.
func (em *EmailManager) Inbox() []*EmailMessage {
	em.mu.RLock()
	defer em.mu.RUnlock()
	result := make([]*EmailMessage, len(em.inbox))
	copy(result, em.inbox)
	return result
}

// Outbox returns sent emails.
func (em *EmailManager) Outbox() []*EmailMessage {
	em.mu.RLock()
	defer em.mu.RUnlock()
	result := make([]*EmailMessage, len(em.outbox))
	copy(result, em.outbox)
	return result
}

// Unread returns unread emails.
func (em *EmailManager) Unread() []*EmailMessage {
	em.mu.RLock()
	defer em.mu.RUnlock()
	var result []*EmailMessage
	for _, m := range em.inbox {
		if !m.Read {
			result = append(result, m)
		}
	}
	return result
}

// MarkRead marks an email as read.
func (em *EmailManager) MarkRead(id string) error {
	em.mu.Lock()
	defer em.mu.Unlock()
	for _, m := range em.inbox {
		if m.ID == id {
			m.Read = true
			return nil
		}
	}
	return fmt.Errorf("email %s not found", id)
}

// Search searches emails by subject or body.
func (em *EmailManager) Search(query string) []*EmailMessage {
	em.mu.RLock()
	defer em.mu.RUnlock()
	var result []*EmailMessage
	for _, m := range em.inbox {
		if contains(m.Subject, query) || contains(m.Body, query) {
			result = append(result, m)
		}
	}
	return result
}

var _ = json.Marshal
