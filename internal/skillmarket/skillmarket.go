// Package skillmarket provides a skill marketplace where agents can publish,
// discover, and share reusable skills. Skills are versioned, rated, and
// categorized for easy discovery.
package skillmarket

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

// SkillStatus represents the publication status of a skill.
type SkillStatus string

const (
	StatusDraft      SkillStatus = "draft"
	StatusPublished  SkillStatus = "published"
	StatusDeprecated SkillStatus = "deprecated"
	StatusRemoved    SkillStatus = "removed"
)

// Category represents a skill category.
type Category string

const (
	CatCoding   Category = "coding"
	CatAnalysis Category = "analysis"
	CatReview   Category = "review"
	CatTesting  Category = "testing"
	CatDevOps   Category = "devops"
	CatSecurity Category = "security"
	CatWriting  Category = "writing"
	CatData     Category = "data"
	CatLearning Category = "learning"
	CatCustom   Category = "custom"
)

// Rating represents a skill rating.
type Rating struct {
	UserID    string    `json:"user_id"`
	Score     int       `json:"score"` // 1-5
	Review    string    `json:"review,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Skill represents a marketplace skill.
type Skill struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Author       string            `json:"author"`
	Category     Category          `json:"category"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Status       SkillStatus       `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	PublishedAt  time.Time         `json:"published_at,omitempty"`
	Downloads    int               `json:"downloads"`
	Rating       float64           `json:"rating"` // average 1-5
	RatingCount  int               `json:"rating_count"`
	Tags         []string          `json:"tags"`
	Requirements []string          `json:"requirements,omitempty"`
	License      string            `json:"license,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	SourceURL    string            `json:"source_url,omitempty"`
	Readme       string            `json:"readme,omitempty"`
	Entrypoint   string            `json:"entrypoint"` // skill function name
	Parameters   []Parameter       `json:"parameters,omitempty"`
	Ratings      []Rating          `json:"-"` // not serialized in main file
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Parameter represents a skill parameter.
type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "boolean", "list"
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// Market manages the skill marketplace.
type Market struct {
	mu      sync.RWMutex
	dir     string
	skills  map[string]*Skill
	ratings map[string][]Rating // skill ID -> ratings
}

// NewMarket creates a new skill marketplace.
func NewMarket(dir string) (*Market, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create market dir: %w", err)
	}
	m := &Market{
		dir:     dir,
		skills:  make(map[string]*Skill),
		ratings: make(map[string][]Rating),
	}
	m.load()
	return m, nil
}

func (m *Market) load() {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}
		var s Skill
		if err := json.Unmarshal(data, &s); err == nil {
			m.skills[s.ID] = &s
		}
	}

	// Load ratings
	ratingsDir := filepath.Join(m.dir, "ratings")
	if entries, err := os.ReadDir(ratingsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			skillID := e.Name()[:len(e.Name())-5] // remove .json
			data, err := os.ReadFile(filepath.Join(ratingsDir, e.Name()))
			if err != nil {
				continue
			}
			var ratings []Rating
			if err := json.Unmarshal(data, &ratings); err == nil {
				m.ratings[skillID] = ratings
			}
		}
	}
}

func (m *Market) save(s *Skill) error {
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(m.dir, s.ID+".json"), data, 0644)
}

func (m *Market) saveRatings(skillID string) error {
	ratingsDir := filepath.Join(m.dir, "ratings")
	os.MkdirAll(ratingsDir, 0755)
	data, _ := json.MarshalIndent(m.ratings[skillID], "", "  ")
	return os.WriteFile(filepath.Join(ratingsDir, skillID+".json"), data, 0644)
}

// Publish publishes a new skill to the marketplace.
func (m *Market) Publish(s *Skill) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s.ID == "" {
		s.ID = fmt.Sprintf("skill-%d", time.Now().UnixNano())
	}
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now
	s.Status = StatusPublished
	s.PublishedAt = now

	if s.Version == "" {
		s.Version = "1.0.0"
	}
	if s.Category == "" {
		s.Category = CatCustom
	}

	m.skills[s.ID] = s
	return m.save(s)
}

// Get retrieves a skill.
func (m *Market) Get(id string) (*Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[id]
	return s, ok
}

// GetByName retrieves a skill by name.
func (m *Market) GetByName(name string) (*Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.skills {
		if s.Name == name {
			return s, true
		}
	}
	return nil, false
}

