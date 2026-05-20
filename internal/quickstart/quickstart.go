// Package quickstart provides an interactive 5-minute onboarding experience.
// Guides users through their first successful Forge interaction with
// guaranteed first win.
//
// First impressions matter.
package quickstart

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Step represents an onboarding step.
type Step struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Action      string   `json:"action"`      // what to do
	Verify      string   `json:"verify"`       // how to verify success
	Tips        []string `json:"tips"`
	NextID      string   `json:"next_id"`
	SkipIf      string   `json:"skip_if,omitempty"` // condition to skip
}

// Result is the result of a quickstart run.
type Result struct {
	CompletedSteps []string    `json:"completed_steps"`
	SkippedSteps   []string    `json:"skipped_steps"`
	StartTime      time.Time   `json:"start_time"`
	EndTime        time.Time   `json:"end_time"`
	FirstWin       string      `json:"first_win"`
	Achievements   []Achievement `json:"achievements"`
}

// Achievement represents an unlocked achievement.
type Achievement struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UnlockedAt  time.Time `json:"unlocked_at"`
}

// Quickstart runs the interactive onboarding.
type Quickstart struct {
	steps       []Step
	completed   map[string]bool
	skipped     map[string]bool
	startTime   time.Time
	achievements []Achievement
	reader      *bufio.Reader
}

// NewQuickstart creates a new quickstart guide.
func NewQuickstart() *Quickstart {
	return &Quickstart{
		steps:     defaultSteps(),
		completed: make(map[string]bool),
		skipped:   make(map[string]bool),
		reader:    bufio.NewReader(os.Stdin),
	}
}

// Steps returns the onboarding steps.
func (q *Quickstart) Steps() []Step {
	return q.steps
}

// Run executes the interactive quickstart.
func (q *Quickstart) Run() (*Result, error) {
	q.startTime = time.Now()

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║     Welcome to Forge Quickstart! 🚀      ║")
	fmt.Println("  ║                                          ║")
	fmt.Println("  ║   5 minutes to your first win.           ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()

	for _, step := range q.steps {
		if !q.executeStep(step) {
			break // user quit
		}
	}

	result := &Result{
		CompletedSteps: q.completedList(),
		SkippedSteps:   q.skippedList(),
		StartTime:      q.startTime,
		EndTime:        time.Now(),
		Achievements:   q.achievements,
	}

	// Set first win
	if len(result.CompletedSteps) > 0 {
		result.FirstWin = result.CompletedSteps[0]
	}

	// Award achievements
	if len(result.CompletedSteps) == len(q.steps) {
		q.achievements = append(q.achievements, Achievement{
			ID:          "quickstart-complete",
			Name:        "Trailblazer",
			Description: "Completed the entire Forge quickstart",
			UnlockedAt:  time.Now(),
		})
		result.Achievements = q.achievements
	}

	fmt.Println()
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Completed in %v\n", result.EndTime.Sub(result.StartTime).Round(time.Second))
	fmt.Printf("  Steps done: %d/%d\n", len(result.CompletedSteps), len(q.steps))
	if result.FirstWin != "" {
		fmt.Printf("  First win: %s ✓\n", result.FirstWin)
	}
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return result, nil
}

func (q *Quickstart) executeStep(step Step) bool {
	fmt.Println()
	fmt.Printf("  ── Step: %s ──\n", step.Title)
	fmt.Printf("  %s\n", step.Description)
	fmt.Println()

	if step.Action != "" {
		fmt.Printf("  → %s\n", step.Action)
	}

	if len(step.Tips) > 0 {
		fmt.Println()
		fmt.Println("  Tips:")
		for _, tip := range step.Tips {
			fmt.Printf("    • %s\n", tip)
		}
	}

	fmt.Println()
	fmt.Print("  Done? (y/skip/quit): ")

	input, _ := q.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "quit", "q", "exit":
		return false
	case "skip", "s":
		q.skipped[step.ID] = true
		fmt.Printf("  Skipped: %s\n", step.Title)
		return true
	default:
		q.completed[step.ID] = true
		fmt.Printf("  ✓ %s complete!\n", step.Title)

		// Award step achievement
		q.achievements = append(q.achievements, Achievement{
			ID:          "step-" + step.ID,
			Name:        step.Title,
			Description: fmt.Sprintf("Completed quickstart step: %s", step.Title),
			UnlockedAt:  time.Now(),
		})
		return true
	}
}

