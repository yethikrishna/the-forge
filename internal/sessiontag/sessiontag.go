// Package sessiontag provides session tagging, filtering, auto-tagging,
// and saved searches for organizing agent sessions.
package sessiontag

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

// Tag represents a session tag.
type Tag struct {
	Name      string    `json:"name"`
	Color     string    `json:"color,omitempty"` // hex color code
	CreatedAt time.Time `json:"created_at"`
	AutoTag   bool      `json:"auto_tag"` // true if auto-generated
	Count     int       `json:"count"`    // number of sessions with this tag
}

// SessionTags represents tags on a session.
type SessionTags struct {
	SessionID string    `json:"session_id"`
	Tags      []string  `json:"tags"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SavedSearch represents a saved search query.
type SavedSearch struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Query     string            `json:"query"`
	Tags      []string          `json:"tags,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	LastUsed  time.Time         `json:"last_used,omitempty"`
	UseCount  int               `json:"use_count"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Store manages session tags.
type Store struct {
	mu       sync.RWMutex
	dir      string
	tags     map[string]*Tag           // tag name -> tag
	sessions map[string]*SessionTags   // session ID -> tags
	searches map[string]*SavedSearch   // search ID -> search
}

// NewStore creates a new session tag store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create tag dir: %w", err)
	}
	s := &Store{
		dir:      dir,
		tags:     make(map[string]*Tag),
		sessions: make(map[string]*SessionTags),
		searches: make(map[string]*SavedSearch),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	// Load tags
	data, err := os.ReadFile(filepath.Join(s.dir, "tags.json"))
	if err == nil {
		json.Unmarshal(data, &s.tags)
	}

	// Load session tags
	data, err = os.ReadFile(filepath.Join(s.dir, "sessions.json"))
	if err == nil {
		json.Unmarshal(data, &s.sessions)
	}

	// Load saved searches
	data, err = os.ReadFile(filepath.Join(s.dir, "searches.json"))
	if err == nil {
		json.Unmarshal(data, &s.searches)
	}
}

func (s *Store) saveTags() error {
	data, _ := json.MarshalIndent(s.tags, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, "tags.json"), data, 0644)
}

func (s *Store) saveSessions() error {
	data, _ := json.MarshalIndent(s.sessions, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, "sessions.json"), data, 0644)
}

func (s *Store) saveSearches() error {
	data, _ := json.MarshalIndent(s.searches, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, "searches.json"), data, 0644)
}

// CreateTag creates a new tag.
func (s *Store) CreateTag(name, color string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tags[name]; exists {
		return fmt.Errorf("tag %q already exists", name)
	}
	s.tags[name] = &Tag{
		Name:      name,
		Color:     color,
		CreatedAt: time.Now(),
	}
	return s.saveTags()
}

// GetTag retrieves a tag.
func (s *Store) GetTag(name string) (*Tag, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tags[name]
	return t, ok
}

// ListTags lists all tags.
func (s *Store) ListTags() []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Tag
	for _, t := range s.tags {
		result = append(result, *t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// DeleteTag removes a tag from all sessions.
func (s *Store) DeleteTag(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tags, name)

	for _, st := range s.sessions {
		for i, t := range st.Tags {
			if t == name {
				st.Tags = append(st.Tags[:i], st.Tags[i+1:]...)
				st.UpdatedAt = time.Now()
				break
			}
		}
	}

	s.saveTags()
	return s.saveSessions()
}

// TagSession adds tags to a session.
func (s *Store) TagSession(sessionID string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.sessions[sessionID]
	if !ok {
		st = &SessionTags{SessionID: sessionID}
		s.sessions[sessionID] = st
	}

	for _, tag := range tags {
		found := false
		for _, existing := range st.Tags {
			if existing == tag {
				found = true
				break
			}
		}
		if !found {
			st.Tags = append(st.Tags, tag)
		}

		// Ensure tag exists in tag registry
		if _, exists := s.tags[tag]; !exists {
			s.tags[tag] = &Tag{
				Name:      tag,
				CreatedAt: time.Now(),
				AutoTag:   false,
			}
		}
		s.tags[tag].Count++
	}
	st.UpdatedAt = time.Now()

	s.saveTags()
	return s.saveSessions()
}

// UntagSession removes tags from a session.
func (s *Store) UntagSession(sessionID string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}

	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
	}

	var newTags []string
	for _, t := range st.Tags {
		if !tagSet[t] {
			newTags = append(newTags, t)
		} else if tag, exists := s.tags[t]; exists {
			tag.Count--
		}
	}
	st.Tags = newTags
	st.UpdatedAt = time.Now()

	s.saveTags()
	return s.saveSessions()
}

// GetSessionTags returns tags for a session.
func (s *Store) GetSessionTags(sessionID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if st, ok := s.sessions[sessionID]; ok {
		return st.Tags
	}
	return nil
}

// FindSessions finds sessions matching the given tags (AND logic).
func (s *Store) FindSessions(tags []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []string
	for _, st := range s.sessions {
		if containsAll(st.Tags, tags) {
			result = append(result, st.SessionID)
		}
	}
	sort.Strings(result)
	return result
}

// FindSessionsAny finds sessions matching any of the given tags (OR logic).
func (s *Store) FindSessionsAny(tags []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []string
	for _, st := range s.sessions {
		for _, t := range st.Tags {
			if tagSet[t] {
				result = append(result, st.SessionID)
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

// AutoTag automatically tags a session based on its content.
func (s *Store) AutoTag(sessionID, prompt, output string) []string {
	var tags []string
	lower := strings.ToLower(prompt + " " + output)

	rules := map[string][]string{
		"bug-fix":    {"bug", "fix", "issue", "error", "crash", "broken"},
		"feature":    {"feature", "implement", "add", "create", "new"},
		"refactor":   {"refactor", "clean", "restructure", "simplify"},
		"security":   {"security", "vulnerability", "cve", "injection", "xss"},
		"test":       {"test", "testing", "coverage", "unit test"},
		"docs":       {"docs", "documentation", "readme", "comment"},
		"perf":       {"performance", "optimize", "speed", "slow", "latency"},
		"review":     {"review", "code review", "pr review"},
		"deployment": {"deploy", "release", "production", "staging"},
		"database":   {"database", "sql", "query", "migration", "schema"},
		"api":        {"api", "endpoint", "rest", "graphql", "handler"},
		"auth":       {"auth", "login", "oauth", "jwt", "token"},
		"docker":     {"docker", "container", "dockerfile", "compose"},
		"ci":         {"ci", "pipeline", "github actions", "jenkins"},
	}

	for tag, keywords := range rules {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				tags = append(tags, tag)
				break
			}
		}
	}

	if len(tags) > 0 {
		s.TagSession(sessionID, tags)
		// Mark as auto-generated
		s.mu.Lock()
		for _, tag := range tags {
			if t, ok := s.tags[tag]; ok {
				t.AutoTag = true
			}
		}
		s.saveTags()
		s.mu.Unlock()
	}

	return tags
}

// SaveSearch saves a search query.
func (s *Store) SaveSearch(name, query string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("search-%d", time.Now().UnixNano())
	s.searches[id] = &SavedSearch{
		ID:        id,
		Name:      name,
		Query:     query,
		Tags:      tags,
		CreatedAt: time.Now(),
	}
	return s.saveSearches()
}

// ListSearches lists saved searches.
func (s *Store) ListSearches() []SavedSearch {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []SavedSearch
	for _, ss := range s.searches {
		result = append(result, *ss)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ExecuteSearch executes a saved search.
func (s *Store) ExecuteSearch(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ss, ok := s.searches[id]
	if !ok {
		return nil, fmt.Errorf("search %s not found", id)
	}

	ss.UseCount++
	ss.LastUsed = time.Now()
	s.saveSearches()

	// Unlock for the find operation
	s.mu.Unlock()
	result := s.FindSessions(ss.Tags)
	s.mu.Lock()

	return result, nil
}

// DeleteSearch removes a saved search.
func (s *Store) DeleteSearch(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.searches, id)
	return s.saveSearches()
}

// Stats returns tag statistics.
func (s *Store) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalSessions := len(s.sessions)
	autoTags := 0
	for _, t := range s.tags {
		if t.AutoTag {
			autoTags++
		}
	}

	return map[string]interface{}{
		"total_tags":     len(s.tags),
		"auto_tags":      autoTags,
		"tagged_sessions": totalSessions,
		"saved_searches": len(s.searches),
	}
}

func containsAll(haystack, needles []string) bool {
	needleSet := make(map[string]bool)
	for _, n := range needles {
		needleSet[n] = true
	}
	for _, h := range haystack {
		delete(needleSet, h)
	}
	return len(needleSet) == 0
}
