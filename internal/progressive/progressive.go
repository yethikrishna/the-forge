// Package progressive implements a Progressive Complexity Ladder for Forge.
// Guides users from Level 0 (simple chat) through Level 5 (full serve),
// with clear milestones, recommended next steps, and skill unlocking.
// Like a video game tutorial that teaches as you play.
package progressive

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

// Level represents a user's proficiency level.
type Level int

const (
	Level0 Level = iota // Curious — just installed
	Level1              // Explorer — first chat, basic usage
	Level2              // Builder — agents, pipelines, cost tracking
	Level3              // Architect — orchestration, multi-agent, worktrees
	Level4              // Operator — serve, dashboard, CI, compliance
	Level5              // Master — plugins, custom agents, community
)

func (l Level) String() string {
	names := []string{"Curious", "Explorer", "Builder", "Architect", "Operator", "Master"}
	if int(l) < len(names) {
		return names[l]
	}
	return "Unknown"
}

func (l Level) Icon() string {
	icons := []string{"🌱", "🧭", "🔨", "🏛️", "⚙️", "👑"}
	if int(l) < len(icons) {
		return icons[l]
	}
	return "❓"
}

// Milestone represents a trackable achievement in the complexity ladder.
type Milestone struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Level       Level    `json:"level"`
	Command     string   `json:"command,omitempty"`
	Category    string   `json:"category"`
	Unlocks     []string `json:"unlocks,omitempty"`
	Completed   bool     `json:"completed"`
	CompletedAt string   `json:"completed_at,omitempty"`
}

// Ladder tracks a user's progression through the complexity levels.
type Ladder struct {
	UserLevel  Level                `json:"user_level"`
	Milestones map[string]*Milestone `json:"milestones"`
	XP         int                  `json:"xp"`
	StorePath  string               `json:"-"`
	mu         sync.Mutex
}

// NewLadder creates a new progressive complexity ladder.
func NewLadder(storePath string) *Ladder {
	l := &Ladder{
		UserLevel:  Level0,
		Milestones: make(map[string]*Milestone),
		StorePath:  storePath,
	}
	l.registerMilestones()
	l.load()
	return l
}

