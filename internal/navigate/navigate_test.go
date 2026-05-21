package navigate

import (
	"context"
	"fmt"
	"testing"
)

func TestNewIndex(t *testing.T) {
	idx := NewIndex(t.TempDir())
	if idx == nil {
		t.Fatal("NewIndex should return an index")
	}
}

func TestAddSymbol(t *testing.T) {
	idx := NewIndex(t.TempDir())

	err := idx.AddSymbol(Symbol{
		Name:    "HandleRequest",
		Kind:    SymbolFunction,
		Package: "http",
		File:    "handler.go",
		Line:    42,
	})
	if err != nil {
		t.Fatalf("AddSymbol error: %v", err)
	}

	stats := idx.Stats()
	if stats.SymbolCount != 1 {
		t.Errorf("SymbolCount = %d, want 1", stats.SymbolCount)
	}
}

func TestAddSymbolDuplicate(t *testing.T) {
	idx := NewIndex(t.TempDir())
	sym := Symbol{Name: "Foo", Kind: SymbolFunction, Package: "pkg", File: "f.go", Line: 1}
	idx.AddSymbol(sym)

	// Same ID should error
	err := idx.AddSymbol(sym)
	if err == nil {
		t.Error("Adding duplicate symbol should error")
	}
}

func TestLookup(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "HandleRequest", Kind: SymbolFunction, Package: "http", File: "h.go", Line: 10})
	idx.AddSymbol(Symbol{Name: "HandleRequest", Kind: SymbolMethod, Package: "server", File: "s.go", Line: 20})

	syms := idx.Lookup("HandleRequest")
	if len(syms) != 2 {
		t.Errorf("Lookup = %d symbols, want 2", len(syms))
	}
}

func TestLookupInPackage(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "Run", Kind: SymbolFunction, Package: "cmd", File: "main.go", Line: 10})
	idx.AddSymbol(Symbol{Name: "Run", Kind: SymbolFunction, Package: "test", File: "test.go", Line: 5})

	syms := idx.LookupInPackage("Run", "cmd")
	if len(syms) != 1 {
		t.Errorf("LookupInPackage = %d, want 1", len(syms))
	}
	if syms[0].Package != "cmd" {
		t.Errorf("Package = %q, want %q", syms[0].Package, "cmd")
	}
}

func TestDefinition(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "server.go", Line: 15})

	syms := idx.Lookup("Serve")
	if len(syms) == 0 {
		t.Fatal("No symbols found")
	}

	sym, err := idx.Definition(syms[0].ID)
	if err != nil {
		t.Fatalf("Definition error: %v", err)
	}
	if sym.Name != "Serve" {
		t.Errorf("Name = %q, want %q", sym.Name, "Serve")
	}
}

func TestDefinitionNotFound(t *testing.T) {
	idx := NewIndex(t.TempDir())
	_, err := idx.Definition("nonexistent")
	if err == nil {
		t.Error("Should error for nonexistent symbol")
	}
}

func TestAddReference(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "s.go", Line: 10})

	syms := idx.Lookup("Serve")
	err := idx.AddReference(Reference{
		SymbolID: syms[0].ID,
		File:     "main.go",
		Line:     30,
		Kind:     "call",
		Context:  "http.Serve()",
	})
	if err != nil {
		t.Fatalf("AddReference error: %v", err)
	}

	refs := idx.References(syms[0].ID)
	if len(refs) != 1 {
		t.Errorf("References = %d, want 1", len(refs))
	}
}

func TestCallGraph(t *testing.T) {
	idx := NewIndex(t.TempDir())

	idx.AddSymbol(Symbol{Name: "main", Kind: SymbolFunction, Package: "cmd", File: "main.go", Line: 1, Callees: []string{"serve-id"}})
	idx.AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "server.go", Line: 10, ID: "serve-id"})

	callers := idx.Callers("serve-id")
	if len(callers) != 1 {
		t.Errorf("Callers = %d, want 1", len(callers))
	}

	callees := idx.Callees(idx.Lookup("main")[0].ID)
	if len(callees) != 1 {
		t.Errorf("Callees = %d, want 1", len(callees))
	}
}

