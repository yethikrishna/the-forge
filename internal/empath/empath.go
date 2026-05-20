// Package empath provides user frustration detection and adaptive response.
// Analyzes conversation patterns, error frequency, and sentiment indicators
// to detect user frustration and adjust agent behavior accordingly.
//
// Read the room.
package empath

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FrustrationLevel represents the detected frustration level.
type FrustrationLevel string

const (
	FrustrationNone    FrustrationLevel = "none"
	FrustrationLow     FrustrationLevel = "low"
	FrustrationMedium  FrustrationLevel = "medium"
	FrustrationHigh    FrustrationLevel = "high"
	FrustrationCritical FrustrationLevel = "critical"
)

// Signal represents a frustration signal detected in user input.
type Signal struct {
	Type      string    `json:"type"`
	Weight    float64   `json:"weight"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// State tracks the user's emotional state over a session.
type State struct {
	Level              FrustrationLevel `json:"level"`
	Score              float64          `json:"score"`
	Signals            []Signal         `json:"signals"`
	LastMessageAt      time.Time        `json:"last_message_at"`
	MessageCount       int              `json:"message_count"`
	ErrorCount         int              `json:"error_count"`
	RepeatCount        int              `json:"repeat_count"`
	ShortResponseCount int              `json:"short_response_count"`
	SessionStart       time.Time        `json:"session_start"`
}

// AdaptiveConfig defines how the agent should adapt to frustration.
type AdaptiveConfig struct {
	Level             FrustrationLevel `json:"level"`
	ResponseStyle     string           `json:"response_style"`
	MaxRetries        int              `json:"max_retries"`
	ShowProgress      bool             `json:"show_progress"`
	OfferAlternatives bool             `json:"offer_alternatives"`
	SlowDown          bool             `json:"slow_down"`
}

// Detector detects user frustration from messages and events.
type Detector struct {
	mu           sync.Mutex
	state        State
	dir          string
	prevMessages []string
}

// NewDetector creates a frustration detector.
func NewDetector(dir string) *Detector {
	return &Detector{
		state: State{
			Level:        FrustrationNone,
			Score:        0,
			Signals:      make([]Signal, 0),
			SessionStart: time.Now(),
		},
		dir:          dir,
		prevMessages: make([]string, 0),
	}
}

// Analyze processes a user message and updates frustration state.
func (d *Detector) Analyze(message string) State {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.state.MessageCount++
	d.state.LastMessageAt = time.Now()

	signals := d.detectSignals(message)
	d.state.Signals = append(d.state.Signals, signals...)

	for _, s := range signals {
		switch s.Type {
		case "repeat":
			d.state.RepeatCount++
		case "short_response":
			d.state.ShortResponseCount++
		}
	}

	d.recalculate()

	d.prevMessages = append(d.prevMessages, normalize(message))
	if len(d.prevMessages) > 10 {
		d.prevMessages = d.prevMessages[len(d.prevMessages)-10:]
	}

	return d.state
}

// RecordError records an error event.
func (d *Detector) RecordError(errMsg string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.state.ErrorCount++
	d.state.Signals = append(d.state.Signals, Signal{
		Type:      "error_loop",
		Weight:    0.3,
		Message:   errMsg,
		Timestamp: time.Now(),
	})

	d.recalculate()
}

func (d *Detector) detectSignals(message string) []Signal {
	var signals []Signal
	now := time.Now()

	// ALL CAPS
	upperCount := 0
	for _, c := range message {
		if c >= 'A' && c <= 'Z' {
			upperCount++
		}
	}
	if len(message) > 5 && float64(upperCount)/float64(len(message)) > 0.7 {
		signals = append(signals, Signal{
			Type: "caps", Weight: 0.3, Message: message, Timestamp: now,
		})
	}

	// Impatience keywords
	impatienceWords := []string{
		"hurry", "faster", "come on", "quickly", "now",
		"seriously", "wtf", "ugh", "argh", "grr",
		"still waiting", "taking forever", "useless",
		"not working", "broken", "sucks", "terrible",
		"frustrating", "annoying", "ridiculous",
	}
	normMsg := strings.ToLower(message)
	for _, word := range impatienceWords {
		if strings.Contains(normMsg, word) {
			signals = append(signals, Signal{
				Type: "impatience", Weight: 0.5, Message: word, Timestamp: now,
			})
			break
		}
	}

	// Short response
	words := strings.Fields(message)
	if len(words) <= 3 && d.state.MessageCount > 2 {
		signals = append(signals, Signal{
			Type: "short_response", Weight: 0.2, Message: message, Timestamp: now,
		})
	}

	// Repeat messages
	normCurrent := normalize(message)
	for _, prev := range d.prevMessages {
		if similarity(normCurrent, prev) > 0.8 && normCurrent != "" {
			signals = append(signals, Signal{
				Type: "repeat", Weight: 0.4, Message: message, Timestamp: now,
			})
			break
		}
	}

	// Multiple punctuation
	if strings.Count(message, "???") > 0 || strings.Count(message, "!!!") > 0 {
		signals = append(signals, Signal{
			Type: "caps", Weight: 0.2, Message: message, Timestamp: now,
		})
	}

	return signals
}

func (d *Detector) recalculate() {
	var totalWeight float64
	now := time.Now()

	for _, s := range d.state.Signals {
		age := now.Sub(s.Timestamp).Minutes()
		decay := 1.0
		if age > 5 {
			decay = 1.0 / (1.0 + age/10.0)
		}
		totalWeight += s.Weight * decay
	}

	errorFactor := float64(d.state.ErrorCount) * 0.15
	repeatFactor := float64(d.state.RepeatCount) * 0.2
	shortFactor := float64(d.state.ShortResponseCount) * 0.1

	d.state.Score = minVal(totalWeight+errorFactor+repeatFactor+shortFactor, 100)

	switch {
	case d.state.Score < 10:
		d.state.Level = FrustrationNone
	case d.state.Score < 25:
		d.state.Level = FrustrationLow
	case d.state.Score < 50:
		d.state.Level = FrustrationMedium
	case d.state.Score < 75:
		d.state.Level = FrustrationHigh
	default:
		d.state.Level = FrustrationCritical
	}
}

// GetAdaptiveConfig returns the recommended agent behavior.
func (d *Detector) GetAdaptiveConfig() AdaptiveConfig {
	d.mu.Lock()
	defer d.mu.Unlock()

	cfg := AdaptiveConfig{
		Level:        d.state.Level,
		ShowProgress: true,
	}

	switch d.state.Level {
	case FrustrationNone:
		cfg.ResponseStyle = "normal"
		cfg.MaxRetries = 3
	case FrustrationLow:
		cfg.ResponseStyle = "concise"
		cfg.MaxRetries = 2
		cfg.OfferAlternatives = true
	case FrustrationMedium:
		cfg.ResponseStyle = "supportive"
		cfg.MaxRetries = 2
		cfg.OfferAlternatives = true
		cfg.SlowDown = true
	case FrustrationHigh:
		cfg.ResponseStyle = "supportive"
		cfg.MaxRetries = 1
		cfg.OfferAlternatives = true
		cfg.SlowDown = true
	case FrustrationCritical:
		cfg.ResponseStyle = "handoff"
		cfg.MaxRetries = 0
		cfg.OfferAlternatives = true
		cfg.SlowDown = true
	}

	return cfg
}

// State returns the current emotional state.
func (d *Detector) State() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

// Reset resets the frustration state.
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state = State{
		Level:        FrustrationNone,
		Score:        0,
		Signals:      make([]Signal, 0),
		SessionStart: time.Now(),
	}
	d.prevMessages = make([]string, 0)
}

// Save persists the emotional state.
func (d *Detector) Save() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(d.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.dir, "empath.json"), data, 0o644)
}

// Load reads the emotional state from disk.
func (d *Detector) Load() error {
	data, err := os.ReadFile(filepath.Join(d.dir, "empath.json"))
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	return json.Unmarshal(data, &d.state)
}

// FormatState renders the emotional state for display.
func FormatState(s State) string {
	return fmt.Sprintf("Level: %s  Score: %.1f/100  Messages: %d  Errors: %d  Repeats: %d",
		s.Level, s.Score, s.MessageCount, s.ErrorCount, s.RepeatCount)
}

// FormatConfig renders the adaptive config for display.
func FormatConfig(c AdaptiveConfig) string {
	return fmt.Sprintf("Style: %s  MaxRetries: %d  Progress: %v  Alternatives: %v  SlowDown: %v",
		c.ResponseStyle, c.MaxRetries, c.ShowProgress, c.OfferAlternatives, c.SlowDown)
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func similarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	wordsA := make(map[string]bool)
	wordsB := make(map[string]bool)
	for _, w := range strings.Fields(a) {
		wordsA[w] = true
	}
	for _, w := range strings.Fields(b) {
		wordsB[w] = true
	}

	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}

	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

func minVal(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
