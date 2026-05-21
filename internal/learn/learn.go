// Package learn provides an interactive terminal tutorial system for Forge.
// Hands-on lessons with progressive complexity, step-by-step instructions,
// verification checks, and progress tracking.
//
// Learn by doing. Not by reading.
package learn

import (
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

// Difficulty represents lesson difficulty.
type Difficulty string

const (
	DiffBeginner     Difficulty = "beginner"
	DiffIntermediate Difficulty = "intermediate"
	DiffAdvanced     Difficulty = "advanced"
)

// StepStatus represents the state of a lesson step.
type StepStatus string

const (
	StepNotStarted StepStatus = "not_started"
	StepInProgress StepStatus = "in_progress"
	StepCompleted  StepStatus = "completed"
	StepSkipped    StepStatus = "skipped"
)

// Step is a single step within a lesson.
type Step struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Instruction string     `json:"instruction"`
	Command     string     `json:"command,omitempty"`     // Suggested command
	Verify      string     `json:"verify,omitempty"`      // Verification command
	VerifyMsg   string     `json:"verify_msg,omitempty"`  // Message on success
	Hint        string     `json:"hint,omitempty"`        // Hint if stuck
	Explanation string     `json:"explanation,omitempty"` // Why this matters
	Status      StepStatus `json:"status"`
	Order       int        `json:"order"`
}

// Lesson is a complete tutorial lesson.
type Lesson struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Difficulty    Difficulty `json:"difficulty"`
	Category      string     `json:"category"`                // e.g. "getting-started", "agents", "pipelines", "security"
	Duration      string     `json:"duration"`                // e.g. "5 min", "15 min"
	Prerequisites []string   `json:"prerequisites,omitempty"` // Lesson IDs
	Steps         []Step     `json:"steps"`
	Tags          []string   `json:"tags,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Progress tracks user progress across lessons.
type Progress struct {
	LessonID    string     `json:"lesson_id"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CurrentStep int        `json:"current_step"`
	StepsDone   int        `json:"steps_done"`
	Status      string     `json:"status"` // not_started, in_progress, completed
	Score       int        `json:"score"`  // 0-100
}

// Store manages lessons and progress.
type Store struct {
	Dir      string
	mu       sync.RWMutex
	lessons  map[string]*Lesson
	progress map[string]*Progress
	pstore   *persistence.Store
}

// NewStore creates or loads a learn store.
func NewStore(dir string) (*Store, error) {
	s := &Store{
		Dir:      dir,
		lessons:  make(map[string]*Lesson),
		progress: make(map[string]*Progress),
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create learn dir: %w", err)
	}
	if err := s.load(); err != nil {
		return s, nil
	}

	ps, err := persistence.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("learn: open persistence store: %w", err)
	}
	s.pstore = ps
	ps.Register("lessons", func() ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return json.MarshalIndent(s.lessons, "", "  ")
	})
	ps.Register("progress", func() ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return json.MarshalIndent(s.progress, "", "  ")
	})

	// Seed built-in lessons if store is empty.
	if len(s.lessons) == 0 {
		s.seedBuiltinLessons()
	}
	return s, nil
}

// Close flushes pending writes and stops the background syncer.
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

// CreateLesson adds a new lesson.
func (s *Store) CreateLesson(lesson Lesson) (*Lesson, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lesson.Title == "" {
		return nil, fmt.Errorf("lesson title is required")
	}
	if lesson.ID == "" {
		lesson.ID = slugify(lesson.Title)
	}
	if _, exists := s.lessons[lesson.ID]; exists {
		return nil, fmt.Errorf("lesson %s already exists", lesson.ID)
	}

	now := time.Now().UTC()
	lesson.CreatedAt = now
	lesson.UpdatedAt = now

	// Assign step IDs and orders.
	for i := range lesson.Steps {
		if lesson.Steps[i].ID == "" {
			lesson.Steps[i].ID = fmt.Sprintf("%s-step-%d", lesson.ID, i+1)
		}
		lesson.Steps[i].Order = i + 1
		if lesson.Steps[i].Status == "" {
			lesson.Steps[i].Status = StepNotStarted
		}
	}

	if lesson.Difficulty == "" {
		lesson.Difficulty = DiffBeginner
	}

	s.lessons[lesson.ID] = &lesson
	s.progress[lesson.ID] = &Progress{
		LessonID:    lesson.ID,
		Status:      "not_started",
		CurrentStep: 0,
	}

	s.markDirty()
	return &lesson, nil
}

