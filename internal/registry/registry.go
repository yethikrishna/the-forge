// Package registry provides a plugin and agent registry — discover,
// install, publish, and rate Forge plugins and agent templates.
// Think npm but for AI agents.
package registry

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

// EntryType defines the type of registry entry.
type EntryType string

const (
	TypePlugin   EntryType = "plugin"
	TypeAgent    EntryType = "agent"
	TypeTemplate EntryType = "template"
	TypeWorkflow EntryType = "workflow"
)

// EntryStatus defines the publication status.
type EntryStatus string

const (
	StatusDraft      EntryStatus = "draft"
	StatusPublished  EntryStatus = "published"
	StatusDeprecated EntryStatus = "deprecated"
	StatusRemoved    EntryStatus = "removed"
)

// Entry represents a registry entry (plugin, agent, template, or workflow).
type Entry struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        EntryType   `json:"type"`
	Version     string      `json:"version"`
	Author      string      `json:"author"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Tags        []string    `json:"tags,omitempty"`
	Status      EntryStatus `json:"status"`
	Homepage    string      `json:"homepage,omitempty"`
	Repository  string      `json:"repository,omitempty"`
	License     string      `json:"license,omitempty"`

	// Stats
	Downloads   int       `json:"downloads"`
	Rating      float64   `json:"rating"`      // 0-5 stars
	RatingCount int       `json:"rating_count"`
	Installs    int       `json:"installs"`

	// Content
	Config      string    `json:"config,omitempty"`      // JSON config
	Readme      string    `json:"readme,omitempty"`      // README content
	Entrypoint  string    `json:"entrypoint,omitempty"`  // Main file/function

	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// Rating represents a user rating.
type Rating struct {
	EntryID   string    `json:"entry_id"`
	UserID    string    `json:"user_id"`
	Score     int       `json:"score"` // 1-5
	Review    string    `json:"review,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Registry manages entries and ratings.
type Registry struct {
	storeDir string
	entries  map[string]*Entry
	ratings  map[string][]Rating // entryID → ratings
	mu       sync.Mutex
}

// NewRegistry creates a new registry.
func NewRegistry(storeDir string) *Registry {
	os.MkdirAll(storeDir, 0755)
	r := &Registry{
		storeDir: storeDir,
		entries:  make(map[string]*Entry),
		ratings:  make(map[string][]Rating),
	}
	r.load()
	return r
}

// Publish adds or updates a registry entry.
func (r *Registry) Publish(name string, entryType EntryType, author, description, version string) *Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := makeID(name, author)
	now := time.Now()

	entry, ok := r.entries[id]
	if ok {
		entry.Version = version
		entry.Description = description
		entry.UpdatedAt = now
		r.save()
		return entry
	}

	entry = &Entry{
		ID:          id,
		Name:        name,
		Type:        entryType,
		Version:     version,
		Author:      author,
		Description: description,
		Status:      StatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	r.entries[id] = entry
	r.save()
	return entry
}

// PublishEntry publishes a draft entry.
func (r *Registry) PublishEntry(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("entry %s not found", id)
	}

	if entry.Status != StatusDraft {
		return fmt.Errorf("can only publish drafts")
	}

	now := time.Now()
	entry.Status = StatusPublished
	entry.PublishedAt = &now
	entry.UpdatedAt = now
	r.save()
	return nil
}

// Deprecate marks an entry as deprecated.
func (r *Registry) Deprecate(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("entry %s not found", id)
	}
	entry.Status = StatusDeprecated
	entry.UpdatedAt = time.Now()
	r.save()
	return nil
}

// Remove removes an entry from the registry.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.entries[id]; !ok {
		return fmt.Errorf("entry %s not found", id)
	}
	delete(r.entries, id)
	delete(r.ratings, id)
	r.save()
	return nil
}

// Get retrieves an entry by ID.
func (r *Registry) Get(id string) (*Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	return e, ok
}

// Search searches entries by name, description, tags, or category.
func (r *Registry) Search(query string) []*Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	query = strings.ToLower(query)
	var results []*Entry

	for _, e := range r.entries {
		if e.Status == StatusRemoved {
			continue
		}
		if matchEntry(e, query) {
			results = append(results, e)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Downloads > results[j].Downloads
	})
	return results
}

