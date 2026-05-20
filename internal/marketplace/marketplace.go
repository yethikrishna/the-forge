// Package marketplace provides an agent marketplace for Forge.
// Browse, install, and publish agent configurations, prompts, and skills.
// Community-driven, versioned, and signed.
//
// Stand on the shoulders of giants. Share what you build.
package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EntryType represents the type of marketplace entry.
type EntryType string

const (
	EntryAgent  EntryType = "agent"
	EntryPrompt EntryType = "prompt"
	EntrySkill  EntryType = "skill"
	EntryPlugin EntryType = "plugin"
)

// Entry represents a marketplace entry.
type Entry struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        EntryType `json:"type"`
	Author      string    `json:"author"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags,omitempty"`
	Downloads   int       `json:"downloads"`
	Rating      float64   `json:"rating"` // 0-5
	Ratings     int       `json:"ratings"`
	Signature   string    `json:"signature,omitempty"`
	SourceURL   string    `json:"source_url,omitempty"`
	Config      string    `json:"config,omitempty"` // JSON config for the entry
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store manages marketplace entries.
type Store struct {
	Dir string
}

// NewStore creates a marketplace store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Publish publishes an entry to the marketplace.
func (s *Store) Publish(e Entry) (*Entry, error) {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return nil, err
	}
	if e.ID == "" {
		e.ID = fmt.Sprintf("%s-%s-%d", e.Type, sanitizeName(e.Name), time.Now().UnixNano())
	}
	if e.Version == "" {
		e.Version = "0.1.0"
	}
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	if err := s.writeEntry(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Get retrieves an entry.
func (s *Store) Get(id string) (*Entry, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("entry %q not found", id)
		}
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Search searches entries by name, type, or tags.
func (s *Store) Search(query string, entryType EntryType) ([]*Entry, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var results []*Entry
	for _, e := range all {
		if entryType != "" && e.Type != entryType {
			continue
		}
		if q != "" {
			match := strings.Contains(strings.ToLower(e.Name), q) ||
				strings.Contains(strings.ToLower(e.Description), q) ||
				containsTag(e.Tags, q)
			if !match {
				continue
			}
		}
		results = append(results, e)
	}
	return results, nil
}

// List returns all entries sorted by downloads.
func (s *Store) List() ([]*Entry, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		entry, err := s.Get(id)
		if err != nil {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Downloads > out[j].Downloads })
	return out, nil
}

// Install increments download count.
func (s *Store) Install(id string) (*Entry, error) {
	e, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	e.Downloads++
	e.UpdatedAt = time.Now()
	return e, s.writeEntry(e)
}

// Rate adds a rating to an entry.
func (s *Store) Rate(id string, score float64) (*Entry, error) {
	e, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if score < 0 || score > 5 {
		return nil, fmt.Errorf("rating must be 0-5, got %.1f", score)
	}
	// Running average
	total := e.Rating * float64(e.Ratings)
	e.Ratings++
	e.Rating = (total + score) / float64(e.Ratings)
	e.UpdatedAt = time.Now()
	return e, s.writeEntry(e)
}

// Unpublish removes an entry.
func (s *Store) Unpublish(id string) error {
	return os.Remove(filepath.Join(s.Dir, id+".json"))
}

// FormatEntry renders an entry for display.
func FormatEntry(e *Entry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: %s (%s)\n", e.Type, e.Name, e.ID))
	sb.WriteString(fmt.Sprintf("  Author:  %s\n", e.Author))
	sb.WriteString(fmt.Sprintf("  Version: %s\n", e.Version))
	sb.WriteString(fmt.Sprintf("  Desc:    %s\n", e.Description))
	if len(e.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("  Tags:    %v\n", e.Tags))
	}
	sb.WriteString(fmt.Sprintf("  Downloads: %d\n", e.Downloads))
	if e.Ratings > 0 {
		sb.WriteString(fmt.Sprintf("  Rating:  %.1f/5 (%d ratings)\n", e.Rating, e.Ratings))
	}
	return sb.String()
}

// FormatSearchResults renders search results.
func FormatSearchResults(entries []*Entry) string {
	if len(entries) == 0 {
		return "No results found."
	}
	var sb strings.Builder
	for _, e := range entries {
		rating := ""
		if e.Ratings > 0 {
			rating = fmt.Sprintf(" %.1f★", e.Rating)
		}
		sb.WriteString(fmt.Sprintf("  %-30s %-8s %s by %s (%d downloads%s)\n",
			e.Name, e.Type, e.Version, e.Author, e.Downloads, rating))
	}
	return sb.String()
}

func containsTag(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func sanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, strings.ToLower(name))
}

func (s *Store) writeEntry(e *Entry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, e.ID+".json"), data, 0644)
}
