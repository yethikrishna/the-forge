// Package clonebehavior records human task execution patterns and creates
// agent configurations that can repeat similar tasks automatically.
//
// The workflow:
//  1. Record: Human performs a task while Forge observes (commands, file edits, decisions)
//  2. Analyze: Extract patterns, decision points, and tool usage
//  3. Generate: Create an agent configuration that automates the pattern
//  4. Validate: Run the generated agent on a similar task to verify
package clonebehavior

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ActionType categorizes the type of action observed.
type ActionType int

const (
	ActionCommand ActionType = iota
	ActionFileRead
	ActionFileWrite
	ActionFileEdit
	ActionDecision
	ActionSearch
	ActionNavigate
	ActionTest
	ActionDeploy
)

func (a ActionType) String() string {
	switch a {
	case ActionCommand:
		return "command"
	case ActionFileRead:
		return "file_read"
	case ActionFileWrite:
		return "file_write"
	case ActionFileEdit:
		return "file_edit"
	case ActionDecision:
		return "decision"
	case ActionSearch:
		return "search"
	case ActionNavigate:
		return "navigate"
	case ActionTest:
		return "test"
	case ActionDeploy:
		return "deploy"
	default:
		return "unknown"
	}
}

// Action represents a single observed action during recording.
type Action struct {
	ID        string
	Type      ActionType
	Timestamp time.Time
	Command   string // for command actions
	FilePath  string // for file actions
	Content   string // content read/written
	OldText   string // for edit actions
	NewText   string // for edit actions
	Query     string // for search actions
	Decision  string // for decision points
	Reasoning string // human's explanation of why
	Duration  time.Duration
}

// Recording is a complete session recording.
type Recording struct {
	ID          string
	Name        string
	Description string
	Actions     []*Action
	StartTime   time.Time
	EndTime     time.Time
	Tags        []string
	Status      RecordingStatus
}

// RecordingStatus tracks the state of a recording.
type RecordingStatus int

const (
	RecordingActive RecordingStatus = iota
	RecordingPaused
	RecordingStopped
	RecordingAnalyzed
)

func (s RecordingStatus) String() string {
	switch s {
	case RecordingActive:
		return "active"
	case RecordingPaused:
		return "paused"
	case RecordingStopped:
		return "stopped"
	case RecordingAnalyzed:
		return "analyzed"
	default:
		return "unknown"
	}
}

// Pattern represents an extracted pattern from recordings.
type Pattern struct {
	ID             string
	Name           string
	Description    string
	Actions        []ActionTemplate
	DecisionPoints []DecisionPoint
	Frequency      int // how many times this pattern appeared
	Confidence     float64
	Tags           []string
}

// ActionTemplate is a parameterized action for replay.
type ActionTemplate struct {
	Type        ActionType
	CommandTmpl string // template with {{.param}} placeholders
	FilePattern string // regex or glob for matching files
	IsRequired  bool
	Order       int
}

// DecisionPoint represents a point where the agent needs to make a choice.
type DecisionPoint struct {
	Description string
	Options     []string
	Default     string
	Condition   string // when this decision point is relevant
}

// AgentConfig is the generated agent configuration.
type AgentConfig struct {
	Name         string
	Description  string
	Instructions string
	Tools        []string
	Parameters   []Parameter
	Patterns     []*Pattern
	Confidence   float64
	Model        string
	Temperature  float64
}

// Parameter defines an input parameter for the generated agent.
type Parameter struct {
	Name        string
	Description string
	Required    bool
	Default     string
	Type        string // string, int, bool, file, path
}

// Recorder captures human actions.
type Recorder struct {
	mu        sync.RWMutex
	recording *Recording
}

// NewRecorder creates a new recorder.
func NewRecorder(name, description string) *Recorder {
	return &Recorder{
		recording: &Recording{
			ID:          fmt.Sprintf("rec-%d", time.Now().UnixMilli()),
			Name:        name,
			Description: description,
			Actions:     make([]*Action, 0),
			StartTime:   time.Now(),
			Status:      RecordingActive,
		},
	}
}

// RecordAction adds an observed action to the recording.
func (r *Recorder) RecordAction(action *Action) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording.Status != RecordingActive {
		return fmt.Errorf("recording is not active (status: %s)", r.recording.Status)
	}

	action.ID = fmt.Sprintf("act-%d-%d", time.Now().UnixMilli(), len(r.recording.Actions))
	action.Timestamp = time.Now()
	r.recording.Actions = append(r.recording.Actions, action)
	return nil
}

