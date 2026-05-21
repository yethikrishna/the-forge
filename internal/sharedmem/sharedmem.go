// Package sharedmem provides shared agent memory (opt-in).
// Cross-team learning with privacy-preserving pattern sharing.
// Agents can contribute and retrieve anonymized patterns.
//
// Learn together, grow together.
package sharedmem

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

// Pattern is a learned behavior pattern.
type Pattern struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Steps       []string          `json:"steps"`
	Conditions  []string          `json:"conditions"`
	Tags        []string          `json:"tags"`
	Contributor string            `json:"contributor"`
	Uses        int               `json:"uses"`
	SuccessRate float64           `json:"success_rate"`
	Privacy     PrivacyLevel      `json:"privacy"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PrivacyLevel controls sharing scope.
type PrivacyLevel string

const (
	PrivacyPrivate  PrivacyLevel = "private"  // Only the contributor
	PrivacyTeam     PrivacyLevel = "team"     // Team members only
	PrivacyPublic   PrivacyLevel = "public"   // Everyone
)

// Insight is a shared learning insight.
type Insight struct {
	ID          string    `json:"id"`
	PatternID   string    `json:"pattern_id"`
	Type        string    `json:"type"` // tip, warning, best_practice, anti_pattern
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Contributor string    `json:"contributor"`
	Votes       int       `json:"votes"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store manages shared agent memory.
type Store struct {
	patterns map[string]*Pattern
	insights map[string]*Insight
	storeDir string
	nextID   int
	insID    int
	mu       sync.RWMutex
}

// NewStore creates a shared memory store.
func NewStore(storeDir string) *Store {
	s := &Store{
		patterns: make(map[string]*Pattern),
		insights: make(map[string]*Insight),
		storeDir: storeDir,
	}
	s.registerDefaults()
	s.load()
	return s
}

func (s *Store) registerDefaults() {
	defaults := []Pattern{
		{
			Name:        "Test after edit",
			Category:    "workflow",
			Description: "Always run tests after editing source files",
			Steps:       []string{"Edit file", "Run tests", "Verify no regressions"},
			Conditions:  []string{"Source file was modified"},
			Tags:        []string{"testing", "workflow"},
			Contributor: "system",
			SuccessRate: 0.95,
			Privacy:     PrivacyPublic,
		},
		{
			Name:        "Search before coding",
			Category:    "workflow",
			Description: "Search codebase for existing implementations before writing new code",
			Steps:       []string{"Search for similar code", "Review existing patterns", "Reuse or adapt"},
			Conditions:  []string{"Starting a new feature or fix"},
			Tags:        []string{"research", "workflow"},
			Contributor: "system",
			SuccessRate: 0.85,
			Privacy:     PrivacyPublic,
		},
		{
			Name:        "Incremental commits",
			Category:    "workflow",
			Description: "Commit frequently with small, focused changes",
			Steps:       []string{"Make small change", "Verify it works", "Commit with descriptive message"},
			Conditions:  []string{"Working on a multi-step task"},
			Tags:        []string{"git", "workflow"},
			Contributor: "system",
			SuccessRate: 0.9,
			Privacy:     PrivacyPublic,
		},
	}

	for i := range defaults {
		s.nextID++
		defaults[i].ID = fmt.Sprintf("pat-%d", s.nextID)
		defaults[i].CreatedAt = time.Now()
		defaults[i].UpdatedAt = time.Now()
		s.patterns[defaults[i].ID] = &defaults[i]
	}
}

// Contribute adds a pattern to shared memory.
func (s *Store) Contribute(p Pattern) (*Pattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	s.nextID++
	p.ID = fmt.Sprintf("pat-%d", s.nextID)
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()
	if p.Metadata == nil {
		p.Metadata = make(map[string]string)
	}

	s.patterns[p.ID] = &p
	s.save()
	return &p, nil
}

// Get returns a pattern by ID.
func (s *Store) Get(id string) (*Pattern, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.patterns[id]
	if !ok {
		return nil, false
	}
	copy := *p
	return &copy, true
}

