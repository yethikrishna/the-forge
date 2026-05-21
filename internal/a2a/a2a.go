// Package a2a implements the Agent-to-Agent (A2A) protocol for inter-framework communication.
// When forges speak to forges — the A2A protocol bridges the gap.
package a2a

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Protocol version
const ProtocolVersion = "0.1.0"

// MessageType represents the type of A2A message.
type MessageType string

const (
	MessageTaskRequest        MessageType = "task_request"
	MessageTaskResponse       MessageType = "task_response"
	MessageCapabilityQuery    MessageType = "capability_query"
	MessageCapabilityResponse MessageType = "capability_response"
	MessageHeartbeat          MessageType = "heartbeat"
	MessageError              MessageType = "error"
	MessageCancel             MessageType = "cancel"
)

// AgentCard describes an agent's capabilities (A2A discovery).
type AgentCard struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Endpoint     string            `json:"endpoint"`
	Capabilities []Capability      `json:"capabilities"`
	AuthSchemes  []string          `json:"auth_schemes"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	RegisteredAt time.Time         `json:"registered_at"`
}

// Capability describes what an agent can do.
type Capability struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	InputSchema  string   `json:"input_schema,omitempty"` // JSON Schema URI
	OutputSchema string   `json:"output_schema,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// Message is an A2A protocol message.
