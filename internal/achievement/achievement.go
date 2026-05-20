// Package achievement tracks user milestones and gamification.
// Celebrate progress. Motivate exploration.
package achievement

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

// Rarity represents achievement rarity.
type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityUncommon  Rarity = "uncommon"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

// Category represents an achievement category.
type Category string

const (
	CategoryOnboarding Category = "onboarding"
	CategoryAgent      Category = "agent"
	CategoryPipeline   Category = "pipeline"
	CategoryOrchestration Category = "orchestration"
	CategoryExploration Category = "exploration"
	CategoryMastery    Category = "mastery"
	CategorySocial     Category = "social"
)

// Achievement represents an achievement definition.
type Achievement struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Rarity      Rarity   `json:"rarity"`
	Icon        string   `json:"icon,omitempty"` // emoji
	Points      int      `json:"points"`
	Hidden      bool     `json:"hidden"` // hidden until unlocked
	Prerequisite string  `json:"prerequisite,omitempty"` // ID of required achievement
}

// Unlock represents an unlocked achievement.
type Unlock struct {
	AchievementID string    `json:"achievement_id"`
	UnlockedAt    time.Time `json:"unlocked_at"`
	AgentID       string    `json:"agent_id,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
}

// Profile represents a user's achievement profile.
type Profile struct {
	Unlocks     []Unlock `json:"unlocks"`
	TotalPoints int      `json:"total_points"`
	Level       int      `json:"level"`
	Title       string   `json:"title"`
}

// Tracker tracks achievements.
type Tracker struct {
	mu           sync.Mutex
	dir          string
	definitions  map[string]*Achievement
	unlocks      []Unlock
	eventCounts  map[string]int // event -> count
}

// NewTracker creates an achievement tracker.
func NewTracker(dir string) (*Tracker, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	t := &Tracker{
		dir:         dir,
		definitions: DefaultAchievements(),
		unlocks:     make([]Unlock, 0),
		eventCounts: make(map[string]int),
	}
	t.load()
	return t, nil
}

// DefaultAchievements returns built-in achievement definitions.
func DefaultAchievements() map[string]*Achievement {
	return map[string]*Achievement{
		"first_chat": {ID: "first_chat", Name: "First Words", Description: "Send your first chat to an agent", Category: CategoryOnboarding, Rarity: RarityCommon, Icon: "💬", Points: 10},
		"first_agent": {ID: "first_agent", Name: "Summoner", Description: "Create your first agent", Category: CategoryOnboarding, Rarity: RarityCommon, Icon: "🤖", Points: 15},
		"first_pipeline": {ID: "first_pipeline", Name: "Plumber", Description: "Create your first pipeline", Category: CategoryPipeline, Rarity: RarityUncommon, Icon: "🔧", Points: 25},
		"first_orchestration": {ID: "first_orchestration", Name: "Conductor", Description: "Run your first orchestration", Category: CategoryOrchestration, Rarity: RarityRare, Icon: "🎼", Points: 50},
		"ten_chats": {ID: "ten_chats", Name: "Conversationalist", Description: "Have 10 agent conversations", Category: CategoryAgent, Rarity: RarityCommon, Icon: "🗣️", Points: 20},
		"hundred_chats": {ID: "hundred_chats", Name: "Chatterbox", Description: "Have 100 agent conversations", Category: CategoryAgent, Rarity: RarityUncommon, Icon: "📢", Points: 50, Prerequisite: "ten_chats"},
		"five_agents": {ID: "five_agents", Name: "Agent Army", Description: "Create 5 agents", Category: CategoryAgent, Rarity: RarityUncommon, Icon: "⚔️", Points: 30, Prerequisite: "first_agent"},
		"first_debate": {ID: "first_debate", Name: "Devil's Advocate", Description: "Run your first debate", Category: CategoryOrchestration, Rarity: RarityRare, Icon: "⚖️", Points: 40},
		"first_breed": {ID: "first_breed", Name: "Evolution", Description: "Breed an agent for the first time", Category: CategoryMastery, Rarity: RarityEpic, Icon: "🧬", Points: 75},
		"first_dream": {ID: "first_dream", Name: "Dreamweaver", Description: "Run the dream command", Category: CategoryMastery, Rarity: RarityEpic, Icon: "💭", Points: 75},
		"cost_saver": {ID: "cost_saver", Name: "Penny Pincher", Description: "Save $10 on LLM costs", Category: CategoryMastery, Rarity: RarityRare, Icon: "💰", Points: 50},
		"mcp_master": {ID: "mcp_master", Name: "Protocol Master", Description: "Use MCP, A2A, and ACP protocols", Category: CategoryExploration, Rarity: RarityEpic, Icon: "🔌", Points: 60},
		"witness_prover": {ID: "witness_prover", Name: "Notary", Description: "Verify a witnessed action", Category: CategoryExploration, Rarity: RarityUncommon, Icon: "📜", Points: 25},
		"integration_hacker": {ID: "integration_hacker", Name: "Connected", Description: "Connect 3 integrations", Category: CategoryExploration, Rarity: RarityUncommon, Icon: "🔗", Points: 30},
		"quick_start": {ID: "quick_start", Name: "Quick Draw", Description: "Complete quickstart in under 5 minutes", Category: CategoryOnboarding, Rarity: RarityRare, Icon: "⚡", Points: 40},
		"pipeline_master": {ID: "pipeline_master", Name: "Flow State", Description: "Create 10 pipelines", Category: CategoryPipeline, Rarity: RarityRare, Icon: "🌊", Points: 50, Prerequisite: "first_pipeline"},
		"forge_master": {ID: "forge_master", Name: "Master Smith", Description: "Unlock all other achievements", Category: CategoryMastery, Rarity: RarityLegendary, Icon: "⚒️", Points: 200, Prerequisite: "pipeline_master"},
	}
}

// TrackEvent records an event and checks for unlocks.
func (t *Tracker) TrackEvent(event string, agentID, sessionID string) []*Achievement {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.eventCounts[event]++
	var newUnlocks []*Achievement

	for id, ach := range t.definitions {
		if t.isUnlocked(id) {
			continue
		}
		if t.checkCondition(ach, event) {
			if t.checkPrerequisite(ach) {
				unlock := Unlock{
					AchievementID: id,
					UnlockedAt:    time.Now(),
					AgentID:       agentID,
					SessionID:     sessionID,
				}
				t.unlocks = append(t.unlocks, unlock)
				newUnlocks = append(newUnlocks, ach)
			}
		}
	}

	if len(newUnlocks) > 0 {
		t.save()
	}

	return newUnlocks
}

func (t *Tracker) checkCondition(ach *Achievement, event string) bool {
	switch ach.ID {
	case "first_chat":
		return event == "chat" && t.eventCounts["chat"] >= 1
	case "first_agent":
		return event == "agent_create" && t.eventCounts["agent_create"] >= 1
	case "first_pipeline":
		return event == "pipeline_create" && t.eventCounts["pipeline_create"] >= 1
	case "first_orchestration":
		return event == "orchestration_run" && t.eventCounts["orchestration_run"] >= 1
	case "ten_chats":
		return t.eventCounts["chat"] >= 10
	case "hundred_chats":
		return t.eventCounts["chat"] >= 100
	case "five_agents":
		return t.eventCounts["agent_create"] >= 5
	case "first_debate":
		return event == "debate_run" && t.eventCounts["debate_run"] >= 1
	case "first_breed":
		return event == "breed_run" && t.eventCounts["breed_run"] >= 1
	case "first_dream":
		return event == "dream_run" && t.eventCounts["dream_run"] >= 1
	case "witness_prover":
		return event == "witness_verify" && t.eventCounts["witness_verify"] >= 1
	default:
		return false
	}
}

func (t *Tracker) checkPrerequisite(ach *Achievement) bool {
	if ach.Prerequisite == "" {
		return true
	}
	return t.isUnlocked(ach.Prerequisite)
}

func (t *Tracker) isUnlocked(id string) bool {
	for _, u := range t.unlocks {
		if u.AchievementID == id {
			return true
		}
	}
	return false
}

// IsUnlocked checks if an achievement is unlocked (exported).
func (t *Tracker) IsUnlocked(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.isUnlocked(id)
}

// GetProfile returns the user's achievement profile.
func (t *Tracker) GetProfile() *Profile {
	t.mu.Lock()
	defer t.mu.Unlock()

	p := &Profile{Unlocks: t.unlocks}
	for _, u := range t.unlocks {
		if ach, ok := t.definitions[u.AchievementID]; ok {
			p.TotalPoints += ach.Points
		}
	}

	// Calculate level (100 points per level)
	p.Level = p.TotalPoints/100 + 1

	// Assign title based on level
	p.Title = levelTitle(p.Level)

	return p
}

func levelTitle(level int) string {
	titles := []string{
		"Apprentice", "Journeyman", "Craftsman", "Artisan",
		"Expert", "Master", "Grandmaster", "Legendary Smith",
	}
	if level > len(titles) {
		return titles[len(titles)-1]
	}
	return titles[level-1]
}

// ListAchievements returns all achievements with unlock status.
func (t *Tracker) ListAchievements() []AchievementStatus {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []AchievementStatus
	for _, ach := range t.definitions {
		status := AchievementStatus{
			Achievement: *ach,
			Unlocked:    t.isUnlocked(ach.ID),
		}
		if status.Unlocked {
			for _, u := range t.unlocks {
				if u.AchievementID == ach.ID {
					status.UnlockedAt = &u.UnlockedAt
					break
				}
			}
		}
		result = append(result, status)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Points > result[j].Points
	})

	return result
}

// AchievementStatus combines an achievement with its unlock status.
type AchievementStatus struct {
	Achievement
	Unlocked    bool       `json:"unlocked"`
	UnlockedAt  *time.Time `json:"unlocked_at,omitempty"`
}

// GetStats returns achievement statistics.
func (t *Tracker) GetStats() map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := len(t.definitions)
	unlocked := len(t.unlocks)
	hidden := 0
	for _, ach := range t.definitions {
		if ach.Hidden && !t.isUnlocked(ach.ID) {
			hidden++
		}
	}

	return map[string]interface{}{
		"total":      total,
		"unlocked":   unlocked,
		"locked":     total - unlocked - hidden,
		"hidden":     hidden,
		"completion": float64(unlocked) / float64(total) * 100,
	}
}

func (t *Tracker) load() {
	data, err := os.ReadFile(filepath.Join(t.dir, "unlocks.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &t.unlocks)

	// Load event counts
	edata, err := os.ReadFile(filepath.Join(t.dir, "events.json"))
	if err != nil {
		return
	}
	json.Unmarshal(edata, &t.eventCounts)
}

func (t *Tracker) save() error {
	data, _ := json.MarshalIndent(t.unlocks, "", "  ")
	os.WriteFile(filepath.Join(t.dir, "unlocks.json"), data, 0o644)

	edata, _ := json.MarshalIndent(t.eventCounts, "", "  ")
	os.WriteFile(filepath.Join(t.dir, "events.json"), edata, 0o644)
	return nil
}

// FormatAchievement renders an achievement for display.
func FormatAchievement(ach *Achievement, unlocked bool) string {
	icon := "🔒"
	if unlocked {
		icon = ach.Icon
		if icon == "" {
			icon = "✅"
		}
	}
	status := "LOCKED"
	if unlocked {
		status = "UNLOCKED"
	}
	return fmt.Sprintf("%s %s [%s] — %s (%d pts, %s)", icon, ach.Name, status, ach.Description, ach.Points, ach.Rarity)
}

// FormatProfile renders a profile for display.
func FormatProfile(p *Profile) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Level %d: %s\n", p.Level, p.Title))
	sb.WriteString(fmt.Sprintf("Points: %d\n", p.TotalPoints))
	sb.WriteString(fmt.Sprintf("Achievements: %d\n", len(p.Unlocks)))
	return sb.String()
}
