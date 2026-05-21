// Package navigate provides semantic code navigation using index + LLM intent understanding.
// It enables developers to jump to definitions, find references, trace call chains,
// and understand code structure through natural language queries.
//
// Navigate combines static analysis (AST-based) with semantic search (embedding-based)
// to provide both precise and fuzzy code navigation.
package navigate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SymbolKind represents the type of a code symbol.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolType      SymbolKind = "type"
	SymbolInterface SymbolKind = "interface"
	SymbolVariable  SymbolKind = "variable"
	SymbolConstant  SymbolKind = "constant"
	SymbolPackage   SymbolKind = "package"
	SymbolField     SymbolKind = "field"
	SymbolImport    SymbolKind = "import"
)

// Symbol represents a code symbol (function, type, variable, etc).
type Symbol struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Kind         SymbolKind        `json:"kind"`
	Package      string            `json:"package"`
	File         string            `json:"file"`
	Line         int               `json:"line"`
	EndLine      int               `json:"end_line"`
	Signature    string            `json:"signature,omitempty"`
	DocComment   string            `json:"doc_comment,omitempty"`
	Receiver     string            `json:"receiver,omitempty"`     // for methods
	Exported     bool              `json:"exported"`
	Embedded     bool              `json:"embedded"`              // embedded interface/struct
	Callers      []string          `json:"callers,omitempty"`     // symbol IDs that call this
	Callees      []string          `json:"callees,omitempty"`     // symbol IDs this calls
	Implements   []string          `json:"implements,omitempty"`  // interface IDs this type implements
	ImplementedBy []string         `json:"implemented_by,omitempty"` // types that implement this interface
	Tags         []string          `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Reference represents a reference to a symbol at a specific location.
type Reference struct {
	SymbolID  string `json:"symbol_id"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Kind      string `json:"kind"` // "definition", "call", "import", "type_assert", "embed"
	Context   string `json:"context,omitempty"` // surrounding line text
}

// NavigationResult represents a navigation query result.
type NavigationResult struct {
	Query     string     `json:"query"`
	Matches   []Match    `json:"matches"`
	Duration  time.Duration `json:"duration"`
	Total     int        `json:"total"`
	Truncated bool       `json:"truncated"`
}

// Match represents a matched symbol with relevance scoring.
type Match struct {
	Symbol    Symbol  `json:"symbol"`
	Relevance float64 `json:"relevance"`
	Reason    string  `json:"reason"`
}

// CallPath represents a call chain between two symbols.
type CallPath struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Path  []string `json:"path"`  // ordered symbol IDs
	Depth int      `json:"depth"`
}

// Index represents the code navigation index.
type Index struct {
	mu       sync.RWMutex
	dir      string
	symbols  map[string]*Symbol    // id -> symbol
	byName   map[string][]*Symbol  // name -> symbols (name can be overloaded)
	byFile   map[string][]*Symbol  // file -> symbols
	byKind   map[SymbolKind][]*Symbol // kind -> symbols
	byPkg    map[string][]*Symbol  // package -> symbols
	refs     map[string][]*Reference // symbol_id -> references
	graph    map[string][]string    // caller_id -> callee_ids (call graph)
	revGraph map[string][]string    // callee_id -> caller_ids (reverse call graph)
	fileHash map[string]string     // file -> content hash (for incremental updates)
}

// NewIndex creates a new navigation index.
func NewIndex(dir string) *Index {
	return &Index{
		dir:      dir,
		symbols:  make(map[string]*Symbol),
		byName:   make(map[string][]*Symbol),
		byFile:   make(map[string][]*Symbol),
		byKind:   make(map[SymbolKind][]*Symbol),
		byPkg:    make(map[string][]*Symbol),
		refs:     make(map[string][]*Reference),
		graph:    make(map[string][]string),
		revGraph: make(map[string][]string),
		fileHash: make(map[string]string),
	}
}