// RecordCommand records a shell command execution.
func (r *Recorder) RecordCommand(cmd string, duration time.Duration) error {
	return r.RecordAction(&Action{
		Type:     ActionCommand,
		Command:  cmd,
		Duration: duration,
	})
}

// RecordFileRead records a file being read.
func (r *Recorder) RecordFileRead(path, content string) error {
	return r.RecordAction(&Action{
		Type:     ActionFileRead,
		FilePath: path,
		Content:  content,
	})
}

// RecordFileWrite records a file being written.
func (r *Recorder) RecordFileWrite(path, content string) error {
	return r.RecordAction(&Action{
		Type:     ActionFileWrite,
		FilePath: path,
		Content:  content,
	})
}

// RecordFileEdit records a file edit.
func (r *Recorder) RecordFileEdit(path, oldText, newText string) error {
	return r.RecordAction(&Action{
		Type:     ActionFileEdit,
		FilePath: path,
		OldText:  oldText,
		NewText:  newText,
	})
}

// RecordDecision records a decision point.
func (r *Recorder) RecordDecision(decision, reasoning string) error {
	return r.RecordAction(&Action{
		Type:      ActionDecision,
		Decision:  decision,
		Reasoning: reasoning,
	})
}

// RecordSearch records a search query.
func (r *Recorder) RecordSearch(query string) error {
	return r.RecordAction(&Action{
		Type:  ActionSearch,
		Query: query,
	})
}

// Pause pauses the recording.
func (r *Recorder) Pause() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recording.Status = RecordingPaused
}

// Resume resumes a paused recording.
func (r *Recorder) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recording.Status == RecordingPaused {
		r.recording.Status = RecordingActive
	}
}

// Stop stops the recording.
func (r *Recorder) Stop() *Recording {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recording.Status = RecordingStopped
	r.recording.EndTime = time.Now()
	return r.recording
}

// GetRecording returns the current recording.
func (r *Recorder) GetRecording() *Recording {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.recording
}

// Analyzer extracts patterns from recordings.
type Analyzer struct{}

// NewAnalyzer creates a new pattern analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// Analyze extracts patterns from a recording.
func (a *Analyzer) Analyze(recording *Recording) ([]*Pattern, error) {
	if len(recording.Actions) == 0 {
		return nil, fmt.Errorf("no actions to analyze")
	}

	var patterns []*Pattern

	// Extract command sequences
	cmdPatterns := a.extractCommandPatterns(recording.Actions)
	patterns = append(patterns, cmdPatterns...)

	// Extract file operation patterns
	filePatterns := a.extractFilePatterns(recording.Actions)
	patterns = append(patterns, filePatterns...)

	// Extract decision points
	decisions := a.extractDecisionPoints(recording.Actions)

	// Attach decisions to patterns
	for _, p := range patterns {
		p.DecisionPoints = decisions
	}

	// If no patterns found, create a linear pattern from all actions
	if len(patterns) == 0 {
		patterns = append(patterns, a.createLinearPattern(recording.Actions))
	}

	// Sort by frequency
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})

	return patterns, nil
}

// extractCommandPatterns identifies recurring command patterns.
func (a *Analyzer) extractCommandPatterns(actions []*Action) []*Pattern {
	var patterns []*Pattern
	cmdCounts := make(map[string]int)

	for _, act := range actions {
		if act.Type == ActionCommand {
			// Normalize command (replace specific values with placeholders)
			normalized := normalizeCommand(act.Command)
			cmdCounts[normalized]++
		}
	}

	for cmd, count := range cmdCounts {
		if count >= 1 {
			patterns = append(patterns, &Pattern{
				ID:          fmt.Sprintf("pat-cmd-%d", len(patterns)),
				Name:        fmt.Sprintf("Command: %s", truncate(cmd, 50)),
				Description: fmt.Sprintf("Run command: %s", cmd),
				Actions: []ActionTemplate{
					{Type: ActionCommand, CommandTmpl: cmd, IsRequired: true, Order: 0},
				},
				Frequency:  count,
				Confidence: confidenceFromFrequency(count),
			})
		}
	}

	return patterns
}

