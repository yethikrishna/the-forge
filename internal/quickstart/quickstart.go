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
	Action      string   `json:"action"` // what to do
	Verify      string   `json:"verify"` // how to verify success
	Tips        []string `json:"tips"`
	NextID      string   `json:"next_id"`
	SkipIf      string   `json:"skip_if,omitempty"` // condition to skip
}

// Result is the result of a quickstart run.
type Result struct {
	CompletedSteps []string      `json:"completed_steps"`
	SkippedSteps   []string      `json:"skipped_steps"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	FirstWin       string        `json:"first_win"`
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
	steps        []Step
	completed    map[string]bool
	skipped      map[string]bool
	startTime    time.Time
	achievements []Achievement
	reader       *bufio.Reader
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

// NewDemoQuickstart creates a quickstart using the 60-second demo flow.
// Each step non-interactively prints the command to run, suitable for
// a clean terminal recording session.
func NewDemoQuickstart() *Quickstart {
	return &Quickstart{
		steps:     DemoSteps(),
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
			ID:          "doctor-fix",
			Title:       "Health check & auto-fix",
			Description: "Diagnose and repair your Forge environment in one shot.",
			Action:      "Run: forge doctor --fix",
			Verify:      "All checks pass or auto-fixed",
			Tips:        []string{"Fixes Go SDK path, WAL files, permissions automatically", "Run 'forge doctor --verbose' for full detail"},
			NextID:      "local-init",
		},
		{
			ID:          "local-init",
			Title:       "Initialize a local project",
			Description: "Scaffold a zero-cloud project with Ollama + DeepSeek R1. No API keys needed.",
			Action:      "Run: forge init --local",
			Verify:      "Forgefile created with local model preset",
			Tips:        []string{"Uses Ollama + DeepSeek R1:8b by default", "Add --preset ollama-qwen for Qwen, --preset lmstudio for LM Studio"},
			NextID:      "learn-first",
		},
		{
			ID:          "learn-first",
			Title:       "Start the interactive tutorial",
			Description: "Walk through lesson 0: Forge in 60 Seconds.",
			Action:      "Run: forge learn 0",
			Verify:      "Lesson started",
			Tips:        []string{"Use 'forge learn list' to see all 7 lessons", "Lesson 7 covers governance + persistence", "'forge learn 0' is the demo lesson"},
			NextID:      "governance",
		},
		{
			ID:          "governance",
			Title:       "Grant consent & register an agent",
			Description: "Enable governance: consent receipt, catalog registration, and cost tracking.",
			Action:      "Run: forge consent grant --user demo --purposes execution,cost,audit\n       forge catalog register --name demo-agent --type agent --owner you\n       forge cost budget --set 10.00",
			Verify:      "Consent granted, agent in catalog, budget set",
			Tips:        []string{"All writes are async via write-behind cache (<100ns hot path)", "Check forge catalog list to see registered agents"},
			NextID:      "mcp-gateway",
		},
		{
			ID:          "mcp-gateway",
			Title:       "Route a request through the MCP gateway",
			Description: "Send a request through Forge's governed MCP v2.1 proxy: auth → rate-limit → schema-validate → audit.",
			Action:      "Run: forge gateway request --method tools/list --client demo",
			Verify:      "Request allowed, audit entry created",
			Tips:        []string{"Check forge gateway audit for the entry", "Add --token to test auth"},
			NextID:      "cost-live",
		},
		{
			ID:          "cost-live",
			Title:       "View live cost dashboard",
			Description: "See real-time token burn rate, projections, and per-agent cost breakdown.",
			Action:      "Run: forge cost live",
			Verify:      "Live cost dashboard appears",
			Tips:        []string{"forge cost compare shows model pricing", "Local models show $0.00 cost — that's the point"},
			NextID:      "",
		},
	}
}

// DemoSteps returns the exact 60-second demo flow used in the promo video.
// Each step prints its own action without waiting for user confirmation,
// designed to be run with --demo flag for a clean terminal recording.
func DemoSteps() []Step {
	return []Step{
		{
			ID:          "demo-doctor",
			Title:       "1/5  forge doctor --fix",
			Description: "Auto-diagnose and repair environment",
			Action:      "forge doctor --fix",
			Verify:      "clean",
			NextID:      "demo-init",
		},
		{
			ID:          "demo-init",
			Title:       "2/5  forge init --local",
			Description: "Zero-cloud project with Ollama/DeepSeek",
			Action:      "forge init --local",
			Verify:      "Forgefile",
			NextID:      "demo-learn",
		},
		{
			ID:          "demo-learn",
			Title:       "3/5  forge learn 0",
			Description: "Interactive demo lesson — Forge in 60 Seconds",
			Action:      "forge learn 0",
			Verify:      "started",
			NextID:      "demo-governance",
		},
		{
			ID:          "demo-governance",
			Title:       "4/5  Governance in action",
			Description: "Consent receipt + catalog registration + cost budget",
			Action:      "forge govern assess --name demo",
			Verify:      "score",
			NextID:      "demo-cost",
		},
		{
			ID:          "demo-cost",
			Title:       "5/5  forge cost live",
			Description: "Real-time cost dashboard — local models show $0.00",
			Action:      "forge cost live --once",
			Verify:      "dashboard",
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
