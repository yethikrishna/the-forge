// Package achievement tracks user milestones and achievements.
// First chat, first pipeline, first orchestration, etc.
// Gamification that actually motivates.
package achievement

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Tier represents achievement rarity.
type Tier string

const (
	TierCommon    Tier = "common"
	TierUncommon  Tier = "uncommon"
	TierRare      Tier = "rare"
	TierEpic      Tier = "epic"
	TierLegendary Tier = "legendary"
)

// Achievement represents a single achievement.
type Achievement struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tier        Tier      `json:"tier"`
	Icon        string    `json:"icon,omitempty"`
	UnlockedAt  time.Time `json:"unlocked_at,omitempty"`
	Unlocked    bool      `json:"unlocked"`
	Progress    float64   `json:"progress"` // 0-1
	Hidden      bool      `json:"hidden"`   // hidden until unlocked
}

// Tracker tracks achievements.
type Tracker struct {
	achievements map[string]*Achievement
	storePath    string
	mu           sync.RWMutex
}

// NewTracker creates a new achievement tracker.
func NewTracker(storePath string) *Tracker {
	t := &Tracker{
		achievements: make(map[string]*Achievement),
		storePath:    storePath,
	}
	t.registerDefaults()
	t.load()
	return t
}

func (t *Tracker) registerDefaults() {
	defaults := []Achievement{
		{ID: "first-chat", Name: "Hello World", Description: "Start your first chat with an agent", Tier: TierCommon, Icon: "💬"},
		{ID: "first-agent", Name: "Agent Creator", Description: "Create your first agent", Tier: TierCommon, Icon: "🤖"},
		{ID: "first-pipeline", Name: "Pipeline Runner", Description: "Run your first pipeline", Tier: TierUncommon, Icon: "🔗"},
		{ID: "first-orchestration", Name: "Orchestrator", Description: "Orchestrate multiple agents", Tier: TierRare, Icon: "🎭"},
		{ID: "first-share", Name: "Sharing is Caring", Description: "Share a session for the first time", Tier: TierCommon, Icon: "📤"},
		{ID: "first-undo", Name: "Time Traveler", Description: "Undo an agent action", Tier: TierCommon, Icon: "⏪"},
		{ID: "first-test", Name: "Test Driven", Description: "Run agent tests", Tier: TierUncommon, Icon: "🧪"},
		{ID: "first-watch", Name: "Watcher", Description: "Watch files for changes", Tier: TierCommon, Icon: "👁️"},
		{ID: "first-quality", Name: "Quality Inspector", Description: "Score agent output quality", Tier: TierUncommon, Icon: "📊"},
		{ID: "first-abtest", Name: "A/B Tester", Description: "Run an A/B test", Tier: TierUncommon, Icon: "🔬"},
		{ID: "first-seed", Name: "Seeder", Description: "Bootstrap a project with forge seed", Tier: TierCommon, Icon: "🌱"},
		{ID: "first-witness", Name: "Witness", Description: "Record a witnessed action", Tier: TierRare, Icon: "🔐"},
		{ID: "first-empath", Name: "Empath", Description: "Analyze user sentiment", Tier: TierUncommon, Icon: "💗"},
		{ID: "cost-conscious", Name: "Cost Conscious", Description: "Review your cost report", Tier: TierCommon, Icon: "💰"},
		{ID: "budget-setter", Name: "Budget Setter", Description: "Set a cost budget", Tier: TierUncommon, Icon: "🎯"},
		{ID: "quickstart-complete", Name: "Trailblazer", Description: "Complete the quickstart guide", Tier: TierUncommon, Icon: "🚀"},
		{ID: "power-user", Name: "Power User", Description: "Use 10 different commands", Tier: TierRare, Icon: "⚡", Hidden: true},
		{ID: "centurion", Name: "Centurion", Description: "Run 100 agent actions", Tier: TierEpic, Icon: "💯", Hidden: true},
		{ID: "night-owl", Name: "Night Owl", Description: "Use Forge after midnight", Tier: TierCommon, Icon: "🦉", Hidden: true},
		{ID: "forge-master", Name: "Forge Master", Description: "Unlock all other achievements", Tier: TierLegendary, Icon: "👑", Hidden: true},
	}

	for _, a := range defaults {
		t.achievements[a.ID] = &Achievement{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			Tier:        a.Tier,
			Icon:        a.Icon,
			Hidden:      a.Hidden,
		}
	}
}

