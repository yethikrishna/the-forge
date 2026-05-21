// Package memory provides persistent agent memory with semantic search.
// A forge remembers every blade it has tempered.
package memory

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

// Memory represents a single memory entry.
type Memory struct {
	ID        string            `json:"id"`
	Agent     string            `json:"agent"`
	Session   string            `json:"session"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	AccessAt  time.Time         `json:"access_at"`
	Score     float64           `json:"-"` // search relevance score, not persisted
}

// Store is the memory store for an agent.
type Store struct {
	mu       sync.RWMutex
	memories map[string]*Memory
	path     string // persistence path
}

// NewStore creates a new memory store.
func NewStore(path string) *Store {
	s := &Store{
		memories: make(map[string]*Memory),
		path:     path,
	}
	s.load()
	return s
}

// Store stores a new memory.
func (s *Store) Store(agent, session, content string, tags []string, metadata map[string]string) *Memory {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("mem-%d", time.Now().UnixNano())
	now := time.Now().UTC()

	m := &Memory{
		ID:        id,
		Agent:     agent,
		Session:   session,
		Content:   content,
		Tags:      tags,
		Metadata:  metadata,
		CreatedAt: now,
		AccessAt:  now,
	}

	s.memories[id] = m
	s.save()

	return m
}

// Get retrieves a memory by ID.
func (s *Store) Get(id string) (*Memory, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.memories[id]
	if ok {
		m.AccessAt = time.Now().UTC()
	}
	return m, ok
}

// Delete removes a memory by ID.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.memories[id]; !ok {
		return false
	}
	delete(s.memories, id)
	s.save()
	return true
}

// Search searches memories by content and tags.
// Returns results sorted by relevance (keyword matching score).
func (s *Store) Search(query string, limit int) []*Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	terms := strings.Fields(strings.ToLower(query))
	var results []*Memory

	for _, m := range s.memories {
		score := 0.0
		lower := strings.ToLower(m.Content)

		for _, term := range terms {
			// Content match
			count := strings.Count(lower, term)
			if count > 0 {
				score += float64(count) * 2.0
			}

			// Tag match (higher weight)
			for _, tag := range m.Tags {
				if strings.EqualFold(tag, term) {
					score += 5.0
				} else if strings.Contains(strings.ToLower(tag), term) {
					score += 2.0
				}
			}

			// Agent match
			if strings.EqualFold(m.Agent, term) {
				score += 3.0
			}
		}

		if score > 0 {
			m.Score = score
			results = append(results, m)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// ListByAgent returns all memories for an agent.
func (s *Store) ListByAgent(agent string) []*Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Memory
	for _, m := range s.memories {
		if m.Agent == agent {
			results = append(results, m)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results
}

// ListBySession returns all memories for a session.
func (s *Store) ListBySession(session string) []*Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Memory
	for _, m := range s.memories {
		if m.Session == session {
			results = append(results, m)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results
}

// ListByTag returns all memories with a specific tag.
func (s *Store) ListByTag(tag string) []*Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Memory
	for _, m := range s.memories {
		for _, t := range m.Tags {
			if strings.EqualFold(t, tag) {
				results = append(results, m)
				break
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results
}

// ListRecent returns the N most recent memories.
func (s *Store) ListRecent(limit int) []*Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*Memory, 0, len(s.memories))
	for _, m := range s.memories {
		all = append(all, m)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	return all
}

// Count returns the total number of stored memories.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.memories)
}

// Agents returns a list of agents with stored memories.
func (s *Store) Agents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := map[string]bool{}
	for _, m := range s.memories {
		seen[m.Agent] = true
	}

	var agents []string
	for a := range seen {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	return agents
}

// Tags returns all unique tags.
func (s *Store) Tags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := map[string]bool{}
	for _, m := range s.memories {
		for _, t := range m.Tags {
			seen[t] = true
		}
	}

	var tags []string
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

// Export exports all memories as JSON.
func (s *Store) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*Memory
	for _, m := range s.memories {
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})

	return json.MarshalIndent(list, "", "  ")
}

// Import imports memories from JSON data.
func (s *Store) Import(data []byte) error {
	var list []*Memory
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("memory: import: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range list {
		if m.ID == "" {
			m.ID = fmt.Sprintf("mem-%d", time.Now().UnixNano())
		}
		s.memories[m.ID] = m
	}
	s.save()

	return nil
}

// load reads memories from disk.
func (s *Store) load() {
	if s.path == "" {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}

	var list []*Memory
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}

	for _, m := range list {
		s.memories[m.ID] = m
	}
}

// save writes memories to disk.
func (s *Store) save() {
	if s.path == "" {
		return
	}

	var list []*Memory
	for _, m := range s.memories {
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}

	dir := filepath.Dir(s.path)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(s.path, data, 0o644)
}
