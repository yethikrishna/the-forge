// Package lsp implements a Language Server Protocol server for Forge.
// Any editor that supports LSP (Neovim, Emacs, VS Code, Sublime, Helix)
// gets Forge features: code actions, diagnostics, hover info, completions.
//
// One protocol. Every editor.
package lsp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// LSP Protocol types (subset needed for Forge integration)

// Position represents a position in a document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a range in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// DiagnosticSeverity represents the severity of a diagnostic.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// Diagnostic represents a diagnostic item.
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
	Code     string             `json:"code,omitempty"`
}

// CodeActionKind represents the kind of a code action.
type CodeActionKind string

const (
	QuickFixCodeAction         CodeActionKind = "quickfix"
	RefactorCodeAction         CodeActionKind = "refactor"
	RefactorExtractCodeAction  CodeActionKind = "refactor.extract"
	RefactorRewriteCodeAction  CodeActionKind = "refactor.rewrite"
	SourceCodeAction           CodeActionKind = "source"
	SourceFixAllCodeAction     CodeActionKind = "source.fixAll"
)

// TextEdit represents a text edit.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit represents edits to workspace files.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// CodeAction represents a code action.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        CodeActionKind `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	IsPreferred bool           `json:"isPreferred,omitempty"`
	Command     *Command       `json:"command,omitempty"`
}

// Command represents a command execution.
type Command struct {
	Title     string   `json:"title"`
	Command   string   `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// MarkupContent represents marked-up content.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover represents a hover result.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// CompletionItem represents a completion item.
type CompletionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind,omitempty"`
	Detail        string         `json:"detail,omitempty"`
	Documentation string         `json:"documentation,omitempty"`
	InsertText    string         `json:"insertText,omitempty"`
}

// CompletionList represents a list of completion items.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// JSONRPC types

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *jsonrpcError `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServerCapabilities defines what the LSP server can do.
type ServerCapabilities struct {
	TextDocumentSync         *TextDocumentSyncOptions  `json:"textDocumentSync,omitempty"`
	CompletionProvider      *CompletionOptions        `json:"completionProvider,omitempty"`
	HoverProvider           bool                      `json:"hoverProvider"`
	CodeActionProvider      *CodeActionOptions        `json:"codeActionProvider,omitempty"`
	DiagnosticProvider      *DiagnosticOptions        `json:"diagnosticProvider,omitempty"`
	ExecuteCommandProvider  *ExecuteCommandOptions    `json:"executeCommandProvider,omitempty"`
}

// TextDocumentSyncOptions defines how documents are synced.
type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"` // 0=none, 1=full, 2=incremental
}

// CompletionOptions defines completion capabilities.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// CodeActionOptions defines code action capabilities.
type CodeActionOptions struct {
	CodeActionKinds []CodeActionKind `json:"codeActionKinds,omitempty"`
}

