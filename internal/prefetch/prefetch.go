// Package prefetch provides predictive context prefetching.
// Learns from usage patterns to pre-load context before the user needs it,
// reducing perceived latency.
//
// Anticipate, don't wait.
package prefetch

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

// ContextType is the type of prefetched context.
type ContextType string

const (
	ContextFile       ContextType = "file"
	ContextDirectory  ContextType = "directory"
	ContextCommand    ContextType = "command"
	ContextHistory    ContextType = "history"
	ContextPattern    ContextType = "pattern"
)

// PrefetchEntry is a predicted context to preload.
type PrefetchEntry struct {
	Type      ContextType `json:"type"`
	Target    string      `json:"target"`
	Priority  float64     `json:"priority"`   // 0-1, higher = more likely needed
	Reason    string      `json:"reason"`
	Loaded    bool        `json:"loaded"`
	Size      int64       `json:"size,omitempty"`
}

// UsageEvent records when a context was accessed.
type UsageEvent struct {
	Type      ContextType `json:"type"`
	Target    string      `json:"target"`
	Timestamp time.Time   `json:"timestamp"`
	Command   string      `json:"command,omitempty"` // triggering command
}

// Pattern represents a learned usage pattern.
type Pattern struct {
	Trigger   string  `json:"trigger"`    // command or file that triggers
	Prefetch  string  `json:"prefetch"`   // what to prefetch
	Count     int     `json:"count"`      // how often this pattern occurred
	Recency   float64 `json:"recency"`    // 0-1, how recent
	Type      ContextType `json:"type"`
}

// Predictor learns usage patterns and predicts context to prefetch.
type Predictor struct {
	patterns   []Pattern
	history    []UsageEvent
	storeDir   string
	maxHistory int
	mu         sync.RWMutex
}

// NewPredictor creates a context prefetch predictor.
func NewPredictor(storeDir string) *Predictor {
	p := &Predictor{
		patterns:   make([]Pattern, 0),
		history:    make([]UsageEvent, 0),
		storeDir:   storeDir,
		maxHistory: 500,
	}
	p.load()
	return p
}

// Record records a context usage event.
func (p *Predictor) Record(eventType ContextType, target, command string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	evt := UsageEvent{
		Type:      eventType,
		Target:    target,
		Timestamp: time.Now(),
		Command:   command,
	}

	p.history = append(p.history, evt)
	if len(p.history) > p.maxHistory {
		p.history = p.history[len(p.history)-p.maxHistory:]
	}

	// Learn pattern: when this command runs, this context is accessed
	if command != "" {
		p.learnPattern(command, eventType, target)
	}

	p.save()
}

// Predict predicts what context should be prefetched for a given command.
func (p *Predictor) Predict(command string, maxEntries int) []PrefetchEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	scores := make(map[string]*PrefetchEntry)

	// Score based on learned patterns
	for _, pat := range p.patterns {
		if pat.Trigger == command || matchesTrigger(command, pat.Trigger) {
			if _, ok := scores[pat.Prefetch]; !ok {
				scores[pat.Prefetch] = &PrefetchEntry{
					Type:     pat.Type,
					Target:   pat.Prefetch,
					Priority: 0,
					Reason:   fmt.Sprintf("Pattern: %q → %q (%d occurrences)", pat.Trigger, pat.Prefetch, pat.Count),
				}
			}
			entry := scores[pat.Prefetch]
			entry.Priority += float64(pat.Count) * 0.1 * pat.Recency
		}
	}

	// Score based on recent history (last 20 events)
	recentCount := 20
	if len(p.history) < recentCount {
		recentCount = len(p.history)
	}
	for i := len(p.history) - recentCount; i < len(p.history); i++ {
		evt := p.history[i]
		age := time.Since(evt.Timestamp).Hours()
		recency := 1.0 / (1.0 + age)

		if _, ok := scores[evt.Target]; !ok {
			scores[evt.Target] = &PrefetchEntry{
				Type:     evt.Type,
				Target:   evt.Target,
				Priority: 0,
				Reason:   "Recently accessed",
			}
		}
		scores[evt.Target].Priority += recency * 0.3
	}

	// Convert to sorted slice
	entries := make([]PrefetchEntry, 0, len(scores))
	for _, entry := range scores {
		// Normalize priority to 0-1
		if entry.Priority > 1.0 {
			entry.Priority = 1.0
		}
		entries = append(entries, *entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Priority > entries[j].Priority
	})

	if maxEntries > 0 && len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	return entries
}

// GetPatterns returns learned patterns sorted by frequency.
func (p *Predictor) GetPatterns() []Pattern {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]Pattern, len(p.patterns))
	copy(result, p.patterns)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

// GetHistory returns recent usage events.
func (p *Predictor) GetHistory(limit int) []UsageEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit <= 0 || limit > len(p.history) {
		limit = len(p.history)
	}
	result := make([]UsageEvent, limit)
	copy(result, p.history[len(p.history)-limit:])
	return result
}

// ClearHistory clears all history and patterns.
func (p *Predictor) ClearHistory() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.history = p.history[:0]
	p.patterns = p.patterns[:0]
	p.save()
}

// Stats returns predictor statistics.
func (p *Predictor) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"patterns":    len(p.patterns),
		"history":     len(p.history),
		"max_history": p.maxHistory,
	}
}

func (p *Predictor) learnPattern(trigger string, ctxType ContextType, target string) {
	// Update existing pattern or create new
	for i := range p.patterns {
		if p.patterns[i].Trigger == trigger && p.patterns[i].Prefetch == target {
			p.patterns[i].Count++
			p.patterns[i].Recency = 1.0
			return
		}
	}

	// New pattern
	p.patterns = append(p.patterns, Pattern{
		Trigger:  trigger,
		Prefetch: target,
		Count:    1,
		Recency:  1.0,
		Type:     ctxType,
	})

	// Trim patterns to 200
	if len(p.patterns) > 200 {
		p.patterns = p.patterns[len(p.patterns)-200:]
	}
}

func matchesTrigger(command, trigger string) bool {
	// Fuzzy match: trigger could be a prefix or contain the command
	return strings.HasPrefix(command, trigger) || strings.Contains(command, trigger)
}

func (p *Predictor) save() {
	if p.storeDir == "" {
		return
	}
	os.MkdirAll(p.storeDir, 0755)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"patterns": p.patterns,
		"history":  p.history,
	}, "", "  ")
	os.WriteFile(filepath.Join(p.storeDir, "prefetch.json"), data, 0644)
}

func (p *Predictor) load() {
	if p.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(p.storeDir, "prefetch.json"))
	if err != nil {
		return
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if raw, ok := stored["patterns"]; ok {
		json.Unmarshal(raw, &p.patterns)
	}
	if raw, ok := stored["history"]; ok {
		json.Unmarshal(raw, &p.history)
	}
}

// FormatEntry formats a prefetch entry for display.
func FormatEntry(e *PrefetchEntry) string {
	status := "pending"
	if e.Loaded {
		status = "loaded"
	}
	return fmt.Sprintf("[%s] %-10s %-30s priority=%.2f (%s)",
		status, e.Type, e.Target, e.Priority, e.Reason)
}

// FormatPattern formats a pattern for display.
func FormatPattern(p *Pattern) string {
	return fmt.Sprintf("%-20s → %-30s (count: %d, recency: %.2f)",
		p.Trigger, p.Prefetch, p.Count, p.Recency)
}
