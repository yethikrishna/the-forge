// Package blink deploys agents as bots on Slack, Discord, Telegram, and GitHub.
// Each bot is a long-running process that receives messages, routes them to
// the agent, and returns responses.
package blink

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Platform identifies the messaging platform.
type Platform string

const (
	PlatformSlack    Platform = "slack"
	PlatformDiscord  Platform = "discord"
	PlatformTelegram Platform = "telegram"
	PlatformGitHub   Platform = "github"
	PlatformWebhook  Platform = "webhook"
)

// BotConfig configures a deployed bot.
type BotConfig struct {
	Name        string            `json:"name"`
	Platform    Platform          `json:"platform"`
	Token       string            `json:"token"`        // platform auth token
	WebhookURL  string            `json:"webhook_url"`  // incoming webhook
	Channel     string            `json:"channel"`      // default channel/room
	AgentID     string            `json:"agent_id"`     // linked agent
	Handler     string            `json:"handler"`      // handler script or "echo"
	AutoRespond bool              `json:"auto_respond"` // respond to all messages
	Mentions    bool              `json:"mentions"`     // only respond to @mentions
	Env         map[string]string `json:"env,omitempty"`
	RateLimit   int               `json:"rate_limit"` // max msgs/min
}

// BotState is the runtime state of a deployed bot.
type BotState string

const (
	BotDeployed  BotState = "deployed"
	BotRunning   BotState = "running"
	BotStopped   BotState = "stopped"
	BotFailed    BotState = "failed"
)

// Bot is a running or stopped deployed bot.
type Bot struct {
	ID          string     `json:"id"`
	Config      BotConfig  `json:"config"`
	State       BotState   `json:"state"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	StoppedAt   *time.Time `json:"stopped_at,omitempty"`
	MessageCount int       `json:"message_count"`
	ErrorCount  int        `json:"error_count"`
	LastMessage *time.Time `json:"last_message,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// IncomingMessage is a message received by the bot.
type IncomingMessage struct {
	From      string            `json:"from"`
	Channel   string            `json:"channel"`
	Content   string            `json:"content"`
	Mentions  []string          `json:"mentions,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// OutgoingMessage is a response from the bot.
type OutgoingMessage struct {
	Content  string `json:"content"`
	Channel  string `json:"channel,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
}

// BotHandler processes incoming messages and returns responses.
type BotHandler func(ctx context.Context, bot *Bot, msg IncomingMessage) (OutgoingMessage, error)

// BotManager manages deployed bots.
type BotManager struct {
	bots    map[string]*Bot
	handlers map[string]BotHandler
	mu      sync.RWMutex
}

// NewBotManager creates a bot manager.
func NewBotManager() *BotManager {
	return &BotManager{
		bots:     make(map[string]*Bot),
		handlers: make(map[string]BotHandler),
	}
}

// RegisterHandler registers a message handler for a handler name.
func (bm *BotManager) RegisterHandler(name string, handler BotHandler) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.handlers[name] = handler
}

// Deploy creates and starts a bot.
func (bm *BotManager) Deploy(config BotConfig) (*Bot, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("blink: name is required")
	}
	if config.Platform == "" {
		return nil, fmt.Errorf("blink: platform is required")
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	id := fmt.Sprintf("bot-%s-%d", config.Platform, time.Now().UnixNano())

	bot := &Bot{
		ID:     id,
		Config: config,
		State:  BotDeployed,
	}

	// Register default echo handler if none specified
	if _, ok := bm.handlers[config.Handler]; !ok {
		bm.handlers[config.Handler] = echoHandler
	}

	bm.bots[id] = bot
	return bot, nil
}

// Start starts a deployed bot.
func (bm *BotManager) Start(ctx context.Context, id string) (*Bot, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bot, ok := bm.bots[id]
	if !ok {
		return nil, fmt.Errorf("blink: bot %s not found", id)
	}

	if bot.State == BotRunning {
		return bot, nil
	}

	now := time.Now()
	bot.StartedAt = &now
	bot.State = BotRunning

	return bot, nil
}

// Stop stops a running bot.
func (bm *BotManager) Stop(id string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bot, ok := bm.bots[id]
	if !ok {
		return fmt.Errorf("blink: bot %s not found", id)
	}

	now := time.Now()
	bot.StoppedAt = &now
	bot.State = BotStopped
	return nil
}

// HandleMessage processes an incoming message for a bot.
func (bm *BotManager) HandleMessage(ctx context.Context, botID string, msg IncomingMessage) (OutgoingMessage, error) {
	bm.mu.RLock()
	bot, ok := bm.bots[botID]
	handler := bm.handlers[bot.Config.Handler]
	bm.mu.RUnlock()

	if !ok {
		return OutgoingMessage{}, fmt.Errorf("blink: bot %s not found", botID)
	}

	if bot.State != BotRunning {
		return OutgoingMessage{}, fmt.Errorf("blink: bot %s is not running", botID)
	}

	now := time.Now()
	bot.LastMessage = &now
	bot.MessageCount++

	resp, err := handler(ctx, bot, msg)
	if err != nil {
		bot.ErrorCount++
		return OutgoingMessage{}, err
	}

	return resp, nil
}

// Get retrieves a bot by ID.
func (bm *BotManager) Get(id string) (*Bot, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	bot, ok := bm.bots[id]
	if !ok {
		return nil, fmt.Errorf("blink: bot %s not found", id)
	}
	return bot, nil
}

// List returns all bots.
func (bm *BotManager) List() []*Bot {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	result := make([]*Bot, 0, len(bm.bots))
	for _, b := range bm.bots {
		result = append(result, b)
	}
	return result
}

// ListByPlatform returns bots filtered by platform.
func (bm *BotManager) ListByPlatform(platform Platform) []*Bot {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	var result []*Bot
	for _, b := range bm.bots {
		if b.Config.Platform == platform {
			result = append(result, b)
		}
	}
	return result
}

// Remove removes a bot.
func (bm *BotManager) Remove(id string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.bots, id)
}

// ServeHTTP implements http.Handler for webhook bots.
func (bm *BotManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var msg IncomingMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg.Timestamp = time.Now()

	// Find bot by path
	botID := r.URL.Path[1:] // strip leading /
	resp, err := bm.HandleMessage(r.Context(), botID, msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func echoHandler(ctx context.Context, bot *Bot, msg IncomingMessage) (OutgoingMessage, error) {
	return OutgoingMessage{
		Content: fmt.Sprintf("[%s] %s", bot.Config.Name, msg.Content),
		Channel: msg.Channel,
	}, nil
}