// List returns patterns, optionally filtered by category and privacy.
func (s *Store) List(category string, privacy PrivacyLevel) []Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Pattern
	for _, p := range s.patterns {
		if category != "" && p.Category != category {
			continue
		}
		if privacy != "" && p.Privacy != privacy {
			continue
		}
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SuccessRate > result[j].SuccessRate
	})
	return result
}

// Search finds patterns matching a query.
func (s *Store) Search(query string) []Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var result []Pattern
	for _, p := range s.patterns {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Description), q) ||
			strings.Contains(strings.ToLower(p.Category), q) ||
			tagsMatch(p.Tags, q) {
			result = append(result, *p)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SuccessRate > result[j].SuccessRate
	})
	return result
}

// RecordUse records a pattern usage.
func (s *Store) RecordUse(id string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.patterns[id]
	if !ok {
		return fmt.Errorf("pattern %q not found", id)
	}
	p.Uses++
	// Update rolling success rate
	total := float64(p.Uses)
	oldRate := p.SuccessRate * (total - 1) / total
	newVal := 0.0
	if success {
		newVal = 1.0
	}
	p.SuccessRate = oldRate + newVal/total
	p.UpdatedAt = time.Now()
	s.save()
	return nil
}

// VoteUp votes on an insight.
func (s *Store) VoteUp(insightID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ins, ok := s.insights[insightID]
	if !ok {
		return fmt.Errorf("insight %q not found", insightID)
	}
	ins.Votes++
	s.save()
	return nil
}

// AddInsight adds a shared insight.
func (s *Store) AddInsight(patternID, insightType, title, content, contributor string) (*Insight, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.insID++
	ins := &Insight{
		ID:          fmt.Sprintf("ins-%d", s.insID),
		PatternID:   patternID,
		Type:        insightType,
		Title:       title,
		Content:     content,
		Contributor: contributor,
		CreatedAt:   time.Now(),
	}
	s.insights[ins.ID] = ins
	s.save()
	return ins, nil
}

// GetInsights returns insights for a pattern.
func (s *Store) GetInsights(patternID string) []Insight {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Insight
	for _, ins := range s.insights {
		if ins.PatternID == patternID {
			result = append(result, *ins)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Votes > result[j].Votes
	})
	return result
}

// Top returns most successful patterns.
func (s *Store) Top(limit int) []Pattern {
	all := s.List("", "")
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all
}

// Stats returns store statistics.
func (s *Store) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	catCount := make(map[string]int)
	for _, p := range s.patterns {
		catCount[p.Category]++
	}
	return map[string]interface{}{
		"patterns":    len(s.patterns),
		"insights":    len(s.insights),
		"categories":  catCount,
	}
}

func tagsMatch(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func (s *Store) save() {
	if s.storeDir == "" {
		return
	}
	os.MkdirAll(s.storeDir, 0755)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"patterns": s.patterns,
		"insights": s.insights,
	}, "", "  ")
	os.WriteFile(filepath.Join(s.storeDir, "shared_memory.json"), data, 0644)
}

func (s *Store) load() {
	if s.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(s.storeDir, "shared_memory.json"))
	if err != nil {
		return
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if raw, ok := stored["patterns"]; ok {
		json.Unmarshal(raw, &s.patterns)
	}
	if raw, ok := stored["insights"]; ok {
		json.Unmarshal(raw, &s.insights)
	}
	s.nextID = len(s.patterns)
	s.insID = len(s.insights)
}

// FormatPattern formats a pattern for display.
func FormatPattern(p *Pattern) string {
	return fmt.Sprintf("%s  [%s] %.0f%% success  uses:%d  %s",
		p.ID, p.Category, p.SuccessRate*100, p.Uses, p.Name)
}

// FormatInsight formats an insight for display.
func FormatInsight(ins *Insight) string {
	return fmt.Sprintf("[%s] %s (%d votes) — %s",
		ins.Type, ins.Title, ins.Votes, ins.Content)
}