func (q *Quickstart) completedList() []string {
	var list []string
	for _, step := range q.steps {
		if q.completed[step.ID] {
			list = append(list, step.ID)
		}
	}
	return list
}

func (q *Quickstart) skippedList() []string {
	var list []string
	for _, step := range q.steps {
		if q.skipped[step.ID] {
			list = append(list, step.ID)
		}
	}
	return list
}

func defaultSteps() []Step {
	return []Step{
		{
			ID:          "check-env",
			Title:       "Check your environment",
			Description: "Verify Forge is installed and ready.",
			Action:      "Run: forge doctor",
			Verify:      "All checks pass",
			Tips:        []string{"Fix any red items before continuing"},
			NextID:      "first-chat",
		},
		{
			ID:          "first-chat",
			Title:       "Your first chat",
			Description: "Start a conversation with your first agent.",
			Action:      "Run: forge chat \"Hello, what can you do?\"",
			Verify:      "Agent responds",
			Tips:        []string{"Try asking about Forge features", "Ask in natural language"},
			NextID:      "create-agent",
		},
		{
			ID:          "create-agent",
			Title:       "Create your first agent",
			Description: "Define an agent with a name and purpose.",
			Action:      "Run: forge agent create my-agent --model=gpt-4",
			Verify:      "Agent appears in 'forge agent list'",
			Tips:        []string{"Start simple — you can customize later", "The default model works great"},
			NextID:      "run-pipeline",
		},
		{
			ID:          "run-pipeline",
			Title:       "Run your first pipeline",
			Description: "Execute a multi-step agent workflow.",
			Action:      "Run: forge run my-agent \"Summarize this article: https://example.com\"",
			Verify:      "Pipeline completes with output",
			Tips:        []string{"Watch each step complete in real-time", "Check 'forge history' after"},
			NextID:      "review-costs",
		},
		{
			ID:          "review-costs",
			Title:       "Review your costs",
			Description: "See how much your agents have spent.",
			Action:      "Run: forge cost report",
			Verify:      "Cost report appears",
			Tips:        []string{"Set budgets to avoid surprises", "Use 'forge cost budget' to set limits"},
			NextID:      "explore-more",
		},
		{
			ID:          "explore-more",
			Title:       "Explore more",
			Description: "Discover what else Forge can do.",
			Action:      "Run: forge help",
			Verify:      "Command list appears",
			Tips:        []string{"Try 'forge seed' to bootstrap a project", "Use 'forge watch' during development", "Check 'forge tune' to optimize agents"},
			NextID:      "",
		},
	}
}

// FormatResult formats a quickstart result for display.
func FormatResult(r *Result) string {
	s := fmt.Sprintf("Quickstart completed: %d/%d steps\n", len(r.CompletedSteps), len(r.CompletedSteps)+len(r.SkippedSteps))
	s += fmt.Sprintf("Duration: %v\n", r.EndTime.Sub(r.StartTime).Round(time.Second))
	if r.FirstWin != "" {
		s += fmt.Sprintf("First win: %s\n", r.FirstWin)
	}
	if len(r.Achievements) > 0 {
		s += fmt.Sprintf("Achievements: %d unlocked\n", len(r.Achievements))
		for _, a := range r.Achievements {
			s += fmt.Sprintf("  🏆 %s — %s\n", a.Name, a.Description)
		}
	}
	return s
}

// FormatStep formats a step for display.
func FormatStep(s Step) string {
	out := fmt.Sprintf("  %s\n", s.Title)
	out += fmt.Sprintf("  %s\n", s.Description)
	if s.Action != "" {
		out += fmt.Sprintf("  → %s\n", s.Action)
	}
	return out
}
