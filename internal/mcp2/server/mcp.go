// Package mcp implements a Model Context Protocol server for Forge.
// Expose Forge capabilities (run, build, test, search) as MCP tools
// that any MCP-compatible client can use.
//
// Protocol spec: https://modelcontextprotocol.io
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Version is the MCP protocol version.
const Version = "2024-11-05"

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	ErrorParseError     = -32700
	ErrorInvalidRequest = -32600
	ErrorMethodNotFound = -32601
	ErrorInvalidParams  = -32602
	ErrorInternal       = -32603
)

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"arguments,omitempty"`
}

// ToolResult represents a tool execution result.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Resource represents an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ServerInfo represents server metadata.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Prompt represents an MCP prompt template.
type Prompt struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Arguments   []PromptArg `json:"arguments,omitempty"`
}

// PromptArg represents a prompt argument.
type PromptArg struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Handler handles MCP method calls.
type Handler func(params json.RawMessage) (interface{}, error)

// Server is an MCP server.
type Server struct {
	Info         ServerInfo
	tools        []Tool
	resources    []Resource
	prompts      []Prompt
	handlers     map[string]Handler
	toolHandlers map[string]func(map[string]interface{}) (ToolResult, error)
}

// NewServer creates an MCP server.
func NewServer(name, version string) *Server {
	s := &Server{
		Info:         ServerInfo{Name: name, Version: version},
		handlers:     make(map[string]Handler),
		toolHandlers: make(map[string]func(map[string]interface{}) (ToolResult, error)),
	}

	// Register standard methods
	s.handlers["initialize"] = s.handleInitialize
	s.handlers["tools/list"] = s.handleToolsList
	s.handlers["tools/call"] = s.handleToolsCall
	s.handlers["resources/list"] = s.handleResourcesList
	s.handlers["prompts/list"] = s.handlePromptsList
	s.handlers["ping"] = s.handlePing

	return s
}

// RegisterTool registers a tool with its handler.
func (s *Server) RegisterTool(tool Tool, handler func(map[string]interface{}) (ToolResult, error)) {
	s.tools = append(s.tools, tool)
	s.toolHandlers[tool.Name] = handler
}

// RegisterResource registers a resource.
func (s *Server) RegisterResource(resource Resource) {
	s.resources = append(s.resources, resource)
}

// RegisterPrompt registers a prompt template.
func (s *Server) RegisterPrompt(prompt Prompt) {
	s.prompts = append(s.prompts, prompt)
}

// Handle processes a JSON-RPC request.
func (s *Server) Handle(req JSONRPCRequest) JSONRPCResponse {
	handler, ok := s.handlers[req.Method]
	if !ok {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: ErrorMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}

	result, err := handler(req.Params)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: ErrorInternal, Message: err.Error()},
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// HandleRaw processes a raw JSON request bytes.
func (s *Server) HandleRaw(data []byte) JSONRPCResponse {
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: ErrorParseError, Message: "parse error"},
		}
	}
	return s.Handle(req)
}

// Method handlers

func (s *Server) handleInitialize(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"protocolVersion": Version,
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
			"prompts":   map[string]interface{}{},
		},
		"serverInfo": s.Info,
	}, nil
}

func (s *Server) handleToolsList(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"tools": s.tools,
	}, nil
}

func (s *Server) handleToolsCall(params json.RawMessage) (interface{}, error) {
	var call ToolCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	handler, ok := s.toolHandlers[call.Name]
	if !ok {
		return ToolResult{
			IsError: true,
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("unknown tool: %s", call.Name)}},
		}, nil
	}

	return handler(call.Args)
}

func (s *Server) handleResourcesList(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"resources": s.resources,
	}, nil
}

func (s *Server) handlePromptsList(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"prompts": s.prompts,
	}, nil
}

func (s *Server) handlePing(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{}, nil
}

// ForgeBuiltins returns the built-in Forge MCP tools.
func ForgeBuiltins() []Tool {
	return []Tool{
		{
			Name:        "forge_run",
			Description: "Run a Forge agent with a prompt",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{"type": "string", "description": "The prompt to send"},
					"model":  map[string]interface{}{"type": "string", "description": "Model to use"},
				},
				"required": []string{"prompt"},
			},
		},
		{
			Name:        "forge_build",
			Description: "Build the current project",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Project path"},
				},
			},
		},
		{
			Name:        "forge_test",
			Description: "Run tests for the current project",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{"type": "string", "description": "Test pattern"},
				},
			},
		},
		{
			Name:        "forge_search",
			Description: "Search the web",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search query"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "forge_session",
			Description: "Get current session info",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// WriteStdio reads JSON-RPC from stdin and writes responses to stdout.
func (s *Server) WriteStdio() {
	decoder := json.NewDecoder(os.Stdin)

	for decoder.More() {
		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: ErrorParseError, Message: err.Error()},
			}
			data, _ := json.Marshal(resp)
			fmt.Println(string(data))
			continue
		}

		resp := s.Handle(req)
		data, _ := json.Marshal(resp)
		fmt.Println(string(data))
	}
}

// FormatServerInfo renders server info for display.
func FormatServerInfo(s *Server) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("MCP Server: %s v%s\n", s.Info.Name, s.Info.Version))
	sb.WriteString(fmt.Sprintf("  Protocol:    %s\n", Version))
	sb.WriteString(fmt.Sprintf("  Tools:       %d\n", len(s.tools)))
	sb.WriteString(fmt.Sprintf("  Resources:   %d\n", len(s.resources)))
	sb.WriteString(fmt.Sprintf("  Prompts:     %d\n", len(s.prompts)))
	return sb.String()
}

// FormatTools renders available tools for display.
func FormatTools(tools []Tool) string {
	var sb strings.Builder
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", tool.Name, tool.Description))
	}
	return sb.String()
}

// RegisterForgeTools registers all built-in Forge tools with handlers.
func (s *Server) RegisterForgeTools() {
	for _, tool := range ForgeBuiltins() {
		name := tool.Name
		s.RegisterTool(tool, func(args map[string]interface{}) (ToolResult, error) {
			return ToolResult{
				Content: []ContentBlock{{
					Type: "text",
					Text: fmt.Sprintf("%s executed with args: %v", name, args),
				}},
			}, nil
		})
	}
}

// RegisterForgeResources registers Forge resources.
func (s *Server) RegisterForgeResources() {
	s.RegisterResource(Resource{
		URI:         "forge://config",
		Name:        "Forge Configuration",
		Description: "Current Forge configuration",
		MimeType:    "application/json",
	})
	s.RegisterResource(Resource{
		URI:         "forge://status",
		Name:        "Forge Status",
		Description: "Current Forge system status",
		MimeType:    "application/json",
	})
	s.RegisterResource(Resource{
		URI:         "forge://agents",
		Name:        "Active Agents",
		Description: "List of active agents",
		MimeType:    "application/json",
	})
}

// RegisterForgePrompts registers Forge prompt templates.
func (s *Server) RegisterForgePrompts() {
	s.RegisterPrompt(Prompt{
		Name:        "forge_code_review",
		Description: "Generate a code review for the given code",
		Arguments: []PromptArg{
			{Name: "code", Description: "Code to review", Required: true},
			{Name: "language", Description: "Programming language", Required: false},
		},
	})
	s.RegisterPrompt(Prompt{
		Name:        "forge_explain",
		Description: "Explain the given code",
		Arguments: []PromptArg{
			{Name: "code", Description: "Code to explain", Required: true},
		},
	})
	s.RegisterPrompt(Prompt{
		Name:        "forge_test_gen",
		Description: "Generate tests for the given code",
		Arguments: []PromptArg{
			{Name: "code", Description: "Code to generate tests for", Required: true},
			{Name: "framework", Description: "Test framework to use", Required: false},
		},
	})
}

// ServeStdio runs the MCP server over stdin/stdout.
func (s *Server) ServeStdio(ctx interface{ Done() <-chan struct{} }) error {
	go func() {
		<-ctx.Done()
		os.Exit(0)
	}()
	s.WriteStdio()
	return nil
}

// ServeHTTP starts an HTTP server for MCP (SSE + messages endpoint).
func (s *Server) ServeHTTP(addr string) error {
	http.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		fmt.Fprintf(w, "event: endpoint\ndata: /messages?session=%s\n\n", "default")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})

	http.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		resp := s.Handle(req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	fmt.Printf("MCP HTTP server listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

// HandleRequest processes a request with context (alias for Handle).
func (s *Server) HandleRequest(ctx interface{}, req JSONRPCRequest) JSONRPCResponse {
	return s.Handle(req)
}