func TestFindCallPath(t *testing.T) {
	idx := NewIndex(t.TempDir())

	idx.AddSymbol(Symbol{Name: "A", Kind: SymbolFunction, Package: "p", File: "a.go", Line: 1, ID: "a", Callees: []string{"b"}})
	idx.AddSymbol(Symbol{Name: "B", Kind: SymbolFunction, Package: "p", File: "b.go", Line: 1, ID: "b", Callees: []string{"c"}})
	idx.AddSymbol(Symbol{Name: "C", Kind: SymbolFunction, Package: "p", File: "c.go", Line: 1, ID: "c"})

	path := idx.FindCallPath("a", "c")
	if path == nil {
		t.Fatal("Should find call path A → B → C")
	}
	if path.Depth != 2 {
		t.Errorf("Depth = %d, want 2", path.Depth)
	}
	if len(path.Path) != 3 {
		t.Errorf("Path length = %d, want 3", len(path.Path))
	}
}

func TestFindCallPathNotFound(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "A", Kind: SymbolFunction, Package: "p", File: "a.go", Line: 1, ID: "a"})
	idx.AddSymbol(Symbol{Name: "B", Kind: SymbolFunction, Package: "p", File: "b.go", Line: 1, ID: "b"})

	path := idx.FindCallPath("a", "b")
	if path != nil {
		t.Error("Should not find path between unconnected nodes")
	}
}

func TestFindCallPathSelf(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "A", Kind: SymbolFunction, Package: "p", File: "a.go", Line: 1, ID: "a"})

	path := idx.FindCallPath("a", "a")
	if path == nil {
		t.Fatal("Self-path should work")
	}
	if path.Depth != 0 {
		t.Errorf("Self-path depth = %d, want 0", path.Depth)
	}
}

func TestSearchExact(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "HandleRequest", Kind: SymbolFunction, Package: "http", File: "h.go", Line: 10})
	idx.AddSymbol(Symbol{Name: "HandleResponse", Kind: SymbolFunction, Package: "http", File: "h.go", Line: 20})

	result := idx.Search("HandleRequest", 10)
	if len(result.Matches) == 0 {
		t.Fatal("Search should find matches")
	}
	if result.Matches[0].Symbol.Name != "HandleRequest" {
		t.Errorf("First match = %q, want %q", result.Matches[0].Symbol.Name, "HandleRequest")
	}
	if result.Matches[0].Relevance != 1.0 {
		t.Errorf("Exact match relevance = %.2f, want 1.0", result.Matches[0].Relevance)
	}
}

func TestSearchSubstring(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "HandleRequest", Kind: SymbolFunction, Package: "http", File: "h.go", Line: 10})
	idx.AddSymbol(Symbol{Name: "HandleResponse", Kind: SymbolFunction, Package: "http", File: "h.go", Line: 20})
	idx.AddSymbol(Symbol{Name: "ProcessData", Kind: SymbolFunction, Package: "data", File: "d.go", Line: 5})

	result := idx.Search("Handle", 10)
	if result.Total < 2 {
		t.Errorf("Should find at least 2 'Handle' symbols, got %d", result.Total)
	}
}

func TestSearchPackageQualified(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "Println", Kind: SymbolFunction, Package: "fmt", File: "print.go", Line: 10})
	idx.AddSymbol(Symbol{Name: "Println", Kind: SymbolFunction, Package: "log", File: "log.go", Line: 5})

	result := idx.Search("fmt.Println", 10)
	if len(result.Matches) == 0 {
		t.Fatal("Package-qualified search should find matches")
	}
	if result.Matches[0].Symbol.Package != "fmt" {
		t.Errorf("Package = %q, want %q", result.Matches[0].Symbol.Package, "fmt")
	}
}

func TestSearchLimit(t *testing.T) {
	idx := NewIndex(t.TempDir())
	for i := 0; i < 20; i++ {
		idx.AddSymbol(Symbol{
			Name:    fmt.Sprintf("Handler%d", i),
			Kind:    SymbolFunction,
			Package: "http",
			File:    "h.go",
			Line:    i + 1,
		})
	}

	result := idx.Search("Handler", 5)
	if len(result.Matches) > 5 {
		t.Errorf("Should limit to 5 matches, got %d", len(result.Matches))
	}
	if !result.Truncated {
		t.Error("Should be truncated with limit 5")
	}
}

func TestSymbolsByFile(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "A", Kind: SymbolFunction, Package: "p", File: "a.go", Line: 1})
	idx.AddSymbol(Symbol{Name: "B", Kind: SymbolFunction, Package: "p", File: "a.go", Line: 5})
	idx.AddSymbol(Symbol{Name: "C", Kind: SymbolFunction, Package: "p", File: "b.go", Line: 1})

	syms := idx.SymbolsByFile("a.go")
	if len(syms) != 2 {
		t.Errorf("SymbolsByFile = %d, want 2", len(syms))
	}
}

