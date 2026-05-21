// Package knowledge provides a persistent knowledge base for AI agents.
// Store, search, and retrieve knowledge entries with auto-indexing,
// deduplication, and relevance scoring. Like a vector database but
// simpler — built for single-binary deployment.
package knowledge

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EntryType defines the type of knowledge entry.
type EntryType string

const (
	TypeFact       EntryType = "fact"
	TypeProcedure  EntryType = "procedure"
	TypeDecision   EntryType = "decision"
	TypePattern    EntryType = "pattern"
	TypeError      EntryType = "error"
	TypeReference  EntryType = "reference"
	TypeInsight    EntryType = "insight"
)

// Entry represents a knowledge entry.
type Entry struct {
	ID          string    `json:"id"`
	Type        EntryType `json:"type"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Source      string    `json:"source,omitempty"`  // Where this knowledge came from
	AgentID     string    `json:"agent_id,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Category    string    `json:"category,omitempty"`
	Confidence  float64   `json:"confidence"`  // 0-1
	AccessCount int       `json:"access_count"`
	Hash        string    `json:"hash"`        // Content hash for dedup
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SearchResult represents a search hit.
type SearchResult struct {
	Entry     *Entry  `json:"entry"`
	Score     float64 `json:"score"`
	MatchType string  `json:"match_type"` // "title", "content", "tag"
}

// Store is a knowledge base store.
type Store struct {
	storeDir string
	entries  map[string]*Entry
	index    map[string][]string // tag → entry IDs
	mu       sync.Mutex
}

// NewStore creates a new knowledge store.
func NewStore(storeDir string) *Store {
	os.MkdirAll(storeDir, 0755)
	s := &Store{
		storeDir: storeDir,
		entries:  make(map[string]*Entry),
		index:    make(map[string][]string),
	}
	s.load()
	return s
}

// Add adds a knowledge entry.
func (s *Store) Add(entryType EntryType, title, content, source string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Check for duplicates
	hash := contentHash(content)
	for _, e := range s.entries {
		if e.Hash == hash {
			return e, nil // Already exists
		}
	}

	now := time.Now()
	id := fmt.Sprintf("kb-%x", sha256.Sum256([]byte(title+content)))[:16]

	entry := &Entry{
		ID:         id,
		Type:       entryType,
		Title:      title,
		Content:    content,
		Source:     source,
		Hash:       hash,
		Confidence: 1.0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	s.entries[id] = entry
	s.save()
	return entry, nil
}

// Get retrieves an entry by ID.
func (s *Store) Get(id string) (*Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if ok {
		e.AccessCount++
	}
	return e, ok
}

// Update updates an entry.
func (s *Store) Update(id string, title, content string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return fmt.Errorf("entry %s not found", id)
	}

	if title != "" {
		e.Title = title
	}
	if content != "" {
		e.Content = content
		e.Hash = contentHash(content)
	}
	if tags != nil {
		e.Tags = tags
		// Rebuild tag index
		s.rebuildIndex()
	}
	e.UpdatedAt = time.Now()
	s.save()
	return nil
}

// Delete removes an entry.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[id]; !ok {
		return fmt.Errorf("entry %s not found", id)
	}
	delete(s.entries, id)
	s.rebuildIndex()
	s.save()
	return nil
}

// Search searches the knowledge base.
func (s *Store) Search(query string, limit int) []SearchResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 10
	}

	query = strings.ToLower(query)
	var results []SearchResult

	for _, e := range s.entries {
		score := 0.0
		matchType := ""

		// Title match (highest weight)
		if strings.Contains(strings.ToLower(e.Title), query) {
			score += 3.0
			matchType = "title"
		}

		// Content match
		if strings.Contains(strings.ToLower(e.Content), query) {
			score += 1.0
			if matchType == "" {
				matchType = "content"
			}
		}

		// Tag match
		for _, tag := range e.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				score += 2.0
				if matchType == "" {
					matchType = "tag"
				}
			}
		}

		// Category match
		if strings.Contains(strings.ToLower(e.Category), query) {
			score += 1.5
		}

		if score > 0 {
			// Boost by confidence and recency
			score *= e.Confidence
			results = append(results, SearchResult{
				Entry:     e,
				Score:     score,
				MatchType: matchType,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// List lists all entries.
func (s *Store) List() []*Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*Entry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByType lists entries of a specific type.
func (s *Store) ListByType(entryType EntryType) []*Entry {
	var result []*Entry
	for _, e := range s.List() {
		if e.Type == entryType {
			result = append(result, e)
		}
	}
	return result
}

// ListByTag lists entries with a specific tag.
func (s *Store) ListByTag(tag string) []*Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids, ok := s.index[strings.ToLower(tag)]
	if !ok {
		return nil
	}

	var result []*Entry
	for _, id := range ids {
		if e, ok := s.entries[id]; ok {
			result = append(result, e)
		}
	}
	return result
}

// SetTags sets tags on an entry.
func (s *Store) SetTags(id string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return fmt.Errorf("entry %s not found", id)
	}
	e.Tags = tags
	s.rebuildIndex()
	s.save()
	return nil
}

// SetConfidence sets the confidence score of an entry.
func (s *Store) SetConfidence(id string, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return fmt.Errorf("entry %s not found", id)
	}
	if confidence < 0 || confidence > 1 {
		return fmt.Errorf("confidence must be 0-1")
	}
	e.Confidence = confidence
	e.UpdatedAt = time.Now()
	s.save()
	return nil
}

// Stats returns knowledge base statistics.
func (s *Store) Stats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	byType := make(map[EntryType]int)
	totalAccess := 0
	for _, e := range s.entries {
		byType[e.Type]++
		totalAccess += e.AccessCount
	}

	return map[string]interface{}{
		"total_entries": len(s.entries),
		"by_type":       byType,
		"total_accesses": totalAccess,
		"index_size":    len(s.index),
	}
}

// Deduplicate removes duplicate entries (same content hash).
func (s *Store) Deduplicate() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]string) // hash → ID
	removed := 0

	for id, e := range s.entries {
		if existingID, ok := seen[e.Hash]; ok {
			// Keep the one with higher confidence
			existing := s.entries[existingID]
			if e.Confidence > existing.Confidence {
				delete(s.entries, existingID)
				seen[e.Hash] = id
			} else {
				delete(s.entries, id)
			}
			removed++
		} else {
			seen[e.Hash] = id
		}
	}

	s.rebuildIndex()
	s.save()
	return removed
}

// Export exports the knowledge base as JSON.
func (s *Store) Export() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.MarshalIndent(s.entries, "", "  ")
}

// Import imports entries from JSON data.
func (s *Store) Import(data []byte) (int, error) {
	var entries map[string]*Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	imported := 0
	for id, e := range entries {
		if _, exists := s.entries[id]; !exists {
			s.entries[id] = e
			imported++
		}
	}

	s.rebuildIndex()
	s.save()
	return imported, nil
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

func (s *Store) rebuildIndex() {
	s.index = make(map[string][]string)
	for id, e := range s.entries {
		for _, tag := range e.Tags {
			key := strings.ToLower(tag)
			s.index[key] = append(s.index[key], id)
		}
	}
}

func (s *Store) save() {
	data, _ := json.MarshalIndent(s.entries, "", "  ")
	os.WriteFile(filepath.Join(s.storeDir, "knowledge.json"), data, 0644)
}

func (s *Store) load() {
	data, err := os.ReadFile(filepath.Join(s.storeDir, "knowledge.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.entries)
	s.rebuildIndex()
}
