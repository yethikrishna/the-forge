// Package suna provides access to Suna's 3000+ integrations.
// Forge agents connect to external services — SaaS, APIs, databases —
// through Suna's integration layer, avoiding the need to build each
// integration from scratch.
package suna

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// IntegrationStatus represents the connection status of an integration.
type IntegrationStatus string

const (
	IntConnected    IntegrationStatus = "connected"
	IntDisconnected IntegrationStatus = "disconnected"
	IntError        IntegrationStatus = "error"
	IntPending      IntegrationStatus = "pending"
)

// Integration represents an external service integration.
type Integration struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"` // crm, devops, storage, communication, etc.
	Provider    string            `json:"provider"` // github, slack, aws, google, etc.
	Description string            `json:"description"`
	Status      IntegrationStatus `json:"status"`
	AuthType    string            `json:"auth_type"` // oauth2, api_key, basic, token
	Scopes      []string          `json:"scopes"`
	LastSync    time.Time         `json:"last_sync"`
	Config      map[string]string `json:"config"`
	WebhookURL  string            `json:"webhook_url,omitempty"`
}

// IntegrationAction represents an action that can be performed on an integration.
type IntegrationAction struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Method      string                 `json:"method"` // GET, POST, PUT, DELETE
	Path        string                 `json:"path"`
	Parameters  []IntegrationParam     `json:"parameters"`
}

// IntegrationParam describes a parameter for an integration action.
type IntegrationParam struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
}

// IntegrationManager manages external integrations via Suna.
type IntegrationManager struct {
	bridge        *Bridge
	mu            sync.RWMutex
	integrations  map[string]*Integration
}

// NewIntegrationManager creates a new integration manager.
func NewIntegrationManager(bridge *Bridge) *IntegrationManager {
	return &IntegrationManager{
		bridge:       bridge,
		integrations: make(map[string]*Integration),
	}
}

// List returns all configured integrations.
func (im *IntegrationManager) List(ctx context.Context) ([]*Integration, error) {
	var integrations []*Integration
	if err := im.bridge.GetJSON(ctx, "/api/integrations", &integrations); err != nil {
		return nil, fmt.Errorf("list integrations: %w", err)
	}
	im.mu.Lock()
	for _, i := range integrations {
		im.integrations[i.ID] = i
	}
	im.mu.Unlock()
	return integrations, nil
}

// Get returns a specific integration by ID.
func (im *IntegrationManager) Get(ctx context.Context, id string) (*Integration, error) {
	im.mu.RLock()
	if i, ok := im.integrations[id]; ok {
		im.mu.RUnlock()
		return i, nil
	}
	im.mu.RUnlock()

	var integ Integration
	if err := im.bridge.GetJSON(ctx, "/api/integrations/"+id, &integ); err != nil {
		return nil, fmt.Errorf("get integration %s: %w", id, err)
	}
	im.mu.Lock()
	im.integrations[integ.ID] = &integ
	im.mu.Unlock()
	return &integ, nil
}

// Connect establishes a connection to an external service.
func (im *IntegrationManager) Connect(ctx context.Context, provider string, config map[string]string) (*Integration, error) {
	payload := map[string]interface{}{
		"provider": provider,
		"config":   config,
	}
	var integ Integration
	if err := im.bridge.PostJSON(ctx, "/api/integrations/connect", payload, &integ); err != nil {
		return nil, fmt.Errorf("connect %s: %w", provider, err)
	}
	im.mu.Lock()
	im.integrations[integ.ID] = &integ
	im.mu.Unlock()
	return &integ, nil
}

// Disconnect removes a connection to an external service.
func (im *IntegrationManager) Disconnect(ctx context.Context, id string) error {
	if err := im.bridge.DeleteJSON(ctx, "/api/integrations/"+id); err != nil {
		return fmt.Errorf("disconnect %s: %w", id, err)
	}
	im.mu.Lock()
	if integ, ok := im.integrations[id]; ok {
		integ.Status = IntDisconnected
	}
	im.mu.Unlock()
	return nil
}

// Execute performs an action on a connected integration.
func (im *IntegrationManager) Execute(ctx context.Context, integrationID, action string, params map[string]interface{}) (interface{}, error) {
	payload := map[string]interface{}{
		"integrationId": integrationID,
		"action":        action,
		"parameters":    params,
	}
	var result interface{}
	if err := im.bridge.PostJSON(ctx, "/api/integrations/execute", payload, &result); err != nil {
		return nil, fmt.Errorf("execute %s/%s: %w", integrationID, action, err)
	}
	return result, nil
}

// ListActions returns all available actions for an integration.
func (im *IntegrationManager) ListActions(ctx context.Context, integrationID string) ([]IntegrationAction, error) {
	var actions []IntegrationAction
	path := fmt.Sprintf("/api/integrations/%s/actions", integrationID)
	if err := im.bridge.GetJSON(ctx, path, &actions); err != nil {
		return nil, fmt.Errorf("list actions for %s: %w", integrationID, err)
	}
	return actions, nil
}

// IsConnected checks if a specific provider is connected.
func (im *IntegrationManager) IsConnected(ctx context.Context, provider string) bool {
	integrations, err := im.List(ctx)
	if err != nil {
		return false
	}
	for _, i := range integrations {
		if i.Provider == provider && i.Status == IntConnected {
			return true
		}
	}
	return false
}

// Search finds integrations by name, category, or provider.
func (im *IntegrationManager) Search(ctx context.Context, query string) ([]*Integration, error) {
	path := fmt.Sprintf("/api/integrations/search?q=%s", query)
	var integrations []*Integration
	if err := im.bridge.GetJSON(ctx, path, &integrations); err != nil {
		return nil, fmt.Errorf("search integrations: %w", err)
	}
	return integrations, nil
}
