// Package suna provides mobile access to the Forge org via the Suna mobile app.
// The mobile app (rebranded as Forge Mobile) gives org owners visibility
// and control from their phone — real-time status, approvals, and chat.
package suna

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MobileNotificationType represents the type of push notification.
type MobileNotificationType string

const (
	NotifAlert       MobileNotificationType = "alert"
	NotifApproval    MobileNotificationType = "approval"
	NotifStatus      MobileNotificationType = "status"
	NotifStandup     MobileNotificationType = "standup"
	NotifEscalation  MobileNotificationType = "escalation"
	NotifCost        MobileNotificationType = "cost"
)

// MobileNotification is a push notification to the mobile app.
type MobileNotification struct {
	ID        string                 `json:"id"`
	Type      MobileNotificationType `json:"type"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Priority  int                    `json:"priority"` // 0-5, 5=critical
	AgentID   string                 `json:"agent_id"`
	Division  string                 `json:"division"`
	ActionURL string                 `json:"action_url,omitempty"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	Read      bool                   `json:"read"`
}

// MobileApproval represents an approval request sent to mobile.
type MobileApproval struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	AgentID     string                 `json:"agent_id"`
	Division    string                 `json:"division"`
	RiskLevel   string                 `json:"risk_level"` // low, medium, high, critical
	Actions     []ApprovalAction       `json:"actions"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   time.Time              `json:"created_at"`
	RespondedAt *time.Time             `json:"responded_at"`
	Response    string                 `json:"response"`
}

// ApprovalAction represents a choice the user can make.
type ApprovalAction struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Style   string `json:"style"` // primary, danger, default
	Confirm string `json:"confirm,omitempty"`
}

// MobileManager manages mobile interactions.
type MobileManager struct {
	bridge *Bridge
	mu     sync.RWMutex
}

// NewMobileManager creates a new mobile manager.
func NewMobileManager(bridge *Bridge) *MobileManager {
	return &MobileManager{bridge: bridge}
}

// SendNotification pushes a notification to the mobile app.
func (mm *MobileManager) SendNotification(ctx context.Context, notif MobileNotification) error {
	if notif.Title == "" {
		return fmt.Errorf("notification title is required")
	}
	if notif.Type == "" {
		notif.Type = NotifStatus
	}
	return mm.bridge.PostJSON(ctx, "/api/mobile/notify", notif, nil)
}

// RequestApproval sends an approval request to the mobile app.
func (mm *MobileManager) RequestApproval(ctx context.Context, approval MobileApproval) (string, error) {
	if approval.Title == "" {
		return "", fmt.Errorf("approval title is required")
	}
	if len(approval.Actions) == 0 {
		approval.Actions = []ApprovalAction{
			{ID: "approve", Label: "Approve", Style: "primary"},
			{ID: "reject", Label: "Reject", Style: "danger"},
		}
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := mm.bridge.PostJSON(ctx, "/api/mobile/approval", approval, &result); err != nil {
		return "", fmt.Errorf("request approval: %w", err)
	}
	return result.ID, nil
}

// GetApprovalStatus checks if an approval has been responded to.
func (mm *MobileManager) GetApprovalStatus(ctx context.Context, id string) (*MobileApproval, error) {
	var approval MobileApproval
	if err := mm.bridge.GetJSON(ctx, "/api/mobile/approval/"+id, &approval); err != nil {
		return nil, fmt.Errorf("get approval %s: %w", id, err)
	}
	return &approval, nil
}

// ListNotifications returns recent notifications.
func (mm *MobileManager) ListNotifications(ctx context.Context, limit int) ([]*MobileNotification, error) {
	if limit <= 0 {
		limit = 20
	}
	var notifs []*MobileNotification
	path := fmt.Sprintf("/api/mobile/notifications?limit=%d", limit)
	if err := mm.bridge.GetJSON(ctx, path, &notifs); err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return notifs, nil
}

// MarkRead marks a notification as read.
func (mm *MobileManager) MarkRead(ctx context.Context, id string) error {
	return mm.bridge.PostJSON(ctx, fmt.Sprintf("/api/mobile/notifications/%s/read", id), nil, nil)
}

// SendQuickStatus pushes a brief org status update to mobile.
func (mm *MobileManager) SendQuickStatus(ctx context.Context, summary string) error {
	return mm.SendNotification(ctx, MobileNotification{
		Type:     NotifStatus,
		Title:    "Forge Org Status",
		Body:     summary,
		Priority: 1,
	})
}

// SendEscalation pushes an urgent escalation to mobile.
func (mm *MobileManager) SendEscalation(ctx context.Context, agentID, message string) error {
	return mm.SendNotification(ctx, MobileNotification{
		Type:     NotifEscalation,
		Title:    "Escalation Required",
		Body:     message,
		Priority: 5,
		AgentID:  agentID,
	})
}

// SendCostAlert pushes a cost threshold alert.
func (mm *MobileManager) SendCostAlert(ctx context.Context, division string, currentSpend, limit float64) error {
	return mm.SendNotification(ctx, MobileNotification{
		Type:     NotifCost,
		Title:    "Cost Alert",
		Body:     fmt.Sprintf("%s division has spent $%.2f of $%.2f limit", division, currentSpend, limit),
		Priority: 3,
		Division: division,
	})
}
