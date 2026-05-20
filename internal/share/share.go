// Package share provides session export as self-contained HTML.
// Share your forge sessions like Jupyter notebooks.
package share

import (
	_ "embed"
	"fmt"
	"html"
	"strings"
	"time"
)

// SessionEntry is a single message or action in a session.
type SessionEntry struct {
	Role      string    // "user", "assistant", "system", "tool"
	Content   string    // Text content
	Timestamp time.Time // When this entry was created
	Meta      string    // Optional metadata (e.g., tool name, duration)
}

// Session is a collection of entries representing a full interaction.
type Session struct {
	Title   string
	Created time.Time
	Model   string
	Entries []SessionEntry
}

//go:embed template.html
var htmlTemplate string

// ExportHTML generates a self-contained HTML page from a session.
func ExportHTML(session Session) (string, error) {
	var entriesHTML strings.Builder

	for i, entry := range session.Entries {
		cssClass := "entry-" + entry.Role
		if entry.Role == "tool" {
			cssClass = "entry-tool"
		}

		// Escape HTML in content
		content := html.EscapeString(entry.Content)
		// Preserve line breaks
		content = strings.ReplaceAll(content, "\n", "<br>")

		// Format timestamp
		ts := entry.Timestamp.Format("15:04:05")
		if entry.Timestamp.IsZero() {
			ts = ""
		}

		meta := ""
		if entry.Meta != "" {
			meta = fmt.Sprintf(`<span class="meta">%s</span>`, html.EscapeString(entry.Meta))
		}

		entriesHTML.WriteString(fmt.Sprintf(
			`<div class="entry %s" id="entry-%d">
  <div class="entry-header">
    <span class="role">%s</span>
    <span class="timestamp">%s</span>
    %s
  </div>
  <div class="entry-content">%s</div>
</div>
`,
			cssClass, i,
			html.EscapeString(entry.Role),
			ts,
			meta,
			content,
		))
	}

	title := html.EscapeString(session.Title)
	if title == "" {
		title = "Forge Session"
	}

	created := session.Created.Format("January 2, 2006 15:04")
	if session.Created.IsZero() {
		created = time.Now().Format("January 2, 2006 15:04")
	}

	model := html.EscapeString(session.Model)
	if model == "" {
		model = "Unknown"
	}

	// Replace template placeholders
	result := strings.ReplaceAll(htmlTemplate, "{{TITLE}}", title)
	result = strings.ReplaceAll(result, "{{CREATED}}", created)
	result = strings.ReplaceAll(result, "{{MODEL}}", model)
	result = strings.ReplaceAll(result, "{{ENTRIES}}", entriesHTML.String())
	result = strings.ReplaceAll(result, "{{ENTRY_COUNT}}", fmt.Sprintf("%d", len(session.Entries)))

	return result, nil
}

// ExportMarkdown generates a Markdown export of a session.
func ExportMarkdown(session Session) string {
	var b strings.Builder

	b.WriteString("# ")
	b.WriteString(session.Title)
	b.WriteString("\n\n")

	if !session.Created.IsZero() {
		b.WriteString(fmt.Sprintf("**Date:** %s  \n", session.Created.Format("January 2, 2006 15:04")))
	}
	if session.Model != "" {
		b.WriteString(fmt.Sprintf("**Model:** %s  \n", session.Model))
	}
	b.WriteString(fmt.Sprintf("**Entries:** %d  \n", len(session.Entries)))
	b.WriteString("\n---\n\n")

	for _, entry := range session.Entries {
		switch entry.Role {
		case "user":
			b.WriteString("## 👤 User\n\n")
		case "assistant":
			b.WriteString("## 🤖 Assistant\n\n")
		case "system":
			b.WriteString("## ⚙️ System\n\n")
		case "tool":
			b.WriteString("## 🔧 Tool")
			if entry.Meta != "" {
				b.WriteString(fmt.Sprintf(" (%s)", entry.Meta))
			}
			b.WriteString("\n\n")
		default:
			b.WriteString(fmt.Sprintf("## %s\n\n", entry.Role))
		}

		if !entry.Timestamp.IsZero() {
			b.WriteString(fmt.Sprintf("*%s*\n\n", entry.Timestamp.Format("15:04:05")))
		}

		b.WriteString(entry.Content)
		b.WriteString("\n\n---\n\n")
	}

	return b.String()
}
