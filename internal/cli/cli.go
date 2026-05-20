// Package cli provides terminal UI helpers for building interactive CLIs:
// progress spinners, prompts, and user input utilities.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Spinner displays an animated spinner in the terminal.
type Spinner struct {
	frames  []string
	message string
	writer  io.Writer
	done    chan struct{}
	active  bool
}

// Default spinner frames.
var defaultFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ForgeSpinner uses sword-themed frames.
var forgeFrames = []string{"⚔️ ", "🔥 ", "✨ ", "🗡️ ", "⚡ "}

// NewSpinner creates a new spinner with a message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  defaultFrames,
		message: message,
		writer:  os.Stderr,
		done:    make(chan struct{}),
	}
}

// NewForgeSpinner creates a spinner with Forge-themed animation.
func NewForgeSpinner(message string) *Spinner {
	return &Spinner{
		frames:  forgeFrames,
		message: message,
		writer:  os.Stderr,
		done:    make(chan struct{}),
	}
}

// WithWriter sets the output writer for the spinner.
func (s *Spinner) WithWriter(w io.Writer) *Spinner {
	s.writer = w
	return s
}

// WithFrames sets custom animation frames.
func (s *Spinner) WithFrames(frames []string) *Spinner {
	s.frames = frames
	return s
}

// Start begins the spinner animation.
func (s *Spinner) Start() *Spinner {
	if s.active {
		return s
	}
	s.active = true
	go s.run()
	return s
}

// Stop ends the spinner animation and clears the line.
func (s *Spinner) Stop() {
	if !s.active {
		return
	}
	s.active = false
	close(s.done)
	fmt.Fprintf(s.writer, "\r\033[K")
}

// Message updates the spinner message.
func (s *Spinner) Message(msg string) {
	s.message = msg
}

// run animates the spinner frames.
func (s *Spinner) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			frame := s.frames[i%len(s.frames)]
			fmt.Fprintf(s.writer, "\r\033[K%s %s", frame, s.message)
			i++
		}
	}
}

// SpinWhile runs a function with a spinner.
func SpinWhile(message string, fn func() error) error {
	return SpinWhileWithSpinner(NewSpinner(message), fn)
}

// SpinWhileWithSpinner runs a function with a custom spinner.
func SpinWhileWithSpinner(s *Spinner, fn func() error) error {
	s.Start()
	defer s.Stop()
	return fn()
}

// Prompt reads a line of input from the user.
func Prompt(message string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s ", message)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("prompt: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// PromptDefault reads input with a default value.
func PromptDefault(message, defaultVal string) (string, error) {
	result, err := Prompt(fmt.Sprintf("%s [%s]", message, defaultVal))
	if err != nil {
		return "", err
	}
	if result == "" {
		return defaultVal, nil
	}
	return result, nil
}

// Confirm asks a yes/no question.
func Confirm(message string, defaultYes bool) (bool, error) {
	suffix := "(Y/n)"
	if !defaultYes {
		suffix = "(y/N)"
	}
	result, err := Prompt(fmt.Sprintf("%s %s", message, suffix))
	if err != nil {
		return false, err
	}
	result = strings.ToLower(strings.TrimSpace(result))
	if result == "" {
		return defaultYes, nil
	}
	return result == "y" || result == "yes", nil
}

// Select presents a list of options and returns the selection index.
func Select(message string, options []string) (int, error) {
	fmt.Fprintf(os.Stderr, "%s\n", message)
	for i, opt := range options {
		fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, opt)
	}
	result, err := Prompt("Enter number")
	if err != nil {
		return -1, err
	}
	idx := 0
	fmt.Sscanf(result, "%d", &idx)
	idx-- // Convert to 0-based
	if idx < 0 || idx >= len(options) {
		return -1, fmt.Errorf("invalid selection: %s", result)
	}
	return idx, nil
}

// Password reads a password without echo (falls back to normal input).
func Password(message string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s ", message)
	// Note: proper password hiding requires terminal.RawMode which needs
	// golang.org/x/term. For now, we just read from stdin.
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("password: %w", err)
	}
	fmt.Fprintln(os.Stderr)
	return strings.TrimSpace(line), nil
}

// Step represents a step in a multi-step progress display.
type Step struct {
	Name   string
	Status StepStatus
}

// StepStatus indicates the state of a step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepFailed
	StepSkipped
)

// StepTracker tracks and displays multi-step progress.
type StepTracker struct {
	steps []Step
}

// NewStepTracker creates a tracker for the given step names.
func NewStepTracker(names []string) *StepTracker {
	steps := make([]Step, len(names))
	for i, name := range names {
		steps[i] = Step{Name: name, Status: StepPending}
	}
	return &StepTracker{steps: steps}
}

// Start marks a step as running.
func (t *StepTracker) Start(idx int) {
	if idx >= 0 && idx < len(t.steps) {
		t.steps[idx].Status = StepRunning
		t.render()
	}
}

// Done marks a step as completed.
func (t *StepTracker) Done(idx int) {
	if idx >= 0 && idx < len(t.steps) {
		t.steps[idx].Status = StepDone
		t.render()
	}
}

// Fail marks a step as failed.
func (t *StepTracker) Fail(idx int) {
	if idx >= 0 && idx < len(t.steps) {
		t.steps[idx].Status = StepFailed
		t.render()
	}
}

// Skip marks a step as skipped.
func (t *StepTracker) Skip(idx int) {
	if idx >= 0 && idx < len(t.steps) {
		t.steps[idx].Status = StepSkipped
		t.render()
	}
}

func (t *StepTracker) render() {
	symbols := map[StepStatus]string{
		StepPending: "○",
		StepRunning: "◉",
		StepDone:    "✓",
		StepFailed:  "✗",
		StepSkipped: "⊘",
	}
	for _, step := range t.steps {
		fmt.Fprintf(os.Stderr, "  %s %s\n", symbols[step.Status], step.Name)
	}
}
