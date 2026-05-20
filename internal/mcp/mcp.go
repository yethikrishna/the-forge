// Package mcp provides a Model Context Protocol server implementation.
// The forge speaks many tongues to many tools.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Version is the MCP protocol version.
const Version = "2024-11-05"

// ServerInfo describes the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolHandler handles a tool call.
type ToolHandler func(ctx context.Context, args json.RawMessage) (string, error)

// Resource describes an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// ResourceHandler handles a resource read.
type ResourceHandler func(ctx context.Context, uri string) (string, string, error)

// Prompt describes an MCP prompt template.
type Prompt struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Arguments   []PromptArg  `json:"arguments,omitempty"`
}

// PromptArg is a prompt template argument.
type PromptArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// PromptHandler handles a prompt request.
type PromptHandler func(ctx context.Context, args map[string]string) ([]PromptMessage, error)

// PromptMessage is a message in a prompt response.
type PromptMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// TextContent is text content in a prompt message.
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Server is an MCP protocol server.
type Server struct {
	info     ServerInfo
	tools    map[string]Tool
	handlers map[string]ToolHandler
	resources map[string]Resource
	resHandlers map[string]ResourceHandler
	prompts  map[string]Prompt
	promptHandlers map[string]PromptHandler
	mu       sync.RWMutex
}

// NewServer creates a new MCP server.
func NewServer(name, version string) *Server {
	return &Server{
		info:     ServerInfo{Name: name, Version: version},
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
		resources: make(map[string]Resource),
		resHandlers: make(map[string]ResourceHandler),
		prompts:  make(map[string]Prompt),
		promptHandlers: make(map[string]PromptHandler),
	}
}

// RegisterTool registers an MCP tool.
func (s *Server) RegisterTool(name, description string, schema interface{}, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tools[name] = Tool{
		Name:        name,
		Description: description,
		InputSchema: schema,
	}
	s.handlers[name] = handler
}

// RegisterResource registers an MCP resource.
func (s *Server) RegisterResource(uri, name, description, mimeType string, handler ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resources[uri] = Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		MimeType:    mimeType,
	}
	s.resHandlers[uri] = handler
}

// RegisterPrompt registers an MCP prompt template.
func (s *Server) RegisterPrompt(name, description string, args []PromptArg, handler PromptHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prompts[name] = Prompt{
		Name:        name,
		Description: description,
		Arguments:   args,
	}
	s.promptHandlers[name] = handler
}

// JSONRPCRequest is a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HandleRequest handles a single JSON-RPC request.
func (s *Server) HandleRequest(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(ctx, req)
	case "prompts/list":
		return s.handlePromptsList(req)
	case "prompts/get":
		return s.handlePromptsGet(ctx, req)
	case "ping":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	default:
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": Version,
			"serverInfo":      s.info,
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
				"prompts":   map[string]interface{}{},
			},
		},
	}
}

func (s *Server) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "invalid params"},
		}
	}

	s.mu.RLock()
	handler, ok := s.handlers[params.Name]
	s.mu.RUnlock()

	if !ok {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: fmt.Sprintf("tool not found: %s", params.Name)},
		}
	}

	result, err := handler(ctx, params.Arguments)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type":  "text",
						"text":  fmt.Sprintf("Error: %v", err),
						"isError": true,
					},
				},
			},
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": result,
				},
			},
		},
	}
}

func (s *Server) handleResourcesList(req JSONRPCRequest) JSONRPCResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make([]Resource, 0, len(s.resources))
	for _, r := range s.resources {
		resources = append(resources, r)
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"resources": resources},
	}
}

func (s *Server) handleResourcesRead(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "invalid params"},
		}
	}

	s.mu.RLock()
	handler, ok := s.resHandlers[params.URI]
	s.mu.RUnlock()

	if !ok {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: fmt.Sprintf("resource not found: %s", params.URI)},
		}
	}

	text, mimeType, err := handler(ctx, params.URI)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32603, Message: err.Error()},
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"uri":      params.URI,
					"mimeType": mimeType,
					"text":     text,
				},
			},
		},
	}
}

func (s *Server) handlePromptsList(req JSONRPCRequest) JSONRPCResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prompts := make([]Prompt, 0, len(s.prompts))
	for _, p := range s.prompts {
		prompts = append(prompts, p)
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"prompts": prompts},
	}
}

func (s *Server) handlePromptsGet(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "invalid params"},
		}
	}

	s.mu.RLock()
	handler, ok := s.promptHandlers[params.Name]
	s.mu.RUnlock()

	if !ok {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: fmt.Sprintf("prompt not found: %s", params.Name)},
		}
	}

	messages, err := handler(ctx, params.Arguments)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32603, Message: err.Error()},
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"messages": messages,
		},
	}
}