// extractFilePatterns identifies file operation patterns.
func (a *Analyzer) extractFilePatterns(actions []*Action) []*Pattern {
	var patterns []*Pattern
	fileOps := make(map[string][]ActionType)

	for _, act := range actions {
		if act.FilePath != "" {
			dir := fileDir(act.FilePath)
			fileOps[dir] = append(fileOps[dir], act.Type)
		}
	}

	for dir, ops := range fileOps {
		actionTemplates := make([]ActionTemplate, len(ops))
		for i, op := range ops {
			actionTemplates[i] = ActionTemplate{
				Type:        op,
				FilePattern: dir + "/*",
				IsRequired:  true,
				Order:       i,
			}
		}

		patterns = append(patterns, &Pattern{
			ID:          fmt.Sprintf("pat-file-%d", len(patterns)),
			Name:        fmt.Sprintf("File operations in %s", dir),
			Description: fmt.Sprintf("Sequence of file operations in %s: %v", dir, ops),
			Actions:     actionTemplates,
			Frequency:   1,
			Confidence:  0.6,
		})
	}

	return patterns
}

// extractDecisionPoints identifies decision points in the recording.
func (a *Analyzer) extractDecisionPoints(actions []*Action) []DecisionPoint {
	var decisions []DecisionPoint

	for _, act := range actions {
		if act.Type == ActionDecision {
			decisions = append(decisions, DecisionPoint{
				Description: act.Decision,
				Condition:   act.Reasoning,
				Default:     "proceed",
				Options:     []string{"proceed", "skip", "modify"},
			})
		}
	}

	return decisions
}

// createLinearPattern creates a single linear pattern from all actions.
func (a *Analyzer) createLinearPattern(actions []*Action) *Pattern {
	templates := make([]ActionTemplate, len(actions))
	for i, act := range actions {
		templates[i] = ActionTemplate{
			Type:       act.Type,
			Order:      i,
			IsRequired: true,
		}
		if act.Type == ActionCommand {
			templates[i].CommandTmpl = normalizeCommand(act.Command)
		}
		if act.FilePath != "" {
			templates[i].FilePattern = act.FilePath
		}
	}

	return &Pattern{
		ID:          "pat-linear",
		Name:        "Linear task sequence",
		Description: "All recorded actions in sequence",
		Actions:     templates,
		Frequency:   1,
		Confidence:  0.5,
	}
}

// Generator creates agent configurations from patterns.
type Generator struct{}

// NewGenerator creates a new agent config generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates an AgentConfig from a recording and its patterns.
func (g *Generator) Generate(recording *Recording, patterns []*Pattern) *AgentConfig {
	// Collect all required tools
	toolSet := make(map[string]bool)
	for _, p := range patterns {
		for _, tmpl := range p.Actions {
			switch tmpl.Type {
			case ActionCommand:
				toolSet["exec"] = true
			case ActionFileRead:
				toolSet["read"] = true
			case ActionFileWrite, ActionFileEdit:
				toolSet["read"] = true
				toolSet["write"] = true
			case ActionSearch:
				toolSet["search"] = true
			case ActionTest:
				toolSet["exec"] = true
				toolSet["read"] = true
			}
		}
	}

	var tools []string
	for t := range toolSet {
		tools = append(tools, t)
	}
	sort.Strings(tools)

	// Generate instructions
	instructions := g.generateInstructions(recording, patterns)

	// Generate parameters from decision points
	params := g.generateParameters(patterns)

	// Calculate confidence
	confidence := 0.5
	if len(patterns) > 1 {
		confidence = 0.7
	}
	if len(recording.Actions) > 10 {
		confidence += 0.1
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return &AgentConfig{
		Name:         recording.Name + "-agent",
		Description:  fmt.Sprintf("Auto-generated agent from recording %q", recording.Name),
		Instructions: instructions,
		Tools:        tools,
		Parameters:   params,
		Patterns:     patterns,
		Confidence:   confidence,
		Model:        "gpt-4.1-mini",
		Temperature:  0.1,
	}
}

// generateInstructions creates agent instructions from patterns.
func (g *Generator) generateInstructions(recording *Recording, patterns []*Pattern) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s Agent\n\n", recording.Name))
	b.WriteString(fmt.Sprintf("This agent was auto-generated from a recording of a human performing: %s\n\n", recording.Description))
	b.WriteString("## Workflow\n\n")

	for i, p := range patterns {
		b.WriteString(fmt.Sprintf("### Pattern %d: %s\n", i+1, p.Name))
		b.WriteString(fmt.Sprintf("%s\n\n", p.Description))
		b.WriteString("Steps:\n")
		for _, tmpl := range p.Actions {
			b.WriteString(fmt.Sprintf("1. [%s] ", tmpl.Type))
			if tmpl.CommandTmpl != "" {
				b.WriteString(fmt.Sprintf("Run: `%s`\n", tmpl.CommandTmpl))
			} else if tmpl.FilePattern != "" {
				b.WriteString(fmt.Sprintf("Operate on: %s\n", tmpl.FilePattern))
			} else {
				b.WriteString("Execute action\n")
			}
		}
		b.WriteString("\n")
	}

	if len(patterns) > 0 {
		for _, dp := range patterns[0].DecisionPoints {
			b.WriteString(fmt.Sprintf("### Decision: %s\n", dp.Description))
			b.WriteString(fmt.Sprintf("Condition: %s\n", dp.Condition))
			b.WriteString(fmt.Sprintf("Options: %s\n\n", strings.Join(dp.Options, ", ")))
		}
	}

	return b.String()
}

