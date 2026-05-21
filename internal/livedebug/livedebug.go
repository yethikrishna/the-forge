// Package livedebug provides real-time collaborative debugging with an
// agent watching the terminal. It captures terminal output, process
// state, and environment information, then provides AI-assisted
// debugging suggestions in real-time.
//
// The workflow:
//  1. Start a debug session attached to a process or command
//  2. The session captures stdout, stderr, exit codes, and signals
//  3. The agent analyzes the output and provides suggestions
//  4. The user can accept suggestions, ask questions, or continue
package livedebug

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// SessionState represents the state of a debug session.
type SessionState int

const (
	StateStarting SessionState = iota
	StateRunning
	StateWaitingInput
	StateAnalyzing
	StateSuggesting
	StateStopped
	StateError
)

func (s SessionState) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateWaitingInput:
		return "waiting_input"
	case StateAnalyzing:
		return "analyzing"
	case StateSuggesting:
		return "suggesting"
	case StateStopped:
		return "stopped"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// OutputLine represents a line of captured output.
type OutputLine struct {
	Stream    string    // "stdout" or "stderr"
	Content   string
	Timestamp time.Time
}

// DebugEvent represents an event in the debug session.
type DebugEvent struct {
	ID        string
	Type      string // "output", "error", "exit", "signal", "suggestion", "user_input"
	Timestamp time.Time
	Data      string
	Metadata  map[string]string
}

// Suggestion represents an AI-generated debugging suggestion.
type Suggestion struct {
	ID          string
	Confidence  float64
	Category    string // "fix", "investigate", "workaround", "question"
	Title       string
	Description string
	Action      string // suggested command or code change
	AutoApply   bool   // whether this can be applied automatically
}

// Session represents a live debug session.
type Session struct {
	ID           string
	Command      string
	WorkingDir   string
	Environment  map[string]string
	State        SessionState
	StartTime    time.Time
	EndTime      time.Time
	ExitCode     int
	Output       []OutputLine
	Events       []*DebugEvent
	Suggestions  []*Suggestion
	ErrorPattern string
	RootCause    string
	FixApplied   string
}

// Engine manages live debug sessions.
type Engine struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	analyzer *Analyzer
}

// NewEngine creates a new debug engine.
func NewEngine() *Engine {
	return &Engine{
		sessions: make(map[string]*Session),
		analyzer: NewAnalyzer(),
	}
}

// StartSession creates a new debug session.
func (e *Engine) StartSession(command, workdir string, env map[string]string) *Session {
	session := &Session{
		ID:          fmt.Sprintf("dbg-%d", time.Now().UnixMilli()),
		Command:     command,
		WorkingDir:  workdir,
		Environment: env,
		State:       StateStarting,
		StartTime:   time.Now(),
		Output:      make([]OutputLine, 0),
		Events:      make([]*DebugEvent, 0),
		Suggestions: make([]*Suggestion, 0),
	}

	e.mu.Lock()
	e.sessions[session.ID] = session
	e.mu.Unlock()

	session.State = StateRunning
	e.addEvent(session, "start", command)

	return session
}

// AddOutput adds a line of output to the session.
func (e *Engine) AddOutput(sessionID, stream, content string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	line := OutputLine{
		Stream:    stream,
		Content:   content,
		Timestamp: time.Now(),
	}
	session.Output = append(session.Output, line)

	// Check for errors in stderr
	if stream == "stderr" {
		session.State = StateAnalyzing
		suggestions := e.analyzer.AnalyzeError(content, session)
		if len(suggestions) > 0 {
			session.Suggestions = append(session.Suggestions, suggestions...)
			session.State = StateSuggesting
		} else {
			session.State = StateRunning
		}
	}

	return nil
}

// SetExitCode records the process exit code.
func (e *Engine) SetExitCode(sessionID string, code int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.ExitCode = code
	session.EndTime = time.Now()

	if code == 0 {
		session.State = StateStopped
	} else {
		session.State = StateError
		e.addEvent(session, "exit", fmt.Sprintf("exit code %d", code))

		// Analyze the full output for root cause
		session.RootCause = e.analyzer.FindRootCause(session.Output)
	}

	return nil
}