// AddSymbol adds a symbol to the index.
func (idx *Index) AddSymbol(sym Symbol) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if sym.ID == "" {
		sym.ID = symbolID(sym.Name, sym.Package, sym.File, sym.Line)
	}

	if _, exists := idx.symbols[sym.ID]; exists {
		return fmt.Errorf("symbol %s already exists", sym.ID)
	}

	idx.symbols[sym.ID] = &sym

	idx.byName[sym.Name] = append(idx.byName[sym.Name], &sym)
	idx.byFile[sym.File] = append(idx.byFile[sym.File], &sym)
	idx.byKind[sym.Kind] = append(idx.byKind[sym.Kind], &sym)
	idx.byPkg[sym.Package] = append(idx.byPkg[sym.Package], &sym)

	// Build call graph edges
	for _, calleeID := range sym.Callees {
		idx.graph[sym.ID] = append(idx.graph[sym.ID], calleeID)
		idx.revGraph[calleeID] = append(idx.revGraph[calleeID], sym.ID)
	}

	return nil
}

// AddReference adds a reference to the index.
func (idx *Index) AddReference(ref Reference) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.symbols[ref.SymbolID]; !ok {
		return fmt.Errorf("symbol %s not found", ref.SymbolID)
	}

	idx.refs[ref.SymbolID] = append(idx.refs[ref.SymbolID], &ref)
	return nil
}

// AddEdge adds a call graph edge.
func (idx *Index) AddEdge(fromID, toID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.graph[fromID] = append(idx.graph[fromID], toID)
	idx.revGraph[toID] = append(idx.revGraph[toID], fromID)
}

// Lookup finds symbols by exact name.
func (idx *Index) Lookup(name string) []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byName[name]
}

// LookupInPackage finds symbols by name in a specific package.
func (idx *Index) LookupInPackage(name, pkg string) []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []*Symbol
	for _, sym := range idx.byName[name] {
		if sym.Package == pkg {
			results = append(results, sym)
		}
	}
	return results
}

// Definition finds the definition of a symbol.
func (idx *Index) Definition(symbolID string) (*Symbol, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	sym, ok := idx.symbols[symbolID]
	if !ok {
		return nil, fmt.Errorf("symbol %s not found", symbolID)
	}
	return sym, nil
}

// References finds all references to a symbol.
func (idx *Index) References(symbolID string) []*Reference {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.refs[symbolID]
}

// Callers finds all callers of a symbol.
func (idx *Index) Callers(symbolID string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.revGraph[symbolID]
}

// Callees finds all symbols called by a symbol.
func (idx *Index) Callees(symbolID string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.graph[symbolID]
}

// FindCallPath finds a call path between two symbols using BFS.
func (idx *Index) FindCallPath(fromID, toID string) *CallPath {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if fromID == toID {
		return &CallPath{From: fromID, To: toID, Path: []string{fromID}, Depth: 0}
	}

	// BFS
	visited := map[string]string{} // node -> parent
	queue := []string{fromID}
	visited[fromID] = ""

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range idx.graph[current] {
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = current

			if next == toID {
				// Reconstruct path
				var path []string
				node := toID
				for node != "" {
					path = append([]string{node}, path...)
					node = visited[node]
				}
				return &CallPath{
					From:  fromID,
					To:    toID,
					Path:  path,
					Depth: len(path) - 1,
				}
			}
			queue = append(queue, next)
		}
	}

	return nil // no path found
}

// Implementations finds all types that implement an interface.
func (idx *Index) Implementations(ifaceID string) []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	iface, ok := idx.symbols[ifaceID]
	if !ok || iface.Kind != SymbolInterface {
		return nil
	}

	var results []*Symbol
	for _, typeID := range iface.ImplementedBy {
		if sym, ok := idx.symbols[typeID]; ok {
			results = append(results, sym)
		}
	}
	return results
}

