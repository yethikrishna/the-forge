// Package suna provides access to Suna's 60+ skills from Forge agents.
// Skills are pre-built capabilities — web search, document processing,
// data analysis, API integrations — that agents invoke on demand.
package suna

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SkillCategory groups related skills.
type SkillCategory string

const (
	CategoryResearch     SkillCategory = "research"
	CategoryDataAnalysis SkillCategory = "data_analysis"
	CategoryWriting      SkillCategory = "writing"
	CategoryDevelopment  SkillCategory = "development"
	CategoryIntegration  SkillCategory = "integration"
	CategoryCommunication SkillCategory = "communication"
	CategoryMedia        SkillCategory = "media"
	CategoryFinance      SkillCategory = "finance"
	CategorySecurity     SkillCategory = "security"
	CategoryMarketing    SkillCategory = "marketing"
	CategoryLegal        SkillCategory = "legal"
)

// Skill represents a Suna skill available to Forge agents.
type Skill struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    SkillCategory  `json:"category"`
	Version     string         `json:"version"`
	Author      string         `json:"author"`
	Parameters  []SkillParam   `json:"parameters"`
	Tags        []string       `json:"tags"`
	Rating      float64        `json:"rating"`
	Installs    int            `json:"installs"`
	Verified    bool           `json:"verified"`
}

// SkillParam describes a parameter for a skill.
type SkillParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // string, number, boolean, array, object
	Required    bool   `json:"required"`
	Default     interface{} `json:"default"`
	Description string `json:"description"`
}

// SkillInvocation is a request to execute a skill.
type SkillInvocation struct {
	SkillID   string                 `json:"skill_id"`
	AgentID   string                 `json:"agent_id"`
	Division  string                 `json:"division"`
	Parameters map[string]interface{} `json:"parameters"`
	Timeout   int                    `json:"timeout"` // seconds
}

// SkillResult is the output of a skill execution.
type SkillResult struct {
	Success   bool                   `json:"success"`
	Output    interface{}            `json:"output"`
	Artifacts []SkillArtifact        `json:"artifacts"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	CostUSD   float64                `json:"cost_usd"`
}

// SkillArtifact is a file or data produced by a skill.
type SkillArtifact struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // file, url, data
	Content  string `json:"content,omitempty"`
	URL      string `json:"url,omitempty"`
	FilePath string `json:"file_path,omitempty"`
}

// SkillManager manages Suna skills for Forge agents.
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

// List returns all available skills.
func (sm *SkillManager) List(ctx context.Context) ([]*Skill, error) {
	var skills []*Skill
	if err := sm.bridge.GetJSON(ctx, "/api/skills", &skills); err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	sm.mu.Lock()
	for _, s := range skills {
		sm.skills[s.ID] = s
	}
	sm.mu.Unlock()
	return skills, nil
}

// Get returns a specific skill by ID.
func (sm *SkillManager) Get(ctx context.Context, id string) (*Skill, error) {
	sm.mu.RLock()
	if s, ok := sm.skills[id]; ok {
		sm.mu.RUnlock()
		return s, nil
	}
	sm.mu.RUnlock()

	var skill Skill
	if err := sm.bridge.GetJSON(ctx, "/api/skills/"+id, &skill); err != nil {
		return nil, fmt.Errorf("get skill %s: %w", id, err)
	}
	sm.mu.Lock()
	sm.skills[skill.ID] = &skill
	sm.mu.Unlock()
	return &skill, nil
}

// Search finds skills matching a query.
func (sm *SkillManager) Search(ctx context.Context, query string, category SkillCategory) ([]*Skill, error) {
	path := fmt.Sprintf("/api/skills/search?q=%s", query)
	if category != "" {
		path += fmt.Sprintf("&category=%s", category)
	}
	var skills []*Skill
	if err := sm.bridge.GetJSON(ctx, path, &skills); err != nil {
		return nil, fmt.Errorf("search skills: %w", err)
	}
	return skills, nil
}

// Invoke executes a skill and returns the result.
func (sm *SkillManager) Invoke(ctx context.Context, inv SkillInvocation) (*SkillResult, error) {
	start := time.Now()

	payload := map[string]interface{}{
		"skillId":    inv.SkillID,
		"agentId":    inv.AgentID,
		"division":   inv.Division,
		"parameters": inv.Parameters,
		"timeout":    inv.Timeout,
	}

	var result SkillResult
	if err := sm.bridge.PostJSON(ctx, "/api/skills/invoke", payload, &result); err != nil {
		return nil, fmt.Errorf("invoke skill %s: %w", inv.SkillID, err)
	}
	result.Duration = time.Since(start)
	return &result, nil
}

// Install makes a skill available to Forge agents.
func (sm *SkillManager) Install(ctx context.Context, skillID string) (*Skill, error) {
	var skill Skill
	if err := sm.bridge.PostJSON(ctx, "/api/skills/"+skillID+"/install", nil, &skill); err != nil {
		return nil, fmt.Errorf("install skill %s: %w", skillID, err)
	}
	sm.mu.Lock()
	sm.skills[skill.ID] = &skill
	sm.mu.Unlock()
	return &skill, nil
}

// Uninstall removes a skill.
func (sm *SkillManager) Uninstall(ctx context.Context, skillID string) error {
	if err := sm.bridge.DeleteJSON(ctx, "/api/skills/"+skillID); err != nil {
		return fmt.Errorf("uninstall skill %s: %w", skillID, err)
	}
	sm.mu.Lock()
	delete(sm.skills, skillID)
	sm.mu.Unlock()
	return nil
}

// ByCategory returns skills filtered by category.
func (sm *SkillManager) ByCategory(ctx context.Context, category SkillCategory) ([]*Skill, error) {
	var skills []*Skill
	path := fmt.Sprintf("/api/skills?category=%s", category)
	if err := sm.bridge.GetJSON(ctx, path, &skills); err != nil {
		return nil, fmt.Errorf("skills by category %s: %w", category, err)
	}
	return skills, nil
}

// ForDivision returns recommended skills for a specific division.
func (sm *SkillManager) ForDivision(division string) []SkillCategory {
	switch division {
	case "engineering":
		return []SkillCategory{CategoryDevelopment, CategorySecurity}
	case "research":
		return []SkillCategory{CategoryResearch, CategoryDataAnalysis}
	case "marketing":
		return []SkillCategory{CategoryMarketing, CategoryWriting, CategoryMedia}
	case "finance":
		return []SkillCategory{CategoryFinance, CategoryDataAnalysis}
	case "legal":
		return []SkillCategory{CategoryLegal, CategoryResearch}
	case "operations":
		return []SkillCategory{CategoryIntegration, CategorySecurity}
	default:
		return []SkillCategory{CategoryResearch, CategoryWriting}
	}
}