// GetLesson retrieves a lesson by ID.
func (s *Store) GetLesson(id string) (*Lesson, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.lessons[id]
	if !ok {
		return nil, fmt.Errorf("lesson %s not found", id)
	}
	return l, nil
}

// ListLessons returns all lessons, optionally filtered.
func (s *Store) ListLessons(filters map[string]string) ([]*Lesson, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Lesson
	for _, l := range s.lessons {
		if matchesLessonFilters(l, filters) {
			results = append(results, l)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		order := map[Difficulty]int{DiffBeginner: 0, DiffIntermediate: 1, DiffAdvanced: 2}
		if order[results[i].Difficulty] != order[results[j].Difficulty] {
			return order[results[i].Difficulty] < order[results[j].Difficulty]
		}
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results, nil
}

// DeleteLesson removes a lesson.
func (s *Store) DeleteLesson(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.lessons[id]; !ok {
		return fmt.Errorf("lesson %s not found", id)
	}
	delete(s.lessons, id)
	delete(s.progress, id)
	s.markDirty()
	return nil
}

// StartLesson begins a lesson.
func (s *Store) StartLesson(id string) (*Lesson, *Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.lessons[id]
	if !ok {
		return nil, nil, fmt.Errorf("lesson %s not found", id)
	}

	p, ok := s.progress[id]
	if !ok {
		p = &Progress{LessonID: id, Status: "not_started"}
		s.progress[id] = p
	}

	if p.Status == "completed" {
		// Restart.
		p.Status = "in_progress"
		p.CurrentStep = 0
		p.StepsDone = 0
		p.CompletedAt = nil
		for i := range l.Steps {
			l.Steps[i].Status = StepNotStarted
		}
	}

	now := time.Now().UTC()
	p.StartedAt = &now
	p.Status = "in_progress"
	if p.CurrentStep == 0 && len(l.Steps) > 0 {
		p.CurrentStep = 1
		l.Steps[0].Status = StepInProgress
	}

	s.markDirty()
	return l, p, nil
}

// GetCurrentStep returns the current step for a lesson.
func (s *Store) GetCurrentStep(lessonID string) (*Step, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, ok := s.lessons[lessonID]
	if !ok {
		return nil, fmt.Errorf("lesson %s not found", lessonID)
	}
	p := s.progress[lessonID]
	if p == nil || p.CurrentStep < 1 || p.CurrentStep > len(l.Steps) {
		return nil, fmt.Errorf("no current step")
	}
	return &l.Steps[p.CurrentStep-1], nil
}

// CompleteStep marks a step as done and advances.
func (s *Store) CompleteStep(lessonID string, stepID string) (*Step, *Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.lessons[lessonID]
	if !ok {
		return nil, nil, fmt.Errorf("lesson %s not found", lessonID)
	}
	p := s.progress[lessonID]
	if p == nil || p.Status != "in_progress" {
		return nil, nil, fmt.Errorf("lesson not in progress")
	}

	// Find and complete the step.
	var completed *Step
	for i := range l.Steps {
		if l.Steps[i].ID == stepID {
			l.Steps[i].Status = StepCompleted
			completed = &l.Steps[i]
			p.StepsDone++
			break
		}
	}
	if completed == nil {
		return nil, nil, fmt.Errorf("step %s not found", stepID)
	}

	// Advance to next step or complete lesson.
	if p.CurrentStep < len(l.Steps) {
		p.CurrentStep++
		l.Steps[p.CurrentStep-1].Status = StepInProgress
	} else {
		// Lesson complete.
		p.Status = "completed"
		now := time.Now().UTC()
		p.CompletedAt = &now
		p.Score = computeScore(l, p)
	}

	s.markDirty()
	return completed, p, nil
}

// SkipStep skips a step.
func (s *Store) SkipStep(lessonID, stepID string) (*Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.lessons[lessonID]
	if !ok {
		return nil, fmt.Errorf("lesson %s not found", lessonID)
	}
	p := s.progress[lessonID]
	if p == nil {
		return nil, fmt.Errorf("lesson not started")
	}

	for i := range l.Steps {
		if l.Steps[i].ID == stepID {
			l.Steps[i].Status = StepSkipped
			break
		}
	}

	if p.CurrentStep < len(l.Steps) {
		p.CurrentStep++
		l.Steps[p.CurrentStep-1].Status = StepInProgress
	} else {
		p.Status = "completed"
		now := time.Now().UTC()
		p.CompletedAt = &now
		p.Score = computeScore(l, p)
	}

	s.markDirty()
	return p, nil
}

// GetProgress returns progress for a lesson.
func (s *Store) GetProgress(lessonID string) (*Progress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.progress[lessonID]
	if !ok {
		return nil, fmt.Errorf("no progress for lesson %s", lessonID)
	}
	return p, nil
}

// GetAllProgress returns all progress records.
func (s *Store) GetAllProgress() map[string]*Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*Progress, len(s.progress))
	for k, v := range s.progress {
		result[k] = v
	}
	return result
}