func (l *Ladder) registerMilestones() {
	milestones := []Milestone{
		// Level 0 → 1: Curious → Explorer
		{ID: "install", Name: "First Install", Description: "Install Forge on your machine", Level: Level0, Command: "forge version", Category: "getting-started"},
		{ID: "first-chat", Name: "Hello Agent", Description: "Start your first chat with an AI agent", Level: Level1, Command: "forge chat", Category: "chat"},
		{ID: "first-doctor", Name: "Health Check", Description: "Run Forge doctor to verify your setup", Level: Level1, Command: "forge doctor", Category: "diagnostics"},
		{ID: "first-models", Name: "Model Explorer", Description: "List available AI models", Level: Level1, Command: "forge models list", Category: "models"},

		// Level 1 → 2: Explorer → Builder
		{ID: "first-cost", Name: "Cost Conscious", Description: "Check LLM pricing with forge cost", Level: Level1, Command: "forge cost", Category: "cost"},
		{ID: "first-agent", Name: "Agent Creator", Description: "Create your first AI agent", Level: Level2, Command: "forge agents create", Category: "agents"},
		{ID: "first-init", Name: "Project Init", Description: "Initialize a Forge project", Level: Level2, Command: "forge init", Category: "config"},
		{ID: "first-pipeline", Name: "Pipeline Runner", Description: "Run your first agent pipeline", Level: Level2, Command: "forge pipeline run", Category: "pipelines"},
		{ID: "first-memory", Name: "Memory Keeper", Description: "Store and retrieve agent memories", Level: Level2, Command: "forge memory store", Category: "memory"},
		{ID: "first-seed", Name: "Seeder", Description: "Bootstrap a project from natural language", Level: Level2, Command: "forge seed", Category: "projects"},

		// Level 2 → 3: Builder → Architect
		{ID: "first-orchestrate", Name: "Orchestrator", Description: "Orchestrate multiple agents", Level: Level3, Command: "forge orchestrate", Category: "orchestration"},
		{ID: "first-worktree", Name: "Parallel Worker", Description: "Use git worktrees for parallel agents", Level: Level3, Command: "forge worktree create", Category: "git"},
		{ID: "first-review", Name: "Code Reviewer", Description: "Run an AI-powered code review", Level: Level3, Command: "forge review", Category: "review"},
		{ID: "first-debate", Name: "Debater", Description: "Run a multi-agent debate", Level: Level3, Command: "forge debate", Category: "multi-agent"},
		{ID: "first-abtest", Name: "A/B Tester", Description: "Compare two agent configurations", Level: Level3, Command: "forge abtest", Category: "quality"},
		{ID: "first-workspace", Name: "Workspace Manager", Description: "Manage multi-repo workspaces", Level: Level3, Command: "forge workspace init", Category: "workspaces"},

		// Level 3 → 4: Architect → Operator
		{ID: "first-serve", Name: "Server Operator", Description: "Start the Forge API server", Level: Level4, Command: "forge serve", Category: "server"},
		{ID: "first-ci", Name: "CI Runner", Description: "Run a Forge CI pipeline", Level: Level4, Command: "forge ci run", Category: "ci"},
		{ID: "first-schedule", Name: "Scheduler", Description: "Schedule a recurring agent task", Level: Level4, Command: "forge schedule create", Category: "automation"},
		{ID: "first-compliance", Name: "Compliance Officer", Description: "Generate a compliance report", Level: Level4, Command: "forge compliance soc2", Category: "compliance"},
		{ID: "first-dashboard", Name: "Dashboard Viewer", Description: "Open the Forge web dashboard", Level: Level4, Command: "forge dashboard", Category: "dashboard"},
		{ID: "first-witness", Name: "Witness", Description: "Record a cryptographic proof of agent action", Level: Level4, Command: "forge witness record", Category: "trust"},

		// Level 4 → 5: Operator → Master
		{ID: "first-plugin", Name: "Plugin Builder", Description: "Create a custom Forge plugin", Level: Level5, Command: "forge plugin create", Category: "plugins"},
		{ID: "first-breed", Name: "Evolver", Description: "Breed an optimized agent configuration", Level: Level5, Command: "forge breed", Category: "evolution"},
		{ID: "first-lsp", Name: "IDE Integrator", Description: "Connect Forge to your IDE via LSP", Level: Level5, Command: "forge lsp", Category: "ide"},
		{ID: "first-mcp", Name: "Protocol Master", Description: "Run Forge as an MCP server", Level: Level5, Command: "forge mcp serve", Category: "protocols"},
		{ID: "first-bridge", Name: "Bridge Builder", Description: "Bridge between MCP, A2A, and ACP protocols", Level: Level5, Command: "forge bridge serve", Category: "protocols"},
		{ID: "first-dream", Name: "Dreamer", Description: "Run offline agent improvement", Level: Level5, Command: "forge dream analyze", Category: "optimization"},
	}

	for _, m := range milestones {
		l.Milestones[m.ID] = &Milestone{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
			Level:       m.Level,
			Command:     m.Command,
			Category:    m.Category,
			Unlocks:     m.Unlocks,
		}
	}
}

// Complete marks a milestone as completed and awards XP.
func (l *Ladder) Complete(milestoneID string) (*Milestone, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	m, ok := l.Milestones[milestoneID]
	if !ok {
		return nil, fmt.Errorf("milestone %q not found", milestoneID)
	}

	if m.Completed {
		return m, nil // Already completed
	}

	m.Completed = true
	m.CompletedAt = time.Now().Format(time.RFC3339)

	// Award XP based on level
	xp := xpForLevel(m.Level)
	l.XP += xp

	// Check for level up
	l.recalculateLevel()

	l.save()
	return m, nil
}

// CurrentLevel returns the user's current level.
func (l *Ladder) CurrentLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.UserLevel
}

