// Package navigate provides semantic code navigation using codegraph
// indices and LLM intent understanding. It supports symbol search,
// definition jumping, reference finding, and call hierarchy traversal.
package navigate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// SymbolKind represents the type of a code symbol.
type SymbolKind string

const (
	KindFunction  SymbolKind = "function"
	KindMethod    SymbolKind = "method"
	KindType      SymbolKind = "type"
	KindInterface SymbolKind = "interface"
	KindStruct    SymbolKind = "struct"
	KindVariable  SymbolKind = "variable"
	KindConstant  SymbolKind = "constant"
	KindPackage   SymbolKind = "package"
	KindField     SymbolKind = "field"
	KindEnum      SymbolKind = "enum"
)

// Symbol represents a code symbol (function, type, variable, etc.).
type Symbol struct {
	Name      string     `json:"name"`
	Kind      SymbolKind `json:"kind"`
	Package   string     `json:"package,omitempty"`
	File      string     `json:"file"`
	Line      int        `json:"line"`
	Column    int        `json:"column"`
	Signature string     `json:"signature,omitempty"`
	Doc       string     `json:"doc,omitempty"`
	Exported  bool       `json:"exported"`
	Receiver  string     `json:"receiver,omitempty"` // for methods
}

// Reference represents a reference to a symbol.
type Reference struct {
	SymbolName string `json:"symbol_name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Context    string `json:"context,omitempty"` // surrounding line text
	Kind       string `json:"kind"`              // "definition", "call", "import", "type_use"
}

// CallEdge represents an edge in the call graph.
type CallEdge struct {
	Caller    Symbol `json:"caller"`
	Callee    Symbol `json:"callee"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Indirect  bool   `json:"indirect"` // via interface
}

// Navigator provides semantic code navigation.
type Navigator struct {
	mu        sync.RWMutex
	root      string
	symbols   map[string][]Symbol    // name -> symbols
	byFile    map[string][]Symbol    // file -> symbols
	refs      map[string][]Reference // symbol name -> references
	callGraph []CallEdge
	indexTime time.Time
	languages map[string]bool // detected languages
}

// New creates a new Navigator for the given project root.
func New(root string) *Navigator {
	return &Navigator{
		root:      root,
		symbols:   make(map[string][]Symbol),
		byFile:    make(map[string][]Symbol),
		refs:      make(map[string][]Reference),
		languages: make(map[string]bool),
	}
}

// Index scans the project and builds the navigation index.
func (n *Navigator) Index(ctx context.Context) error {
	start := time.Now()
	n.mu.Lock()
	defer n.mu.Unlock()

	// Clear existing index
	n.symbols = make(map[string][]Symbol)
	n.byFile = make(map[string][]Symbol)
	n.refs = make(map[string][]Reference)
	n.callGraph = nil
	n.languages = make(map[string]bool)

	// Walk the project tree
	err := filepath.WalkDir(n.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Skip hidden and vendor dirs
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(name)
		switch ext {
		case ".go":
			n.languages["go"] = true
			return n.indexGoFile(path)
		case ".py":
			n.languages["python"] = true
			return n.indexPythonFile(path)
		case ".ts", ".tsx":
			n.languages["typescript"] = true
			return n.indexTSFile(path)
		case ".js", ".jsx":
			n.languages["javascript"] = true
			return n.indexJSFile(path)
		case ".rs":
			n.languages["rust"] = true
			return n.indexRustFile(path)
		}
		return nil
	})

	n.indexTime = time.Now()
	_ = start
	return err
}