// ResetProgress clears progress for a lesson.
func (s *Store) ResetProgress(lessonID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.lessons[lessonID]
	if !ok {
		return fmt.Errorf("lesson %s not found", lessonID)
	}

	s.progress[lessonID] = &Progress{
		LessonID:    lessonID,
		Status:      "not_started",
		CurrentStep: 0,
	}
	for i := range l.Steps {
		l.Steps[i].Status = StepNotStarted
	}
	s.markDirty()
	return nil
}

// Stats returns learning statistics.
type Stats struct {
	TotalLessons    int                `json:"total_lessons"`
	CompletedCount  int                `json:"completed_count"`
	InProgressCount int                `json:"in_progress_count"`
	NotStartedCount int                `json:"not_started_count"`
	AvgScore        float64            `json:"avg_score"`
	ByDifficulty    map[Difficulty]int `json:"by_difficulty"`
	ByCategory      map[string]int     `json:"by_category"`
	TotalSteps      int                `json:"total_steps"`
	StepsCompleted  int                `json:"steps_completed"`
}

// GetStats returns aggregate learning statistics.
func (s *Store) GetStats() *Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		TotalLessons: len(s.lessons),
		ByDifficulty: make(map[Difficulty]int),
		ByCategory:   make(map[string]int),
	}

	var totalScore float64
	var scoredCount int
	for _, l := range s.lessons {
		stats.ByDifficulty[l.Difficulty]++
		stats.ByCategory[l.Category]++
		stats.TotalSteps += len(l.Steps)
	}
	for _, p := range s.progress {
		switch p.Status {
		case "completed":
			stats.CompletedCount++
			totalScore += float64(p.Score)
			scoredCount++
		case "in_progress":
			stats.InProgressCount++
		default:
			stats.NotStartedCount++
		}
		stats.StepsCompleted += p.StepsDone
	}
	if scoredCount > 0 {
		stats.AvgScore = totalScore / float64(scoredCount)
	}
	return stats
}

// ExportProgress exports all progress as JSON.
func (s *Store) ExportProgress() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.MarshalIndent(s.progress, "", "  ")
}

// --- helpers ---

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var result string
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result += string(c)
		}
	}
	return result
}

func computeScore(l *Lesson, p *Progress) int {
	if len(l.Steps) == 0 {
		return 100
	}
	completed := 0
	for _, step := range l.Steps {
		if step.Status == StepCompleted {
			completed++
		}
	}
	return (completed * 100) / len(l.Steps)
}

