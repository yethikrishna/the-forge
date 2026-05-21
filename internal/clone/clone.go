// Package clone records human task execution and creates agent
// configurations that can repeat the recorded behavior.
//
// Watch, learn, repeat.
package clone

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Recording is a captured human task session.
type Recording struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Steps       []Step     `json:"steps"`
	Tags        []string   `json:"tags"`
	CreatedAt   time.Time  `json:"created_at"`
	Status      string     `json:"status"` // recording, done
}

// Step is a single recorded action.
type Step struct {
	Index     int               `json:"index"`
	Type      string            `json:"type"` // command, edit, search, browse, decision
	Content   string            `json:"content"`
	Target    string            `json:"target,omitempty"`
	Result    string            `json:"result,omitempty"`
	Duration  time.Duration     `json:"duration"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Behavior is a generated agent behavior from a recording.
type Behavior struct {
	ID           string    `json:"id"`
	RecordingID  string    `json:"recording_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Instructions string    `json:"instructions"`
	Patterns     []Pattern `json:"patterns"`
	Tags         []string  `json:"tags"`
	Uses         int       `json:"uses"`
	CreatedAt    time.Time `json:"created_at"`
}

// Pattern is a detected behavior pattern.
type Pattern struct {
	Name        string   `json:"name"`
	Trigger     string   `json:"trigger"`
	Action      string   `json:"action"`
	Conditions  []string `json:"conditions,omitempty"`
	Frequency   int      `json:"frequency"`
}

// Recorder records human task execution.
type Recorder struct {
	recordings map[string]*Recording
	behaviors  map[string]*Behavior
	storeDir   string
	nextID     int
	behID      int
	active     *Recording
	mu         sync.RWMutex
}

// NewRecorder creates a behavior recorder.
func NewRecorder(storeDir string) *Recorder {
	r := &Recorder{
		recordings: make(map[string]*Recording),
		behaviors:  make(map[string]*Behavior),
		storeDir:   storeDir,
	}
	r.load()
	return r
}

// StartRecording starts a new recording session.
func (r *Recorder) StartRecording(name, description string) *Recording {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	rec := &Recording{
		ID:          fmt.Sprintf("rec-%d", r.nextID),
		Name:        name,
		Description: description,
		Steps:       []Step{},
		Tags:        []string{},
		CreatedAt:   time.Now(),
		Status:      "recording",
	}
	r.recordings[rec.ID] = rec
	r.active = rec
	r.save()
	return rec
}

// RecordStep adds a step to the active recording.
func (r *Recorder) RecordStep(stepType, content, target, result string, duration time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active == nil {
		return fmt.Errorf("no active recording")
	}

	step := Step{
		Index:    len(r.active.Steps) + 1,
		Type:     stepType,
		Content:  content,
		Target:   target,
		Result:   result,
		Duration: duration,
		Metadata: make(map[string]string),
	}
	r.active.Steps = append(r.active.Steps, step)
	r.save()
	return nil
}

// StopRecording stops the active recording.
func (r *Recorder) StopRecording() (*Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active == nil {
		return nil, fmt.Errorf("no active recording")
	}
	r.active.Status = "done"
	rec := r.active
	r.active = nil
	r.save()
	return rec, nil
}

// GenerateBehavior generates an agent behavior from a recording.
func (r *Recorder) GenerateBehavior(recordingID string) (*Behavior, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.recordings[recordingID]
	if !ok {
		return nil, fmt.Errorf("recording %q not found", recordingID)
	}
	if rec.Status != "done" {
		return nil, fmt.Errorf("recording is still in progress")
	}
	if len(rec.Steps) == 0 {
		return nil, fmt.Errorf("recording has no steps")
	}

	r.behID++
	behavior := &Behavior{
		ID:           fmt.Sprintf("beh-%d", r.behID),
		RecordingID:  recordingID,
		Name:         rec.Name,
		Description:  fmt.Sprintf("Generated from recording: %s", rec.Name),
		Instructions: generateInstructions(rec.Steps),
		Patterns:     detectPatterns(rec.Steps),
		Tags:         extractBehaviorTags(rec.Steps),
		CreatedAt:    time.Now(),
	}

	r.behaviors[behavior.ID] = behavior
	r.save()
	return behavior, nil
}

// GetRecording returns a recording.
func (r *Recorder) GetRecording(id string) (*Recording, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.recordings[id]
	if !ok {
		return nil, false
	}
	copy := *rec
	return &copy, true
}

// GetBehavior returns a behavior.
func (r *Recorder) GetBehavior(id string) (*Behavior, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.behaviors[id]
	if !ok {
		return nil, false
	}
	copy := *b
	return &copy, true
}

