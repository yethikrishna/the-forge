// Package behaviorclone provides behavioral cloning for agents.
// It records human task execution (keystrokes, commands, decisions, timing)
// and generates a replayable agent script that can repeat the task.
//
// "Watch the master. Become the master."
package behaviorclone

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EventType represents the type of recorded event.
type EventType string

const (
	EventKeystroke EventType = "keystroke"
	EventCommand   EventType = "command"
	EventDecision  EventType = "decision"
	EventWait      EventType = "wait"
	EventOutput    EventType = "output"
	EventFileRead  EventType = "file-read"
	EventFileWrite EventType = "file-write"
	EventSearch    EventType = "search"
	EventNavigate  EventType = "navigate"
	EventEdit      EventType = "edit"
	EventVerify    EventType = "verify"
)

// Event represents a single recorded action during task execution.
type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Duration  time.Duration     `json:"duration,omitempty"`
	Data      string            `json:"data,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Success   bool              `json:"success"`
}

// Recording represents a complete task recording.
type Recording struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Events      []*Event  `json:"events"`
	Duration    time.Duration `json:"duration"`
	Tags        []string `json:"tags,omitempty"`
	AgentModel  string   `json:"agent_model,omitempty"`
}

// ClonePolicy controls how the cloned agent behaves.
type ClonePolicy struct {
	FastForward    bool          `json:"fast_forward"`     // Skip waits
	AdaptiveTiming bool          `json:"adaptive_timing"`  // Adjust timing based on output
	MaxRetries     int           `json:"max_retries"`      // Retry failed commands
	RetryDelay     time.Duration `json:"retry_delay"`      // Delay between retries
	VerifySteps    bool          `json:"verify_steps"`     // Verify each step succeeded
	StopOnError    bool          `json:"stop_on_error"`    // Stop on first error
	Interactive    bool          `json:"interactive"`      // Prompt for decisions
}

// DefaultPolicy returns a sensible default clone policy.
func DefaultPolicy() ClonePolicy {
	return ClonePolicy{
		FastForward:    true,
		AdaptiveTiming: true,
		MaxRetries:     3,
		RetryDelay:     2 * time.Second,
		VerifySteps:    true,
		StopOnError:    true,
		Interactive:    false,
	}
}

// Script represents a generated agent script.
type Script struct {
	ID          string        `json:"id"`
	RecordingID string        `json:"recording_id"`
	Name        string        `json:"name"`
	Steps       []*ScriptStep `json:"steps"`
	Policy      ClonePolicy   `json:"policy"`
	CreatedAt   time.Time     `json:"created_at"`
}

// ScriptStep represents a step in the generated script.
type ScriptStep struct {
	Order       int           `json:"order"`
	Type        EventType     `json:"type"`
	Action      string        `json:"action"`
	Expect      string        `json:"expect,omitempty"` // Expected output pattern
	Timeout     time.Duration `json:"timeout,omitempty"`
	Retryable   bool          `json:"retryable"`
	Optional    bool          `json:"optional"`
	Description string        `json:"description,omitempty"`
}

// Recorder records human task execution.
type Recorder struct {
	mu        sync.Mutex
	recording *Recording
	startTime time.Time
	lastEvent time.Time
	dir       string
}

// NewRecorder creates a new task recorder.
func NewRecorder(name, dir string) *Recorder {
	return &Recorder{
		recording: &Recording{
			ID:        generateID("rec"),
			Name:      name,
			CreatedAt: time.Now(),
			Events:    make([]*Event, 0),
		},
		dir: dir,
	}
}

// Start begins recording.
func (r *Recorder) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startTime = time.Now()
	r.lastEvent = time.Now()
}

// RecordEvent records a single event.
func (r *Recorder) RecordEvent(eventType EventType, data string, success bool, meta map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	duration := now.Sub(r.lastEvent)
	r.lastEvent = now

	event := &Event{
		ID:        generateID("evt"),
		Type:      eventType,
		Timestamp: now,
		Duration:  duration,
		Data:      data,
		Metadata:  meta,
		Success:   success,
	}

	r.recording.Events = append(r.recording.Events, event)
}

// Stop ends recording and returns the recording.
func (r *Recorder) Stop() *Recording {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.recording.Duration = time.Since(r.startTime)
	return r.recording
}

// Save persists the recording to disk.
func (r *Recorder) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	os.MkdirAll(r.dir, 0o755)
	path := filepath.Join(r.dir, r.recording.ID+".json")
	data, err := json.MarshalIndent(r.recording, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadRecording loads a recording from disk.
func LoadRecording(path string) (*Recording, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rec Recording
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// Generator generates agent scripts from recordings.
type Generator struct {
	mu     sync.Mutex
	policy ClonePolicy
	dir    string
}

// NewGenerator creates a new script generator.
func NewGenerator(policy ClonePolicy, dir string) *Generator {
	return &Generator{
		policy: policy,
		dir:    dir,
	}
}

// Generate creates an agent script from a recording.
func (g *Generator) Generate(_ context.Context, recording *Recording) (*Script, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	script := &Script{
		ID:          generateID("scr"),
		RecordingID: recording.ID,
		Name:        recording.Name + "-clone",
		Policy:      g.policy,
		CreatedAt:   time.Now(),
		Steps:       make([]*ScriptStep, 0),
	}

	// Analyze patterns in the recording
	patterns := g.analyzePatterns(recording)

	order := 0
	for _, event := range recording.Events {
		// Skip wait events in fast-forward mode
		if event.Type == EventWait && g.policy.FastForward {
			continue
		}

		// Skip output events (they're results, not actions)
		if event.Type == EventOutput {
			continue
		}

		order++
		step := &ScriptStep{
			Order:       order,
			Type:        event.Type,
			Action:      event.Data,
			Retryable:   g.isRetryable(event),
			Optional:    !event.Success,
			Description: g.describeStep(event, patterns),
		}

		if g.policy.AdaptiveTiming {
			step.Timeout = event.Duration * 2
			if step.Timeout < 5*time.Second {
				step.Timeout = 5 * time.Second
			}
		}

		script.Steps = append(script.Steps, step)
	}

	return script, nil
}

// Save persists the script to disk.
func (g *Generator) Save(script *Script) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	os.MkdirAll(g.dir, 0o755)
	path := filepath.Join(g.dir, script.ID+".json")
	data, err := json.MarshalIndent(script, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadScript loads a script from disk.
func LoadScript(path string) (*Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var script Script
	if err := json.Unmarshal(data, &script); err != nil {
		return nil, err
	}
	return &script, nil
}

// Pattern represents a detected pattern in task execution.
type Pattern struct {
	Type        string  `json:"type"` // "edit-then-verify", "search-then-navigate", etc.
	Confidence  float64 `json:"confidence"`
	Occurrences int     `json:"occurrences"`
}

// analyzePatterns detects common patterns in a recording.
func (g *Generator) analyzePatterns(recording *Recording) []*Pattern {
	patterns := make([]*Pattern, 0)
	eventTypes := make([]EventType, len(recording.Events))

	for i, e := range recording.Events {
		eventTypes[i] = e.Type
	}

	// Detect edit-then-verify pattern
	editVerifyCount := 0
	for i := 0; i < len(eventTypes)-1; i++ {
		if eventTypes[i] == EventEdit && eventTypes[i+1] == EventVerify {
			editVerifyCount++
		}
	}
	if editVerifyCount > 0 {
		confidence := float64(editVerifyCount) / float64(len(eventTypes))
		patterns = append(patterns, &Pattern{
			Type:        "edit-then-verify",
			Confidence:  confidence,
			Occurrences: editVerifyCount,
		})
	}

	// Detect search-then-navigate pattern
	searchNavCount := 0
	for i := 0; i < len(eventTypes)-1; i++ {
		if eventTypes[i] == EventSearch && eventTypes[i+1] == EventNavigate {
			searchNavCount++
		}
	}
	if searchNavCount > 0 {
		confidence := float64(searchNavCount) / float64(len(eventTypes))
		patterns = append(patterns, &Pattern{
			Type:        "search-then-navigate",
			Confidence:  confidence,
			Occurrences: searchNavCount,
		})
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Confidence > patterns[j].Confidence
	})

	return patterns
}

func (g *Generator) isRetryable(event *Event) bool {
	switch event.Type {
	case EventCommand, EventVerify, EventSearch:
		return true
	default:
		return false
	}
}

func (g *Generator) describeStep(event *Event, patterns []*Pattern) string {
	switch event.Type {
	case EventCommand:
		return fmt.Sprintf("Run command: %s", truncate(event.Data, 80))
	case EventEdit:
		return fmt.Sprintf("Edit file: %s", truncate(event.Data, 80))
	case EventSearch:
		return fmt.Sprintf("Search for: %s", truncate(event.Data, 60))
	case EventNavigate:
		return fmt.Sprintf("Navigate to: %s", truncate(event.Data, 60))
	case EventDecision:
		return fmt.Sprintf("Decide: %s", truncate(event.Data, 60))
	case EventVerify:
		return fmt.Sprintf("Verify: %s", truncate(event.Data, 60))
	case EventFileRead:
		return fmt.Sprintf("Read file: %s", truncate(event.Data, 60))
	case EventFileWrite:
		return fmt.Sprintf("Write file: %s", truncate(event.Data, 60))
	case EventWait:
		return fmt.Sprintf("Wait %s", event.Duration.Round(time.Second))
	default:
		return string(event.Type)
	}
}

// ExportForgefile generates a Forgefile (YAML) from a script.
func ExportForgefile(script *Script) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Auto-generated from recording: %s\n", script.RecordingID))
	sb.WriteString(fmt.Sprintf("# Clone: %s\n\n", script.Name))
	sb.WriteString("tasks:\n")

	for _, step := range script.Steps {
		sb.WriteString(fmt.Sprintf("  - name: step-%d\n", step.Order))
		sb.WriteString(fmt.Sprintf("    type: %s\n", step.Type))
		sb.WriteString(fmt.Sprintf("    action: %q\n", step.Action))
		if step.Expect != "" {
			sb.WriteString(fmt.Sprintf("    expect: %q\n", step.Expect))
		}
		if step.Timeout > 0 {
			sb.WriteString(fmt.Sprintf("    timeout: %s\n", step.Timeout))
		}
		if step.Retryable {
			sb.WriteString("    retry: true\n")
		}
		if step.Optional {
			sb.WriteString("    optional: true\n")
		}
		sb.WriteString(fmt.Sprintf("    # %s\n", step.Description))
	}

	return sb.String()
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
