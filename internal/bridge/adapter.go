package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Adapter connects a protocol endpoint to the bridge.
// Each adapter knows how to send/receive messages in its native protocol
// and convert them to/from the bridge's internal Message format.
type Adapter interface {
	// Protocol returns the protocol this adapter handles.
	Protocol() Protocol
	// Name returns a human-readable name for this adapter instance.
	Name() string
	// Send sends a translated message out on this adapter's protocol.
	Send(ctx context.Context, msg *Message) error
	// Receive returns a channel of incoming messages from this protocol.
	Receive() <-chan *Message
	// Close shuts down the adapter.
	Close() error
	// Status returns the adapter's current status.
	Status() AdapterStatus
}

// AdapterStatus represents the health of an adapter.
type AdapterStatus struct {
	Name      string    `json:"name"`
	Protocol  Protocol  `json:"protocol"`
	Connected bool      `json:"connected"`
	LastMsgAt time.Time `json:"last_msg_at,omitempty"`
	Sent      int64     `json:"sent"`
	Received  int64     `json:"received"`
	Errors    int64     `json:"errors"`
	Error     string    `json:"error,omitempty"`
}

// MCPAdapter adapts an MCP server connection.
type MCPAdapter struct {
	name     string
	addr     string
	incoming chan *Message
	mu       sync.Mutex
	status   AdapterStatus
}

// NewMCPAdapter creates an adapter for an MCP endpoint.
func NewMCPAdapter(name, addr string) *MCPAdapter {
	return &MCPAdapter{
		name:     name,
		addr:     addr,
		incoming: make(chan *Message, 64),
		status: AdapterStatus{
			Name:     name,
			Protocol: ProtocolMCP,
		},
	}
}

func (a *MCPAdapter) Protocol() Protocol { return ProtocolMCP }
func (a *MCPAdapter) Name() string       { return a.name }

func (a *MCPAdapter) Send(ctx context.Context, msg *Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Convert bridge message to MCP JSON-RPC format
	params, _ := json.Marshal(msg.Params)
	mcpReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"method":  msg.Method,
	}
	if len(params) > 0 && string(params) != "null" {
		mcpReq["params"] = msg.Params
	}

	_ = mcpReq // In production, send via HTTP/SSE to a.addr

	a.status.Sent++
	a.status.LastMsgAt = time.Now()
	return nil
}

func (a *MCPAdapter) Receive() <-chan *Message { return a.incoming }

func (a *MCPAdapter) Close() error {
	close(a.incoming)
	a.status.Connected = false
	return nil
}

func (a *MCPAdapter) Status() AdapterStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// Inject allows pushing a message into the adapter from outside (for testing or forwarding).
func (a *MCPAdapter) Inject(msg *Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.incoming <- msg
	a.status.Received++
	a.status.LastMsgAt = time.Now()
	a.status.Connected = true
}

// A2AAdapter adapts an A2A (Agent-to-Agent) endpoint.
type A2AAdapter struct {
	name     string
	addr     string
	incoming chan *Message
	mu       sync.Mutex
	status   AdapterStatus
}

// NewA2AAdapter creates an adapter for an A2A endpoint.
func NewA2AAdapter(name, addr string) *A2AAdapter {
	return &A2AAdapter{
		name:     name,
		addr:     addr,
		incoming: make(chan *Message, 64),
		status: AdapterStatus{
			Name:     name,
			Protocol: ProtocolA2A,
		},
	}
}

func (a *A2AAdapter) Protocol() Protocol { return ProtocolA2A }
func (a *A2AAdapter) Name() string       { return a.name }

func (a *A2AAdapter) Send(ctx context.Context, msg *Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	// A2A uses HTTP with JSON body
	_ = a.addr // In production, POST to a.addr
	a.status.Sent++
	a.status.LastMsgAt = time.Now()
	return nil
}

func (a *A2AAdapter) Receive() <-chan *Message { return a.incoming }

func (a *A2AAdapter) Close() error {
	close(a.incoming)
	a.status.Connected = false
	return nil
}

func (a *A2AAdapter) Status() AdapterStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// Inject pushes a message into the adapter.
func (a *A2AAdapter) Inject(msg *Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.incoming <- msg
	a.status.Received++
	a.status.LastMsgAt = time.Now()
	a.status.Connected = true
}

// ACPAdapter adapts an ACP endpoint.
type ACPAdapter struct {
	name     string
	addr     string
	incoming chan *Message
	mu       sync.Mutex
	status   AdapterStatus
}

// NewACPAdapter creates an adapter for an ACP endpoint.
func NewACPAdapter(name, addr string) *ACPAdapter {
	return &ACPAdapter{
		name:     name,
		addr:     addr,
		incoming: make(chan *Message, 64),
		status: AdapterStatus{
			Name:     name,
			Protocol: ProtocolACP,
		},
	}
}

func (a *ACPAdapter) Protocol() Protocol { return ProtocolACP }
func (a *ACPAdapter) Name() string       { return a.name }

func (a *ACPAdapter) Send(ctx context.Context, msg *Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	// ACP uses HTTP REST
	_ = a.addr
	a.status.Sent++
	a.status.LastMsgAt = time.Now()
	return nil
}

func (a *ACPAdapter) Receive() <-chan *Message { return a.incoming }

func (a *ACPAdapter) Close() error {
	close(a.incoming)
	a.status.Connected = false
	return nil
}

func (a *ACPAdapter) Status() AdapterStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// Inject pushes a message into the adapter.
func (a *ACPAdapter) Inject(msg *Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.incoming <- msg
	a.status.Received++
	a.status.LastMsgAt = time.Now()
	a.status.Connected = true
}

// FormatAdapterStatus renders adapter status for display.
func FormatAdapterStatus(s AdapterStatus) string {
	status := "disconnected"
	if s.Connected {
		status = "connected"
	}
	lastMsg := "never"
	if !s.LastMsgAt.IsZero() {
		lastMsg = s.LastMsgAt.Format(time.RFC3339)
	}
	return fmt.Sprintf("%-20s %-6s %-12s sent:%d recv:%d err:%d last:%s",
		s.Name, s.Protocol, status, s.Sent, s.Received, s.Errors, lastMsg)
}