// Search performs a fuzzy search across all symbols.
func (idx *Index) Search(query string, limit int) *NavigationResult {
	start := time.Now()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	var matches []Match
	queryLower := strings.ToLower(query)

	// Exact name match (highest relevance)
	for _, sym := range idx.byName[query] {
		matches = append(matches, Match{Symbol: *sym, Relevance: 1.0, Reason: "exact name match"})
	}

	// Case-insensitive name match
	for name, syms := range idx.byName {
		if name == query {
			continue
		}
		if strings.EqualFold(name, query) {
			for _, sym := range syms {
				matches = append(matches, Match{Symbol: *sym, Relevance: 0.9, Reason: "case-insensitive name match"})
			}
		}
	}

	// Substring match
	for name, syms := range idx.byName {
		if strings.EqualFold(name, query) {
			continue
		}
		if strings.Contains(strings.ToLower(name), queryLower) {
			for _, sym := range syms {
				matches = append(matches, Match{Symbol: *sym, Relevance: 0.7, Reason: "substring match"})
			}
		}
	}

	// Package + name match (e.g. "fmt.Println")
	if parts := strings.SplitN(query, ".", 2); len(parts) == 2 {
		for _, sym := range idx.byPkg[parts[0]] {
			if strings.EqualFold(sym.Name, parts[1]) {
				matches = append(matches, Match{Symbol: *sym, Relevance: 0.95, Reason: "package.name match"})
			}
		}
	}

	// Doc comment search
	for _, sym := range idx.symbols {
		if strings.Contains(strings.ToLower(sym.DocComment), queryLower) {
			matches = append(matches, Match{Symbol: *sym, Relevance: 0.4, Reason: "doc comment match"})
		}
	}

	// Deduplicate by symbol ID
	seen := make(map[string]bool)
	var deduped []Match
	for _, m := range matches {
		if !seen[m.Symbol.ID] {
			seen[m.Symbol.ID] = true
			deduped = append(deduped, m)
		}
	}

	// Sort by relevance
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].Relevance > deduped[j].Relevance
	})

	truncated := len(deduped) > limit
	if len(deduped) > limit {
		deduped = deduped[:limit]
	}

	return &NavigationResult{
		Query:     query,
		Matches:   deduped,
		Duration:  time.Since(start),
		Total:     len(deduped),
		Truncated: truncated,
	}
}

// SymbolsByFile returns all symbols in a file.
func (idx *Index) SymbolsByFile(file string) []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byFile[file]
}

// SymbolsByKind returns all symbols of a given kind.
func (idx *Index) SymbolsByKind(kind SymbolKind) []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byKind[kind]
}

// SymbolsByPackage returns all symbols in a package.
func (idx *Index) SymbolsByPackage(pkg string) []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byPkg[pkg]
}

// Stats returns index statistics.
func (idx *Index) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	stats := IndexStats{
		SymbolCount:   len(idx.symbols),
		FileCount:     len(idx.byFile),
		PackageCount:  len(idx.byPkg),
		ReferenceCount: 0,
		EdgeCount:     0,
		ByKind:        make(map[SymbolKind]int),
	}

	for _, refs := range idx.refs {
		stats.ReferenceCount += len(refs)
	}
	for _, edges := range idx.graph {
		stats.EdgeCount += len(edges)
	}

	for kind, syms := range idx.byKind {
		stats.ByKind[kind] = len(syms)
	}

	return stats
}

// IndexStats holds index statistics.
type IndexStats struct {
	SymbolCount    int                `json:"symbol_count"`
	FileCount      int                `json:"file_count"`
	PackageCount   int                `json:"package_count"`
	ReferenceCount int                `json:"reference_count"`
	EdgeCount      int                `json:"edge_count"`
	ByKind         map[SymbolKind]int `json:"by_kind"`
}

// Save persists the index to disk.
func (idx *Index) Save() error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if err := os.MkdirAll(idx.dir, 0755); err != nil {
		return fmt.Errorf("create index dir: %w", err)
	}

	// Save symbols
	symData, err := json.MarshalIndent(idx.symbols, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal symbols: %w", err)
	}
	if err := os.WriteFile(filepath.Join(idx.dir, "symbols.json"), symData, 0644); err != nil {
		return err
	}

	// Save call graph
	graphData, err := json.MarshalIndent(idx.graph, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	if err := os.WriteFile(filepath.Join(idx.dir, "callgraph.json"), graphData, 0644); err != nil {
		return err
	}

	// Save references
	refData, err := json.MarshalIndent(idx.refs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal refs: %w", err)
	}
	return os.WriteFile(filepath.Join(idx.dir, "references.json"), refData, 0644)
}