// generateParameters creates input parameters from decision points.
func (g *Generator) generateParameters(patterns []*Pattern) []Parameter {
	paramSet := make(map[string]Parameter)

	for _, p := range patterns {
		for _, dp := range p.DecisionPoints {
			key := sanitizeName(dp.Description)
			if _, exists := paramSet[key]; !exists {
				paramSet[key] = Parameter{
					Name:        key,
					Description: dp.Description,
					Required:    false,
					Default:     dp.Default,
					Type:        "string",
				}
			}
		}
	}

	// Add common parameters
	paramSet["target_path"] = Parameter{
		Name:        "target_path",
		Description: "Path to the target file or directory",
		Required:    true,
		Type:        "path",
	}

	var params []Parameter
	for _, p := range paramSet {
		params = append(params, p)
	}
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	return params
}

// Store persists recordings and generated configs.
type Store struct {
	mu         sync.RWMutex
	recordings map[string]*Recording
	configs    map[string]*AgentConfig
}

// NewStore creates a new store.
func NewStore() *Store {
	return &Store{
		recordings: make(map[string]*Recording),
		configs:    make(map[string]*AgentConfig),
	}
}

// SaveRecording saves a recording.
func (s *Store) SaveRecording(rec *Recording) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordings[rec.ID] = rec
	return nil
}

// GetRecording retrieves a recording.
func (s *Store) GetRecording(id string) (*Recording, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.recordings[id]
	if !ok {
		return nil, fmt.Errorf("recording %s not found", id)
	}
	return r, nil
}

// ListRecordings returns all recordings.
func (s *Store) ListRecordings() []*Recording {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Recording, 0, len(s.recordings))
	for _, r := range s.recordings {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.After(result[j].StartTime)
	})
	return result
}

// SaveConfig saves a generated agent config.
func (s *Store) SaveConfig(cfg *AgentConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[cfg.Name] = cfg
	return nil
}

// GetConfig retrieves an agent config.
func (s *Store) GetConfig(name string) (*AgentConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.configs[name]
	if !ok {
		return nil, fmt.Errorf("config %s not found", name)
	}
	return c, nil
}

// ListConfigs returns all agent configs.
func (s *Store) ListConfigs() []*AgentConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AgentConfig, 0, len(s.configs))
	for _, c := range s.configs {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Helper functions

func normalizeCommand(cmd string) string {
	// Replace specific paths, numbers, and hashes with placeholders
	parts := strings.Fields(cmd)
	for i, p := range parts {
		if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "./") {
			parts[i] = "{{.path}}"
		} else if isHex(p) && len(p) > 6 {
			parts[i] = "{{.hash}}"
		} else if isNumeric(p) && len(p) > 3 {
			parts[i] = "{{.number}}"
		}
	}
	return strings.Join(parts, " ")
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func fileDir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return "."
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	result := make([]byte, 0, len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

func confidenceFromFrequency(freq int) float64 {
	if freq >= 5 {
		return 0.9
	}
	if freq >= 3 {
		return 0.8
	}
	if freq >= 2 {
		return 0.7
	}
	return 0.5
}

// NewRecordingID generates a unique recording ID.
func NewRecordingID() string {
	return fmt.Sprintf("rec-%d", time.Now().UnixMilli())
}
