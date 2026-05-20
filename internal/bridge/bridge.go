// Package bridge provides universal protocol translation between
// MCP, A2A, and ACP. Agents using any protocol can communicate
// with agents using any other protocol through this bridge.
//
// Every language. One conversation.
package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Protocol identifies an agent communication protocol.
type Protocol string

const (
	ProtocolMCP Protocol = "mcp"   // Model Context Protocol
	ProtocolA2A Protocol = "a2a"   // Agent-to-Agent Protocol
	ProtocolACP Protocol = "acp"   // Agent Client Protocol
)

// AllProtocols returns all supported protocols.
func AllProtocols() []Protocol {
	return []Protocol{ProtocolMCP, ProtocolA2A, ProtocolACP}
}

// Message represents a protocol-agnostic message.
type Message struct {
	ID          string                 `json:"id"`
	Source      Protocol               `json:"source"`
	Target      Protocol               `json:"target"`
	Type        string                 `json:"type"` // request, response, notification, error
	Method      string                 `json:"method"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Result      interface{}            `json:"result,omitempty"`
	Error       *MessageError          `json:"error,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Converted   bool                   `json:"converted"`
}

// MessageError represents an error in a message.
type MessageError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ConversionRule defines how to translate between protocols.
type ConversionRule struct {
	Source      Protocol `json:"source"`
	Target      Protocol `json:"target"`
	SourceType  string   `json:"source_type"` // request, response, notification
	MethodMap   string   `json:"method_map"`  // source_method -> target_method
	Transform   string   `json:"transform"`   // jq-style transform expression
	Description string  `json:"description"`
}

// Bridge translates messages between protocols.
type Bridge struct {
	mu    sync.RWMutex
	dir   string
	rules []ConversionRule
	log   []*Message
}

// NewBridge creates a protocol bridge.
func NewBridge(dir string) (*Bridge, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	b := &Bridge{
		dir:   dir,
		rules: DefaultConversionRules(),
		log:   make([]*Message, 0),
	}
	b.loadRules()
	return b, nil
}

// DefaultConversionRules returns built-in protocol mappings.
func DefaultConversionRules() []ConversionRule {
	return []ConversionRule{
		// MCP → A2A
		{Source: ProtocolMCP, Target: ProtocolA2A, SourceType: "request", MethodMap: "tools/list→agent/capabilities", Description: "MCP tool listing → A2A capability query"},
		{Source: ProtocolMCP, Target: ProtocolA2A, SourceType: "request", MethodMap: "tools/call→agent/execute", Description: "MCP tool call → A2A task execution"},
		{Source: ProtocolMCP, Target: ProtocolA2A, SourceType: "response", MethodMap: "tools/list→capabilities", Description: "MCP tool list response → A2A capabilities"},
		// A2A → MCP
		{Source: ProtocolA2A, Target: ProtocolMCP, SourceType: "request", MethodMap: "agent/capabilities→tools/list", Description: "A2A capability query → MCP tool listing"},
		{Source: ProtocolA2A, Target: ProtocolMCP, SourceType: "request", MethodMap: "agent/execute→tools/call", Description: "A2A task execution → MCP tool call"},
		// ACP → MCP
		{Source: ProtocolACP, Target: ProtocolMCP, SourceType: "request", MethodMap: "acp/task→tools/call", Description: "ACP task → MCP tool call"},
		{Source: ProtocolACP, Target: ProtocolMCP, SourceType: "notification", MethodMap: "acp/event→notifications/message", Description: "ACP event → MCP notification"},
		// MCP → ACP
		{Source: ProtocolMCP, Target: ProtocolACP, SourceType: "request", MethodMap: "tools/call→acp/task", Description: "MCP tool call → ACP task"},
		{Source: ProtocolMCP, Target: ProtocolACP, SourceType: "response", MethodMap: "tools/call→acp/result", Description: "MCP result → ACP task result"},
		// A2A ↔ ACP
		{Source: ProtocolA2A, Target: ProtocolACP, SourceType: "request", MethodMap: "agent/execute→acp/task", Description: "A2A execute → ACP task"},
		{Source: ProtocolACP, Target: ProtocolA2A, SourceType: "response", MethodMap: "acp/result→agent/result", Description: "ACP result → A2A result"},
	}
}

// Translate converts a message from one protocol to another.
func (b *Bridge) Translate(msg *Message) (*Message, error) {
	if msg.Source == msg.Target {
		return msg, nil
	}

	rule := b.findRule(msg.Source, msg.Target, msg.Type, msg.Method)
	if rule == nil {
		return nil, fmt.Errorf("no conversion rule: %s %s/%s → %s", msg.Source, msg.Type, msg.Method, msg.Target)
	}

	// Parse method mapping
	parts := strings.SplitN(rule.MethodMap, "→", 2)
	targetMethod := msg.Method
	if len(parts) == 2 {
		targetMethod = parts[1]
	}

	translated := &Message{
		ID:        msg.ID,
		Source:    msg.Source,
		Target:    msg.Target,
		Type:      msg.Type,
		Method:    targetMethod,
		Params:    make(map[string]interface{}),
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
		Converted: true,
	}

	// Copy params with protocol-specific adaptations
	for k, v := range msg.Params {
		translated.Params[k] = v
	}
	for k, v := range msg.Metadata {
		translated.Metadata[k] = v
	}
	translated.Metadata["bridge_rule"] = rule.MethodMap
	translated.Metadata["original_method"] = msg.Method

	// Protocol-specific param adaptation
	b.adaptParams(msg, translated)

	// Copy result/error
	translated.Result = msg.Result
	translated.Error = msg.Error

	// Log the translation
	b.mu.Lock()
	b.log = append(b.log, translated)
	if len(b.log) > 1000 {
		b.log = b.log[len(b.log)-500:]
	}
	b.mu.Unlock()

	return translated, nil
}