// Search searches skills by query.
func (m *Market) Search(query string) []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	var result []Skill
	for _, s := range m.skills {
		if s.Status != StatusPublished {
			continue
		}
		if strings.Contains(strings.ToLower(s.Name), query) ||
			strings.Contains(strings.ToLower(s.Description), query) ||
			strings.Contains(strings.ToLower(string(s.Category)), query) {
			result = append(result, *s)
			continue
		}
		for _, tag := range s.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				result = append(result, *s)
				break
			}
		}
	}
	return result
}

// ListByCategory lists skills by category.
func (m *Market) ListByCategory(cat Category) []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Skill
	for _, s := range m.skills {
		if s.Category == cat && s.Status == StatusPublished {
			result = append(result, *s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Downloads > result[j].Downloads
	})
	return result
}

// Trending returns the most downloaded skills.
func (m *Market) Trending(limit int) []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Skill
	for _, s := range m.skills {
		if s.Status == StatusPublished {
			result = append(result, *s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Downloads > result[j].Downloads
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// TopRated returns the highest rated skills.
func (m *Market) TopRated(limit int) []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Skill
	for _, s := range m.skills {
		if s.Status == StatusPublished && s.RatingCount > 0 {
			result = append(result, *s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Rating > result[j].Rating
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// Rate adds a rating to a skill.
func (m *Market) Rate(skillID, userID string, score int, review string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.skills[skillID]
	if !ok {
		return fmt.Errorf("skill %s not found", skillID)
	}
	if score < 1 || score > 5 {
		return fmt.Errorf("score must be 1-5")
	}

	rating := Rating{
		UserID:    userID,
		Score:     score,
		Review:    review,
		CreatedAt: time.Now(),
	}

	m.ratings[skillID] = append(m.ratings[skillID], rating)

	// Recalculate average
	var total float64
	for _, r := range m.ratings[skillID] {
		total += float64(r.Score)
	}
	s.RatingCount = len(m.ratings[skillID])
	s.Rating = total / float64(s.RatingCount)
	s.UpdatedAt = time.Now()

	m.save(s)
	m.saveRatings(skillID)
	return nil
}

// Download increments the download count.
func (m *Market) Download(skillID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.skills[skillID]
	if !ok {
		return fmt.Errorf("skill %s not found", skillID)
	}
	s.Downloads++
	s.UpdatedAt = time.Now()
	return m.save(s)
}

// Deprecate marks a skill as deprecated.
func (m *Market) Deprecate(skillID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.skills[skillID]
	if !ok {
		return fmt.Errorf("skill %s not found", skillID)
	}
	s.Status = StatusDeprecated
	s.UpdatedAt = time.Now()
	return m.save(s)
}

// Remove removes a skill from the marketplace.
func (m *Market) Remove(skillID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.skills[skillID]; !ok {
		return fmt.Errorf("skill %s not found", skillID)
	}
	delete(m.skills, skillID)
	os.Remove(filepath.Join(m.dir, skillID+".json"))
	return nil
}

// Categories returns all unique categories.
func (m *Market) Categories() []Category {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[Category]bool)
	for _, s := range m.skills {
		seen[s.Category] = true
	}

	var result []Category
	for c := range seen {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// Stats returns marketplace statistics.
type Stats struct {
	TotalSkills    int              `json:"total_skills"`
	Published      int              `json:"published"`
	Deprecated     int              `json:"deprecated"`
	TotalDownloads int              `json:"total_downloads"`
	AvgRating      float64          `json:"avg_rating"`
	ByCategory     map[Category]int `json:"by_category"`
}

// Stats returns marketplace statistics.
func (m *Market) Stats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		ByCategory: make(map[Category]int),
	}

	var totalRating float64
	var ratedCount int
	for _, s := range m.skills {
		stats.TotalSkills++
		stats.TotalDownloads += s.Downloads
		stats.ByCategory[s.Category]++
		if s.Status == StatusPublished {
			stats.Published++
		}
		if s.Status == StatusDeprecated {
			stats.Deprecated++
		}
		if s.RatingCount > 0 {
			totalRating += s.Rating
			ratedCount++
		}
	}
	if ratedCount > 0 {
		stats.AvgRating = totalRating / float64(ratedCount)
	}

	return stats
}
