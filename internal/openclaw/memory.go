// Package openclaw provides memory storage backed by OpenClaw memory files.
// Forge agents use OpenClaw's memory system for persistent knowledge —
// working memory, project memory, organizational memory, and skill memory.
//
// Memory isn't a context window. It's a knowledge base that compounds.
package openclaw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MemoryType classifies the type of memory.
type MemoryType string

const (
	MemoryWorking      MemoryType = "working"      // current session
	MemoryProject      MemoryType = "project"      // per-project context
	MemoryOrg          MemoryType = "org"           // organizational knowledge
	MemorySkill        MemoryType = "skill"         // accumulated expertise
	MemoryDaily        MemoryType = "daily"         // daily logs
	MemoryInstitutional MemoryType = "institutional" // lessons learned
)

// MemoryEntry represents a piece of stored knowledge.
type MemoryEntry struct {
	ID        string            `json:"id"`
	Type      MemoryType        `json:"type"`
	AgentID   string            `json:"agent_id"`
	Division  string            `json:"division"`
	Key       string            `json:"key"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata"`
}

// MemorySearchResult is a result from memory search.
type MemorySearchResult struct {
	Entry     MemoryEntry `json:"entry"`
	Score     float64     `json:"score"`
	Highlight string      `json:"highlight,omitempty"`
}

// MemoryManager provides memory operations backed by OpenClaw memory files.
type MemoryManager struct {
	bridge *Bridge
	mu     sync.RWMutex
	cache  map[string]*MemoryEntry // key -> entry
	dir    string                   // workspace memory directory
}

// NewMemoryManager creates a new memory manager.
func NewMemoryManager(bridge *Bridge) *MemoryManager {
	dir := filepath.Join(bridge.WorkspaceDir(), "memory")
	return &MemoryManager{
		bridge: bridge,
		cache:  make(map[string]*MemoryEntry),
		dir:    dir,
	}
}