// DiagnosticOptions defines diagnostic capabilities.
type DiagnosticOptions struct {
	Identifier            string `json:"identifier,omitempty"`
	InterFileDependencies bool  `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool  `json:"workspaceDiagnostics"`
}

// ExecuteCommandOptions defines command execution capabilities.
type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

// ForgeAction represents a Forge-specific LSP action.
type ForgeAction struct {
	ID          string
	Title       string
	Kind        CodeActionKind
	Description string
	Handler     func(uri string, range_ Range) ([]TextEdit, error)
}

// Server is an LSP server for Forge.
type Server struct {
	mu       sync.RWMutex
	capabilities ServerCapabilities
	actions  map[string]*ForgeAction
	docs     map[string]string // URI -> content
	version  string
}

// NewServer creates an LSP server for Forge.
func NewServer(version string) *Server {
	s := &Server{
		version: version,
		docs:    make(map[string]string),
		actions: make(map[string]*ForgeAction),
		capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    1, // full sync
			},
			HoverProvider: true,
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{".", ":", "/"},
			},
			CodeActionProvider: &CodeActionOptions{
				CodeActionKinds: []CodeActionKind{
					QuickFixCodeAction,
					RefactorCodeAction,
					SourceCodeAction,
				},
			},
			DiagnosticProvider: &DiagnosticOptions{
				Identifier:            "forge",
				InterFileDependencies: true,
				WorkspaceDiagnostics:  true,
			},
			ExecuteCommandProvider: &ExecuteCommandOptions{
				Commands: []string{
					"forge.explain",
					"forge.refactor",
					"forge.generateTests",
					"forge.review",
					"forge.fix",
				},
			},
		},
	}

	// Register built-in Forge actions
	s.RegisterAction(&ForgeAction{
		ID:          "forge.explain",
		Title:       "Explain with Forge",
		Kind:        QuickFixCodeAction,
		Description: "Explain the selected code using an AI agent",
	})
	s.RegisterAction(&ForgeAction{
		ID:          "forge.refactor",
		Title:       "Refactor with Forge",
		Kind:        RefactorRewriteCodeAction,
		Description: "Refactor the selected code using an AI agent",
	})
	s.RegisterAction(&ForgeAction{
		ID:          "forge.generateTests",
		Title:       "Generate tests with Forge",
		Kind:        SourceCodeAction,
		Description: "Generate tests for the selected code",
	})
	s.RegisterAction(&ForgeAction{
		ID:          "forge.review",
		Title:       "Review with Forge",
		Kind:        QuickFixCodeAction,
		Description: "Review the selected code for issues",
	})
	s.RegisterAction(&ForgeAction{
		ID:          "forge.fix",
		Title:       "Fix with Forge",
		Kind:        QuickFixCodeAction,
		Description: "Auto-fix issues in the selected code",
	})

	return s
}

// RegisterAction registers a Forge LSP action.
func (s *Server) RegisterAction(action *ForgeAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions[action.ID] = action
}

// Handle processes a JSON-RPC request.
func (s *Server) Handle(data []byte) jsonrpcResponse {
	var req jsonrpcRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			Error:   &jsonrpcError{Code: -32700, Message: "parse error"},
		}
	}

	var result interface{}
	var err error

	switch req.Method {
	case "initialize":
		result, err = s.handleInitialize(req.Params)
	case "initialized":
		result = struct{}{}
	case "shutdown":
		result = nil
	case "exit":
		os.Exit(0)
	case "textDocument/didOpen":
		result, err = s.handleDidOpen(req.Params)
	case "textDocument/didChange":
		result, err = s.handleChange(req.Params)
	case "textDocument/didClose":
		result, err = s.handleDidClose(req.Params)
	case "textDocument/hover":
		result, err = s.handleHover(req.Params)
	case "textDocument/completion":
		result, err = s.handleCompletion(req.Params)
	case "textDocument/codeAction":
		result, err = s.handleCodeAction(req.Params)
	case "textDocument/diagnostic":
		result, err = s.handleDiagnostic(req.Params)
	case "workspace/executeCommand":
		result, err = s.handleExecuteCommand(req.Params)
	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}

	if err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32603, Message: err.Error()},
		}
	}

	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleInitialize(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"capabilities": s.capabilities,
		"serverInfo": map[string]interface{}{
			"name":    "forge-lsp",
			"version": s.version,
		},
	}, nil
}

func (s *Server) handleDidOpen(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Text    string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	s.mu.Unlock()
	return nil, nil
}

func (s *Server) handleChange(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if len(p.ContentChanges) > 0 {
		s.mu.Lock()
		s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
		s.mu.Unlock()
	}
	return nil, nil
}

func (s *Server) handleDidClose(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	s.mu.Lock()
	delete(s.docs, p.TextDocument.URI)
	s.mu.Unlock()
	return nil, nil
}

func (s *Server) handleHover(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position Position `json:"position"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	content, exists := s.docs[p.TextDocument.URI]
	s.mu.RUnlock()

	if !exists {
		return nil, nil
	}

	line := getLine(content, p.Position.Line)
	hoverText := fmt.Sprintf("**Forge LSP**\n\nLine %d: `%s`", p.Position.Line+1, strings.TrimSpace(line))

	return &Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: hoverText,
		},
	}, nil
}

