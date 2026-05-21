// Package sessiontag provides session tagging and organization.
// Tag sessions, filter by tags, auto-tag based on content,
// and save searches for quick access.
//
// Organize or drown.
package sessiontag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Color represents a tag color.
type Color string

const (
	ColorRed    Color = "red"
	ColorOrange Color = "orange"
	ColorYellow Color = "yellow"
	ColorGreen  Color = "green"
	ColorBlue   Color = "blue"
	ColorPurple Color = "purple"
	ColorGray   Color = "gray"
)

// Tag represents a session tag.
type Tag struct {
	Name      string    `json:"name"`
	Color     Color     `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	Count     int       `json:"count"` // sessions with this tag
}

// Session represents a tagged session.
type Session struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Tags      []string          `json:"tags"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// AutoRule is a rule for auto-tagging sessions.
type AutoRule struct {
	ID       string `json:"id"`
	Tag      string `json:"tag"`
	Pattern  string `json:"pattern"`  // regex to match title/content
	Field    string `json:"field"`    // title, content, metadata
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// SavedSearch is a saved filter combination.
type SavedSearch struct {
	Name      string   `json:"name"`
	Tags      []string `json:"tags"`
	Query     string   `json:"query"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager manages session tags.
type Manager struct {
	sessions     map[string]*Session
	tags         map[string]*Tag
	autoRules    []AutoRule
	savedSearches []SavedSearch
	storeDir     string
	mu           sync.Mutex
}

// NewManager creates a session tag manager.
func NewManager(storeDir string) *Manager {
	m := &Manager{
		sessions:     make(map[string]*Session),
		tags:         make(map[string]*Tag),
		autoRules:    make([]AutoRule, 0),
		storeDir:     storeDir,
	}
	m.registerDefaultTags()
	m.registerDefaultRules()
	m.load()
	return m
}

func (m *Manager) registerDefaultTags() {
	defaults := []struct {
		name  string
		color Color
	}{
		{"bug", ColorRed}, {"feature", ColorGreen}, {"refactor", ColorBlue},
		{"docs", ColorGray}, {"test", ColorOrange}, {"urgent", ColorRed},
		{"review", ColorPurple}, {"wip", ColorYellow},
	}
	for _, d := range defaults {
		m.tags[d.name] = &Tag{
			Name:      d.name,
			Color:     d.color,
			CreatedAt: time.Now(),
		}
	}
}

func (m *Manager) registerDefaultRules() {
	rules := []AutoRule{
		{ID: "auto-fix", Tag: "bug", Pattern: `(?i)(fix|bug|patch|hotfix)`, Field: "title", Enabled: true, Priority: 10},
		{ID: "auto-feat", Tag: "feature", Pattern: `(?i)(feat|feature|add|implement)`, Field: "title", Enabled: true, Priority: 10},
		{ID: "auto-docs", Tag: "docs", Pattern: `(?i)(doc|readme|guide|tutorial)`, Field: "title", Enabled: true, Priority: 10},
		{ID: "auto-test", Tag: "test", Pattern: `(?i)(test|spec|verify)`, Field: "title", Enabled: true, Priority: 10},
		{ID: "auto-urgent", Tag: "urgent", Pattern: `(?i)(urgent|critical|asap|hot)`, Field: "title", Enabled: true, Priority: 20},
	}
	m.autoRules = append(m.autoRules, rules...)
}

// CreateTag creates a new tag.
func (m *Manager) CreateTag(name string, color Color) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tags[name]; exists {
		return fmt.Errorf("tag %q already exists", name)
	}
	m.tags[name] = &Tag{
		Name:      name,
		Color:     color,
		CreatedAt: time.Now(),
	}
	m.save()
	return nil
}

// DeleteTag removes a tag from all sessions.
func (m *Manager) DeleteTag(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tags[name]; !exists {
		return fmt.Errorf("tag %q not found", name)
	}

	delete(m.tags, name)
	for _, s := range m.sessions {
		s.Tags = removeString(s.Tags, name)
	}
	m.save()
	return nil
}

// ListTags returns all tags.
func (m *Manager) ListTags() []Tag {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]Tag, 0, len(m.tags))
	for _, t := range m.tags {
		count := 0
		for _, s := range m.sessions {
			if containsString(s.Tags, t.Name) {
				count++
			}
		}
		result = append(result, Tag{Name: t.Name, Color: t.Color, Count: count, CreatedAt: t.CreatedAt})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// TagSession adds tags to a session.
func (m *Manager) TagSession(sessionID string, tags []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		session = &Session{
			ID:        sessionID,
			Tags:      []string{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  make(map[string]string),
		}
		m.sessions[sessionID] = session
	}

	for _, tag := range tags {
		if !containsString(session.Tags, tag) {
			session.Tags = append(session.Tags, tag)
		}
	}
	session.UpdatedAt = time.Now()
	m.save()
	return nil
}

// UntagSession removes tags from a session.
func (m *Manager) UntagSession(sessionID string, tags []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}

	for _, tag := range tags {
		session.Tags = removeString(session.Tags, tag)
	}
	session.UpdatedAt = time.Now()
	m.save()
	return nil
}

// AutoTag applies auto-tagging rules to a session.
func (m *Manager) AutoTag(sessionID, title string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		session = &Session{
			ID:        sessionID,
			Title:     title,
			Tags:      []string{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  make(map[string]string),
		}
		m.sessions[sessionID] = session
	}
	session.Title = title

	var applied []string
	for _, rule := range m.autoRules {
		if !rule.Enabled {
			continue
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		text := ""
		switch rule.Field {
		case "title":
			text = title
		}

		if re.MatchString(text) && !containsString(session.Tags, rule.Tag) {
			session.Tags = append(session.Tags, rule.Tag)
			applied = append(applied, rule.Tag)
		}
	}

	session.UpdatedAt = time.Now()
	m.save()
	return applied
}

// FindByTags finds sessions matching all specified tags.
func (m *Manager) FindByTags(tags []string) []Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []Session
	for _, s := range m.sessions {
		if allTagsMatch(s.Tags, tags) {
			result = append(result, *s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// FindByQuery finds sessions matching a text query.
func (m *Manager) FindByQuery(query string) []Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	q := strings.ToLower(query)
	var result []Session
	for _, s := range m.sessions {
		if strings.Contains(strings.ToLower(s.Title), q) ||
			strings.Contains(strings.ToLower(s.ID), q) {
			result = append(result, *s)
		}
	}
	return result
}

// GetSession returns a session by ID.
func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

// ListSessions returns all sessions.
func (m *Manager) ListSessions() []Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// AddAutoRule adds an auto-tagging rule.
func (m *Manager) AddAutoRule(rule AutoRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoRules = append(m.autoRules, rule)
	m.save()
}

// SaveSearch saves a search for later reuse.
func (m *Manager) SaveSearch(name string, tags []string, query string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedSearches = append(m.savedSearches, SavedSearch{
		Name: name, Tags: tags, Query: query, CreatedAt: time.Now(),
	})
	m.save()
}

// GetSavedSearches returns all saved searches.
func (m *Manager) GetSavedSearches() []SavedSearch {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.savedSearches
}

func (m *Manager) save() {
	if m.storeDir == "" {
		return
	}
	os.MkdirAll(m.storeDir, 0755)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"sessions":      m.sessions,
		"tags":          m.tags,
		"auto_rules":    m.autoRules,
		"saved_searches": m.savedSearches,
	}, "", "  ")
	os.WriteFile(filepath.Join(m.storeDir, "session_tags.json"), data, 0644)
}

func (m *Manager) load() {
	if m.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.storeDir, "session_tags.json"))
	if err != nil {
		return
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if raw, ok := stored["sessions"]; ok {
		json.Unmarshal(raw, &m.sessions)
	}
	if raw, ok := stored["tags"]; ok {
		json.Unmarshal(raw, &m.tags)
	}
	if raw, ok := stored["auto_rules"]; ok {
		json.Unmarshal(raw, &m.autoRules)
	}
	if raw, ok := stored["saved_searches"]; ok {
		json.Unmarshal(raw, &m.savedSearches)
	}
}

// FormatSession formats a session for display.
func FormatSession(s *Session) string {
	tags := strings.Join(s.Tags, ", ")
	if tags == "" {
		tags = "(none)"
	}
	return fmt.Sprintf("%-20s %-30s [%s] %s",
		s.ID, truncate(s.Title, 28), tags, s.UpdatedAt.Format("2006-01-02"))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}

func allTagsMatch(sessionTags, requiredTags []string) bool {
	for _, req := range requiredTags {
		if !containsString(sessionTags, req) {
			return false
		}
	}
	return true
}