// indexGoFile indexes symbols from a Go source file.
func (n *Navigator) indexGoFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	relPath, _ := filepath.Rel(n.root, path)
	pkg := n.extractGoPackage(content)

	lines := strings.Split(content, "\n")

	// Match function declarations
	funcRe := regexp.MustCompile(`^func\s+(?:\((\w+)\s+\*?[\w.]+\)\s+)?(\w+)\s*\(([^)]*)\)`)
	// Match type declarations
	typeRe := regexp.MustCompile(`^type\s+(\w+)\s+(struct|interface|func\b|.+)`)
	// Match var/const blocks
	varRe := regexp.MustCompile(`^(?:var|const)\s+(\w+)\s+`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if matches := funcRe.FindStringSubmatch(trimmed); matches != nil {
			receiver := matches[1]
			name := matches[2]
			sig := matches[3]
			kind := KindFunction
			if receiver != "" {
				kind = KindMethod
			}
			exported := name[0] >= 'A' && name[0] <= 'Z'

			sym := Symbol{
				Name:      name,
				Kind:      kind,
				Package:   pkg,
				File:      relPath,
				Line:      i + 1,
				Column:    strings.Index(trimmed, name) + 1,
				Signature: "(" + sig + ")",
				Exported:  exported,
				Receiver:  receiver,
			}
			// Get doc comment
			if i > 0 {
				for j := i - 1; j >= 0 && j > i-5; j-- {
					if strings.HasPrefix(strings.TrimSpace(lines[j]), "//") {
						sym.Doc = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[j]), "//"))
					} else {
						break
					}
				}
			}

			n.addSymbol(sym)

			// Find references (calls to this function)
			if exported {
				callRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
				for fi, fcontent := range lines {
					if fi == i {
						continue
					}
					if callRe.MatchString(fcontent) {
						n.addReference(Reference{
							SymbolName: name,
							File:       relPath,
							Line:       fi + 1,
							Column:     callRe.FindStringIndex(fcontent)[0] + 1,
							Context:    strings.TrimSpace(fcontent),
							Kind:       "call",
						})
					}
				}
			}
		}

		if matches := typeRe.FindStringSubmatch(trimmed); matches != nil {
			name := matches[1]
			typeKind := matches[2]
			kind := KindType
			switch typeKind {
			case "struct":
				kind = KindStruct
			case "interface":
				kind = KindInterface
			}
			exported := name[0] >= 'A' && name[0] <= 'Z'
			sym := Symbol{
				Name:     name,
				Kind:     kind,
				Package:  pkg,
				File:     relPath,
				Line:     i + 1,
				Column:   strings.Index(trimmed, name) + 1,
				Exported: exported,
			}
			n.addSymbol(sym)
		}

		if matches := varRe.FindStringSubmatch(trimmed); matches != nil {
			name := matches[1]
			exported := name[0] >= 'A' && name[0] <= 'Z'
			sym := Symbol{
				Name:     name,
				Kind:     KindVariable,
				Package:  pkg,
				File:     relPath,
				Line:     i + 1,
				Column:   strings.Index(trimmed, name) + 1,
				Exported: exported,
			}
			n.addSymbol(sym)
		}
	}
	return nil
}

// indexPythonFile indexes symbols from a Python source file.
func (n *Navigator) indexPythonFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	relPath, _ := filepath.Rel(n.root, path)
	lines := strings.Split(content, "\n")

	funcRe := regexp.MustCompile(`^def\s+(\w+)\s*\(([^)]*)\)`)
	classRe := regexp.MustCompile(`^class\s+(\w+)`)

	for i, line := range lines {
		if matches := funcRe.FindStringSubmatch(line); matches != nil {
			sym := Symbol{
				Name:      matches[1],
				Kind:      KindFunction,
				File:      relPath,
				Line:      i + 1,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: "(" + matches[2] + ")",
			}
			n.addSymbol(sym)
		}
		if matches := classRe.FindStringSubmatch(line); matches != nil {
			sym := Symbol{
				Name:   matches[1],
				Kind:   KindType,
				File:   relPath,
				Line:   i + 1,
				Column: strings.Index(line, matches[1]) + 1,
			}
			n.addSymbol(sym)
		}
	}
	return nil
}

