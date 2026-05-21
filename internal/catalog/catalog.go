// Package catalog provides a unified agent and tool registry — like Databricks Unity Catalog
// for AI agents. It tracks agent definitions, tools, models, and data sources with
// governance metadata: ownership, lineage, tags, classifications, and access policies.
//
// Know what you have. Govern what you run.
package catalog

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

	"github.com/forge/sword/internal/persistence"
)

// EntryType classifies what kind of catalog entry this is.
type EntryType string

const (
	TypeAgent      EntryType = "agent"       // An AI agent definition
	TypeTool       EntryType = "tool"        // A tool/plugin
	TypeModel      EntryType = "model"       // A model configuration
	TypeDataSource EntryType = "data_source" // A data source (index, corpus, API)
	TypePipeline   EntryType = "pipeline"    // A pipeline/workflow definition
	TypePrompt     EntryType = "prompt"      // A prompt template
	TypeSecret     EntryType = "secret"      // A secret/credential reference (metadata only)
)

// Classification marks the sensitivity level of a catalog entry.
type Classification string

const (
	ClassPublic      Classification = "public"
	ClassInternal    Classification = "internal"
	ClassConfidential Classification = "confidential"
	ClassRestricted  Classification = "restricted"
)

// EntryStatus represents the lifecycle state of an entry.
type EntryStatus string

const (
	StatusActive   EntryStatus = "active"
	StatusDraft    EntryStatus = "draft"
	StatusDisabled EntryStatus = "disabled"
	StatusArchived EntryStatus = "archived"
)

// Entry is a single catalog item.
type Entry struct {
	ID             string            `json:"id"`
	Type           EntryType         `json:"type"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace,omitempty"` // e.g. "default", "team-ml", "org-finance"
	Description    string            `json:"description,omitempty"`
	Version        string            `json:"version,omitempty"`
	Status         EntryStatus       `json:"status"`
	Owner          string            `json:"owner,omitempty"`
	Classification Classification     `json:"classification,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Schema         json.RawMessage   `json:"schema,omitempty"` // Structured schema (input/output types, capabilities)
	Dependencies   []string          `json:"dependencies,omitempty"` // IDs of entries this depends on
	URI            string            `json:"uri,omitempty"` // Location (file path, URL, registry ref)
	Checksum       string            `json:"checksum,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	CreatedBy      string            `json:"created_by,omitempty"`
	UpdatedBy      string            `json:"updated_by,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// AuditLog tracks changes to the catalog.
type AuditLog struct {
	ID        string    `json:"id"`
	EntryID   string    `json:"entry_id"`
	Action    string    `json:"action"` // register, update, deprecate, archive, transfer
	User      string    `json:"user"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details,omitempty"`
	Before    string    `json:"before,omitempty"` // JSON snapshot before
	After     string    `json:"after,omitempty"`  // JSON snapshot after
}

// Stats holds catalog statistics.
type Stats struct {
	TotalEntries       int                `json:"total_entries"`
	EntriesByType      map[EntryType]int  `json:"entries_by_type"`
	EntriesByStatus    map[EntryStatus]int `json:"entries_by_status"`
	EntriesByClass     map[Classification]int `json:"entries_by_class"`
	Namespaces         []string           `json:"namespaces"`
	Tags               []string           `json:"all_tags"`
	AuditLogEntries    int                `json:"audit_log_entries"`
	OldestEntry        time.Time          `json:"oldest_entry"`
	NewestEntry        time.Time          `json:"newest_entry"`
}

// Store manages the catalog.
type Store struct {
	Dir       string
	mu        sync.RWMutex
	entries   map[string]*Entry
	auditLogs []*AuditLog
	pstore    *persistence.Store
}

// NewStore creates or loads a catalog store.
func NewStore(dir string) (*Store, error) {
	s := &Store{
		Dir:     dir,
		entries: make(map[string]*Entry),
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create catalog dir: %w", err)
	}
	if err := s.load(); err != nil {
		return s, nil
	}

	ps, err := persistence.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("catalog: open persistence store: %w", err)
	}
	s.pstore = ps
	ps.Register("entries", func() ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return json.MarshalIndent(s.entries, "", "  ")
	})
	ps.Register("audit", func() ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return json.MarshalIndent(s.auditLogs, "", "  ")
	})
	return s, nil
}

// Close flushes pending writes and shuts down the background syncer.
func (s *Store) Close() error {
	if s.pstore != nil {
		return s.pstore.Close()
	}
	return nil
}

// Flush forces an immediate write of all dirty keys to disk.
func (s *Store) Flush() error {
	if s.pstore != nil {
		return s.pstore.Flush()
	}
	return nil
}