// ListRecordings returns all recordings.
func (r *Recorder) ListRecordings() []Recording {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Recording, 0, len(r.recordings))
	for _, rec := range r.recordings {
		result = append(result, *rec)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListBehaviors returns all behaviors.
func (r *Recorder) ListBehaviors() []Behavior {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Behavior, 0, len(r.behaviors))
	for _, b := range r.behaviors {
		result = append(result, *b)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// RecordBehaviorUse records a behavior usage.
func (r *Recorder) RecordBehaviorUse(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.behaviors[id]
	if !ok {
		return fmt.Errorf("behavior %q not found", id)
	}
	b.Uses++
	r.save()
	return nil
}

// DeleteRecording removes a recording.
func (r *Recorder) DeleteRecording(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.recordings[id]; !ok {
		return fmt.Errorf("not found")
	}
	delete(r.recordings, id)
	r.save()
	return nil
}

// DeleteBehavior removes a behavior.
func (r *Recorder) DeleteBehavior(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.behaviors[id]; !ok {
		return fmt.Errorf("not found")
	}
	delete(r.behaviors, id)
	r.save()
	return nil
}

func generateInstructions(steps []Step) string {
	var b strings.Builder
	b.WriteString("Follow these steps in order:\n\n")
	for _, s := range steps {
		b.WriteString(fmt.Sprintf("%d. [%s] %s\n", s.Index, s.Type, s.Content))
		if s.Target != "" {
			b.WriteString(fmt.Sprintf("   Target: %s\n", s.Target))
		}
		if s.Result != "" {
			b.WriteString(fmt.Sprintf("   Expected result: %s\n", truncate(s.Result, 100)))
		}
	}
	return b.String()
}

func detectPatterns(steps []Step) []Pattern {
	// Count step types
	typeCount := make(map[string]int)
	for _, s := range steps {
		typeCount[s.Type]++
	}

	var patterns []Pattern

	// Command pattern
	if typeCount["command"] > 0 {
		patterns = append(patterns, Pattern{
			Name:      "command_execution",
			Trigger:   "task requires running commands",
			Action:    "execute shell commands",
			Frequency: typeCount["command"],
		})
	}

	// Edit pattern
	if typeCount["edit"] > 0 {
		patterns = append(patterns, Pattern{
			Name:      "file_editing",
			Trigger:   "task requires modifying files",
			Action:    "edit source files",
			Frequency: typeCount["edit"],
		})
	}

	// Search pattern
	if typeCount["search"] > 0 {
		patterns = append(patterns, Pattern{
			Name:      "information_gathering",
			Trigger:   "task requires understanding context",
			Action:    "search and read relevant files",
			Frequency: typeCount["search"],
		})
	}

	// Decision pattern
	if typeCount["decision"] > 0 {
		patterns = append(patterns, Pattern{
			Name:      "decision_making",
			Trigger:   "multiple approaches available",
			Action:    "evaluate options and choose best path",
			Frequency: typeCount["decision"],
		})
	}

	// Browse pattern
	if typeCount["browse"] > 0 {
		patterns = append(patterns, Pattern{
			Name:      "web_research",
			Trigger:   "need external information",
			Action:    "search web and browse documentation",
			Frequency: typeCount["browse"],
		})
	}

	return patterns
}

func extractBehaviorTags(steps []Step) []string {
	tagSet := make(map[string]bool)
	for _, s := range steps {
		tagSet[s.Type] = true
	}
	var tags []string
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (r *Recorder) save() {
	if r.storeDir == "" {
		return
	}
	os.MkdirAll(r.storeDir, 0755)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"recordings": r.recordings,
		"behaviors":  r.behaviors,
	}, "", "  ")
	os.WriteFile(filepath.Join(r.storeDir, "clone.json"), data, 0644)
}

func (r *Recorder) load() {
	if r.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(r.storeDir, "clone.json"))
	if err != nil {
		return
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if raw, ok := stored["recordings"]; ok {
		json.Unmarshal(raw, &r.recordings)
	}
	if raw, ok := stored["behaviors"]; ok {
		json.Unmarshal(raw, &r.behaviors)
	}
	r.nextID = len(r.recordings)
	r.behID = len(r.behaviors)
}

// FormatRecording formats a recording for display.
func FormatRecording(rec *Recording) string {
	status := rec.Status
	if status == "" {
		status = "unknown"
	}
	return fmt.Sprintf("%s  [%s]  %d steps  %s",
		rec.ID, status, len(rec.Steps), rec.Name)
}

// FormatBehavior formats a behavior for display.
func FormatBehavior(b *Behavior) string {
	return fmt.Sprintf("%s  %s  %d patterns  uses:%d  %s",
		b.ID, b.Name, len(b.Patterns), b.Uses, b.CreatedAt.Format("2006-01-02"))
}
