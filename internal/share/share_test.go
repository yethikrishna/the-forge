package share

import (
	"strings"
	"testing"
	"time"
)

func TestExportHTML(t *testing.T) {
	session := Session{
		Title:   "Test Session",
		Model:   "claude-sonnet-4-20250514",
		Created: time.Date(2026, 5, 20, 19, 0, 0, 0, time.UTC),
		Entries: []SessionEntry{
			{Role: "user", Content: "Hello, forge!", Timestamp: time.Date(2026, 5, 20, 19, 0, 0, 0, time.UTC)},
			{Role: "assistant", Content: "Welcome back.\nReady to build.", Timestamp: time.Date(2026, 5, 20, 19, 0, 1, 0, time.UTC)},
			{Role: "tool", Content: "go build ./...", Timestamp: time.Date(2026, 5, 20, 19, 0, 2, 0, time.UTC), Meta: "build"},
		},
	}

	html, err := ExportHTML(session)
	if err != nil {
		t.Fatalf("ExportHTML failed: %v", err)
	}

	// Verify key content
	checks := []string{
		"Test Session",
		"claude-sonnet-4-20250514",
		"Hello, forge!",
		"Welcome back.<br>Ready to build.",
		"go build ./...",
		"entry-user",
		"entry-assistant",
		"entry-tool",
		"3 entries",
		"May 20, 2026 19:00",
	}

	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("HTML missing expected content: %q", check)
		}
	}
}

func TestExportHTMLEscaping(t *testing.T) {
	session := Session{
		Title: "<script>alert('xss')</script>",
		Entries: []SessionEntry{
			{Role: "user", Content: "<b>bold</b> & <i>italic</i>"},
		},
	}

	html, err := ExportHTML(session)
	if err != nil {
		t.Fatalf("ExportHTML failed: %v", err)
	}

	// Should be escaped
	if strings.Contains(html, "<script>alert('xss')</script>") {
		t.Error("Title should be HTML-escaped")
	}
	if strings.Contains(html, "<b>bold</b>") {
		t.Error("Content should be HTML-escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("Expected escaped script tags")
	}
}

func TestExportHTMLDefaults(t *testing.T) {
	session := Session{}

	html, err := ExportHTML(session)
	if err != nil {
		t.Fatalf("ExportHTML failed: %v", err)
	}

	if !strings.Contains(html, "Forge Session") {
		t.Error("Expected default title 'Forge Session'")
	}
	if !strings.Contains(html, "Unknown") {
		t.Error("Expected default model 'Unknown'")
	}
}

func TestExportMarkdown(t *testing.T) {
	session := Session{
		Title:   "MD Test",
		Model:   "gpt-5-mini",
		Created: time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		Entries: []SessionEntry{
			{Role: "user", Content: "Write a hello world"},
			{Role: "assistant", Content: "Here it is:\n```go\nfmt.Println(\"hello\")\n```"},
			{Role: "tool", Content: "compiled successfully", Meta: "go build"},
		},
	}

	md := ExportMarkdown(session)

	checks := []string{
		"# MD Test",
		"gpt-5-mini",
		"Write a hello world",
		"fmt.Println",
		"go build",
		"👤 User",
		"🤖 Assistant",
		"🔧 Tool",
	}

	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("Markdown missing expected content: %q", check)
		}
	}
}

func TestExportMarkdownEmpty(t *testing.T) {
	session := Session{Title: "Empty"}
	md := ExportMarkdown(session)
	if !strings.Contains(md, "# Empty") {
		t.Error("Expected title in markdown")
	}
}