// Register creates a new catalog entry.
func (s *Store) Register(entry Entry) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Name == "" {
		return nil, fmt.Errorf("entry name is required")
	}
	if entry.Type == "" {
		return nil, fmt.Errorf("entry type is required")
	}

	// Generate ID: namespace/name/version
	if entry.ID == "" {
		entry.ID = makeEntryID(entry.Namespace, entry.Name, entry.Version)
	}

	// Check for duplicate.
	if _, exists := s.entries[entry.ID]; exists {
		return nil, fmt.Errorf("entry %s already exists", entry.ID)
	}

	now := time.Now().UTC()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	if entry.Status == "" {
		entry.Status = StatusActive
	}
	if entry.Labels == nil {
		entry.Labels = make(map[string]string)
	}
	if entry.Metadata == nil {
		entry.Metadata = make(map[string]string)
	}

	// Compute checksum of the entry for integrity.
	if entry.Checksum == "" {
		entry.Checksum = computeEntryChecksum(&entry)
	}

	s.entries[entry.ID] = &entry
	s.logAudit(entry.ID, "register", entry.CreatedBy, "registered "+string(entry.Type), "", &entry)
	s.markDirty()
	return &entry, nil
}

// Get retrieves an entry by ID.
func (s *Store) Get(id string) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[id]
	if !ok {
		return nil, fmt.Errorf("entry %s not found", id)
	}
	return e, nil
}

// GetByName retrieves an entry by namespace + name (+ optional version).
func (s *Store) GetByName(namespace, name, version string) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id := makeEntryID(namespace, name, version)
	e, ok := s.entries[id]
	if !ok {
		return nil, fmt.Errorf("entry %s not found", id)
	}
	return e, nil
}

// Update modifies an existing catalog entry.
func (s *Store) Update(id string, updates func(*Entry)) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return nil, fmt.Errorf("entry %s not found", id)
	}

	// Snapshot before.
	before, _ := json.Marshal(e)

	updates(e)
	e.UpdatedAt = time.Now().UTC()
	e.Checksum = computeEntryChecksum(e)

	after, _ := json.Marshal(e)
	s.logAudit(id, "update", e.UpdatedBy, "", string(before), nil)
	_ = after
	s.markDirty()
	return e, nil
}

// Delete removes an entry from the catalog.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return fmt.Errorf("entry %s not found", id)
	}

	s.logAudit(id, "delete", "", "deleted "+e.Name, "", nil)
	delete(s.entries, id)
	s.markDirty()
	return nil
}

// List returns entries matching optional filters.
func (s *Store) List(filters map[string]string) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Entry
	for _, e := range s.entries {
		if matchesEntryFilters(e, filters) {
			results = append(results, e)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})
	return results, nil
}

// Search performs a text search across name, description, and tags.
func (s *Store) Search(query string) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var results []*Entry

	for _, e := range s.entries {
		// Use strings.Contains with strings.ToLower once per field; avoid
		// allocating lowercase strings for every entry by short-circuiting early.
		if containsFold(e.Name, q) || containsFold(e.Description, q) || hasTagMatch(e, q) {
			results = append(results, e)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})
	return results, nil
}

// containsFold reports whether s contains substr, case-insensitively.
// substr must already be lowercase.
func containsFold(s, substrLower string) bool {
	return strings.Contains(strings.ToLower(s), substrLower)
}

// GetDependencies returns all entries that the given entry depends on.
func (s *Store) GetDependencies(entryID string) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[entryID]
	if !ok {
		return nil, fmt.Errorf("entry %s not found", entryID)
	}

	var deps []*Entry
	for _, depID := range e.Dependencies {
		if dep, ok := s.entries[depID]; ok {
			deps = append(deps, dep)
		}
	}
	return deps, nil
}

// GetDependents returns all entries that depend on the given entry.
func (s *Store) GetDependents(entryID string) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var dependents []*Entry
	for _, e := range s.entries {
		for _, dep := range e.Dependencies {
			if dep == entryID {
				dependents = append(dependents, e)
				break
			}
		}
	}
	return dependents, nil
}

// Transfer changes ownership of an entry.
func (s *Store) Transfer(entryID, newOwner, by string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[entryID]
	if !ok {
		return nil, fmt.Errorf("entry %s not found", entryID)
	}

	oldOwner := e.Owner
	e.Owner = newOwner
	e.UpdatedBy = by
	e.UpdatedAt = time.Now().UTC()

	s.logAudit(entryID, "transfer", by, fmt.Sprintf("owner changed from %s to %s", oldOwner, newOwner), "", nil)
	s.markDirty()
	return e, nil
}

// Deprecate marks an entry as deprecated (archived).
func (s *Store) Deprecate(entryID, replacement, by string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[entryID]
	if !ok {
		return nil, fmt.Errorf("entry %s not found", entryID)
	}

	e.Status = StatusArchived
	e.UpdatedBy = by
	e.UpdatedAt = time.Now().UTC()
	if replacement != "" {
		e.Metadata["deprecated_by"] = replacement
	}

	s.logAudit(entryID, "deprecate", by, "deprecated", "", nil)
	s.markDirty()
	return e, nil
}