// Load reads the index from disk.
func (idx *Index) Load() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Load symbols
	symData, err := os.ReadFile(filepath.Join(idx.dir, "symbols.json"))
	if err != nil {
		return fmt.Errorf("read symbols: %w", err)
	}
	if err := json.Unmarshal(symData, &idx.symbols); err != nil {
		return fmt.Errorf("unmarshal symbols: %w", err)
	}

	// Rebuild lookup maps
	idx.byName = make(map[string][]*Symbol)
	idx.byFile = make(map[string][]*Symbol)
	idx.byKind = make(map[SymbolKind][]*Symbol)
	idx.byPkg = make(map[string][]*Symbol)

	for id, sym := range idx.symbols {
		idx.byName[sym.Name] = append(idx.byName[sym.Name], sym)
		idx.byFile[sym.File] = append(idx.byFile[sym.File], sym)
		idx.byKind[sym.Kind] = append(idx.byKind[sym.Kind], sym)
		idx.byPkg[sym.Package] = append(idx.byPkg[sym.Package], sym)
		_ = id
	}

	// Load call graph
	graphData, err := os.ReadFile(filepath.Join(idx.dir, "callgraph.json"))
	if err != nil {
		return fmt.Errorf("read graph: %w", err)
	}
	if err := json.Unmarshal(graphData, &idx.graph); err != nil {
		return fmt.Errorf("unmarshal graph: %w", err)
	}

	// Rebuild reverse graph
	idx.revGraph = make(map[string][]string)
	for from, tos := range idx.graph {
		for _, to := range tos {
			idx.revGraph[to] = append(idx.revGraph[to], from)
		}
	}

	// Load references
	refData, err := os.ReadFile(filepath.Join(idx.dir, "references.json"))
	if err != nil {
		return fmt.Errorf("read refs: %w", err)
	}
	return json.Unmarshal(refData, &idx.refs)
}

// RemoveFile removes all symbols and references for a file (for incremental updates).
func (idx *Index) RemoveFile(file string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	syms, ok := idx.byFile[file]
	if !ok {
		return
	}

	for _, sym := range syms {
		delete(idx.symbols, sym.ID)
		delete(idx.refs, sym.ID)
		delete(idx.graph, sym.ID)

		// Remove from reverse graph
		for _, callees := range idx.graph {
			for i, callee := range callees {
				if callee == sym.ID {
					idx.graph[sym.ID] = append(callees[:i], callees[i+1:]...)
					break
				}
			}
		}
		delete(idx.revGraph, sym.ID)
	}

	delete(idx.byFile, file)
	delete(idx.fileHash, file)

	// Rebuild byName, byKind, byPkg
	idx.rebuildLookupMaps()
}

func (idx *Index) rebuildLookupMaps() {
	idx.byName = make(map[string][]*Symbol)
	idx.byKind = make(map[SymbolKind][]*Symbol)
	idx.byPkg = make(map[string][]*Symbol)

	for _, sym := range idx.symbols {
		idx.byName[sym.Name] = append(idx.byName[sym.Name], sym)
		idx.byKind[sym.Kind] = append(idx.byKind[sym.Kind], sym)
		idx.byPkg[sym.Package] = append(idx.byPkg[sym.Package], sym)
	}
}