// Store writes a memory entry. If an entry with the same key exists, it's updated.
func (mm *MemoryManager) Store(ctx context.Context, entry MemoryEntry) error {
	if entry.Key == "" {
		return fmt.Errorf("memory key is required")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.UpdatedAt = time.Now()

	// Store via gateway API
	payload := map[string]interface{}{
		"type":      entry.Type,
		"agentId":   entry.AgentID,
		"division":  entry.Division,
		"key":       entry.Key,
		"content":   entry.Content,
		"tags":      entry.Tags,
		"metadata":  entry.Metadata,
	}
	if err := mm.bridge.PostJSON(ctx, "/api/memory", payload, nil); err != nil {
		// Fall back to local file storage
		return mm.storeLocal(entry)
	}

	mm.mu.Lock()
	mm.cache[entry.Key] = &entry
	mm.mu.Unlock()
	return nil
}

// Retrieve gets a memory entry by key.
func (mm *MemoryManager) Retrieve(ctx context.Context, key string) (*MemoryEntry, error) {
	mm.mu.RLock()
	if e, ok := mm.cache[key]; ok {
		mm.mu.RUnlock()
		return e, nil
	}
	mm.mu.RUnlock()

	var entry MemoryEntry
	if err := mm.bridge.GetJSON(ctx, "/api/memory/"+key, &entry); err != nil {
		// Try local file
		return mm.retrieveLocal(key)
	}
	mm.mu.Lock()
	mm.cache[key] = &entry
	mm.mu.Unlock()
	return &entry, nil
}

// Search searches memory entries by query.
func (mm *MemoryManager) Search(ctx context.Context, query string, limit int) ([]*MemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	path := fmt.Sprintf("/api/memory/search?q=%s&limit=%d", query, limit)
	var results []*MemorySearchResult
	if err := mm.bridge.GetJSON(ctx, path, &results); err != nil {
		// Fall back to local search
		return mm.searchLocal(query, limit)
	}
	return results, nil
}

// SearchByType searches memory filtered by type.
func (mm *MemoryManager) SearchByType(ctx context.Context, query string, memType MemoryType, limit int) ([]*MemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	path := fmt.Sprintf("/api/memory/search?q=%s&type=%s&limit=%d", query, memType, limit)
	var results []*MemorySearchResult
	if err := mm.bridge.GetJSON(ctx, path, &results); err != nil {
		return nil, fmt.Errorf("search memory by type: %w", err)
	}
	return results, nil
}

// Delete removes a memory entry.
func (mm *MemoryManager) Delete(ctx context.Context, key string) error {
	if err := mm.bridge.Delete(ctx, "/api/memory/"+key); err != nil {
		// Try local delete
		return mm.deleteLocal(key)
	}
	mm.mu.Lock()
	delete(mm.cache, key)
	mm.mu.Unlock()
	return nil
}

// ListByDivision returns all memory entries for a division.
func (mm *MemoryManager) ListByDivision(ctx context.Context, division string) ([]*MemoryEntry, error) {
	path := fmt.Sprintf("/api/memory?division=%s", division)
	var entries []*MemoryEntry
	if err := mm.bridge.GetJSON(ctx, path, &entries); err != nil {
		return nil, fmt.Errorf("list memory for division %s: %w", division, err)
	}
	return entries, nil
}

// AppendToDaily appends content to the daily memory file.
func (mm *MemoryManager) AppendToDaily(date string, content string) error {
	filename := fmt.Sprintf("%s.md", date) // e.g., "2026-05-21.md"
	path := filepath.Join(mm.dir, filename)
	os.MkdirAll(mm.dir, 0755)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open daily memory %s: %w", filename, err)
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05")
	_, err = fmt.Fprintf(f, "\n## %s\n%s\n", timestamp, content)
	return err
}

// ReadDaily reads the daily memory file for a given date.
func (mm *MemoryManager) ReadDaily(date string) (string, error) {
	filename := fmt.Sprintf("%s.md", date)
	path := filepath.Join(mm.dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read daily memory %s: %w", filename, err)
	}
	return string(data), nil
}

// storeLocal saves a memory entry to local files as fallback.
func (mm *MemoryManager) storeLocal(entry MemoryEntry) error {
	os.MkdirAll(mm.dir, 0755)
	// Use key as filename with sanitization
	safeKey := strings.ReplaceAll(entry.Key, "/", "_")
	safeKey = strings.ReplaceAll(safeKey, " ", "-")
	path := filepath.Join(mm.dir, safeKey+".json")

	data := fmt.Sprintf("# Memory: %s\n# Type: %s\n# Agent: %s\n# Division: %s\n# Updated: %s\n\n%s\n",
		entry.Key, entry.Type, entry.AgentID, entry.Division,
		entry.UpdatedAt.Format(time.RFC3339), entry.Content)
	return os.WriteFile(path, []byte(data), 0644)
}

// retrieveLocal reads a memory entry from local files.
func (mm *MemoryManager) retrieveLocal(key string) (*MemoryEntry, error) {
	safeKey := strings.ReplaceAll(key, "/", "_")
	safeKey = strings.ReplaceAll(safeKey, " ", "-")
	path := filepath.Join(mm.dir, safeKey+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("memory %s not found: %w", key, err)
	}
	return &MemoryEntry{
		Key:     key,
		Content: string(data),
	}, nil
}

// searchLocal does a simple text search across local memory files.
func (mm *MemoryManager) searchLocal(query string, limit int) ([]*MemorySearchResult, error) {
	entries, err := os.ReadDir(mm.dir)
	if err != nil {
		return nil, nil
	}
	var results []*MemorySearchResult
	lowerQuery := strings.ToLower(query)
	for _, e := range entries {
		if e.IsDir() || len(results) >= limit {
			break
		}
		data, err := os.ReadFile(filepath.Join(mm.dir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(strings.ToLower(content), lowerQuery) {
			results = append(results, &MemorySearchResult{
				Entry: MemoryEntry{
					Key:     strings.TrimSuffix(e.Name(), ".json"),
					Content: content,
				},
				Score: 1.0,
			})
		}
	}
	return results, nil
}

// deleteLocal removes a local memory file.
func (mm *MemoryManager) deleteLocal(key string) error {
	safeKey := strings.ReplaceAll(key, "/", "_")
	safeKey = strings.ReplaceAll(safeKey, " ", "-")
	path := filepath.Join(mm.dir, safeKey+".json")
	return os.Remove(path)
}
