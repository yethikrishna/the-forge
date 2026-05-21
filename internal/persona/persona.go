// Package persona provides persistent agent personas with style preferences,
// memory, and trust scores. Personas enable consistent agent behavior across
// sessions and support persona switching for different tasks.
package persona

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

// Style represents an agent's communication style.
type Style struct {
	Tone       string   `json:"tone"`        // "formal", "casual", "technical", "friendly"
	Verbosity  string   `json:"verbosity"`   // "concise", "moderate", "detailed"
	Humor      float64  `json:"humor"`       // 0-1
	Proactivity float64 `json:"proactivity"` // 0-1, how proactive to be
	Emojis     bool     `json:"emojis"`
	Headers    bool     `json:"headers"`     // use markdown headers
	CodeBlocks bool     `json:"code_blocks"` // prefer code blocks
	Language   string   `json:"language"`    // preferred language
	Formats    []string `json:"formats"`     // preferred output formats
}

// Preference represents a persona preference.
type Preference struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Priority  int    `json:"priority"` // 1-5, higher = more important
	Reason    string `json:"reason,omitempty"`
}

// TrustLevel represents the trust level for a persona.
type TrustLevel string

const (
	TrustUntrusted TrustLevel = "untrusted"
	TrustLimited   TrustLevel = "limited"
	TrustStandard  TrustLevel = "standard"
	TrustTrusted   TrustLevel = "trusted"
	TrustFull      TrustLevel = "full"
)

