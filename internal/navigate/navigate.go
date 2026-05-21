// Package navigate provides semantic code navigation using codebase indexing
// and LLM-intent understanding. Unlike grep or basic search, navigate
// understands code structure: functions, types, interfaces, imports,
// call chains, and cross-file relationships.
//
// Find anything, understand everything.
package navigate

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// SymbolKind classifies a code symbol.
type SymbolKind string

const (
	KindFunction  SymbolKind = "function"
	KindMethod    SymbolKind = "method"
	KindType      SymbolKind = "type"
	KindInterface SymbolKind = "interface"
	KindStruct    SymbolKind = "struct"
	KindConst     SymbolKind = "const"
	KindVar       SymbolKind = "var"
	KindPackage   SymbolKind = "package"
	KindImport    SymbolKind = "import"
	KindField     SymbolKind = "field"
	KindEnum      SymbolKind = "enum"
	KindTrait     SymbolKind = "trait"
	KindClass     SymbolKind = "class"
)

// Symbol represents a navigable code symbol.
type Symbol struct {
	Name      string     `json:"name"`
	Kind      SymbolKind `json:"kind"`
	File      string     `json:"file"`
	Line      int        `json:"line"`
	EndLine   int        `json:"end_line"`
	Package   string     `json:"package,omitempty"`
	Signature string     `json:"signature,omitempty"`
	Doc       string     `json:"doc,omitempty"`
	Receivers []string   `json:"receivers,omitempty"` // Go: method receivers
	Exports   bool       `json:"exports"`
	Children  []string   `json:"children,omitempty"` // child symbol IDs
}

