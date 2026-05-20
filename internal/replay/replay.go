// Package replay provides session recording and replay for AI agent interactions.
// The forge remembers every strike of the sword.
package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Event represents a single event in a session recording.
type Event struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"` // "input", "output", "error", "system", "tool_call", "tool_result"
	Content   string      `json:"content"`
	Metadata  interface{} `json:"metadata,omitempty"`
}

// Session represents a recorded session.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Agent     string    `json:"agent"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Events    []Event   `json:"events"`
}

// Recorder records session events.
type Recorder struct {
	session *Session
	path    string
}

// NewRecorder creates a new session recorder.
func NewRecorder(sessionID, agent, model string) *Recorder {
	now := time.Now()
	return &Recorder{
		session: &Session{
			ID:        sessionID,
			Name:      sessionID,
			Agent:     agent,
			Model:     model,
			CreatedAt: now,
			UpdatedAt: now,
			Events:    []Event{},
		},
	}
}

// Record adds an event to the session recording.
func (r *Recorder) Record(eventType, content string) {
	r.session.Events = append(r.session.Events, Event{
		Timestamp: time.Now(),
		Type:      eventType,
		Content:   content,
	})
	r.session.UpdatedAt = time.Now()
}

// RecordWithMetadata adds an event with metadata.
func (r *Recorder) RecordWithMetadata(eventType, content string, metadata interface{}) {
	r.session.Events = append(r.session.Events, Event{
		Timestamp: time.Now(),
		Type:      eventType,
		Content:   content,
		Metadata:  metadata,
	})
	r.session.UpdatedAt = time.Now()
}

// Save persists the session recording to disk.
func (r *Recorder) Save(dir string) error {
	if dir == "" {
		dir = defaultReplayDir()
	}
	os.MkdirAll(dir, 0o755)

	path := filepath.Join(dir, r.session.ID+".json")
	data, err := json.MarshalIndent(r.session, "", "  ")
	if err != nil {
		return fmt.Errorf("replay: marshal: %w", err)
	}
	r.path = path
	return os.WriteFile(path, data, 0o644)
}

// Session returns the recorded session.
func (r *Recorder) Session() *Session {
	return r.session
}

// LoadSession loads a session recording from disk.
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("replay: read %s: %w", path, err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("replay: parse: %w", err)
	}
	return &session, nil
}

// ListSessions lists all recorded sessions.
func ListSessions(dir string) ([]Session, error) {
	if dir == "" {
		dir = defaultReplayDir()
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("replay: list: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		session, err := LoadSession(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		sessions = append(sessions, *session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Replay replays a session's events with optional speed control.
func Replay(session *Session, speed float64, onEvent func(Event)) {
	if speed <= 0 {
		speed = 1.0
	}

	for i, event := range session.Events {
		onEvent(event)

		if i < len(session.Events)-1 {
			delay := session.Events[i+1].Timestamp.Sub(event.Timestamp)
			if delay > 0 {
				scaledDelay := time.Duration(float64(delay) / speed)
				if scaledDelay > 2*time.Second {
					scaledDelay = 2 * time.Second
				}
				time.Sleep(scaledDelay)
			}
		}
	}
}

// Summary generates a text summary of a session.
func Summary(session *Session) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Session: %s\n", session.ID)
	fmt.Fprintf(&b, "Agent:   %s\n", session.Agent)
	fmt.Fprintf(&b, "Model:   %s\n", session.Model)
	fmt.Fprintf(&b, "Created: %s\n", session.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Events:  %d\n", len(session.Events))

	// Count event types
	counts := make(map[string]int)
	for _, e := range session.Events {
		counts[e.Type]++
	}
	fmt.Fprintln(&b, "\nEvent Breakdown:")
	for t, c := range counts {
		fmt.Fprintf(&b, "  %-15s %d\n", t, c)
	}

	// Total tokens estimate (from input/output events)
	var inputLen, outputLen int
	for _, e := range session.Events {
		switch e.Type {
		case "input":
			inputLen += len(e.Content)
		case "output":
			outputLen += len(e.Content)
		}
	}
	fmt.Fprintf(&b, "\n  Input chars:  %d\n", inputLen)
	fmt.Fprintf(&b, "  Output chars: %d\n", outputLen)

	return b.String()
}

func defaultReplayDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "replay")
}