// ServeStdio starts the MCP server on stdin/stdout.
func (s *Server) ServeStdio(ctx context.Context) error {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			continue
		}

		resp := s.HandleRequest(ctx, req)
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("mcp: encode response: %w", err)
		}
	}
}

// ServeHTTP starts the MCP server as an HTTP endpoint (SSE transport).
func (s *Server) ServeHTTP(addr string) error {
	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send server endpoint
		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		flusher.Flush()

		// Keep alive
		<-r.Context().Done()
	})

	// Message endpoint
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		resp := s.HandleRequest(r.Context(), req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	return http.ListenAndServe(addr, mux)
}

// RegisterForgeTools registers the standard Forge tools.
func (s *Server) RegisterForgeTools() {
	// forge/execute — Execute a command in a sandbox
	s.RegisterTool("forge/execute", "Execute a command in the forge sandbox",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{"type": "string", "description": "Command to execute"},
				"language": map[string]interface{}{"type": "string", "description": "Language runtime (bash, python, go)"},
			},
			"required": []string{"command"},
		},
		func(_ context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Command  string `json:"command"`
				Language string `json:"language"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("invalid args: %w", err)
			}
			return fmt.Sprintf("Executed [%s]: %s (simulated)", params.Language, params.Command), nil
		},
	)

	// forge/search — Search the codebase
	s.RegisterTool("forge/search", "Search the codebase semantically",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string", "description": "Search query"},
				"limit": map[string]interface{}{"type": "integer", "description": "Max results"},
			},
			"required": []string{"query"},
		},
		func(_ context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("invalid args: %w", err)
			}
			return fmt.Sprintf("Search results for %q (simulated)", params.Query), nil
		},
	)

	// forge/cost — Check LLM pricing
	s.RegisterTool("forge/cost", "Get LLM model pricing information",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"model": map[string]interface{}{"type": "string", "description": "Model name to look up"},
			},
		},
		func(_ context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("invalid args: %w", err)
			}
			if params.Model == "" {
				return "Use forge cost compare for full pricing comparison", nil
			}
			return fmt.Sprintf("Pricing for %s (simulated)", params.Model), nil
		},
	)
}

// RegisterForgeResources registers the standard Forge resources.
func (s *Server) RegisterForgeResources() {
	// forge://config — Current configuration
	s.RegisterResource("forge://config", "Forge Configuration", "Current forge.yaml configuration", "application/json",
		func(_ context.Context, _ string) (string, string, error) {
			return `{"status": "active", "version": "0.4.0"}`, "application/json", nil
		},
	)

	// forge://status — System status
	s.RegisterResource("forge://status", "Forge Status", "Current system status", "application/json",
		func(_ context.Context, _ string) (string, string, error) {
			return `{"status": "running", "agents": 0}`, "application/json", nil
		},
	)
}

// RegisterForgePrompts registers the standard Forge prompts.
func (s *Server) RegisterForgePrompts() {
	s.RegisterPrompt("forge/code-review", "Code review with Forge agents",
		[]PromptArg{
			{Name: "file", Description: "File path to review", Required: true},
			{Name: "focus", Description: "Review focus (security, performance, style)", Required: false},
		},
		func(_ context.Context, args map[string]string) ([]PromptMessage, error) {
			focus := "general"
			if f, ok := args["focus"]; ok && f != "" {
				focus = f
			}
			return []PromptMessage{
				{
					Role: "user",
					Content: TextContent{
						Type: "text",
						Text: fmt.Sprintf("Review the file %s with focus on %s. Use forge tools to read and analyze the code.", args["file"], focus),
					},
				},
			}, nil
		},
	)

	s.RegisterPrompt("forge/fix-bug", "Fix a bug with Forge assistance",
		[]PromptArg{
			{Name: "description", Description: "Bug description", Required: true},
			{Name: "file", Description: "File with the bug", Required: false},
		},
		func(_ context.Context, args map[string]string) ([]PromptMessage, error) {
			prompt := fmt.Sprintf("Fix this bug: %s", args["description"])
			if file, ok := args["file"]; ok && file != "" {
				prompt += fmt.Sprintf(" in file %s", file)
			}
			return []PromptMessage{
				{
					Role: "user",
					Content: TextContent{
						Type: "text",
						Text: prompt + ". Use forge/search to find relevant code and forge/execute to test your fix.",
					},
				},
			}, nil
		},
	)
}

// IsMCPRequest checks if a JSON message is an MCP JSON-RPC request.
func IsMCPRequest(data []byte) bool {
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return false
	}
	return req.JSONRPC == "2.0" && strings.HasPrefix(req.Method, "tools/") ||
		strings.HasPrefix(req.Method, "resources/") ||
		strings.HasPrefix(req.Method, "prompts/") ||
		req.Method == "initialize" ||
		req.Method == "initialized" ||
		req.Method == "ping"
}