// Unlock unlocks an achievement.
func (t *Tracker) Unlock(id string) (*Achievement, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	a, ok := t.achievements[id]
	if !ok {
		return nil, fmt.Errorf("achievement %q not found", id)
	}

	if a.Unlocked {
		return a, nil // already unlocked
	}

	a.Unlocked = true
	a.UnlockedAt = time.Now()
	a.Progress = 1.0

	t.save()

	// Check if forge-master should unlock
	t.checkForgeMaster()

	return a, nil
}

// SetProgress updates progress on an achievement.
func (t *Tracker) SetProgress(id string, progress float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	a, ok := t.achievements[id]
	if !ok {
		return fmt.Errorf("achievement %q not found", id)
	}

	if progress > 1.0 {
		progress = 1.0
	}
	a.Progress = progress

	if progress >= 1.0 && !a.Unlocked {
		a.Unlocked = true
		a.UnlockedAt = time.Now()
	}

	t.save()
	return nil
}

// Get returns a specific achievement.
func (t *Tracker) Get(id string) (*Achievement, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	a, ok := t.achievements[id]
	if !ok {
		return nil, false
	}
	copy := *a
	return &copy, true
}

// List returns all achievements.
func (t *Tracker) List() []Achievement {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]Achievement, 0, len(t.achievements))
	for _, a := range t.achievements {
		if a.Hidden && !a.Unlocked {
			continue // don't reveal hidden achievements
		}
		result = append(result, *a)
	}

	sort.Slice(result, func(i, j int) bool {
		// Unlocked first, then by tier
		if result[i].Unlocked != result[j].Unlocked {
			return result[i].Unlocked
		}
		return tierOrder(result[i].Tier) < tierOrder(result[j].Tier)
	})

	return result
}

// ListAll returns all achievements including hidden.
func (t *Tracker) ListAll() []Achievement {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]Achievement, 0, len(t.achievements))
	for _, a := range t.achievements {
		result = append(result, *a)
	}
	return result
}

// UnlockedCount returns the number of unlocked achievements.
func (t *Tracker) UnlockedCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, a := range t.achievements {
		if a.Unlocked {
			count++
		}
	}
	return count
}

// TotalCount returns total achievements (including hidden).
func (t *Tracker) TotalCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.achievements)
}

// Stats returns achievement statistics.
func (t *Tracker) Stats() Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := Stats{
		Total:    len(t.achievements),
		Tiers:    make(map[Tier]int),
		Unlocked: make(map[Tier]int),
	}

	for _, a := range t.achievements {
		stats.Tiers[a.Tier]++
		if a.Unlocked {
			stats.UnlockedTotal++
			stats.Unlocked[a.Tier]++
		}
	}

	return stats
}

// Stats holds achievement statistics.
type Stats struct {
	Total         int          `json:"total"`
	UnlockedTotal int          `json:"unlocked_total"`
	Tiers         map[Tier]int `json:"tiers"`
	Unlocked      map[Tier]int `json:"unlocked"`
}

func (t *Tracker) checkForgeMaster() {
	// Count non-hidden, non-forge-master unlocked
	count := 0
	total := 0
	for _, a := range t.achievements {
		if a.ID == "forge-master" {
			continue
		}
		if a.Hidden {
			continue
		}
		total++
		if a.Unlocked {
			count++
		}
	}

	fm, ok := t.achievements["forge-master"]
	if ok && count >= total && total > 0 {
		if !fm.Unlocked {
			fm.Unlocked = true
			fm.UnlockedAt = time.Now()
			fm.Progress = 1.0
		}
	}
}

func (t *Tracker) save() {
	if t.storePath == "" {
		return
	}

	data, err := json.MarshalIndent(t.achievements, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(filepath.Dir(t.storePath), 0755)
	os.WriteFile(t.storePath, data, 0644)
}

func (t *Tracker) load() {
	if t.storePath == "" {
		return
	}

	data, err := os.ReadFile(t.storePath)
	if err != nil {
		return
	}

	var saved map[string]*Achievement
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}

	for id, a := range saved {
		if existing, ok := t.achievements[id]; ok {
			existing.Unlocked = a.Unlocked
			existing.UnlockedAt = a.UnlockedAt
			existing.Progress = a.Progress
		}
	}
}

func tierOrder(t Tier) int {
	switch t {
	case TierCommon:
		return 0
	case TierUncommon:
		return 1
	case TierRare:
		return 2
	case TierEpic:
		return 3
	case TierLegendary:
		return 4
	default:
		return 5
	}
}

// FormatAchievement formats an achievement for display.
func FormatAchievement(a Achievement) string {
	status := "🔒"
	if a.Unlocked {
		status = "✓"
	}

	icon := a.Icon
	if icon == "" {
		icon = "🏆"
	}

	return fmt.Sprintf("%s %s [%s] %s — %s", status, icon, a.Tier, a.Name, a.Description)
}