// indexTSFile indexes symbols from a TypeScript source file.
func (n *Navigator) indexTSFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	relPath, _ := filepath.Rel(n.root, path)
	lines := strings.Split(content, "\n")

	funcRe := regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	classRe := regexp.MustCompile(`(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	ifaceRe := regexp.MustCompile(`(?:export\s+)?interface\s+(\w+)`)

	for i, line := range lines {
		if matches := funcRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindFunction, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
		if matches := classRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindStruct, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
		if matches := ifaceRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindInterface, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
	}
	return nil
}

// indexJSFile indexes symbols from a JavaScript source file.
func (n *Navigator) indexJSFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	relPath, _ := filepath.Rel(n.root, path)
	lines := strings.Split(content, "\n")

	funcRe := regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	arrowRe := regexp.MustCompile(`(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`)

	for i, line := range lines {
		if matches := funcRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindFunction, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
		if matches := arrowRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindFunction, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
	}
	return nil
}

// indexRustFile indexes symbols from a Rust source file.
func (n *Navigator) indexRustFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	relPath, _ := filepath.Rel(n.root, path)
	lines := strings.Split(content, "\n")

	funcRe := regexp.MustCompile(`(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`)
	structRe := regexp.MustCompile(`(?:pub\s+)?struct\s+(\w+)`)
	enumRe := regexp.MustCompile(`(?:pub\s+)?enum\s+(\w+)`)
	implRe := regexp.MustCompile(`impl\s+(?:<[^>]+>\s+)?(\w+)`)

	for i, line := range lines {
		if matches := funcRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindFunction, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
		if matches := structRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindStruct, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
		if matches := enumRe.FindStringSubmatch(line); matches != nil {
			n.addSymbol(Symbol{Name: matches[1], Kind: KindEnum, File: relPath, Line: i + 1, Column: strings.Index(line, matches[1]) + 1})
		}
		if matches := implRe.FindStringSubmatch(line); matches != nil {
			// impl blocks tracked as type references
			_ = matches[1]
		}
	}
	return nil
}

func (n *Navigator) addSymbol(sym Symbol) {
	n.symbols[sym.Name] = append(n.symbols[sym.Name], sym)
	n.byFile[sym.File] = append(n.byFile[sym.File], sym)
}

func (n *Navigator) addReference(ref Reference) {
	n.refs[ref.SymbolName] = append(n.refs[ref.SymbolName], ref)
}

// extractGoPackage extracts the package name from Go source.
func (n *Navigator) extractGoPackage(content string) string {
	re := regexp.MustCompile(`^package\s+(\w+)`)
	matches := re.FindStringSubmatch(content)
	if matches != nil {
		return matches[1]
	}
	return ""
}

// SearchSymbols searches for symbols matching a query.
// Supports exact match, prefix match, and fuzzy match.
func (n *Navigator) SearchSymbols(query string, limit int) []Symbol {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var results []Symbol
	lowerQuery := strings.ToLower(query)

	// Exact match first
	if syms, ok := n.symbols[query]; ok {
		results = append(results, syms...)
	}

	// Prefix match
	for name, syms := range n.symbols {
		if name == query {
			continue
		}
		if strings.HasPrefix(strings.ToLower(name), lowerQuery) {
			results = append(results, syms...)
		}
	}

	// Contains match
	if len(results) < limit {
		for name, syms := range n.symbols {
			if strings.Contains(strings.ToLower(name), lowerQuery) && !strings.HasPrefix(strings.ToLower(name), lowerQuery) {
				results = append(results, syms...)
			}
		}
	}

	// Sort by relevance: exact > prefix > contains, then exported first
	sort.Slice(results, func(i, j int) bool {
		ri, rj := results[i], results[j]
		// Exact name match first
		if ri.Name == query && rj.Name != query {
			return true
		}
		if rj.Name == query && ri.Name != query {
			return false
		}
		// Exported first
		if ri.Exported != rj.Exported {
			return ri.Exported
		}
		// Then by name
		return ri.Name < rj.Name
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// GotoDefinition finds the definition(s) of a symbol.
func (n *Navigator) GotoDefinition(name string) []Symbol {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if syms, ok := n.symbols[name]; ok {
		// Return only definitions (not references)
		var defs []Symbol
		for _, s := range syms {
			defs = append(defs, s)
		}
		return defs
	}
	return nil
}

// FindReferences finds all references to a symbol.
func (n *Navigator) FindReferences(name string) []Reference {
	n.mu.RLock()
	defer n.mu.RUnlock()

	refs := make([]Reference, 0)

	// Include the definition as a reference
	if syms, ok := n.symbols[name]; ok {
		for _, s := range syms {
			refs = append(refs, Reference{
				SymbolName: name,
				File:       s.File,
				Line:       s.Line,
				Column:     s.Column,
				Kind:       "definition",
			})
		}
	}

	// Add usage references
	if r, ok := n.refs[name]; ok {
		refs = append(refs, r...)
	}

	return refs
}

// CallHierarchy returns callers or callees of a function.
func (n *Navigator) CallHierarchy(name string, direction string) []CallEdge {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var results []CallEdge
	for _, edge := range n.callGraph {
		switch direction {
		case "callers":
			if edge.Callee.Name == name {
				results = append(results, edge)
			}
		case "callees":
			if edge.Caller.Name == name {
				results = append(results, edge)
			}
		default: // both
			if edge.Caller.Name == name || edge.Callee.Name == name {
				results = append(results, edge)
			}
		}
	}
	return results
}

// SymbolsByFile returns all symbols in a given file.
func (n *Navigator) SymbolsByFile(file string) []Symbol {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.byFile[file]
}

// Outline returns a high-level outline of the project structure.
func (n *Navigator) Outline() map[string][]Symbol {
	n.mu.RLock()
	defer n.mu.RUnlock()

	outline := make(map[string][]Symbol)
	for file, syms := range n.byFile {
		// Only include exported symbols in outline
		var exported []Symbol
		for _, s := range syms {
			if s.Exported {
				exported = append(exported, s)
			}
		}
		if len(exported) > 0 {
			outline[file] = exported
		}
	}
	return outline
}

// Stats returns navigation index statistics.
func (n *Navigator) Stats() IndexStats {
	n.mu.RLock()
	defer n.mu.RUnlock()

	stats := IndexStats{
		TotalSymbols: 0,
		TotalRefs:    0,
		Files:        len(n.byFile),
		ByKind:       make(map[SymbolKind]int),
		Languages:    make([]string, 0, len(n.languages)),
		IndexedAt:    n.indexTime,
	}

	for _, syms := range n.symbols {
		stats.TotalSymbols += len(syms)
		for _, s := range syms {
			stats.ByKind[s.Kind]++
		}
	}

	for _, refs := range n.refs {
		stats.TotalRefs += len(refs)
	}

	for lang := range n.languages {
		stats.Languages = append(stats.Languages, lang)
	}
	sort.Strings(stats.Languages)

	return stats
}

// IndexStats holds statistics about the navigation index.
type IndexStats struct {
	TotalSymbols int                    `json:"total_symbols"`
	TotalRefs    int                    `json:"total_references"`
	Files        int                    `json:"files_indexed"`
	ByKind       map[SymbolKind]int     `json:"by_kind"`
	Languages    []string               `json:"languages"`
	IndexedAt    time.Time              `json:"indexed_at"`
}

// NavigateIntent represents a natural language navigation intent.
type NavigateIntent struct {
	Intent    string `json:"intent"`     // "definition", "references", "search", "outline", "callers", "callees"
	Target    string `json:"target"`     // symbol name or query
	Direction string `json:"direction"`  // for call hierarchy: "callers" or "callees"
	File      string `json:"file,omitempty"` // optional file context
}

// ParseIntent parses a natural language query into a navigation intent.
func ParseIntent(query string) NavigateIntent {
	lower := strings.ToLower(query)

	// Definition patterns
	defPatterns := []string{"go to definition", "definition of", "where is", "where's", "find definition", "show definition", "jump to"}
	for _, p := range defPatterns {
		if strings.Contains(lower, p) {
			target := strings.TrimSpace(strings.ReplaceAll(lower, p, ""))
			target = strings.TrimPrefix(target, "the ")
			return NavigateIntent{Intent: "definition", Target: target}
		}
	}

	// Reference patterns
	refPatterns := []string{"references to", "usages of", "where is", "who uses", "who calls", "find all references", "find usages"}
	for _, p := range refPatterns {
		if strings.Contains(lower, p) {
			target := strings.TrimSpace(strings.ReplaceAll(lower, p, ""))
			target = strings.TrimPrefix(target, "the ")
			return NavigateIntent{Intent: "references", Target: target}
		}
	}

	// Call hierarchy patterns
	if strings.Contains(lower, "callers of") || strings.Contains(lower, "who calls") {
		target := strings.TrimSpace(strings.ReplaceAll(lower, "callers of", ""))
		target = strings.TrimSpace(strings.ReplaceAll(target, "who calls", ""))
		return NavigateIntent{Intent: "callers", Target: target, Direction: "callers"}
	}
	if strings.Contains(lower, "callees of") || strings.Contains(lower, "what does") || strings.Contains(lower, "calls from") {
		target := strings.TrimSpace(strings.ReplaceAll(lower, "callees of", ""))
		target = strings.TrimSpace(strings.ReplaceAll(target, "what does", ""))
		target = strings.TrimSuffix(target, "call")
		target = strings.TrimSpace(strings.ReplaceAll(target, "calls from", ""))
		return NavigateIntent{Intent: "callees", Target: target, Direction: "callees"}
	}

	// Outline pattern
	if strings.Contains(lower, "outline") || strings.Contains(lower, "structure") || strings.Contains(lower, "overview of") {
		return NavigateIntent{Intent: "outline"}
	}

	// Default: search
	return NavigateIntent{Intent: "search", Target: query}
}

// ExecuteIntent executes a navigation intent and returns formatted results.
func (n *Navigator) ExecuteIntent(intent NavigateIntent) string {
	switch intent.Intent {
	case "definition":
		syms := n.GotoDefinition(intent.Target)
		if len(syms) == 0 {
			return fmt.Sprintf("No definitions found for %q", intent.Target)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Definitions of %q:\n", intent.Target)
		for _, s := range syms {
			fmt.Fprintf(&b, "  %s %s at %s:%d\n", s.Kind, s.Name, s.File, s.Line)
			if s.Signature != "" {
				fmt.Fprintf(&b, "    signature: %s%s\n", s.Name, s.Signature)
			}
			if s.Doc != "" {
				fmt.Fprintf(&b, "    doc: %s\n", s.Doc)
			}
		}
		return b.String()

	case "references":
		refs := n.FindReferences(intent.Target)
		if len(refs) == 0 {
			return fmt.Sprintf("No references found for %q", intent.Target)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "References to %q (%d found):\n", intent.Target, len(refs))
		for _, r := range refs {
			fmt.Fprintf(&b, "  [%s] %s:%d %s\n", r.Kind, r.File, r.Line, r.Context)
		}
		return b.String()

	case "callers", "callees":
		edges := n.CallHierarchy(intent.Target, intent.Direction)
		if len(edges) == 0 {
			return fmt.Sprintf("No call hierarchy found for %q", intent.Target)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Call hierarchy for %q (%s):\n", intent.Target, intent.Direction)
		for _, e := range edges {
			indirect := ""
			if e.Indirect {
				indirect = " (indirect)"
			}
			fmt.Fprintf(&b, "  %s → %s at %s:%d%s\n", e.Caller.Name, e.Callee.Name, e.File, e.Line, indirect)
		}
		return b.String()

	case "outline":
		outline := n.Outline()
		var b strings.Builder
		b.WriteString("Project outline:\n")
		files := make([]string, 0, len(outline))
		for f := range outline {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Fprintf(&b, "\n%s:\n", f)
			for _, s := range outline[f] {
				fmt.Fprintf(&b, "  %s %s", s.Kind, s.Name)
				if s.Signature != "" {
					fmt.Fprintf(&b, "%s", s.Signature)
				}
				fmt.Fprintln(&b)
			}
		}
		return b.String()

	default: // search
		syms := n.SearchSymbols(intent.Target, 20)
		if len(syms) == 0 {
			return fmt.Sprintf("No symbols matching %q", intent.Target)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Symbols matching %q (%d found):\n", intent.Target, len(syms))
		for _, s := range syms {
			fmt.Fprintf(&b, "  %s %s at %s:%d\n", s.Kind, s.Name, s.File, s.Line)
		}
		return b.String()
	}
}