// AddUserInput records user input during the debug session.
func (e *Engine) AddUserInput(sessionID, input string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	e.addEvent(session, "user_input", input)
	session.State = StateRunning

	return nil
}

// GetSuggestions returns current suggestions for a session.
func (e *Engine) GetSuggestions(sessionID string) ([]*Suggestion, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session.Suggestions, nil
}

// ApplySuggestion marks a suggestion as applied.
func (e *Engine) ApplySuggestion(sessionID, suggestionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	for _, s := range session.Suggestions {
		if s.ID == suggestionID {
			session.FixApplied = s.Action
			e.addEvent(session, "fix_applied", s.Action)
			return nil
		}
	}

	return fmt.Errorf("suggestion %s not found", suggestionID)
}

// GetSession returns a debug session.
func (e *Engine) GetSession(id string) (*Session, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s, ok := e.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return s, nil
}

// ListSessions returns all debug sessions.
func (e *Engine) ListSessions() []*Session {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Session, 0, len(e.sessions))
	for _, s := range e.sessions {
		result = append(result, s)
	}
	return result
}

// StopSession stops a debug session.
func (e *Engine) StopSession(sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.State = StateStopped
	session.EndTime = time.Now()
	e.addEvent(session, "stop", "session stopped by user")

	return nil
}

// AskQuestion submits a question about the current debugging state.
func (e *Engine) AskQuestion(sessionID, question string) (*Suggestion, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	e.addEvent(session, "question", question)

	// Generate a contextual response
	suggestion := e.analyzer.AnswerQuestion(question, session)
	if suggestion != nil {
		session.Suggestions = append(session.Suggestions, suggestion)
	}

	return suggestion, nil
}

func (e *Engine) addEvent(session *Session, eventType, data string) {
	event := &DebugEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
		Metadata:  make(map[string]string),
	}
	session.Events = append(session.Events, event)
}

// Analyzer provides AI-assisted debugging analysis.
type Analyzer struct {
	patterns []ErrorPattern
}

// ErrorPattern matches common error patterns and provides suggestions.
type ErrorPattern struct {
	Pattern     string
	Category    string
	Title       string
	Description string
	Action      string
	AutoApply   bool
}

// NewAnalyzer creates a new debug analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		patterns: builtinPatterns(),
	}
}

// AnalyzeError analyzes an error line and returns suggestions.
func (a *Analyzer) AnalyzeError(line string, session *Session) []*Suggestion {
	var suggestions []*Suggestion

	for _, p := range a.patterns {
		if strings.Contains(strings.ToLower(line), strings.ToLower(p.Pattern)) {
			suggestions = append(suggestions, &Suggestion{
				ID:          fmt.Sprintf("sug-%d", time.Now().UnixNano()),
				Confidence:  0.8,
				Category:    p.Category,
				Title:       p.Title,
				Description: p.Description,
				Action:      p.Action,
				AutoApply:   p.AutoApply,
			})
		}
	}

	// Context-aware suggestions based on session state
	if len(session.Output) > 50 {
		suggestions = append(suggestions, &Suggestion{
			ID:          fmt.Sprintf("sug-ctx-%d", time.Now().UnixNano()),
			Confidence:  0.5,
			Category:    "investigate",
			Title:       "Large output detected",
			Description: "The command produced significant output. Consider filtering or paginating.",
			Action:      "Add --verbose=false or | head -50",
			AutoApply:   false,
		})
	}

	return suggestions
}

// FindRootCause analyzes the full output to determine the root cause.
func (a *Analyzer) FindRootCause(output []OutputLine) string {
	var errors []string
	for _, line := range output {
		if line.Stream == "stderr" {
			errors = append(errors, line.Content)
		}
	}

	if len(errors) == 0 {
		return "No errors found in stderr"
	}

	// Return the first error as the most likely root cause
	if len(errors) > 0 {
		return errors[0]
	}

	return "Unknown error"
}