// GetStats returns catalog statistics.
func (s *Store) GetStats() *Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		TotalEntries:    len(s.entries),
		EntriesByType:   make(map[EntryType]int),
		EntriesByStatus: make(map[EntryStatus]int),
		EntriesByClass:  make(map[Classification]int),
		AuditLogEntries: len(s.auditLogs),
	}

	nsMap := make(map[string]bool)
	tagMap := make(map[string]bool)

	for _, e := range s.entries {
		stats.EntriesByType[e.Type]++
		stats.EntriesByStatus[e.Status]++
		if e.Classification != "" {
			stats.EntriesByClass[e.Classification]++
		}
		if e.Namespace != "" {
			nsMap[e.Namespace] = true
		}
		for _, t := range e.Tags {
			tagMap[t] = true
		}
		if stats.OldestEntry.IsZero() || e.CreatedAt.Before(stats.OldestEntry) {
			stats.OldestEntry = e.CreatedAt
		}
		if e.CreatedAt.After(stats.NewestEntry) {
			stats.NewestEntry = e.CreatedAt
		}
	}

	for ns := range nsMap {
		stats.Namespaces = append(stats.Namespaces, ns)
	}
	for t := range tagMap {
		stats.Tags = append(stats.Tags, t)
	}
	sort.Strings(stats.Namespaces)
	sort.Strings(stats.Tags)

	return stats
}

// GetAuditLog returns the audit log, optionally filtered by entry ID.
func (s *Store) GetAuditLog(entryID string) ([]*AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*AuditLog
	for _, l := range s.auditLogs {
		if entryID == "" || l.EntryID == entryID {
			results = append(results, l)
		}
	}
	return results, nil
}

// ExportJSON exports all entries as JSON.
func (s *Store) ExportJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	export := struct {
		Entries  []*Entry     `json:"entries"`
		Audit    []*AuditLog  `json:"audit_log"`
		Exported time.Time    `json:"exported"`
	}{
		Exported: time.Now().UTC(),
	}
	for _, e := range s.entries {
		export.Entries = append(export.Entries, e)
	}
	export.Audit = s.auditLogs
	return json.MarshalIndent(export, "", "  ")
}

// --- helpers ---

func makeEntryID(namespace, name, version string) string {
	parts := []string{}
	if namespace != "" {
		parts = append(parts, namespace)
	}
	parts = append(parts, name)
	id := strings.Join(parts, "/")
	if version != "" {
		id += "@" + version
	}
	return id
}

func computeEntryChecksum(e *Entry) string {
	h := sha256.New()
	h.Write([]byte(e.ID))
	h.Write([]byte(e.Name))
	h.Write([]byte(e.Type))
	h.Write([]byte(e.Version))
	h.Write(e.Schema)
	if data, err := json.Marshal(e.Metadata); err == nil {
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:24]
}

func (s *Store) logAudit(entryID, action, user, details, before string, after *Entry) {
	var afterStr string
	if after != nil {
		data, _ := json.Marshal(after)
		afterStr = string(data)
	}
	s.auditLogs = append(s.auditLogs, &AuditLog{
		ID:        fmt.Sprintf("audit-%d-%s", time.Now().UnixMilli(), action),
		EntryID:   entryID,
		Action:    action,
		User:      user,
		Timestamp: time.Now().UTC(),
		Details:   details,
		Before:    before,
		After:     afterStr,
	})
}

func matchesEntryFilters(e *Entry, filters map[string]string) bool {
	for k, v := range filters {
		switch k {
		case "type":
			if string(e.Type) != v {
				return false
			}
		case "namespace":
			if e.Namespace != v {
				return false
			}
		case "owner":
			if e.Owner != v {
				return false
			}
		case "status":
			if string(e.Status) != v {
				return false
			}
		case "classification":
			if string(e.Classification) != v {
				return false
			}
		case "tag":
			if !hasTag(e, v) {
				return false
			}
		case "name":
			if !strings.Contains(strings.ToLower(e.Name), strings.ToLower(v)) {
				return false
			}
		}
	}
	return true
}

func hasTag(e *Entry, tag string) bool {
	for _, t := range e.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func hasTagMatch(e *Entry, query string) bool {
	for _, t := range e.Tags {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}

// --- persistence ---

func (s *Store) load() error {
	entriesPath := filepath.Join(s.Dir, "entries.json")
	auditPath := filepath.Join(s.Dir, "audit.json")

	if data, err := os.ReadFile(entriesPath); err == nil {
		if err := json.Unmarshal(data, &s.entries); err != nil {
			return fmt.Errorf("unmarshal entries: %w", err)
		}
	}
	if data, err := os.ReadFile(auditPath); err == nil {
		if err := json.Unmarshal(data, &s.auditLogs); err != nil {
			return fmt.Errorf("unmarshal audit: %w", err)
		}
	}
	return nil
}

// markDirty tells the persistence store that both entries and audit need flushing.
// Must be called with s.mu held (write lock).
func (s *Store) markDirty() {
	if s.pstore != nil {
		s.pstore.Dirty("entries")
		s.pstore.Dirty("audit")
	}
}
