// Package capability provides an agent capability registry.
// Declare what agents can do, discover agents by capability,
// and route tasks to the most capable agent.
//
// Know what your agents can do. Route accordingly.
package capability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Level represents proficiency level.
type Level int

const (
	LevelNone     Level = 0
	LevelBasic    Level = 1
	LevelIntermediate Level = 2
	LevelAdvanced Level = 3
	LevelExpert   Level = 4
)

func (l Level) String() string {
	switch l {
	case LevelNone:
		return "none"
	case LevelBasic:
		return "basic"
	case LevelIntermediate:
		return "intermediate"
	case LevelAdvanced:
		return "advanced"
	case LevelExpert:
		return "expert"
	default:
		return "unknown"
	}
}

// ParseLevel converts a string to a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "basic":
		return LevelBasic
	case "intermediate":
		return LevelIntermediate
	case "advanced":
		return LevelAdvanced
	case "expert":
		return LevelExpert
	default:
		return LevelNone
	}
}

// Capability represents a single agent capability.
type Capability struct {
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Level       Level             `json:"level"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"` // success_rate, avg_time, etc.
}

// AgentCaps represents an agent's capabilities.
type AgentCaps struct {
	AgentID      string       `json:"agent_id"`
	AgentName    string       `json:"agent_name"`
	Model        string       `json:"model,omitempty"`
	Capabilities []Capability `json:"capabilities"`
	RegisteredAt time.Time    `json:"registered_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	IsActive     bool         `json:"is_active"`
}

// MatchResult holds the result of matching agents to a capability.
type MatchResult struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Level     Level  `json:"level"`
	Score     float64 `json:"score"` // weighted score (0-100)
}

// Registry manages agent capabilities.
type Registry struct {
	Dir    string
	agents map[string]*AgentCaps
}

// NewRegistry creates a capability registry.
func NewRegistry(dir string) *Registry {
	return &Registry{
		Dir:    dir,
		agents: make(map[string]*AgentCaps),
	}
}

// Register registers an agent's capabilities.
func (r *Registry) Register(caps AgentCaps) error {
	if caps.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}

	now := time.Now()
	if caps.RegisteredAt.IsZero() {
		caps.RegisteredAt = now
	}
	caps.UpdatedAt = now
	caps.IsActive = true

	r.agents[caps.AgentID] = &caps

	if r.Dir != "" {
		return r.saveToDisk(&caps)
	}
	return nil
}

// Deregister removes an agent from the registry.
func (r *Registry) Deregister(agentID string) error {
	if caps, ok := r.agents[agentID]; ok {
		caps.IsActive = false
		caps.UpdatedAt = time.Now()
		if r.Dir != "" {
			return r.saveToDisk(caps)
		}
	}
	return nil
}

// Get returns an agent's capabilities.
func (r *Registry) Get(agentID string) (*AgentCaps, bool) {
	caps, ok := r.agents[agentID]
	return caps, ok
}

// List returns all registered agents.
func (r *Registry) List(activeOnly bool) []*AgentCaps {
	var agents []*AgentCaps
	for _, caps := range r.agents {
		if activeOnly && !caps.IsActive {
			continue
		}
		agents = append(agents, caps)
	}
	sort.Slice(agents, func(i, k int) bool {
		return agents[i].AgentName < agents[k].AgentName
	})
	return agents
}

// FindByCapability finds agents with a specific capability.
func (r *Registry) FindByCapability(capName string, minLevel Level) []MatchResult {
	var results []MatchResult

	for _, caps := range r.agents {
		if !caps.IsActive {
			continue
		}

		for _, cap := range caps.Capabilities {
			if strings.EqualFold(cap.Name, capName) && cap.Level >= minLevel {
				score := float64(cap.Level) / float64(LevelExpert) * 100

				// Adjust score by success rate if available
				if sr, ok := cap.Metrics["success_rate"]; ok {
					score = score * 0.7 + sr * 100 * 0.3
				}

				results = append(results, MatchResult{
					AgentID:   caps.AgentID,
					AgentName: caps.AgentName,
					Level:     cap.Level,
					Score:     score,
				})
				break
			}
		}
	}

	sort.Slice(results, func(i, k int) bool {
		return results[i].Score > results[k].Score
	})

	return results
}

// FindByCategory finds agents with capabilities in a category.
func (r *Registry) FindByCategory(category string) []MatchResult {
	var results []MatchResult

	for _, caps := range r.agents {
		if !caps.IsActive {
			continue
		}

		for _, cap := range caps.Capabilities {
			if strings.EqualFold(cap.Category, category) {
				results = append(results, MatchResult{
					AgentID:   caps.AgentID,
					AgentName: caps.AgentName,
					Level:     cap.Level,
					Score:     float64(cap.Level) / float64(LevelExpert) * 100,
				})
				break
			}
		}
	}

	sort.Slice(results, func(i, k int) bool {
		return results[i].Score > results[k].Score
	})

	return results
}

// BestAgent returns the best agent for a capability.
func (r *Registry) BestAgent(capName string) (*MatchResult, error) {
	results := r.FindByCapability(capName, LevelBasic)
	if len(results) == 0 {
		return nil, fmt.Errorf("no agent found with capability %q", capName)
	}
	return &results[0], nil
}

// Load loads the registry from disk.
func (r *Registry) Load() error {
	if r.Dir == "" {
		return nil
	}

	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.Dir, e.Name()))
		if err != nil {
			continue
		}
		var caps AgentCaps
		if err := json.Unmarshal(data, &caps); err != nil {
			continue
		}
		r.agents[caps.AgentID] = &caps
	}

	return nil
}

func (r *Registry) saveToDisk(caps *AgentCaps) error {
	os.MkdirAll(r.Dir, 0o755)
	data, err := json.MarshalIndent(caps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.Dir, caps.AgentID+".json"), data, 0o644)
}

// FormatCaps renders agent capabilities for display.
func FormatCaps(caps *AgentCaps) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%s)\n", caps.AgentName, caps.AgentID))
	if caps.Model != "" {
		sb.WriteString(fmt.Sprintf("  Model: %s\n", caps.Model))
	}
	sb.WriteString(fmt.Sprintf("  Active: %v | Registered: %s\n", caps.IsActive, caps.RegisteredAt.Format("2006-01-02")))
	sb.WriteString("  Capabilities:\n")
	for _, cap := range caps.Capabilities {
		sb.WriteString(fmt.Sprintf("    %-20s %-12s [%s] %s\n", cap.Name, cap.Category, cap.Level, cap.Description))
	}
	return sb.String()
}

// FormatMatchResult renders match results for display.
func FormatMatchResult(results []MatchResult) string {
	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("  %-20s %-15s level: %-12s score: %.0f\n", r.AgentName, r.AgentID, r.Level, r.Score))
	}
	return sb.String()
}