// AnswerQuestion generates a contextual answer to a debugging question.
func (a *Analyzer) AnswerQuestion(question string, session *Session) *Suggestion {
	lower := strings.ToLower(question)

	// Common debugging questions
	if strings.Contains(lower, "why") && strings.Contains(lower, "fail") {
		return &Suggestion{
			ID:          fmt.Sprintf("sug-q-%d", time.Now().UnixNano()),
			Confidence:  0.7,
			Category:    "investigate",
			Title:       "Failure analysis",
			Description: session.RootCause,
			Action:      "Check the error message above for details",
			AutoApply:   false,
		}
	}

	if strings.Contains(lower, "how") && strings.Contains(lower, "fix") {
		if len(session.Suggestions) > 0 {
			best := session.Suggestions[len(session.Suggestions)-1]
			return &Suggestion{
				ID:          fmt.Sprintf("sug-q-%d", time.Now().UnixNano()),
				Confidence:  0.6,
				Category:    "fix",
				Title:       "Suggested fix",
				Description: best.Description,
				Action:      best.Action,
				AutoApply:   best.AutoApply,
			}
		}
		return &Suggestion{
			ID:          fmt.Sprintf("sug-q-%d", time.Now().UnixNano()),
			Confidence:  0.3,
			Category:    "investigate",
			Title:       "No fix available",
			Description: "No previous suggestions to base a fix on. Try providing more context.",
			Action:      "forge debug --live with a different command",
			AutoApply:   false,
		}
	}

	return &Suggestion{
		ID:          fmt.Sprintf("sug-q-%d", time.Now().UnixNano()),
		Confidence:  0.4,
		Category:    "investigate",
		Title:       "Debugging insight",
		Description: fmt.Sprintf("Question: %s. State: %s. Output lines: %d.", question, session.State, len(session.Output)),
		Action:      "Try 'forge debug --live' with verbose output for more details",
		AutoApply:   false,
	}
}

func builtinPatterns() []ErrorPattern {
	return []ErrorPattern{
		{
			Pattern: "permission denied",
			Category: "fix",
			Title: "Permission denied",
			Description: "The process doesn't have permission to access a file or resource",
			Action: "chmod +x <file> or run with sudo",
			AutoApply: false,
		},
		{
			Pattern: "no such file or directory",
			Category: "fix",
			Title: "File not found",
			Description: "A required file or directory doesn't exist",
			Action: "Verify the path exists: ls -la <path>",
			AutoApply: false,
		},
		{
			Pattern: "connection refused",
			Category: "fix",
			Title: "Connection refused",
			Description: "The target service is not running or not listening on the expected port",
			Action: "Check if the service is running: ps aux | grep <service>",
			AutoApply: false,
		},
		{
			Pattern: "timeout",
			Category: "investigate",
			Title: "Timeout",
			Description: "The operation timed out, possibly due to network issues or slow service",
			Action: "Increase timeout or check network connectivity",
			AutoApply: false,
		},
		{
			Pattern: "out of memory",
			Category: "fix",
			Title: "Out of memory",
			Description: "The process ran out of memory",
			Action: "Reduce data size or increase memory limits",
			AutoApply: false,
		},
		{
			Pattern: "segmentation fault",
			Category: "investigate",
			Title: "Segmentation fault",
			Description: "The process tried to access invalid memory",
			Action: "Run with a debugger: gdb --args <command>",
			AutoApply: false,
		},
		{
			Pattern: "undefined reference",
			Category: "fix",
			Title: "Undefined reference",
			Description: "A symbol is referenced but not defined — linker error",
			Action: "Check that all required libraries are linked",
			AutoApply: false,
		},
		{
			Pattern: "cannot find package",
			Category: "fix",
			Title: "Package not found",
			Description: "A required package is not installed or not in the module path",
			Action: "go mod tidy && go mod download",
			AutoApply: true,
		},
		{
			Pattern: "syntax error",
			Category: "fix",
			Title: "Syntax error",
			Description: "There's a syntax error in the source code",
			Action: "Check the line number in the error message",
			AutoApply: false,
		},
		{
			Pattern: "address already in use",
			Category: "fix",
			Title: "Port already in use",
			Description: "Another process is using the same port",
			Action: "Find and kill the process: lsof -i :<port>",
			AutoApply: false,
		},
	}
}