// adaptParams adapts parameters between protocol formats.
func (b *Bridge) adaptParams(src, dst *Message) {
	switch {
	case src.Source == ProtocolMCP && dst.Target == ProtocolA2A:
		// MCP uses "arguments" → A2A uses "input"
		if args, ok := src.Params["arguments"]; ok {
			dst.Params["input"] = args
			delete(dst.Params, "arguments")
		}
		if name, ok := src.Params["name"]; ok {
			dst.Params["tool"] = name
			delete(dst.Params, "name")
		}
	case src.Source == ProtocolA2A && dst.Target == ProtocolMCP:
		// A2A uses "input" → MCP uses "arguments"
		if input, ok := src.Params["input"]; ok {
			dst.Params["arguments"] = input
			delete(dst.Params, "input")
		}
		if tool, ok := src.Params["tool"]; ok {
			dst.Params["name"] = tool
			delete(dst.Params, "tool")
		}
	case src.Source == ProtocolACP && dst.Target == ProtocolMCP:
		if task, ok := src.Params["task"]; ok {
			dst.Params["name"] = task
			delete(dst.Params, "task")
		}
		if params, ok := src.Params["params"]; ok {
			dst.Params["arguments"] = params
			delete(dst.Params, "params")
		}
	case src.Source == ProtocolMCP && dst.Target == ProtocolACP:
		if name, ok := src.Params["name"]; ok {
			dst.Params["task"] = name
			delete(dst.Params, "name")
		}
		if args, ok := src.Params["arguments"]; ok {
			dst.Params["params"] = args
			delete(dst.Params, "arguments")
		}
	}
}

// findRule finds a matching conversion rule.
func (b *Bridge) findRule(source, target Protocol, msgType, method string) *ConversionRule {
	// First pass: exact match
	for _, r := range b.rules {
		if r.Source == source && r.Target == target && r.SourceType == msgType {
			parts := strings.SplitN(r.MethodMap, "→", 2)
			if len(parts) == 2 && parts[0] == method {
				return &r
			}
		}
	}
	// Second pass: prefix fallback
	for _, r := range b.rules {
		if r.Source == source && r.Target == target && r.SourceType == msgType {
			parts := strings.SplitN(r.MethodMap, "→", 2)
			if len(parts) == 2 && strings.HasPrefix(method, strings.Split(parts[0], "/")[0]+"/") {
				return &r
			}
		}
	}
	return nil
}

// GetLog returns recent translation log entries.
func (b *Bridge) GetLog(limit int) []*Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if limit <= 0 || limit > len(b.log) {
		limit = len(b.log)
	}
	start := len(b.log) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*Message, len(b.log[start:]))
	copy(result, b.log[start:])
	return result
}

// ListRules returns all conversion rules.
func (b *Bridge) ListRules() []ConversionRule {
	return b.rules
}

// AddRule adds a custom conversion rule.
func (b *Bridge) AddRule(rule ConversionRule) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rules = append(b.rules, rule)
	return b.saveRules()
}

// loadRules loads custom rules from disk.
func (b *Bridge) loadRules() {
	data, err := os.ReadFile(filepath.Join(b.dir, "rules.json"))
	if err != nil {
		return
	}
	var customRules []ConversionRule
	if err := json.Unmarshal(data, &customRules); err != nil {
		return
	}
	b.rules = append(b.rules, customRules...)
}

// saveRules persists custom rules.
func (b *Bridge) saveRules() error {
	// Only save rules that aren't defaults
	data, err := json.MarshalIndent(b.rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.dir, "rules.json"), data, 0o644)
}

// FormatMessage renders a message for display.
func FormatMessage(m *Message) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Message: %s\n", m.ID))
	sb.WriteString(fmt.Sprintf("  %s → %s\n", m.Source, m.Target))
	sb.WriteString(fmt.Sprintf("  Type:   %s\n", m.Type))
	sb.WriteString(fmt.Sprintf("  Method: %s\n", m.Method))
	if m.Converted {
		sb.WriteString("  [CONVERTED]\n")
	}
	if m.Error != nil {
		sb.WriteString(fmt.Sprintf("  Error:  %d %s\n", m.Error.Code, m.Error.Message))
	}
	sb.WriteString(fmt.Sprintf("  Time:   %s\n", m.Timestamp.Format(time.RFC3339)))
	return sb.String()
}

// FormatRule renders a conversion rule for display.
func FormatRule(r ConversionRule) string {
	return fmt.Sprintf("%s→%s [%s] %s — %s", r.Source, r.Target, r.SourceType, r.MethodMap, r.Description)
}
