// Package playbook auto-generates playbooks from solved agent sessions.
// Extracts step-by-step procedures from successful agent runs
// for reuse and documentation.
//
// Success has a pattern. Find it.
package playbook

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

// Step is a single step in a playbook.
type Step struct {
	Index     int               `json:"index"`
	Title     string            `json:"title"`
	Action    string            `json:"action"`
	Target    string            `json:"target,omitempty"`
	Result    string            `json:"result,omitempty"`
	Success   bool              `json:"success"`
	Duration  time.Duration     `json:"duration"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Playbook is a generated playbook from a session.
type Playbook struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SessionID   string    `json:"session_id"`
	AgentID     string    `json:"agent_id"`
	Steps       []Step    `json:"steps"`
	Tags        []string  `json:"tags"`
	SuccessRate float64   `json:"success_rate"` // 0-1
	Uses        int       `json:"uses"`
	Source      string    `json:"source"` // auto, manual
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SessionLog represents a solved agent session for extraction.
type SessionLog struct {
	SessionID string    `json:"session_id"`
	AgentID   string    `json:"agent_id"`
	Goal      string    `json:"goal"`
	Outcome   string    `json:"outcome"`
	Success   bool      `json:"success"`
	Actions   []Action  `json:"actions"`
	Timestamp time.Time `json:"timestamp"`
}

// Action represents a single agent action from a session log.
type Action struct {
	Type     string    `json:"type"` // read, write, execute, search, api_call
	Target   string    `json:"target"`
	Input    string    `json:"input,omitempty"`
	Output   string    `json:"output,omitempty"`
	Success  bool      `json:"success"`
	Duration float64   `json:"duration_ms"`
	Time     time.Time `json:"time"`
}

// Generator generates playbooks from session logs.
type Generator struct {
	playbooks map[string]*Playbook
	storeDir  string
	nextID    int
	mu        sync.RWMutex
}

// NewGenerator creates a playbook generator.
func NewGenerator(storeDir string) *Generator {
	g := &Generator{
		playbooks: make(map[string]*Playbook),
		storeDir:  storeDir,
	}
	g.load()
	return g
}

// Generate generates a playbook from a session log.
func (g *Generator) Generate(log SessionLog) (*Playbook, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(log.Actions) == 0 {
		return nil, fmt.Errorf("session has no actions")
	}

	g.nextID++
	pb := &Playbook{
		ID:          fmt.Sprintf("pb-%d", g.nextID),
		Name:        extractName(log.Goal),
		Description: fmt.Sprintf("Auto-generated from session %s (agent: %s)", log.SessionID, log.AgentID),
		SessionID:   log.SessionID,
		AgentID:     log.AgentID,
		Steps:       extractSteps(log.Actions),
		Tags:        extractTags(log),
		SuccessRate: calcSuccessRate(log.Actions),
		Source:      "auto",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	g.playbooks[pb.ID] = pb
	g.save()
	return pb, nil
}

// Create creates a manual playbook.
func (g *Generator) Create(name, description string, steps []Step) *Playbook {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nextID++
	pb := &Playbook{
		ID:          fmt.Sprintf("pb-%d", g.nextID),
		Name:        name,
		Description: description,
		Steps:       steps,
		Source:      "manual",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	for i := range pb.Steps {
		pb.Steps[i].Index = i + 1
	}

	g.playbooks[pb.ID] = pb
	g.save()
	return pb
}

// Get returns a playbook by ID.
func (g *Generator) Get(id string) (*Playbook, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	pb, ok := g.playbooks[id]
	if !ok {
		return nil, false
	}
	copy := *pb
	return &copy, true
}

// List returns all playbooks.
func (g *Generator) List() []Playbook {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Playbook, 0, len(g.playbooks))
	for _, pb := range g.playbooks {
		result = append(result, *pb)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// Search finds playbooks matching a query.
func (g *Generator) Search(query string) []Playbook {
	g.mu.RLock()
	defer g.mu.RUnlock()

	q := strings.ToLower(query)
	var result []Playbook
	for _, pb := range g.playbooks {
		if strings.Contains(strings.ToLower(pb.Name), q) ||
			strings.Contains(strings.ToLower(pb.Description), q) ||
			tagsMatch(pb.Tags, q) {
			result = append(result, *pb)
		}
	}
	return result
}

// RecordUse records a playbook usage.
func (g *Generator) RecordUse(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	pb, ok := g.playbooks[id]
	if !ok {
		return fmt.Errorf("playbook %q not found", id)
	}
	pb.Uses++
	pb.UpdatedAt = time.Now()
	g.save()
	return nil
}

// Delete removes a playbook.
func (g *Generator) Delete(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.playbooks[id]; !ok {
		return fmt.Errorf("playbook %q not found", id)
	}
	delete(g.playbooks, id)
	g.save()
	return nil
}

// Top returns most-used playbooks.
func (g *Generator) Top(limit int) []Playbook {
	g.mu.RLock()
	defer g.mu.RUnlock()

	all := make([]Playbook, 0, len(g.playbooks))
	for _, pb := range g.playbooks {
		all = append(all, *pb)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Uses > all[j].Uses
	})
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all
}

func extractSteps(actions []Action) []Step {
	var steps []Step
	for i, a := range actions {
		step := Step{
			Index:    i + 1,
			Title:    fmt.Sprintf("%s %s", a.Type, a.Target),
			Action:   a.Type,
			Target:   a.Target,
			Result:   truncate(a.Output, 100),
			Success:  a.Success,
			Duration: time.Duration(a.Duration) * time.Millisecond,
		}
		steps = append(steps, step)
	}
	return steps
}

func extractName(goal string) string {
	if len(goal) > 60 {
		return goal[:57] + "..."
	}
	return goal
}

func extractTags(log SessionLog) []string {
	tagSet := make(map[string]bool)

	// Tag by action types
	for _, a := range log.Actions {
		tagSet[a.Type] = true
	}

	// Tag by outcome
	if log.Success {
		tagSet["successful"] = true
	} else {
		tagSet["failed"] = true
	}

	var tags []string
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

func calcSuccessRate(actions []Action) float64 {
	if len(actions) == 0 {
		return 0
	}
	success := 0
	for _, a := range actions {
		if a.Success {
			success++
		}
	}
	return float64(success) / float64(len(actions))
}

func tagsMatch(tags []string, query string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (g *Generator) save() {
	if g.storeDir == "" {
		return
	}
	os.MkdirAll(g.storeDir, 0755)
	data, _ := json.MarshalIndent(g.playbooks, "", "  ")
	os.WriteFile(filepath.Join(g.storeDir, "playbooks.json"), data, 0644)
}

func (g *Generator) load() {
	if g.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(g.storeDir, "playbooks.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &g.playbooks)
	g.nextID = len(g.playbooks)
}

// FormatPlaybook formats a playbook for display.
func FormatPlaybook(pb *Playbook) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Playbook:    %s\n", pb.Name))
	b.WriteString(fmt.Sprintf("ID:          %s\n", pb.ID))
	b.WriteString(fmt.Sprintf("Source:      %s\n", pb.Source))
	b.WriteString(fmt.Sprintf("Steps:       %d\n", len(pb.Steps)))
	b.WriteString(fmt.Sprintf("Success:     %.0f%%\n", pb.SuccessRate*100))
	b.WriteString(fmt.Sprintf("Uses:        %d\n", pb.Uses))
	b.WriteString(fmt.Sprintf("Tags:        %s\n", strings.Join(pb.Tags, ", ")))

	if len(pb.Steps) > 0 {
		b.WriteString("\nSteps:\n")
		for _, s := range pb.Steps {
			status := "✓"
			if !s.Success {
				status = "✗"
			}
			b.WriteString(fmt.Sprintf("  %d. [%s] %s\n", s.Index, status, s.Title))
			if s.Result != "" {
				b.WriteString(fmt.Sprintf("     → %s\n", s.Result))
			}
		}
	}
	return b.String()
}
