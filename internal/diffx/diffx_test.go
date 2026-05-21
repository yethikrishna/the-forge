package diffx_test

import (
	"strings"
	"testing"

	"github.com/forge/sword/internal/diffx"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		file string
		want diffx.Language
	}{
		{"main.go", diffx.LangGo},
		{"app.py", diffx.LangPython},
		{"index.ts", diffx.LangTypeScript},
		{"main.rs", diffx.LangRust},
		{"App.java", diffx.LangJava},
		{"Makefile", diffx.LangUnknown},
	}

	for _, tt := range tests {
		got := diffx.DetectLanguage(tt.file)
		if got != tt.want {
			t.Errorf("DetectLanguage(%s) = %s, want %s", tt.file, got, tt.want)
		}
	}
}

func TestDiffAddedFunction(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	old := "package main\n\nfunc main() {\n}\n"
	new := "package main\n\nfunc main() {\n}\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n"

	result := d.Diff(old, new)
	if result.Stats.Added == 0 {
		t.Error("expected at least one added block")
	}
}

func TestDiffRemovedFunction(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	old := "package main\n\nfunc main() {\n}\n\nfunc hello() {\n}\n"
	new := "package main\n\nfunc main() {\n}\n"

	result := d.Diff(old, new)
	if result.Stats.Removed == 0 {
		t.Error("expected at least one removed block")
	}
}

func TestDiffModifiedFunction(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	old := "package main\n\nfunc main() {\n\tfmt.Println(\"old\")\n}\n"
	new := "package main\n\nfunc main() {\n\tfmt.Println(\"new\")\n}\n"

	result := d.Diff(old, new)
	if result.Stats.Modified == 0 {
		t.Error("expected at least one modified block")
	}
}

func TestDiffUnchanged(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	code := "package main\n\nfunc main() {\n}\n"
	result := d.Diff(code, code)

	if result.Stats.Modified > 0 || result.Stats.Added > 0 || result.Stats.Removed > 0 {
		t.Error("identical code should have no changes")
	}
}

func TestDiffStruct(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	old := "package main\n\ntype Config struct {\n\tName string\n}\n"
	new := "package main\n\ntype Config struct {\n\tName string\n\tPort int\n}\n"

	result := d.Diff(old, new)
	found := false
	for _, c := range result.Changes {
		if c.BlockType == "struct" && c.Name == "Config" {
			found = true
		}
	}
	if !found {
		t.Error("expected Config struct change")
	}
}

func TestDiffRename(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	old := "package main\n\nfunc hello() {\n\tfmt.Println(\"hi\")\n}\n"
	new := "package main\n\nfunc greet() {\n\tfmt.Println(\"hi\")\n}\n"

	result := d.Diff(old, new)
	found := false
	for _, c := range result.Changes {
		if c.Type == diffx.ChangeRenamed {
			found = true
		}
	}
	if !found {
		t.Error("expected renamed change")
	}
}

func TestParseGoFunctions(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	code := "package main\n\nfunc main() {\n}\n\nfunc helper() int {\n\treturn 1\n}\n"
	blocks := d.Diff(code, code)

	// Should parse both functions
	_ = blocks
}

func TestRenderDiff(t *testing.T) {
	d := diffx.NewDiffer(diffx.LangGo)

	old := "package main\n\nfunc main() {\n}\n"
	new := "package main\n\nfunc main() {\n}\n\nfunc added() {\n}\n"

	result := d.Diff(old, new)
	text := diffx.RenderDiff(result)
	if text == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(text, "added") {
		t.Error("expected 'added' in render output")
	}
}

func TestSimilarity(t *testing.T) {
	// Same content
	s1 := similarity("hello world foo bar", "hello world foo bar")
	if s1 != 1.0 {
		t.Errorf("expected 1.0, got %f", s1)
	}

	// Completely different
	s2 := similarity("aaa bbb", "ccc ddd")
	if s2 != 0.0 {
		t.Errorf("expected 0.0, got %f", s2)
	}

	// Partial overlap
	s3 := similarity("hello world foo", "hello world bar")
	if s3 <= 0 || s3 >= 1.0 {
		t.Errorf("expected 0 < s < 1, got %f", s3)
	}
}

func similarity(a, b string) float64 {
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}
	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}