func TestSymbolsByKind(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "A", Kind: SymbolFunction, Package: "p", File: "f.go", Line: 1})
	idx.AddSymbol(Symbol{Name: "MyType", Kind: SymbolType, Package: "p", File: "f.go", Line: 5})
	idx.AddSymbol(Symbol{Name: "B", Kind: SymbolFunction, Package: "p", File: "f.go", Line: 10})

	fns := idx.SymbolsByKind(SymbolFunction)
	if len(fns) != 2 {
		t.Errorf("Functions = %d, want 2", len(fns))
	}
}

func TestRemoveFile(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "A", Kind: SymbolFunction, Package: "p", File: "a.go", Line: 1})
	idx.AddSymbol(Symbol{Name: "B", Kind: SymbolFunction, Package: "p", File: "b.go", Line: 1})

	idx.RemoveFile("a.go")

	if len(idx.SymbolsByFile("a.go")) != 0 {
		t.Error("File symbols should be removed")
	}
	if idx.Stats().SymbolCount != 1 {
		t.Errorf("SymbolCount = %d, want 1", idx.Stats().SymbolCount)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	idx := NewIndex(dir)

	idx.AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "s.go", Line: 10})
	idx.AddEdge("serve-id", "listen-id")

	if err := idx.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	idx2 := NewIndex(dir)
	if err := idx2.Load(); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if idx2.Stats().SymbolCount != 1 {
		t.Errorf("Loaded SymbolCount = %d, want 1", idx2.Stats().SymbolCount)
	}
}

func TestNavigator(t *testing.T) {
	nav := NewNavigator(t.TempDir())
	if nav == nil {
		t.Fatal("NewNavigator should return a navigator")
	}
}

func TestNavigatorGoToDefinition(t *testing.T) {
	nav := NewNavigator(t.TempDir())
	nav.Index().AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "s.go", Line: 10})

	result, err := nav.GoToDefinition(context.Background(), "Serve")
	if err != nil {
		t.Fatalf("GoToDefinition error: %v", err)
	}
	if len(result.Matches) == 0 {
		t.Error("Should find definition")
	}
}

func TestNavigatorFindReferences(t *testing.T) {
	nav := NewNavigator(t.TempDir())
	nav.Index().AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "s.go", Line: 10})

	syms := nav.Index().Lookup("Serve")
	nav.Index().AddReference(Reference{SymbolID: syms[0].ID, File: "main.go", Line: 30, Kind: "call"})

	result, err := nav.FindReferences(context.Background(), "Serve")
	if err != nil {
		t.Fatalf("FindReferences error: %v", err)
	}
	if result.Total < 1 {
		t.Error("Should find at least 1 reference")
	}
}

func TestNavigatorTraceCallChain(t *testing.T) {
	nav := NewNavigator(t.TempDir())
	nav.Index().AddSymbol(Symbol{Name: "main", Kind: SymbolFunction, Package: "cmd", File: "main.go", Line: 1, Callees: []string{"serve-id"}})
	nav.Index().AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "s.go", Line: 10, ID: "serve-id"})

	path, err := nav.TraceCallChain(context.Background(), "main", "Serve")
	if err != nil {
		t.Fatalf("TraceCallChain error: %v", err)
	}
	if path == nil {
		t.Fatal("Should find call path")
	}
}

func TestExportMarkdown(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "Serve", Kind: SymbolFunction, Package: "http", File: "s.go", Line: 10})
	idx.AddSymbol(Symbol{Name: "Handler", Kind: SymbolInterface, Package: "http", File: "s.go", Line: 20})

	md := idx.ExportMarkdown()
	if md == "" {
		t.Error("ExportMarkdown should not be empty")
	}
	if !contains(md, "Serve") {
		t.Error("Markdown should contain symbol names")
	}
}

func TestIndexStats(t *testing.T) {
	idx := NewIndex(t.TempDir())
	idx.AddSymbol(Symbol{Name: "A", Kind: SymbolFunction, Package: "p1", File: "a.go", Line: 1})
	idx.AddSymbol(Symbol{Name: "B", Kind: SymbolType, Package: "p2", File: "b.go", Line: 1})

	stats := idx.Stats()
	if stats.SymbolCount != 2 {
		t.Errorf("SymbolCount = %d, want 2", stats.SymbolCount)
	}
	if stats.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", stats.FileCount)
	}
	if stats.PackageCount != 2 {
		t.Errorf("PackageCount = %d, want 2", stats.PackageCount)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