// MilestonesForLevel returns milestones for a specific level.
func (l *Ladder) MilestonesForLevel(level Level) []*Milestone {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []*Milestone
	for _, m := range l.Milestones {
		if m.Level == level {
			result = append(result, m)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// NextSteps returns the recommended next milestones to complete.
func (l *Ladder) NextSteps() []*Milestone {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Find incomplete milestones at or below current level + 1
	var result []*Milestone
	targetLevel := Level(int(l.UserLevel) + 1)
	if targetLevel > Level5 {
		targetLevel = Level5
	}

	for _, m := range l.Milestones {
		if !m.Completed && m.Level <= targetLevel {
			result = append(result, m)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Level != result[j].Level {
			return result[i].Level < result[j].Level
		}
		return result[i].ID < result[j].ID
	})

	return result
}

// Progress returns completion progress per level.
func (l *Ladder) Progress() map[Level]LevelProgress {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := make(map[Level]LevelProgress)
	for level := Level0; level <= Level5; level++ {
		total := 0
		completed := 0
		for _, m := range l.Milestones {
			if m.Level == level {
				total++
				if m.Completed {
					completed++
				}
			}
		}
		result[level] = LevelProgress{
			Level:     level,
			Total:     total,
			Completed: completed,
			Pct:       0,
		}
		if total > 0 {
			result[level] = LevelProgress{
				Level:     level,
				Total:     total,
				Completed: completed,
				Pct:       float64(completed) / float64(total) * 100,
			}
		}
	}
	return result
}

// LevelProgress tracks progress for a single level.
type LevelProgress struct {
	Level     Level   `json:"level"`
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Pct       float64 `json:"pct"`
}

// OverallProgress returns the overall completion percentage.
func (l *Ladder) OverallProgress() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	total := len(l.Milestones)
	if total == 0 {
		return 0
	}
	completed := 0
	for _, m := range l.Milestones {
		if m.Completed {
			completed++
		}
	}
	return float64(completed) / float64(total) * 100
}

// Path returns the recommended learning path as a formatted string.
func (l *Ladder) Path() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	var b strings.Builder

	b.WriteString("Forge Progressive Complexity Ladder\n")
	b.WriteString("===================================\n\n")

	for level := Level0; level <= Level5; level++ {
		current := "  "
		if level == l.UserLevel {
			current = "→ "
		}
		completed := "  "
		if level < l.UserLevel {
			completed = "✅"
		}

		b.WriteString(fmt.Sprintf("%s %s Level %d: %s %s\n", current, completed, level, level.Icon(), level))

		milestones := l.MilestonesForLevel(level)
		for _, m := range milestones {
			status := "⬜"
			if m.Completed {
				status = "✅"
			}
			cmd := ""
			if m.Command != "" {
				cmd = fmt.Sprintf(" (%s)", m.Command)
			}
			b.WriteString(fmt.Sprintf("    %s %s%s\n", status, m.Description, cmd))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Stats returns ladder statistics.
func (l *Ladder) Stats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()

	total := len(l.Milestones)
	completed := 0
	for _, m := range l.Milestones {
		if m.Completed {
			completed++
		}
	}

	return map[string]interface{}{
		"level":         l.UserLevel.String(),
		"level_number":  int(l.UserLevel),
		"xp":            l.XP,
		"total_milestones": total,
		"completed_milestones": completed,
		"overall_pct":   l.OverallProgress(),
	}
}

func (l *Ladder) recalculateLevel() {
	// Level up if all milestones for the current level are complete
	for level := Level0; level < Level5; level++ {
		allDone := true
		hasMilestones := false
		for _, m := range l.Milestones {
			if m.Level == level {
				hasMilestones = true
				if !m.Completed {
					allDone = false
					break
				}
			}
		}
		if hasMilestones && allDone && int(l.UserLevel) <= int(level) {
			l.UserLevel = level + 1
		}
	}
}

func xpForLevel(level Level) int {
	baseXP := []int{10, 25, 50, 100, 200, 500}
	if int(level) < len(baseXP) {
		return baseXP[level]
	}
	return 100
}

func (l *Ladder) save() {
	if l.StorePath == "" {
		return
	}

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(l.StorePath), 0755)
	os.WriteFile(l.StorePath, data, 0644)
}

func (l *Ladder) load() {
	if l.StorePath == "" {
		return
	}

	data, err := os.ReadFile(l.StorePath)
	if err != nil {
		return
	}

	var saved Ladder
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}

	// Merge saved state
	l.UserLevel = saved.UserLevel
	l.XP = saved.XP
	for id, m := range saved.Milestones {
		if existing, ok := l.Milestones[id]; ok {
			existing.Completed = m.Completed
			existing.CompletedAt = m.CompletedAt
		}
	}
}
