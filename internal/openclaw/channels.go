// Package openclaw provides channel-based communication via OpenClaw.
// Forge agents communicate through OpenClaw's channel integrations:
// Slack, Discord, Telegram, WhatsApp, and more.
//
// Division channels, DMs, broadcasts — all routed through OpenClaw channels.
package openclaw

import (
	"context"
	"fmt"
	"time"
)

// ChannelType represents the type of communication channel.
type ChannelType string

const (
	ChannelSlack    ChannelType = "slack"
	ChannelDiscord  ChannelType = "discord"
	ChannelTelegram ChannelType = "telegram"
	ChannelWhatsApp ChannelType = "whatsapp"
	ChannelSignal   ChannelType = "signal"
)

// Message represents a message sent or received on a channel.
type Message struct {
	ID        string            `json:"id"`
	Channel   ChannelType       `json:"channel"`
	ChannelID string            `json:"channel_id"`
	Target    string            `json:"target"`     // recipient: user ID, chat ID, phone
	SenderID  string            `json:"sender_id"`
	Content   string            `json:"content"`
	ThreadID  string            `json:"thread_id,omitempty"`
	ReplyTo   string            `json:"reply_to,omitempty"`
	MediaURL  string            `json:"media_url,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
}

// SendOptions configures how a message is sent.
type SendOptions struct {
	Channel      ChannelType `json:"channel"`
	Target       string      `json:"target"`
	ThreadID     string      `json:"thread_id,omitempty"`
	ReplyTo      string      `json:"reply_to,omitempty"`
	MediaPath    string      `json:"media_path,omitempty"`
	AsVoice      bool        `json:"as_voice,omitempty"`
	Silent       bool        `json:"silent,omitempty"`
	ForceDocument bool        `json:"force_document,omitempty"` // send image as document
}

// ChannelManager manages communication channels via OpenClaw.
type ChannelManager struct {
	bridge *Bridge
}

// NewChannelManager creates a new channel manager.
func NewChannelManager(bridge *Bridge) *ChannelManager {
	return &ChannelManager{bridge: bridge}
}

// Send delivers a message through an OpenClaw channel.
func (cm *ChannelManager) Send(ctx context.Context, opts SendOptions) (*Message, error) {
	if opts.Channel == "" {
		return nil, fmt.Errorf("channel type is required")
	}
	if opts.Target == "" {
		return nil, fmt.Errorf("target is required")
	}

	payload := map[string]interface{}{
		"action":        "send",
		"channel":       opts.Channel,
		"target":        opts.Target,
		"message":       "", // will be overridden by Content if set
		"threadId":      opts.ThreadID,
		"replyTo":       opts.ReplyTo,
		"filePath":      opts.MediaPath,
		"asVoice":       opts.AsVoice,
		"silent":        opts.Silent,
		"forceDocument": opts.ForceDocument,
	}

	var msg Message
	if err := cm.bridge.PostJSON(ctx, "/api/message", payload, &msg); err != nil {
		return nil, fmt.Errorf("send message via %s: %w", opts.Channel, err)
	}
	return &msg, nil
}

// Broadcast sends a message to multiple targets simultaneously.
func (cm *ChannelManager) Broadcast(ctx context.Context, channel ChannelType, targets []string, content string) ([]*Message, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one target is required")
	}

	payload := map[string]interface{}{
		"action":  "broadcast",
		"channel": channel,
		"targets": targets,
		"message": content,
	}

	var msgs []*Message
	if err := cm.bridge.PostJSON(ctx, "/api/message", payload, &msgs); err != nil {
		return nil, fmt.Errorf("broadcast via %s: %w", channel, err)
	}
	return msgs, nil
}

// React adds an emoji reaction to a message.
func (cm *ChannelManager) React(ctx context.Context, channel ChannelType, target, messageID, emoji string) error {
	payload := map[string]interface{}{
		"action":    "send",
		"channel":   channel,
		"target":    target,
		"messageId": messageID,
		"emoji":     emoji,
	}
	return cm.bridge.PostJSON(ctx, "/api/message", payload, nil)
}

// EditMessage edits an existing message.
func (cm *ChannelManager) EditMessage(ctx context.Context, channel ChannelType, target, messageID, newContent string) error {
	payload := map[string]interface{}{
		"action":    "send",
		"channel":   channel,
		"target":    target,
		"messageId": messageID,
		"text":      newContent,
	}
	return cm.bridge.PostJSON(ctx, "/api/message", payload, nil)
}

// DeleteMessage removes a message.
func (cm *ChannelManager) DeleteMessage(ctx context.Context, channel ChannelType, target, messageID string) error {
	return cm.bridge.Delete(ctx, fmt.Sprintf("/api/message/%s/%s/%s", channel, target, messageID))
}

// ChannelConnection represents a connected channel account.
type ChannelConnection struct {
	Channel   ChannelType `json:"channel"`
	Connected bool        `json:"connected"`
	AccountID string      `json:"account_id"`
	Label     string      `json:"label"`
}

// ListConnections returns all connected channel accounts.
func (cm *ChannelManager) ListConnections(ctx context.Context) ([]ChannelConnection, error) {
	var conns []ChannelConnection
	if err := cm.bridge.GetJSON(ctx, "/api/connections", &conns); err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	return conns, nil
}

// IsConnected checks if a specific channel is connected.
func (cm *ChannelManager) IsConnected(ctx context.Context, channel ChannelType) bool {
	conns, err := cm.ListConnections(ctx)
	if err != nil {
		return false
	}
	for _, c := range conns {
		if c.Channel == channel && c.Connected {
			return true
		}
	}
	return false
}

// DivisionChannel returns the channel target for a Forge division.
// Convention: each division gets its own channel (e.g., #forge-engineering on Slack).
func DivisionChannel(division string) string {
	return fmt.Sprintf("#forge-%s", division)
}