// Persona represents a persistent agent persona.
type Persona struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Avatar      string            `json:"avatar,omitempty"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastUsed    time.Time         `json:"last_used,omitempty"`
	UseCount    int               `json:"use_count"`
	Style       Style             `json:"style"`
	Preferences []Preference      `json:"preferences"`
	TrustLevel  TrustLevel        `json:"trust_level"`
	TrustScore  float64           `json:"trust_score"` // 0-100
	ModelPrefs  map[string]string `json:"model_prefs,omitempty"` // task type -> preferred model
	Tags        []string          `json:"tags"`
	MemoryIDs   []string          `json:"memory_ids,omitempty"` // associated memory IDs
	MaxCost     float64           `json:"max_cost,omitempty"`   // per-session cost cap
	Scope       string            `json:"scope,omitempty"`      // "read-only", "src-only", "sandbox", "full"
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Store manages personas.
type Store struct {
	mu       sync.RWMutex
	dir      string
	personas map[string]*Persona
}

// NewStore creates a new persona store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create persona dir: %w", err)
	}
	s := &Store{
		dir:      dir,
		personas: make(map[string]*Persona),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var p Persona
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		s.personas[p.ID] = &p
	}
}

func (s *Store) save(p *Persona) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal persona: %w", err)
	}
	return os.WriteFile(filepath.Join(s.dir, p.ID+".json"), data, 0644)
}

// Create creates a new persona.
func (s *Store) Create(p *Persona) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		p.ID = fmt.Sprintf("persona-%d", time.Now().UnixNano())
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	if p.TrustLevel == "" {
		p.TrustLevel = TrustStandard
	}
	if p.TrustScore == 0 {
		p.TrustScore = 50
	}
	if p.Style.Tone == "" {
		p.Style.Tone = "technical"
	}
	if p.Style.Verbosity == "" {
		p.Style.Verbosity = "moderate"
	}

	s.personas[p.ID] = p
	return s.save(p)
}

// Get retrieves a persona.
func (s *Store) Get(id string) (*Persona, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.personas[id]
	return p, ok
}

// GetByName retrieves a persona by name.
func (s *Store) GetByName(name string) (*Persona, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.personas {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}

// List lists all personas.
func (s *Store) List() []Persona {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Persona
	for _, p := range s.personas {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Update updates a persona.
func (s *Store) Update(p *Persona) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.personas[p.ID]; !ok {
		return fmt.Errorf("persona %s not found", p.ID)
	}
	p.UpdatedAt = time.Now()
	s.personas[p.ID] = p
	return s.save(p)
}

// Delete removes a persona.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.personas[id]; !ok {
		return fmt.Errorf("persona %s not found", id)
	}
	delete(s.personas, id)
	os.Remove(filepath.Join(s.dir, id+".json"))
	return nil
}

// RecordUse records a persona usage.
func (s *Store) RecordUse(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.personas[id]
	if !ok {
		return fmt.Errorf("persona %s not found", id)
	}
	p.UseCount++
	p.LastUsed = time.Now()
	return s.save(p)
}

// UpdateTrust updates a persona's trust score.
func (s *Store) UpdateTrust(id string, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.personas[id]
	if !ok {
		return fmt.Errorf("persona %s not found", id)
	}

	p.TrustScore += delta
	if p.TrustScore > 100 {
		p.TrustScore = 100
	}
	if p.TrustScore < 0 {
		p.TrustScore = 0
	}

	// Update trust level based on score
	switch {
	case p.TrustScore >= 90:
		p.TrustLevel = TrustFull
	case p.TrustScore >= 70:
		p.TrustLevel = TrustTrusted
	case p.TrustScore >= 50:
		p.TrustLevel = TrustStandard
	case p.TrustScore >= 25:
		p.TrustLevel = TrustLimited
	default:
		p.TrustLevel = TrustUntrusted
	}

	p.UpdatedAt = time.Now()
	return s.save(p)
}

// SetPreference sets a preference for a persona.
func (s *Store) SetPreference(id, key, value string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.personas[id]
	if !ok {
		return fmt.Errorf("persona %s not found", id)
	}

	// Check if preference exists
	for i, pref := range p.Preferences {
		if pref.Key == key {
			p.Preferences[i].Value = value
			p.Preferences[i].Priority = priority
			p.UpdatedAt = time.Now()
			return s.save(p)
		}
	}

	// Add new preference
	p.Preferences = append(p.Preferences, Preference{
		Key:      key,
		Value:    value,
		Priority: priority,
	})
	p.UpdatedAt = time.Now()
	return s.save(p)
}

// GetPreference gets a preference for a persona.
func (s *Store) GetPreference(id, key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.personas[id]
	if !ok {
		return "", false
	}
	for _, pref := range p.Preferences {
		if pref.Key == key {
			return pref.Value, true
		}
	}
	return "", false
}

// DefaultPersonas returns a set of built-in personas.
func DefaultPersonas() []Persona {
	return []Persona{
		{
			Name:        "coder",
			Description: "Technical coding assistant focused on writing clean, efficient code",
			Style: Style{
				Tone:       "technical",
				Verbosity:  "concise",
				Humor:      0.1,
				Proactivity: 0.7,
				CodeBlocks: true,
			},
			TrustLevel: TrustTrusted,
			TrustScore: 75,
			Tags:       []string{"coding", "implementation"},
		},
		{
			Name:        "reviewer",
			Description: "Code review specialist focused on quality, security, and best practices",
			Style: Style{
				Tone:       "formal",
				Verbosity:  "detailed",
				Humor:      0,
				Proactivity: 0.3,
				CodeBlocks: true,
			},
			TrustLevel: TrustStandard,
			TrustScore: 60,
			Tags:       []string{"review", "quality", "security"},
		},
		{
			Name:        "planner",
			Description: "Strategic planner that breaks down complex tasks and designs solutions",
			Style: Style{
				Tone:       "friendly",
				Verbosity:  "detailed",
				Humor:      0.2,
				Proactivity: 0.9,
				Headers:    true,
			},
			TrustLevel: TrustTrusted,
			TrustScore: 80,
			Tags:       []string{"planning", "architecture", "design"},
		},
		{
			Name:        "debugger",
			Description: "Debugging specialist that systematically traces and fixes issues",
			Style: Style{
				Tone:       "technical",
				Verbosity:  "moderate",
				Humor:      0.05,
				Proactivity: 0.8,
				CodeBlocks: true,
			},
			TrustLevel: TrustTrusted,
			TrustScore: 70,
			Tags:       []string{"debugging", "troubleshooting"},
		},
		{
			Name:        "explainer",
			Description: "Teacher-like persona that explains concepts clearly with examples",
			Style: Style{
				Tone:       "friendly",
				Verbosity:  "detailed",
				Humor:      0.3,
				Proactivity: 0.4,
				Headers:    true,
				CodeBlocks: true,
			},
			TrustLevel: TrustStandard,
			TrustScore: 65,
			Tags:       []string{"teaching", "documentation", "explanation"},
		},
	}
}

// FormatSystemPrompt generates a system prompt from a persona.
func FormatSystemPrompt(p *Persona) string {
	var b strings.Builder

	fmt.Fprintf(&b, "You are %s. %s\n\n", p.Name, p.Description)

	fmt.Fprintf(&b, "Style:\n")
	fmt.Fprintf(&b, "- Tone: %s\n", p.Style.Tone)
	fmt.Fprintf(&b, "- Verbosity: %s\n", p.Style.Verbosity)
	if p.Style.Humor > 0.3 {
		fmt.Fprintf(&b, "- Use humor occasionally\n")
	}
	if p.Style.Proactivity > 0.6 {
		fmt.Fprintf(&b, "- Be proactive: anticipate needs and take initiative\n")
	}
	if !p.Style.Emojis {
		fmt.Fprintf(&b, "- Do not use emojis\n")
	}
	if p.Style.CodeBlocks {
		fmt.Fprintf(&b, "- Use code blocks for code\n")
	}

	if len(p.Preferences) > 0 {
		fmt.Fprintf(&b, "\nPreferences:\n")
		sort.Slice(p.Preferences, func(i, j int) bool {
			return p.Preferences[i].Priority > p.Preferences[j].Priority
		})
		for _, pref := range p.Preferences {
			fmt.Fprintf(&b, "- %s: %s\n", pref.Key, pref.Value)
		}
	}

	if p.MaxCost > 0 {
		fmt.Fprintf(&b, "\nBudget: $%.2f per session\n", p.MaxCost)
	}

	if p.Scope != "" {
		fmt.Fprintf(&b, "Scope: %s\n", p.Scope)
	}

	return b.String()
}