type Message struct {
	ID            string          `json:"id"`
	Type          MessageType     `json:"type"`
	From          string          `json:"from"`
	To            string          `json:"to"`
	Protocol      string          `json:"protocol"`
	Timestamp     time.Time       `json:"timestamp"`
	Payload       json.RawMessage `json:"payload"`
	ReplyTo       string          `json:"reply_to,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	TTL           int             `json:"ttl,omitempty"` // seconds
}

// TaskRequest is a request to perform a task.
type TaskRequest struct {
	TaskID      string            `json:"task_id"`
	Capability  string            `json:"capability"`
	Input       json.RawMessage   `json:"input"`
	Priority    int               `json:"priority"`
	Deadline    *time.Time        `json:"deadline,omitempty"`
	CallbackURL string            `json:"callback_url,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TaskResponse is the response to a task.
type TaskResponse struct {
	TaskID     string          `json:"task_id"`
	Status     string          `json:"status"` // pending, running, completed, failed
	Output     json.RawMessage `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
	Progress   float64         `json:"progress"`
	DurationMs int64           `json:"duration_ms"`
	Cost       float64         `json:"cost,omitempty"`
}

// CapabilityQuery requests capability information.
type CapabilityQuery struct {
	Query    string   `json:"query"`
	Tags     []string `json:"tags,omitempty"`
	FullCard bool     `json:"full_card"`
}

// CapabilityResponse responds with capabilities.
type CapabilityResponse struct {
	Agents     []AgentCard `json:"agents"`
	TotalCount int         `json:"total_count"`
}

// A2AServer implements the A2A protocol server.
type A2AServer struct {
	agentCard AgentCard
	registry  map[string]*AgentCard
	handlers  map[MessageType]MessageHandler
	inbox     chan Message
	storeDir  string
	mu        sync.RWMutex
}

// MessageHandler handles incoming A2A messages.
type MessageHandler func(msg Message) (Message, error)

// NewA2AServer creates an A2A server.
func NewA2AServer(card AgentCard) *A2AServer {
	srv := &A2AServer{
		agentCard: card,
		registry:  make(map[string]*AgentCard),
		handlers:  make(map[MessageType]MessageHandler),
		inbox:     make(chan Message, 100),
	}

	// Register default handlers
	srv.handlers[MessageTaskRequest] = srv.handleTaskRequest
	srv.handlers[MessageCapabilityQuery] = srv.handleCapabilityQuery
	srv.handlers[MessageHeartbeat] = srv.handleHeartbeat

	return srv
}

// RegisterHandler registers a custom message handler.
func (s *A2AServer) RegisterHandler(msgType MessageType, handler MessageHandler) {
	s.handlers[msgType] = handler
}

// RegisterAgent registers a remote agent.
func (s *A2AServer) RegisterAgent(card AgentCard) {
	s.mu.Lock()
	defer s.mu.Unlock()
	card.RegisteredAt = time.Now().UTC()
	s.registry[card.ID] = &card
}

// UnregisterAgent removes a remote agent.
func (s *A2AServer) UnregisterAgent(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.registry, id)
}

// ListAgents returns all registered agents.
func (s *A2AServer) ListAgents() []*AgentCard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var agents []*AgentCard
	for _, a := range s.registry {
		agents = append(agents, a)
	}
	return agents
}

// FindByCapability finds agents with a specific capability.
func (s *A2AServer) FindByCapability(capability string) []*AgentCard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var agents []*AgentCard
	for _, a := range s.registry {
		for _, c := range a.Capabilities {
			if c.ID == capability || c.Name == capability {
				agents = append(agents, a)
				break
			}
			for _, tag := range c.Tags {
				if tag == capability {
					agents = append(agents, a)
					break
				}
			}
		}
	}
	return agents
}

// FindByTag finds agents with capabilities matching a tag.
func (s *A2AServer) FindByTag(tag string) []*AgentCard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var agents []*AgentCard
	for _, a := range s.registry {
		for _, c := range a.Capabilities {
			for _, t := range c.Tags {
				if t == tag {
					agents = append(agents, a)
					break
				}
			}
		}
	}
	return agents
}

// SendMessage sends an A2A message to a remote agent.
func (s *A2AServer) SendMessage(targetEndpoint string, msg Message) (*Message, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	resp, err := http.Post(targetEndpoint, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var response Message
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &response, nil
}

// NewMessage creates a new A2A message.
func (s *A2AServer) NewMessage(msgType MessageType, to string, payload interface{}) (Message, error) {
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}

	return Message{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Type:      msgType,
		From:      s.agentCard.ID,
		To:        to,
		Protocol:  "a2a/" + ProtocolVersion,
		Timestamp: time.Now().UTC(),
		Payload:   payloadData,
	}, nil
}

// HandleMessage processes an incoming message.
func (s *A2AServer) HandleMessage(msg Message) (Message, error) {
	handler, ok := s.handlers[msg.Type]
	if !ok {
		errPayload, _ := json.Marshal(map[string]string{"error": "unsupported message type"})
		return Message{
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Type:      MessageError,
			From:      s.agentCard.ID,
			To:        msg.From,
			Protocol:  "a2a/" + ProtocolVersion,
			Timestamp: time.Now().UTC(),
			Payload:   errPayload,
		}, nil
	}
	return handler(msg)
}

// ServeHTTP implements http.Handler for the A2A endpoint.
func (s *A2AServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := s.HandleMessage(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetCard returns the agent's card.
func (s *A2AServer) GetCard() AgentCard {
	return s.agentCard
}

// Stats returns A2A server statistics.
func (s *A2AServer) Stats() A2AStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return A2AStats{
		AgentID:          s.agentCard.ID,
		RegisteredAgents: len(s.registry),
		Capabilities:     len(s.agentCard.Capabilities),
		ProtocolVersion:  ProtocolVersion,
	}
}

// A2AStats holds A2A statistics.
type A2AStats struct {
	AgentID          string `json:"agent_id"`
	RegisteredAgents int    `json:"registered_agents"`
	Capabilities     int    `json:"capabilities"`
	ProtocolVersion  string `json:"protocol_version"`
}

// Default handlers

func (s *A2AServer) handleTaskRequest(msg Message) (Message, error) {
	var req TaskRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return s.errorResponse(msg, "invalid task request payload")
	}

	// Default: return pending status
	resp := TaskResponse{
		TaskID:   req.TaskID,
		Status:   "pending",
		Progress: 0,
	}
	return s.NewMessage(MessageTaskResponse, msg.From, resp)
}

func (s *A2AServer) handleCapabilityQuery(msg Message) (Message, error) {
	var query CapabilityQuery
	if err := json.Unmarshal(msg.Payload, &query); err != nil {
		return s.errorResponse(msg, "invalid capability query")
	}

	agents := s.ListAgents()
	if len(query.Tags) > 0 {
		agents = s.FindByTag(query.Tags[0])
	}

	// Convert to non-pointer slice
	resultAgents := make([]AgentCard, len(agents))
	for i, a := range agents {
		resultAgents[i] = *a
	}

	resp := CapabilityResponse{
		Agents:     resultAgents,
		TotalCount: len(agents),
	}
	return s.NewMessage(MessageCapabilityResponse, msg.From, resp)
}

func (s *A2AServer) handleHeartbeat(msg Message) (Message, error) {
	return s.NewMessage(MessageHeartbeat, msg.From, map[string]string{"status": "alive"})
}

func (s *A2AServer) errorResponse(msg Message, errMsg string) (Message, error) {
	payload, _ := json.Marshal(map[string]string{"error": errMsg})
	return Message{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Type:      MessageError,
		From:      s.agentCard.ID,
		To:        msg.From,
		Protocol:  "a2a/" + ProtocolVersion,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}, nil
}

// FormatAgentCard renders an agent card.
func FormatAgentCard(card *AgentCard) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent: %s (%s)\n", card.Name, card.ID))
	sb.WriteString(fmt.Sprintf("  Description: %s\n", card.Description))
	sb.WriteString(fmt.Sprintf("  Endpoint:    %s\n", card.Endpoint))
	sb.WriteString(fmt.Sprintf("  Capabilities: %d\n", len(card.Capabilities)))
	for _, c := range card.Capabilities {
		tags := strings.Join(c.Tags, ", ")
		if tags != "" {
			tags = " [" + tags + "]"
		}
		sb.WriteString(fmt.Sprintf("    %-20s %s%s\n", c.ID, c.Name, tags))
	}
	return sb.String()
}

// FormatStats renders A2A stats.
func FormatStats(stats A2AStats) string {
	return fmt.Sprintf("A2A Protocol Stats:\n  Agent:      %s\n  Registered: %d agents\n  Capabilities: %d\n  Protocol:   %s\n",
		stats.AgentID, stats.RegisteredAgents, stats.Capabilities, stats.ProtocolVersion)
}