// Reference represents a usage/reference of a symbol.
type Reference struct {
	SymbolID string `json:"symbol_id"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Kind     string `json:"kind"` // "call", "type-ref", "import", "assign"
}

// CallEdge represents a call relationship.
type CallEdge struct {
	From string `json:"from"` // caller symbol ID
	To   string `json:"to"`   // callee symbol ID
	File string `json:"file"`
	Line int    `json:"line"`
}

// Index holds the navigable code index.
type Index struct {
	Symbols    map[string]*Symbol    `json:"symbols"`    // ID → Symbol
	References map[string][]Reference `json:"references"` // symbol ID → references
	Calls      []CallEdge            `json:"calls"`
	FileIndex  map[string][]string   `json:"file_index"` // file → symbol IDs
	RootDir    string                `json:"root_dir"`
	Language   string                `json:"language"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// Navigator provides semantic code navigation.
type Navigator struct {
	mu      sync.RWMutex
	index   *Index
	rootDir string
}

// NewNavigator creates a new code navigator.
func NewNavigator(rootDir string) *Navigator {
	return &Navigator{
		rootDir: rootDir,
		index: &Index{
			Symbols:    make(map[string]*Symbol),
			References: make(map[string][]Reference),
			Calls:      make([]CallEdge, 0),
			FileIndex:  make(map[string][]string),
			RootDir:    rootDir,
		},
	}
}

// IndexDir indexes all Go files in the directory tree.
func (n *Navigator) IndexDir() (*Index, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.index = &Index{
		Symbols:    make(map[string]*Symbol),
		References: make(map[string][]Reference),
		Calls:      make([]CallEdge, 0),
		FileIndex:  make(map[string][]string),
		RootDir:    n.rootDir,
		UpdatedAt:  time.Now(),
	}

	err := filepath.WalkDir(n.rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip common non-code directories
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" ||
				base == "__pycache__" || base == ".venv" || base == "dist" ||
				base == "build" || base == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		switch ext {
		case ".go":
			n.indexGoFile(path)
		case ".py":
			n.indexPythonFile(path)
		case ".ts", ".tsx":
			n.indexTypeScriptFile(path)
		case ".rs":
			n.indexRustFile(path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return n.index, nil
}

// Go symbol patterns
var (
	goFuncRe    = regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`)
	goMethodRe  = regexp.MustCompile(`^func\s+\(([^)]+)\)\s+(\w+)\s*\(`)
	goTypeRe    = regexp.MustCompile(`^type\s+(\w+)\s+(struct|interface)`)
	goConstRe   = regexp.MustCompile(`^const\s+(\w+)`)
	goVarRe     = regexp.MustCompile(`^var\s+(\w+)`)
	goPackageRe = regexp.MustCompile(`^package\s+(\w+)`)
	goImportRe  = regexp.MustCompile(`"([^"]+)"`)
)

// Python symbol patterns
var (
	pyFuncRe   = regexp.MustCompile(`^def\s+(\w+)\s*\(`)
	pyClassRe  = regexp.MustCompile(`^class\s+(\w+)`)
	pyImportRe = regexp.MustCompile(`^(?:from|import)\s+([\w.]+)`)
)

// TypeScript symbol patterns
var (
	tsFuncRe      = regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`)
	tsClassRe     = regexp.MustCompile(`(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	tsInterfaceRe = regexp.MustCompile(`(?:export\s+)?interface\s+(\w+)`)
	tsConstRe     = regexp.MustCompile(`(?:export\s+)?const\s+(\w+)`)
	tsTypeRe      = regexp.MustCompile(`(?:export\s+)?type\s+(\w+)`)
)

// Rust symbol patterns
var (
	rsFuncRe  = regexp.MustCompile(`(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`)
	rsStructRe = regexp.MustCompile(`(?:pub\s+)?struct\s+(\w+)`)
	rsEnumRe   = regexp.MustCompile(`(?:pub\s+)?enum\s+(\w+)`)
	rsTraitRe  = regexp.MustCompile(`(?:pub\s+)?trait\s+(\w+)`)
	rsImplRe   = regexp.MustCompile(`impl\s+(?:<[^>]+>\s+)?(\w+)`)
)

func (n *Navigator) indexGoFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	relPath, _ := filepath.Rel(n.rootDir, path)
	pkgName := ""
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Package
		if m := goPackageRe.FindStringSubmatch(line); m != nil {
			pkgName = m[1]
			continue
		}

		// Method (must check before func)
		if m := goMethodRe.FindStringSubmatch(line); m != nil {
			receiver := strings.TrimSpace(m[1])
			methodName := m[2]
			id := symbolID(relPath, lineNum)
			exports := strings.HasPrefix(methodName, strings.ToUpper(string(methodName[0])))

			n.index.Symbols[id] = &Symbol{
				Name:      methodName,
				Kind:      KindMethod,
				File:      relPath,
				Line:      lineNum,
				Package:   pkgName,
				Signature: line,
				Receivers: []string{receiver},
				Exports:   exports,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
			continue
		}

		// Function
		if m := goFuncRe.FindStringSubmatch(line); m != nil {
			funcName := m[1]
			id := symbolID(relPath, lineNum)
			exports := strings.HasPrefix(funcName, strings.ToUpper(string(funcName[0])))

			n.index.Symbols[id] = &Symbol{
				Name:      funcName,
				Kind:      KindFunction,
				File:      relPath,
				Line:      lineNum,
				Package:   pkgName,
				Signature: line,
				Exports:   exports,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
			continue
		}

		// Type (struct/interface)
		if m := goTypeRe.FindStringSubmatch(line); m != nil {
			typeName := m[1]
			kind := KindStruct
			if m[2] == "interface" {
				kind = KindInterface
			}
			id := symbolID(relPath, lineNum)

			n.index.Symbols[id] = &Symbol{
				Name:    typeName,
				Kind:    kind,
				File:    relPath,
				Line:    lineNum,
				Package: pkgName,
				Exports: strings.HasPrefix(typeName, strings.ToUpper(string(typeName[0]))),
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
			continue
		}

		// Const
		if m := goConstRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name:    m[1],
				Kind:    KindConst,
				File:    relPath,
				Line:    lineNum,
				Package: pkgName,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
			continue
		}

		// Var
		if m := goVarRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name:    m[1],
				Kind:    KindVar,
				File:    relPath,
				Line:    lineNum,
				Package: pkgName,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		}
	}
}

func (n *Navigator) indexPythonFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	relPath, _ := filepath.Rel(n.rootDir, path)
	pkgName := filepath.Dir(relPath)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if m := pyClassRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name:    m[1],
				Kind:    KindClass,
				File:    relPath,
				Line:    lineNum,
				Package: pkgName,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := pyFuncRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name:    m[1],
				Kind:    KindFunction,
				File:    relPath,
				Line:    lineNum,
				Package: pkgName,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		}
	}
}

func (n *Navigator) indexTypeScriptFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	relPath, _ := filepath.Rel(n.rootDir, path)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if m := tsInterfaceRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindInterface, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := tsClassRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindClass, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := tsFuncRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindFunction, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := tsTypeRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindType, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := tsConstRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindConst, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		}
	}
}

func (n *Navigator) indexRustFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	relPath, _ := filepath.Rel(n.rootDir, path)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if m := rsTraitRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindTrait, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := rsStructRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindStruct, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := rsEnumRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindEnum, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		} else if m := rsFuncRe.FindStringSubmatch(line); m != nil {
			id := symbolID(relPath, lineNum)
			n.index.Symbols[id] = &Symbol{
				Name: m[1], Kind: KindFunction, File: relPath, Line: lineNum,
			}
			n.index.FileIndex[relPath] = append(n.index.FileIndex[relPath], id)
		}
	}
}

// Navigate performs a semantic navigation query.
type NavigateQuery struct {
	Name     string // symbol name (exact or partial)
	Kind     SymbolKind
	File     string // file path filter
	Package  string // package filter
	Exported bool   // only exported symbols
	Limit    int
}

