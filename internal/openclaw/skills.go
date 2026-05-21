// Package openclaw provides skill access for Forge agents via the OpenClaw skill system.
// Agents can discover, install, and invoke skills — both OpenClaw built-in skills
// and custom Forge skills for organizational operations.
package openclaw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Skill represents an installed or available skill.
type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Location    string            `json:"location"` // filesystem path
	Source      string            `json:"source"`   // "builtin", "marketplace", "custom"
	Divisions   []string          `json:"divisions"` // which divisions use this
	Tags        []string          `json:"tags"`
	InstalledAt time.Time         `json:"installed_at"`
	Metadata    map[string]string `json:"metadata"`
}

// SkillInvocation is a request to execute a skill.
type SkillInvocation struct {
	SkillName string                 `json:"skill_name"`
	AgentID   string                 `json:"agent_id"`
	Division  string                 `json:"division"`
	Input     map[string]interface{} `json:"input"`
	Context   map[string]string      `json:"context"`
}

// SkillResult is the output of a skill execution.
type SkillResult struct {
	Success  bool                   `json:"success"`
	Output   string                 `json:"output"`
	Data     map[string]interface{} `json:"data"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration"`
	CostUSD  float64                `json:"cost_usd"`
}

// SkillManager manages skills for Forge agents.
type SkillManager struct {
	bridge *Bridge
	mu     sync.RWMutex
	skills map[string]*Skill
}

// NewSkillManager creates a new skill manager.
func NewSkillManager(bridge *Bridge) *SkillManager {
	return &SkillManager{
		bridge: bridge,
		skills: make(map[string]*Skill),
	}
}

// List returns all installed skills.
func (sm *SkillManager) List(ctx context.Context) ([]*Skill, error) {
	var skills []*Skill
	if err := sm.bridge.GetJSON(ctx, "/api/skills", &skills); err != nil {
		// Fall back to scanning the skills directory
		return sm.scanSkillDir()
	}
	sm.mu.Lock()
	for _, s := range skills {
		sm.skills[s.Name] = s
	}
	sm.mu.Unlock()
	return skills, nil
}

// Get returns a specific skill by name.
func (sm *SkillManager) Get(ctx context.Context, name string) (*Skill, error) {
	sm.mu.RLock()
	if s, ok := sm.skills[name]; ok {
		sm.mu.RUnlock()
		return s, nil
	}
	sm.mu.RUnlock()

	var skill Skill
	if err := sm.bridge.GetJSON(ctx, "/api/skills/"+name, &skill); err != nil {
		return nil, fmt.Errorf("skill %s not found: %w", name, err)
	}
	sm.mu.Lock()
	sm.skills[skill.Name] = &skill
	sm.mu.Unlock()
	return &skill, nil
}

// Install installs a skill from the marketplace or a URL.
func (sm *SkillManager) Install(ctx context.Context, name, source string) (*Skill, error) {
	payload := map[string]interface{}{
		"name":   name,
		"source": source,
	}
	var skill Skill
	if err := sm.bridge.PostJSON(ctx, "/api/skills/install", payload, &skill); err != nil {
		return nil, fmt.Errorf("install skill %s: %w", name, err)
	}
	sm.mu.Lock()
	sm.skills[skill.Name] = &skill
	sm.mu.Unlock()
	return &skill, nil
}

// Uninstall removes a skill.
func (sm *SkillManager) Uninstall(ctx context.Context, name string) error {
	if err := sm.bridge.Delete(ctx, "/api/skills/"+name); err != nil {
		return fmt.Errorf("uninstall skill %s: %w", name, err)
	}
	sm.mu.Lock()
	delete(sm.skills, name)
	sm.mu.Unlock()
	return nil
}

// Invoke executes a skill and returns the result.
func (sm *SkillManager) Invoke(ctx context.Context, inv SkillInvocation) (*SkillResult, error) {
	start := time.Now()

	payload := map[string]interface{}{
		"skillName": inv.SkillName,
		"agentId":   inv.AgentID,
		"division":  inv.Division,
		"input":     inv.Input,
		"context":   inv.Context,
	}

	var result SkillResult
	if err := sm.bridge.PostJSON(ctx, "/api/skills/invoke", payload, &result); err != nil {
		return nil, fmt.Errorf("invoke skill %s: %w", inv.SkillName, err)
	}

	result.Duration = time.Since(start)
	return &result, nil
}

// ReadSKILL reads a skill's SKILL.md definition file.
func (sm *SkillManager) ReadSKILL(ctx context.Context, name string) (string, error) {
	// Check local cache first
	sm.mu.RLock()
	if s, ok := sm.skills[name]; ok && s.Location != "" {
		sm.mu.RUnlock()
		data, err := os.ReadFile(filepath.Join(s.Location, "SKILL.md"))
		if err == nil {
			return string(data), nil
		}
	} else {
		sm.mu.RUnlock()
	}

	var result struct {
		Content string `json:"content"`
	}
	if err := sm.bridge.GetJSON(ctx, "/api/skills/"+name+"/definition", &result); err != nil {
		return "", fmt.Errorf("read skill %s definition: %w", name, err)
	}
	return result.Content, nil
}

// ForDivision returns skills relevant to a specific division.
func (sm *SkillManager) ForDivision(ctx context.Context, division string) ([]*Skill, error) {
	all, err := sm.List(ctx)
	if err != nil {
		return nil, err
	}
	var filtered []*Skill
	for _, s := range all {
		for _, d := range s.Divisions {
			if d == division {
				filtered = append(filtered, s)
				break
			}
		}
	}
	// If no division-specific skills, return all
	if len(filtered) == 0 {
		return all, nil
	}
	return filtered, nil
}

// scanSkillDir scans the skills directory for installed skills.
func (sm *SkillManager) scanSkillDir() ([]*Skill, error) {
	home, _ := os.UserHomeDir()
	skillDir := filepath.Join(home, ".openclaw", "skills")
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan skill dir: %w", err)
	}

	var skills []*Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillPath := filepath.Join(skillDir, e.Name())
		skillFile := filepath.Join(skillPath, "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}
		skill := &Skill{
			Name:        e.Name(),
			Location:    skillPath,
			Source:      "local",
			InstalledAt: time.Now(),
			Description: extractDescription(string(data)),
		}
		skills = append(skills, skill)
		sm.mu.Lock()
		sm.skills[skill.Name] = skill
		sm.mu.Unlock()
	}
	return skills, nil
}

// extractDescription gets a one-line description from SKILL.md content.
func extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}