func matchesLessonFilters(l *Lesson, filters map[string]string) bool {
	for k, v := range filters {
		switch k {
		case "difficulty":
			if string(l.Difficulty) != v {
				return false
			}
		case "category":
			if l.Category != v {
				return false
			}
		case "tag":
			found := false
			for _, t := range l.Tags {
				if strings.EqualFold(t, v) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// --- built-in lessons ---

func (s *Store) seedBuiltinLessons() {
	lessons := []Lesson{
		{
			ID: "forge-in-60-seconds", Title: "Forge in 60 Seconds", Category: "getting-started",
			Difficulty: DiffBeginner, Duration: "1 min",
			Description: "The exact 5-command sequence from the demo video. Zero cloud, full governance, first agent running in under 60 seconds.",
			Tags: []string{"demo", "quickstart", "local", "governance", "onboarding"},
			Steps: []Step{
				{
					ID: "step-doctor", Title: "Auto-fix your environment",
					Instruction: "Run forge doctor --fix to diagnose and repair your Forge environment automatically.",
					Command: "forge doctor --fix",
					VerifyMsg: "All checks pass or auto-fixed",
					Hint: "If Go SDK is missing, forge doctor --fix will guide you to install it.",
					Explanation: "forge doctor checks Go toolchain, API keys, network, Forgefile, disk space, WAL files, and local model presets. --fix repairs what it can automatically: adds Go SDK to PATH, replays stale WALs, creates missing .forge dirs.",
					Order: 1,
				},
				{
					ID: "step-init", Title: "Initialize a zero-cloud project",
					Instruction: "Create a local project using Ollama + DeepSeek R1. No API keys. No cloud.",
					Command: "forge init --local",
					VerifyMsg: "Forgefile created with local model preset",
					Hint: "Use --preset ollama-qwen for Qwen or --preset lmstudio for LM Studio.",
					Explanation: "forge init --local scaffolds your project with the ollama-deepseek preset: a Forgefile pre-configured for DeepSeek R1:8b, .forge/ directory for governance state, and a README showing the Ollama setup steps. Cost: $0.00.",
					Order: 2,
				},
				{
					ID: "step-learn", Title: "Start the interactive tutorial",
					Instruction: "Jump into lesson 1 to create your first agent.",
					Command: "forge learn start your-first-agent",
					VerifyMsg: "Lesson started",
					Hint: "Use 'forge learn list' to see all 6 available lessons.",
					Explanation: "forge learn is a built-in interactive tutorial system. Lesson 1 walks you through your first agent in 5 steps. Lesson 6 covers the persistence + governance stack you're using right now.",
					Order: 3,
				},
				{
					ID: "step-governance", Title: "Governance in action",
					Instruction: "Run a governance assessment to see Forge's safety-first architecture in action.",
					Command: "forge govern assess --name demo",
					VerifyMsg: "Assessment score displayed",
					Hint: "Try 'forge catalog list' to see registered agents, 'forge cost budget --set 10' to cap spend.",
					Explanation: "Forge's governance stack scores your agent system across security, compliance, cost, audit, resilience, and ethics. The consent receipt, catalog, and cost tracking all work together. Every mutation is async via write-behind cache (≤61ns hot path).",
					Order: 4,
				},
				{
					ID: "step-cost-live", Title: "View the live cost dashboard",
					Instruction: "Open the real-time cost dashboard to see token burn rate and monthly projections.",
					Command: "forge cost live",
					VerifyMsg: "Live cost dashboard shown",
					Hint: "Local models show $0.00 — that's the point. 'forge cost compare' shows cloud model pricing for comparison.",
					Explanation: "forge cost live shows real-time token usage, per-model/per-agent breakdown, burn rate, and projected monthly spend. With local models the cost is always $0. With cloud models, budgets prevent runaway spend.",
					Order: 5,
				},
			},
		},
		{
			ID: "your-first-agent", Title: "Your First Agent", Category: "getting-started",
			Difficulty: DiffBeginner, Duration: "5 min",
			Description: "Create and run your first AI agent with Forge.",
			Steps: []Step{
				{Title: "Check your environment", Instruction: "Verify Forge is installed and configured.", Command: "forge doctor", Verify: "forge doctor", VerifyMsg: "Environment looks good!", Hint: "Run forge doctor to check prerequisites.", Explanation: "Always verify your setup first."},
				{Title: "Initialize a project", Instruction: "Create a new Forge project.", Command: "forge init my-project", Hint: "Use forge init with a project name.", Explanation: "forge init scaffolds a project with sensible defaults."},
				{Title: "Start a chat", Instruction: "Start an interactive chat with an agent.", Command: "forge chat", Hint: "forge chat opens an interactive session.", Explanation: "Chat is the simplest way to interact with agents."},
				{Title: "Check agent status", Instruction: "See what agents are available.", Command: "forge agents list", Explanation: "forge agents shows all configured agents."},
			},
		},
		{
			ID: "building-pipelines", Title: "Building Pipelines", Category: "pipelines",
			Difficulty: DiffIntermediate, Duration: "15 min",
			Description:   "Create multi-step agent pipelines for complex workflows.",
			Prerequisites: []string{"your-first-agent"},
			Steps: []Step{
				{Title: "Understand pipeline syntax", Instruction: "Review the Forgefile pipeline format.", Explanation: "Pipelines chain agents together with input/output contracts."},
				{Title: "Create a pipeline", Instruction: "Define a code-review pipeline.", Command: "forge pipeline create review.yaml", Explanation: "Pipelines are defined in YAML with steps, each being an agent call."},
				{Title: "Run the pipeline", Instruction: "Execute the pipeline.", Command: "forge pipeline run review.yaml", Explanation: "forge pipeline run executes each step sequentially."},
				{Title: "Check pipeline status", Instruction: "View pipeline execution results.", Command: "forge pipeline status", Explanation: "Track pipeline progress and results."},
			},
		},
		{
			ID: "security-basics", Title: "Security Basics", Category: "security",
			Difficulty: DiffIntermediate, Duration: "10 min",
			Description: "Learn sandboxing, jail, and security features.",
			Steps: []Step{
				{Title: "Run diagnostics", Instruction: "Check security configuration.", Command: "forge doctor", Explanation: "forge doctor includes security checks."},
				{Title: "Create a sandbox", Instruction: "Run a command in a sandbox.", Command: "forge exec --sandbox 'echo hello'", Explanation: "Sandboxing isolates agent actions."},
				{Title: "Scan for secrets", Instruction: "Scan your codebase for leaked secrets.", Command: "forge secrets scan", Explanation: "Prevent secret leaks before they happen."},
				{Title: "Review audit log", Instruction: "Check what actions agents have taken.", Command: "forge audit list", Explanation: "Every agent action is logged for accountability."},
			},
		},
		{
			ID: "cost-management", Title: "Cost Management", Category: "cost",
			Difficulty: DiffBeginner, Duration: "5 min",
			Description: "Track and optimize LLM spending across providers.",
			Steps: []Step{
				{Title: "Check pricing", Instruction: "Compare model pricing.", Command: "forge cost compare", Explanation: "forge cost shows pricing across all configured providers."},
				{Title: "Set a budget", Instruction: "Configure a spending limit.", Command: "forge cost budget --set 10.00", Explanation: "Budgets prevent runaway costs."},
				{Title: "View spending", Instruction: "Check current spending.", Command: "forge cost report", Explanation: "Track spending by agent, model, and session."},
			},
		},
		{
			ID: "multi-agent-orchestration", Title: "Multi-Agent Orchestration", Category: "agents",
			Difficulty: DiffAdvanced, Duration: "20 min",
			Description:   "Orchestrate multiple agents for complex tasks.",
			Prerequisites: []string{"building-pipelines"},
			Steps: []Step{
				{Title: "List agents", Instruction: "See available agents.", Command: "forge agents list", Explanation: "Know your team before you deploy."},
				{Title: "Assign roles", Instruction: "Define agent roles (planner, coder, reviewer).", Command: "forge role create planner --model gpt-4", Explanation: "Roles define agent capabilities and constraints."},
				{Title: "Run orchestration", Instruction: "Launch a multi-agent task.", Command: "forge orchestrate --config team.yaml", Explanation: "Orchestration coordinates agents with context sharing."},
				{Title: "Review results", Instruction: "Check orchestration output and costs.", Command: "forge session list", Explanation: "Review what each agent contributed."},
				{Title: "Consensus voting", Instruction: "Run agents with consensus.", Command: "forge consensus run --agents 3 --task 'review'", Explanation: "Consensus runs multiple agents and votes on the best result."},
			},
		},
		{
			ID: "governance-and-persistence", Title: "Governance & Persistence", Category: "governance",
			Difficulty: DiffIntermediate, Duration: "10 min",
			Description:   "Understand Forge's write-behind persistence layer and governance stack. Learn how WAL crash recovery, cost transparency, and catalog work together.",
			Prerequisites: []string{"your-first-agent"},
			Tags:          []string{"persistence", "governance", "catalog", "costlive", "wal"},
			Steps: []Step{
				{
					Title:       "Health-check the environment",
					Instruction: "Run forge doctor to confirm the persistence layer is healthy and no stale WAL files exist.",
					Command:     "forge doctor",
					VerifyMsg:   "Persistence WAL: no stale files (clean state)",
					Hint:        "If you see stale WAL files, run: forge doctor --fix",
					Explanation: "Forge uses a write-behind cache with WAL (Write-Ahead Log) for crash recovery. Every mutation is first logged to a .wal file, then atomically renamed to the target .json. On restart, incomplete WALs are replayed automatically. forge doctor checks this for you.",
				},
				{
					Title:       "Register an agent in the catalog",
					Instruction: "Register a new agent entry in the Forge catalog. The catalog is a Unity-Catalog-style registry with governance metadata.",
					Command:     "forge catalog register --name my-agent --type agent --owner alice",
					Hint:        "Use forge catalog list to see all registered entries.",
					Explanation: "The catalog stores agent definitions, tools, models, and data sources with ownership, lineage, tags, and access policies. Writes go through the write-behind cache — register() returns in <100µs regardless of catalog size.",
				},
				{
					Title:       "Start cost tracking",
					Instruction: "Enable live cost tracking and set a monthly budget.",
					Command:     "forge cost budget --set 10.00",
					VerifyMsg:   "Budget set",
					Hint:        "Run forge cost live to see real-time spend.",
					Explanation: "costlive records every LLM token usage event. Before the persistence fix, each Record() call took ~3ms due to full JSON rewrite. Now it's 61ns — making live dashboards practical even at high agent throughput.",
				},
				{
					Title:       "Run a governance assessment",
					Instruction: "Run a governance assessment to get a scored report across security, compliance, and audit dimensions.",
					Command:     "forge govern assess --name baseline",
					Hint:        "Use forge govern report to export results as Markdown.",
					Explanation: "Governance assessments score your agent system across categories (security, compliance, cost, audit, resilience, ethics). Each Assess() call is now async — the in-memory score is available instantly; disk write happens in the background.",
				},
				{
					Title:       "Flush and verify persistence",
					Instruction: "Flush pending writes to disk and verify the data files exist.",
					Command:     "forge doctor --verbose",
					VerifyMsg:   ".forge directory is writable",
					Hint:        "Look for entries.json, live.json, assessments.json in .forge/catalog, .forge/costlive, .forge/govern.",
					Explanation: "The write-behind cache flushes every 500ms. You can trigger an immediate flush with Flush() in code or by calling forge doctor --fix which performs a WAL replay. This is the contract: callers must Flush()/Close() before reading from a fresh store instance.",
				},
			},
		},
	}

	for _, l := range lessons {
		s.CreateLesson(l)
	}
}

// --- persistence ---

func (s *Store) load() error {
	lessonsPath := filepath.Join(s.Dir, "lessons.json")
	progressPath := filepath.Join(s.Dir, "progress.json")

	if data, err := os.ReadFile(lessonsPath); err == nil {
		if err := json.Unmarshal(data, &s.lessons); err != nil {
			return fmt.Errorf("unmarshal lessons: %w", err)
		}
	}
	if data, err := os.ReadFile(progressPath); err == nil {
		if err := json.Unmarshal(data, &s.progress); err != nil {
			return fmt.Errorf("unmarshal progress: %w", err)
		}
	}
	return nil
}

func (s *Store) markDirty() {
	if s.pstore != nil {
		s.pstore.Dirty("lessons")
		s.pstore.Dirty("progress")
	}
}