// Navigate searches the index for matching symbols.
func (n *Navigator) Navigate(query NavigateQuery) []*Symbol {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var results []*Symbol
	for _, sym := range n.index.Symbols {
		if !matchesQuery(sym, query) {
			continue
		}
		results = append(results, sym)
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	sort.Slice(results, func(i, j int) bool {
		// Prefer exported symbols
		if results[i].Exports != results[j].Exports {
			return results[i].Exports
		}
		// Then by kind (types before functions)
		kindOrder := map[SymbolKind]int{
			KindInterface: 0, KindStruct: 1, KindType: 2,
			KindFunction: 3, KindMethod: 4, KindConst: 5, KindVar: 6,
		}
		oi, _ := kindOrder[results[i].Kind]
		oj, _ := kindOrder[results[j].Kind]
		if oi != oj {
			return oi < oj
		}
		return results[i].File < results[j].File
	})

	return results
}

func matchesQuery(sym *Symbol, q NavigateQuery) bool {
	if q.Name != "" {
		lower := strings.ToLower(sym.Name)
		queryLower := strings.ToLower(q.Name)
		if lower != queryLower && !strings.Contains(lower, queryLower) {
			return false
		}
	}
	if q.Kind != "" && sym.Kind != q.Kind {
		return false
	}
	if q.File != "" && !strings.Contains(sym.File, q.File) {
		return false
	}
	if q.Package != "" && sym.Package != q.Package && !strings.Contains(sym.Package, q.Package) {
		return false
	}
	if q.Exported && !sym.Exports {
		return false
	}
	return true
}

// FindDefinition finds the definition of a symbol by name.
func (n *Navigator) FindDefinition(name string) []*Symbol {
	return n.Navigate(NavigateQuery{Name: name, Limit: 10})
}

// FindInFile returns all symbols in a file.
func (n *Navigator) FindInFile(file string) []*Symbol {
	n.mu.RLock()
	defer n.mu.RUnlock()

	ids, ok := n.index.FileIndex[file]
	if !ok {
		return nil
	}

	results := make([]*Symbol, 0, len(ids))
	for _, id := range ids {
		if sym, ok := n.index.Symbols[id]; ok {
			results = append(results, sym)
		}
	}
	return results
}

// Outline returns the symbol outline for a file.
func (n *Navigator) Outline(file string) string {
	symbols := n.FindInFile(file)
	if len(symbols) == 0 {
		return "No symbols found in " + file
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Outline: %s\n", file)
	fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 60))
	for _, sym := range symbols {
		kind := string(sym.Kind)
		export := ""
		if sym.Exports {
			export = " ★"
		}
		fmt.Fprintf(&b, "  %4d  %-12s  %s%s\n", sym.Line, kind, sym.Name, export)
	}
	return b.String()
}

// Callers finds all symbols that call the given symbol.
func (n *Navigator) Callers(symbolName string) []CallEdge {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var results []CallEdge
	for _, edge := range n.index.Calls {
		if caller, ok := n.index.Symbols[edge.To]; ok && caller.Name == symbolName {
			results = append(results, edge)
		}
	}
	return results
}

// Stats returns index statistics.
func (n *Navigator) Stats() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()

	kindCount := make(map[SymbolKind]int)
	fileCount := make(map[string]bool)
	pkgCount := make(map[string]bool)
	exported := 0

	for _, sym := range n.index.Symbols {
		kindCount[sym.Kind]++
		fileCount[sym.File] = true
		if sym.Package != "" {
			pkgCount[sym.Package] = true
		}
		if sym.Exports {
			exported++
		}
	}

	return map[string]interface{}{
		"total_symbols":  len(n.index.Symbols),
		"total_refs":     len(n.index.References),
		"total_calls":    len(n.index.Calls),
		"files_indexed":  len(fileCount),
		"packages":       len(pkgCount),
		"exported":       exported,
		"by_kind":        kindCount,
		"last_indexed":   n.index.UpdatedAt,
	}
}

// Search performs a fuzzy search across symbol names and signatures.
func (n *Navigator) Search(query string, limit int) []*Symbol {
	if limit <= 0 {
		limit = 20
	}
	return n.Navigate(NavigateQuery{Name: query, Limit: limit})
}

// SymbolTree returns all symbols organized by file.
func (n *Navigator) SymbolTree() map[string][]*Symbol {
	n.mu.RLock()
	defer n.mu.RUnlock()

	tree := make(map[string][]*Symbol)
	for file, ids := range n.index.FileIndex {
		symbols := make([]*Symbol, 0, len(ids))
		for _, id := range ids {
			if sym, ok := n.index.Symbols[id]; ok {
				symbols = append(symbols, sym)
			}
		}
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].Line < symbols[j].Line
		})
		tree[file] = symbols
	}
	return tree
}

func symbolID(file string, line int) string {
	return fmt.Sprintf("%s:%d", file, line)
}
