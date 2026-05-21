// Package issues provides unified issue tracking integration for Forge.
// Supports Jira, Linear, and Notion as providers with a common interface.
// Agents can create, update, search, and sync issues across platforms.
package issues

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

// ProviderType identifies which issue tracker to use.
type ProviderType string

const (
	ProviderJira   ProviderType = "jira"
	ProviderLinear ProviderType = "linear"
	ProviderNotion ProviderType = "notion"
)

// IssueStatus represents the current state of an issue.
type IssueStatus string

const (
	StatusOpen       IssueStatus = "open"
	StatusInProgress IssueStatus = "in_progress"
	StatusDone       IssueStatus = "done"
	StatusClosed     IssueStatus = "closed"
	StatusCancelled  IssueStatus = "cancelled"
	StatusBacklog    IssueStatus = "backlog"
	StatusTodo       IssueStatus = "todo"
)

// Priority represents issue priority.
type Priority string

const (
	PriorityUrgent   Priority = "urgent"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
	PriorityNone     Priority = "none"
	PriorityCritical Priority = "critical"
)

// IssueType categorizes the kind of issue.
type IssueType string

const (
	TypeTask       IssueType = "task"
	TypeBug        IssueType = "bug"
	TypeStory      IssueType = "story"
	TypeEpic       IssueType = "epic"
	TypeSubTask    IssueType = "subtask"
	TypeIncident   IssueType = "incident"
	TypeImprovement IssueType = "improvement"
)