// List lists entries with optional type filter.
func (r *Registry) List(entryType EntryType) []*Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	var results []*Entry
	for _, e := range r.entries {
		if e.Status == StatusRemoved {
			continue
		}
		if entryType == "" || e.Type == entryType {
			results = append(results, e)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Downloads > results[j].Downloads
	})
	return results
}

// Rate adds a rating to an entry.
func (r *Registry) Rate(entryID, userID string, score int, review string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.entries[entryID]; !ok {
		return fmt.Errorf("entry %s not found", entryID)
	}

	if score < 1 || score > 5 {
		return fmt.Errorf("score must be 1-5")
	}

	rating := Rating{
		EntryID:   entryID,
		UserID:    userID,
		Score:     score,
		Review:    review,
		CreatedAt: time.Now(),
	}

	r.ratings[entryID] = append(r.ratings[entryID], rating)

	// Recalculate average
	entry := r.entries[entryID]
	total := 0
	for _, rt := range r.ratings[entryID] {
		total += rt.Score
	}
	entry.Rating = float64(total) / float64(len(r.ratings[entryID]))
	entry.RatingCount = len(r.ratings[entryID])
	entry.UpdatedAt = time.Now()
	r.save()

	return nil
}

// GetRatings gets ratings for an entry.
func (r *Registry) GetRatings(entryID string) []Rating {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ratings[entryID]
}

// RecordDownload increments the download count.
func (r *Registry) RecordDownload(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e, ok := r.entries[id]; ok {
		e.Downloads++
		r.save()
	}
}

// RecordInstall increments the install count.
func (r *Registry) RecordInstall(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e, ok := r.entries[id]; ok {
		e.Installs++
		r.save()
	}
}

// SetTags sets tags on an entry.
func (r *Registry) SetTags(id string, tags []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("entry %s not found", id)
	}
	e.Tags = tags
	e.UpdatedAt = time.Now()
	r.save()
	return nil
}

// Stats returns registry statistics.
func (r *Registry) Stats() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	byType := make(map[EntryType]int)
	byStatus := make(map[EntryStatus]int)
	totalDownloads := 0

	for _, e := range r.entries {
		byType[e.Type]++
		byStatus[e.Status]++
		totalDownloads += e.Downloads
	}

	return map[string]interface{}{
		"total_entries":    len(r.entries),
		"total_downloads":  totalDownloads,
		"by_type":          byType,
		"by_status":        byStatus,
		"total_ratings":    len(r.ratings),
	}
}

// EntryReport generates a human-readable entry report.
func EntryReport(e *Entry) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s: %s (%s)\n", e.Type, e.Name, e.ID))
	b.WriteString(fmt.Sprintf("  Author: %s | Version: %s | License: %s\n", e.Author, e.Version, e.License))
	b.WriteString(fmt.Sprintf("  %s\n", e.Description))
	b.WriteString(fmt.Sprintf("  Downloads: %d | Installs: %d | Rating: %.1f/5 (%d reviews)\n",
		e.Downloads, e.Installs, e.Rating, e.RatingCount))

	if len(e.Tags) > 0 {
		b.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(e.Tags, ", ")))
	}

	b.WriteString(fmt.Sprintf("  Status: %s | Category: %s\n", e.Status, e.Category))

	return b.String()
}

func matchEntry(e *Entry, query string) bool {
	if strings.Contains(strings.ToLower(e.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), query) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Category), query) {
		return true
	}
	for _, tag := range e.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(e.Author), query) {
		return true
	}
	return false
}

func makeID(name, author string) string {
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return fmt.Sprintf("@%s/%s", strings.ToLower(author), slug)
}

func (r *Registry) save() {
	data, _ := json.MarshalIndent(map[string]interface{}{
		"entries": r.entries,
		"ratings": r.ratings,
	}, "", "  ")
	os.WriteFile(filepath.Join(r.storeDir, "registry.json"), data, 0644)
}

func (r *Registry) load() {
	data, err := os.ReadFile(filepath.Join(r.storeDir, "registry.json"))
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if eData, ok := raw["entries"]; ok {
		json.Unmarshal(eData, &r.entries)
	}
	if rData, ok := raw["ratings"]; ok {
		json.Unmarshal(rData, &r.ratings)
	}
}
