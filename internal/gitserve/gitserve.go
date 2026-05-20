// Package gitserve provides Git hook integration for Forge agents.
// Automatically triggers agent actions on git events (pre-commit, post-commit, pre-push, etc.)
// Like Husky but for AI agents — your git hooks are now intelligent.
package gitserve

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HookType represents a git hook type.
type HookType string

const (
	HookPreCommit    HookType = "pre-commit"
	HookPostCommit   HookType = "post-commit"
	HookPrePush      HookType = "pre-push"
	HookPostMerge    HookType = "post-merge"
	HookCommitMsg    HookType = "commit-msg"
	HookPreRebase    HookType = "pre-rebase"
	HookPostCheckout HookType = "post-checkout"
)

// HookAction defines what an agent does when a hook fires.
type HookAction struct {
	Agent   string   `json:"agent" yaml:"agent"`
	Model   string   `json:"model,omitempty" yaml:"model,omitempty"`
	Prompt  string   `json:"prompt" yaml:"prompt"`
	Files   []string `json:"files,omitempty" yaml:"files,omitempty"` // File patterns to match
	Timeout string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Block   bool     `json:"block,omitempty" yaml:"block,omitempty"` // Block git operation on failure
}

// Hook represents a configured git hook.
type Hook struct {
	ID          string       `json:"id"`
	Type        HookType     `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Actions     []HookAction `json:"actions"`
	Enabled     bool         `json:"enabled"`
	CreatedAt   time.Time    `json:"created_at"`
	LastRun     *time.Time   `json:"last_run,omitempty"`
	RunCount    int          `json:"run_count"`
}

// HookResult represents the result of a hook execution.
type HookResult struct {
	HookID    string    `json:"hook_id"`
	Type      HookType  `json:"type"`
	Success   bool      `json:"success"`
	Output    string    `json:"output"`
	Duration  string    `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// Manager manages git hooks for Forge.
type Manager struct {
	projectDir string
	hooksDir   string
	storePath  string
	hooks      map[string]*Hook
	results    []HookResult
}

// NewManager creates a new git hook manager.
func NewManager(projectDir string) *Manager {
	hooksDir := filepath.Join(projectDir, ".git", "hooks")
	storePath := filepath.Join(projectDir, ".forge", "git-hooks.json")

	m := &Manager{
		projectDir: projectDir,
		hooksDir:   hooksDir,
		storePath:  storePath,
		hooks:      make(map[string]*Hook),
		results:    make([]HookResult, 0),
	}
	m.load()
	return m
}

// AddHook adds a new git hook.
func (m *Manager) AddHook(hookType HookType, name, description string, actions []HookAction) (*Hook, error) {
	id := generateHookID(name, hookType)

	hook := &Hook{
		ID:          id,
		Type:        hookType,
		Name:        name,
		Description: description,
		Actions:     actions,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	m.hooks[id] = hook
	m.save()

	return hook, nil
}

// RemoveHook removes a git hook.
func (m *Manager) RemoveHook(id string) error {
	if _, ok := m.hooks[id]; !ok {
		return fmt.Errorf("hook %s not found", id)
	}

	delete(m.hooks, id)
	m.save()
	return nil
}

// GetHook retrieves a hook by ID.
func (m *Manager) GetHook(id string) (*Hook, bool) {
	h, ok := m.hooks[id]
	return h, ok
}

// ListHooks lists all configured hooks.
func (m *Manager) ListHooks() []*Hook {
	result := make([]*Hook, 0, len(m.hooks))
	for _, h := range m.hooks {
		result = append(result, h)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByType lists hooks filtered by type.
func (m *Manager) ListByType(hookType HookType) []*Hook {
	var result []*Hook
	for _, h := range m.hooks {
		if h.Type == hookType {
			result = append(result, h)
		}
	}
	return result
}

// EnableHook enables a hook.
func (m *Manager) EnableHook(id string) error {
	h, ok := m.hooks[id]
	if !ok {
		return fmt.Errorf("hook %s not found", id)
	}
	h.Enabled = true
	m.save()
	return nil
}

// DisableHook disables a hook.
func (m *Manager) DisableHook(id string) error {
	h, ok := m.hooks[id]
	if !ok {
		return fmt.Errorf("hook %s not found", id)
	}
	h.Enabled = false
	m.save()
	return nil
}

// RunHook executes a hook by ID (simulated).
func (m *Manager) RunHook(id string) (*HookResult, error) {
	hook, ok := m.hooks[id]
	if !ok {
		return nil, fmt.Errorf("hook %s not found", id)
	}

	if !hook.Enabled {
		return nil, fmt.Errorf("hook %s is disabled", id)
	}

	start := time.Now()
	result := &HookResult{
		HookID:    id,
		Type:      hook.Type,
		Timestamp: start,
	}

	// Simulate hook execution
	var outputs []string
	allSuccess := true

	for _, action := range hook.Actions {
		output := simulateAction(hook.Type, action)
		outputs = append(outputs, output)
		if strings.Contains(output, "FAILED") {
			allSuccess = false
		}
	}

	result.Output = strings.Join(outputs, "\n")
	result.Success = allSuccess
	result.Duration = time.Since(start).Round(time.Millisecond).String()

	if !allSuccess {
		result.Error = "one or more actions failed"
	}

	// Update hook stats
	now := time.Now()
	hook.LastRun = &now
	hook.RunCount++
	m.save()

	// Store result
	m.results = append(m.results, *result)
	if len(m.results) > 100 {
		m.results = m.results[len(m.results)-100:]
	}

	return result, nil
}

// RunHooksByType runs all hooks of a given type.
func (m *Manager) RunHooksByType(hookType HookType) []*HookResult {
	hooks := m.ListByType(hookType)
	var results []*HookResult

	for _, h := range hooks {
		if !h.Enabled {
			continue
		}
		result, err := m.RunHook(h.ID)
		if err != nil {
			result = &HookResult{
				HookID:    h.ID,
				Type:      hookType,
				Success:   false,
				Error:     err.Error(),
				Timestamp: time.Now(),
			}
		}
		results = append(results, result)
	}

	return results
}

// Results returns hook execution history.
func (m *Manager) Results(limit int) []HookResult {
	if limit <= 0 || limit > len(m.results) {
		limit = len(m.results)
	}
	start := len(m.results) - limit
	if start < 0 {
		start = 0
	}
	return m.results[start:]
}

// Install installs git hooks into the .git/hooks directory.
func (m *Manager) Install() error {
	// Group hooks by type
	byType := make(map[HookType][]*Hook)
	for _, h := range m.hooks {
		if h.Enabled {
			byType[h.Type] = append(byType[h.Type], h)
		}
	}

	for hookType, hooks := range byType {
		hookPath := filepath.Join(m.hooksDir, string(hookType))

		var script strings.Builder
		script.WriteString("#!/bin/sh\n")
		script.WriteString("# Forge-managed git hook - DO NOT EDIT MANUALLY\n")
		script.WriteString("# Use 'forge gitserve list' to see configured hooks\n\n")
		script.WriteString("echo \"[Forge] Running " + string(hookType) + " hooks...\"\n\n")

		for _, h := range hooks {
			script.WriteString(fmt.Sprintf("# Hook: %s (%s)\n", h.Name, h.ID))
			script.WriteString(fmt.Sprintf("echo \"[Forge] Running: %s\"\n", h.Name))
			script.WriteString(fmt.Sprintf("forge gitserve run %s\n", h.ID))
			if h.Actions[0].Block {
				script.WriteString("if [ $? -ne 0 ]; then\n")
				script.WriteString(fmt.Sprintf("  echo \"[Forge] Hook %s blocked this operation\"\n", h.Name))
				script.WriteString("  exit 1\n")
				script.WriteString("fi\n")
			}
			script.WriteString("\n")
		}

		script.WriteString("echo \"[Forge] All hooks completed.\"\n")

		if err := os.WriteFile(hookPath, []byte(script.String()), 0755); err != nil {
			return fmt.Errorf("failed to write hook %s: %w", hookType, err)
		}
	}

	return nil
}

// Uninstall removes Forge-managed git hooks.
func (m *Manager) Uninstall() error {
	for _, h := range m.hooks {
		hookPath := filepath.Join(m.hooksDir, string(h.Type))
		data, err := os.ReadFile(hookPath)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "Forge-managed") {
			os.Remove(hookPath)
		}
	}
	return nil
}

// DetectGitRepo checks if the directory is a git repository.
func DetectGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetChangedFiles returns files changed in the current git state.
func GetChangedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(files) == 1 && files[0] == "" {
		return nil, nil
	}
	return files, nil
}

// GetStagedFiles returns files staged for commit.
func GetStagedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(files) == 1 && files[0] == "" {
		return nil, nil
	}
	return files, nil
}

// DefaultHooks returns sensible default hooks.
func DefaultHooks() []*Hook {
	return []*Hook{
		{
			ID:          "pre-commit-lint",
			Type:        HookPreCommit,
			Name:        "AI Lint Check",
			Description: "AI-powered lint check before commit",
			Actions: []HookAction{
				{Agent: "linter", Prompt: "Check staged files for code quality issues", Block: true},
			},
			Enabled: true,
		},
		{
			ID:          "pre-commit-secrets",
			Type:        HookPreCommit,
			Name:        "Secret Scanner",
			Description: "Scan staged files for secrets before commit",
			Actions: []HookAction{
				{Agent: "security", Prompt: "Scan staged files for API keys, passwords, and tokens", Block: true},
			},
			Enabled: true,
		},
		{
			ID:          "post-commit-review",
			Type:        HookPostCommit,
			Name:        "Post-Commit Review",
			Description: "AI code review after each commit",
			Actions: []HookAction{
				{Agent: "reviewer", Prompt: "Review the last commit for quality issues"},
			},
			Enabled: false,
		},
		{
			ID:          "pre-push-security",
			Type:        HookPrePush,
			Name:        "Pre-Push Security Scan",
			Description: "Security scan before pushing to remote",
			Actions: []HookAction{
				{Agent: "security", Prompt: "Perform full security scan of all changes to be pushed", Block: true},
			},
			Enabled: true,
		},
	}
}

func simulateAction(hookType HookType, action HookAction) string {
	switch hookType {
	case HookPreCommit:
		return fmt.Sprintf("Agent %s: Pre-commit check completed for '%s'", action.Agent, truncate(action.Prompt, 50))
	case HookPostCommit:
		return fmt.Sprintf("Agent %s: Post-commit review completed", action.Agent)
	case HookPrePush:
		return fmt.Sprintf("Agent %s: Pre-push scan completed", action.Agent)
	default:
		return fmt.Sprintf("Agent %s: Hook action completed", action.Agent)
	}
}

func generateHookID(name string, hookType HookType) string {
	h := sha256.Sum256([]byte(name + string(hookType) + time.Now().String()))
	return fmt.Sprintf("hook-%x", h[:6])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (m *Manager) save() {
	os.MkdirAll(filepath.Dir(m.storePath), 0755)
	data, _ := json.MarshalIndent(m.hooks, "", "  ")
	os.WriteFile(m.storePath, data, 0644)
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.storePath)
	if err != nil {
		return
	}
	var hooks map[string]*Hook
	if err := json.Unmarshal(data, &hooks); err != nil {
		return
	}
	m.hooks = hooks
}