// Issue represents a unified issue across all providers.
type Issue struct {
	ID          string            `json:"id"`
	Key         string            `json:"key,omitempty"` // e.g., PROJ-123, FORGE-456
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Status      IssueStatus       `json:"status"`
	Priority    Priority          `json:"priority"`
	Type        IssueType         `json:"type"`
	Assignee    string            `json:"assignee,omitempty"`
	Reporter    string            `json:"reporter,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
	Project     string            `json:"project,omitempty"`
	ProjectKey  string            `json:"project_key,omitempty"`
	Provider    ProviderType      `json:"provider"`
	ExternalID  string            `json:"external_id"` // ID in the remote system
	ExternalURL string            `json:"external_url,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DueDate     *time.Time        `json:"due_date,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Comment represents a comment on an issue.
type Comment struct {
	ID        string    `json:"id"`
	IssueID   string    `json:"issue_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Project represents a project or team in an issue tracker.
type Project struct {
	ID          string       `json:"id"`
	Key         string       `json:"key,omitempty"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Provider    ProviderType `json:"provider"`
	ExternalID  string       `json:"external_id"`
}

// CreateIssueRequest holds fields for creating a new issue.
type CreateIssueRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Status      IssueStatus       `json:"status,omitempty"`
	Priority    Priority          `json:"priority,omitempty"`
	Type        IssueType         `json:"type,omitempty"`
	Assignee    string            `json:"assignee,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
	Project     string            `json:"project,omitempty"`
	ProjectKey  string            `json:"project_key,omitempty"`
	DueDate     *time.Time        `json:"due_date,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UpdateIssueRequest holds fields for updating an issue.
type UpdateIssueRequest struct {
	Title       *string           `json:"title,omitempty"`
	Description *string           `json:"description,omitempty"`
	Status      *IssueStatus      `json:"status,omitempty"`
	Priority    *Priority         `json:"priority,omitempty"`
	Type        *IssueType        `json:"type,omitempty"`
	Assignee    *string           `json:"assignee,omitempty"`
	Labels      *[]string         `json:"labels,omitempty"`
	DueDate     *time.Time        `json:"due_date,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SearchFilter defines search criteria.
type SearchFilter struct {
	Query      string       `json:"query,omitempty"`
	Status     []IssueStatus `json:"status,omitempty"`
	Priority   []Priority   `json:"priority,omitempty"`
	Type       []IssueType  `json:"type,omitempty"`
	Assignee   string       `json:"assignee,omitempty"`
	Project    string       `json:"project,omitempty"`
	Labels     []string     `json:"labels,omitempty"`
	Provider   ProviderType `json:"provider,omitempty"`
	Since      *time.Time   `json:"since,omitempty"`
	Limit      int          `json:"limit,omitempty"`
}

// ProviderConfig holds credentials for a single provider.
type ProviderConfig struct {
	Name      ProviderType `json:"name"`
	URL       string       `json:"url,omitempty"` // Base URL (Jira: https://domain.atlassian.net)
	Token     string       `json:"token"`
	Email     string       `json:"email,omitempty"` // Jira needs email
	ProjectID string       `json:"project_id,omitempty"`
	DatabaseID string      `json:"database_id,omitempty"` // Notion database ID
	TeamID    string       `json:"team_id,omitempty"`     // Linear team ID
	Enabled   bool         `json:"enabled"`
}

// Provider is the interface each tracker must implement.
type Provider interface {
	Type() ProviderType
	ListProjects() ([]Project, error)
	ListIssues(filter SearchFilter) ([]Issue, error)
	GetIssue(id string) (*Issue, error)
	CreateIssue(req CreateIssueRequest) (*Issue, error)
	UpdateIssue(id string, req UpdateIssueRequest) (*Issue, error)
	AddComment(id string, body string) (*Comment, error)
	ListComments(id string) ([]Comment, error)
	Search(filter SearchFilter) ([]Issue, error)
}

// Manager coordinates all issue providers.
type Manager struct {
	mu        sync.RWMutex
	dir       string
	providers map[ProviderType]Provider
	configs   map[ProviderType]*ProviderConfig
	cache     []Issue
	cacheTime time.Time
}

// NewManager creates a new issue manager backed by the given directory.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:       dir,
		providers: make(map[ProviderType]Provider),
		configs:   make(map[ProviderType]*ProviderConfig),
	}
}

// LoadConfigs reads provider configurations from disk.
func (m *Manager) LoadConfigs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	configPath := filepath.Join(m.dir, "providers.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read providers config: %w", err)
	}

	var configs []*ProviderConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("parse providers config: %w", err)
	}

	for _, cfg := range configs {
		m.configs[cfg.Name] = cfg
		if cfg.Enabled {
			p, err := m.createProvider(cfg)
			if err != nil {
				continue // skip broken configs
			}
			m.providers[cfg.Name] = p
		}
	}
	return nil
}

// SaveConfigs persists provider configurations to disk.
func (m *Manager) SaveConfigs() error {
	m.mu.RLock()
	configs := make([]*ProviderConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	m.mu.RUnlock()

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create issues dir: %w", err)
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal configs: %w", err)
	}

	configPath := filepath.Join(m.dir, "providers.json")
	return os.WriteFile(configPath, data, 0o644)
}

// AddProvider adds a new provider configuration and initializes it.
func (m *Manager) AddProvider(cfg *ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configs[cfg.Name] = cfg
	if cfg.Enabled {
		p, err := m.createProvider(cfg)
		if err != nil {
			return fmt.Errorf("create %s provider: %w", cfg.Name, err)
		}
		m.providers[cfg.Name] = p
	}
	return m.SaveConfigs()
}

// RemoveProvider removes a provider configuration.
func (m *Manager) RemoveProvider(name ProviderType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.providers, name)
	delete(m.configs, name)
	return m.SaveConfigs()
}

// GetProvider returns a specific provider by type.
func (m *Manager) GetProvider(name ProviderType) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[name]
	return p, ok
}

// GetProviders returns all active providers.
func (m *Manager) GetProviders() map[ProviderType]Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[ProviderType]Provider, len(m.providers))
	for k, v := range m.providers {
		result[k] = v
	}
	return result
}

// GetConfigs returns all provider configurations.
func (m *Manager) GetConfigs() []*ProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	configs := make([]*ProviderConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	sort.Slice(configs, func(i, j int) bool {
		return string(configs[i].Name) < string(configs[j].Name)
	})
	return configs
}

// ListIssues lists issues across all providers, optionally filtered.
func (m *Manager) ListIssues(filter SearchFilter) ([]Issue, error) {
	m.mu.RLock()
	providers := make(map[ProviderType]Provider, len(m.providers))
	for k, v := range m.providers {
		providers[k] = v
	}
	m.mu.RUnlock()

	var allIssues []Issue
	var errors []string

	for name, p := range providers {
		if filter.Provider != "" && filter.Provider != name {
			continue
		}
		issues, err := p.ListIssues(filter)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		allIssues = append(allIssues, issues...)
	}

	// Sort by updated_at descending
	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].UpdatedAt.After(allIssues[j].UpdatedAt)
	})

	if filter.Limit > 0 && len(allIssues) > filter.Limit {
		allIssues = allIssues[:filter.Limit]
	}

	if len(errors) > 0 && len(allIssues) == 0 {
		return nil, fmt.Errorf("list issues: %s", strings.Join(errors, "; "))
	}

	return allIssues, nil
}

// GetIssue retrieves a specific issue by provider and external ID.
func (m *Manager) GetIssue(provider ProviderType, id string) (*Issue, error) {
	m.mu.RLock()
	p, ok := m.providers[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	return p.GetIssue(id)
}

// CreateIssue creates a new issue in the specified provider.
func (m *Manager) CreateIssue(provider ProviderType, req CreateIssueRequest) (*Issue, error) {
	m.mu.RLock()
	p, ok := m.providers[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	return p.CreateIssue(req)
}

// UpdateIssue updates an existing issue.
func (m *Manager) UpdateIssue(provider ProviderType, id string, req UpdateIssueRequest) (*Issue, error) {
	m.mu.RLock()
	p, ok := m.providers[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	return p.UpdateIssue(id, req)
}

// AddComment adds a comment to an issue.
func (m *Manager) AddComment(provider ProviderType, issueID string, body string) (*Comment, error) {
	m.mu.RLock()
	p, ok := m.providers[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	return p.AddComment(issueID, body)
}

// ListComments lists comments on an issue.
func (m *Manager) ListComments(provider ProviderType, issueID string) ([]Comment, error) {
	m.mu.RLock()
	p, ok := m.providers[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	return p.ListComments(issueID)
}

// Search issues across all providers.
func (m *Manager) Search(filter SearchFilter) ([]Issue, error) {
	return m.ListIssues(filter)
}

// Sync fetches fresh issues from all providers and caches them.
func (m *Manager) Sync() ([]Issue, error) {
	issues, err := m.ListIssues(SearchFilter{})
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.cache = issues
	m.cacheTime = time.Now()
	m.mu.Unlock()

	// Persist cache
	cacheData, _ := json.MarshalIndent(issues, "", "  ")
	cachePath := filepath.Join(m.dir, "cache.json")
	_ = os.WriteFile(cachePath, cacheData, 0o644)

	return issues, nil
}

// GetCachedIssues returns the last synced issues.
func (m *Manager) GetCachedIssues() ([]Issue, time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	issues := make([]Issue, len(m.cache))
	copy(issues, m.cache)
	return issues, m.cacheTime
}

// ResolveIssueKey resolves a human-readable key (e.g., PROJ-123) to an Issue.
func (m *Manager) ResolveIssueKey(key string) (*Issue, error) {
	issues, err := m.ListIssues(SearchFilter{Query: key, Limit: 10})
	if err != nil {
		return nil, err
	}
	for i := range issues {
		if issues[i].Key == key || issues[i].ExternalID == key {
			return &issues[i], nil
		}
	}
	return nil, fmt.Errorf("issue %q not found", key)
}

// createProvider instantiates the correct provider from config.
func (m *Manager) createProvider(cfg *ProviderConfig) (Provider, error) {
	switch cfg.Name {
	case ProviderJira:
		return NewJiraProvider(cfg)
	case ProviderLinear:
		return NewLinearProvider(cfg)
	case ProviderNotion:
		return NewNotionProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Name)
	}
}

// FormatIssue returns a human-readable summary of an issue.
func FormatIssue(i *Issue) string {
	parts := []string{}
	if i.Key != "" {
		parts = append(parts, i.Key)
	}
	parts = append(parts, i.Title)
	status := string(i.Status)
	if status == "" {
		status = "unknown"
	}
	parts = append(parts, fmt.Sprintf("[%s]", status))
	if i.Assignee != "" {
		parts = append(parts, fmt.Sprintf("@%s", i.Assignee))
	}
	parts = append(parts, fmt.Sprintf("(%s)", i.Provider))
	return strings.Join(parts, " ")
}