func (s *Server) handleCompletion(params json.RawMessage) (interface{}, error) {
	items := []CompletionItem{
		{Label: "forge", Kind: 6, Detail: "Forge agent commands", InsertText: "forge"},
		{Label: "forge.yaml", Kind: 17, Detail: "Forge configuration file"},
	}

	// Add Forge command completions
	for _, action := range s.actions {
		items = append(items, CompletionItem{
			Label:      action.ID,
			Kind:       3,
			Detail:     action.Title,
			Documentation: action.Description,
		})
	}

	return &CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil
}

func (s *Server) handleCodeAction(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Range Range `json:"range"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	var actions []CodeAction
	for _, fa := range s.actions {
		actions = append(actions, CodeAction{
			Title: fa.Title,
			Kind:  fa.Kind,
			Command: &Command{
				Title:   fa.Title,
				Command: fa.ID,
				Arguments: []interface{}{
					p.TextDocument.URI,
					p.Range,
				},
			},
		})
	}

	return actions, nil
}

func (s *Server) handleDiagnostic(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	content, exists := s.docs[p.TextDocument.URI]
	s.mu.RUnlock()

	var diagnostics []Diagnostic
	if exists {
		diagnostics = s.analyzeDocument(p.TextDocument.URI, content)
	}

	return map[string]interface{}{
		"items": diagnostics,
	}, nil
}

func (s *Server) handleExecuteCommand(params json.RawMessage) (interface{}, error) {
	var p struct {
		Command   string          `json:"command"`
		Arguments []interface{}   `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	action, exists := s.actions[p.Command]
	if !exists {
		return nil, fmt.Errorf("unknown command: %s", p.Command)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Executed %s: %s", action.ID, action.Description),
	}, nil
}

// analyzeDocument performs basic static analysis for diagnostics.
func (s *Server) analyzeDocument(uri, content string) []Diagnostic {
	var diagnostics []Diagnostic
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		// Check for TODO/FIXME/HACK comments
		for _, marker := range []string{"TODO", "FIXME", "HACK", "XXX"} {
			if idx := strings.Index(line, marker); idx >= 0 {
				diagnostics = append(diagnostics, Diagnostic{
					Range: Range{
						Start: Position{Line: i, Character: idx},
						End:   Position{Line: i, Character: idx + len(marker)},
					},
					Severity: SeverityHint,
					Source:   "forge",
					Message:  fmt.Sprintf("%s found — consider resolving", marker),
					Code:     "forge-" + strings.ToLower(marker),
				})
			}
		}

		// Check for potential secret patterns
		for _, pattern := range []string{"password =", "api_key =", "secret =", "token ="} {
			if strings.Contains(strings.ToLower(line), pattern) {
				diagnostics = append(diagnostics, Diagnostic{
					Range: Range{
						Start: Position{Line: i, Character: 0},
						End:   Position{Line: i, Character: len(line)},
					},
					Severity: SeverityWarning,
					Source:   "forge",
					Message:  "Potential secret or credential detected",
					Code:     "forge-secret",
				})
			}
		}
	}

	return diagnostics
}

// ServeStdio runs the LSP server over stdin/stdout.
func (s *Server) ServeStdio(in io.Reader, out io.Writer) error {
	decoder := json.NewDecoder(in)
	for decoder.More() {
		// Read content-length header
		var length int
		for {
			var line string
			if _, err := fmt.Fscanln(in, &line); err != nil {
				return err
			}
			if line == "" || line == "\r" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &length)
			}
		}

		if length <= 0 {
			continue
		}

		buf := make([]byte, length)
		if _, err := io.ReadFull(in, buf); err != nil {
			return err
		}

		resp := s.Handle(buf)
		data, err := json.Marshal(resp)
		if err != nil {
			continue
		}

		fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(data), data)
	}

	return nil
}

func getLine(content string, lineNum int) string {
	lines := strings.Split(content, "\n")
	if lineNum < 0 || lineNum >= len(lines) {
		return ""
	}
	return lines[lineNum]
}