// ExportMarkdown exports the index as a markdown document.
func (idx *Index) ExportMarkdown() string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var b strings.Builder
	stats := idx.Stats()

	fmt.Fprintf(&b, "# Code Navigation Index\n\n")
	fmt.Fprintf(&b, "**Symbols:** %d | **Files:** %d | **Packages:** %d | **Edges:** %d\n\n",
		stats.SymbolCount, stats.FileCount, stats.PackageCount, stats.EdgeCount)

	// Group by package
	pkgs := make([]string, 0, len(idx.byPkg))
	for pkg := range idx.byPkg {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	for _, pkg := range pkgs {
		fmt.Fprintf(&b, "## %s\n\n", pkg)
		syms := idx.byPkg[pkg]
		sort.Slice(syms, func(i, j int) bool {
			return syms[i].Name < syms[j].Name
		})
		for _, sym := range syms {
			kind := string(sym.Kind)
			if sym.Exported {
				kind = "**" + kind + "**"
			}
			fmt.Fprintf(&b, "- `%s` (%s) — %s:%d\n", sym.Name, kind, sym.File, sym.Line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Navigator provides high-level navigation operations.
type Navigator struct {
	idx    *Index
	dir    string
}

// NewNavigator creates a new code navigator.
func NewNavigator(dir string) *Navigator {
	return &Navigator{
		idx: NewIndex(filepath.Join(dir, ".forge", "navigate")),
		dir: dir,
	}
}

// Index returns the underlying index.
func (n *Navigator) Index() *Index { return n.idx }

// GoToDefinition navigates to the definition of a symbol.
func (n *Navigator) GoToDefinition(ctx context.Context, query string) (*NavigationResult, error) {
	result := n.idx.Search(query, 1)
	if len(result.Matches) == 0 {
		return nil, fmt.Errorf("no definition found for %q", query)
	}
	return result, nil
}

// FindReferences finds all references to a symbol.
func (n *Navigator) FindReferences(ctx context.Context, query string) (*NavigationResult, error) {
	syms := n.idx.Lookup(query)
	if len(syms) == 0 {
		return nil, fmt.Errorf("symbol %q not found", query)
	}

	start := time.Now()
	var matches []Match
	for _, sym := range syms {
		refs := n.idx.References(sym.ID)
		for _, ref := range refs {
			matches = append(matches, Match{
				Symbol:    Symbol{ID: ref.SymbolID, Name: sym.Name, File: ref.File, Line: ref.Line, Kind: sym.Kind},
				Relevance: 1.0,
				Reason:    fmt.Sprintf("reference (%s)", ref.Kind),
			})
		}
	}

	return &NavigationResult{
		Query:   query,
		Matches: matches,
		Duration: time.Since(start),
		Total:   len(matches),
	}, nil
}

// FindCallers finds all callers of a symbol.
func (n *Navigator) FindCallers(ctx context.Context, query string) (*NavigationResult, error) {
	syms := n.idx.Lookup(query)
	if len(syms) == 0 {
		return nil, fmt.Errorf("symbol %q not found", query)
	}

	start := time.Now()
	var matches []Match
	for _, sym := range syms {
		callerIDs := n.idx.Callers(sym.ID)
		for _, cid := range callerIDs {
			if caller, ok := n.idx.symbols[cid]; ok {
				matches = append(matches, Match{
					Symbol:    *caller,
					Relevance: 0.9,
					Reason:    "caller",
				})
			}
		}
	}

	return &NavigationResult{
		Query:   query,
		Matches: matches,
		Duration: time.Since(start),
		Total:   len(matches),
	}, nil
}

// TraceCallChain finds the call chain between two symbols.
func (n *Navigator) TraceCallChain(ctx context.Context, from, to string) (*CallPath, error) {
	fromSyms := n.idx.Lookup(from)
	toSyms := n.idx.Lookup(to)
	if len(fromSyms) == 0 {
		return nil, fmt.Errorf("symbol %q not found", from)
	}
	if len(toSyms) == 0 {
		return nil, fmt.Errorf("symbol %q not found", to)
	}

	return n.idx.FindCallPath(fromSyms[0].ID, toSyms[0].ID), nil
}

// SemanticSearch performs a semantic search across the codebase.
func (n *Navigator) SemanticSearch(ctx context.Context, query string, limit int) *NavigationResult {
	return n.idx.Search(query, limit)
}

// Helper functions

func symbolID(name, pkg, file string, line int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%d", name, pkg, file, line)))
	return fmt.Sprintf("%x", h[:12])
}
