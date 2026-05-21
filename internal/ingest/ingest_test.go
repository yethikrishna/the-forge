package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge/sword/internal/ingest"
)

func TestAddSource(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("test-doc", ingest.SourceFile, "/path/to/doc.txt")

	if s.ID == "" {
		t.Error("expected non-empty ID")
	}
	if s.Name != "test-doc" {
		t.Errorf("expected test-doc, got %s", s.Name)
	}
}

func TestGetSource(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("test", ingest.SourceFile, "test.txt")

	got, ok := p.GetSource(s.ID)
	if !ok {
		t.Error("expected to find source")
	}
	if got.Name != "test" {
		t.Errorf("expected test, got %s", got.Name)
	}
}

func TestListSources(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	p.AddSource("first", ingest.SourceFile, "a.txt")
	p.AddSource("second", ingest.SourceURL, "https://example.com")

	list := p.ListSources()
	if len(list) != 2 {
		t.Errorf("expected 2 sources, got %d", len(list))
	}
}

func TestDeleteSource(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("test", ingest.SourceFile, "test.txt")

	err := p.DeleteSource(s.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := p.GetSource(s.ID)
	if ok {
		t.Error("expected source to be deleted")
	}
}

func TestIngestFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("Hello world.\n\nThis is a test document.\n\nIt has multiple paragraphs."), 0644)

	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("test-doc", ingest.SourceFile, filePath)

	chunks, err := p.Ingest(s.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}

	got, _ := p.GetSource(s.ID)
	if got.ChunkCount != len(chunks) {
		t.Errorf("expected chunk count %d, got %d", len(chunks), got.ChunkCount)
	}
}

func TestIngestInline(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("inline-test", ingest.SourceInline, "This is inline content for testing.")

	chunks, err := p.Ingest(s.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestGetChunks(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("Line one.\n\nLine two.\n\nLine three."), 0644)

	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("test", ingest.SourceFile, filePath)
	p.Ingest(s.ID)

	chunks := p.GetChunks(s.ID)
	if len(chunks) == 0 {
		t.Error("expected chunks")
	}
}

func TestSearch(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("test", ingest.SourceInline, "The quick brown fox jumps over the lazy dog.")
	p.Ingest(s.ID)

	results := p.Search("brown fox", 10)
	if len(results) == 0 {
		t.Error("expected to find matching chunks")
	}
}

func TestDeduplication(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "Same content every time."
	os.WriteFile(filePath, []byte(content), 0644)

	p := ingest.NewPipeline(t.TempDir())
	s := p.AddSource("test", ingest.SourceFile, filePath)

	p.Ingest(s.ID)
	p.Ingest(s.ID) // second ingest should deduplicate

	chunks := p.GetChunks(s.ID)
	if len(chunks) > 1 {
		t.Errorf("expected deduplication, got %d chunks", len(chunks))
	}
}

func TestStats(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	p.AddSource("test1", ingest.SourceFile, "a.txt")
	p.AddSource("test2", ingest.SourceURL, "https://example.com")

	stats := p.Stats()
	if stats["sources"].(int) != 2 {
		t.Errorf("expected 2 sources, got %v", stats["sources"])
	}
}

func TestRenderSource(t *testing.T) {
	s := &ingest.Source{
		Name: "Test",
		Type: ingest.SourceFile,
		Path: "test.txt",
	}
	text := ingest.RenderSource(s)
	if text == "" {
		t.Error("expected non-empty render")
	}
}

func TestIngestAll(t *testing.T) {
	p := ingest.NewPipeline(t.TempDir())
	p.AddSource("inline1", ingest.SourceInline, "Content one.")
	p.AddSource("inline2", ingest.SourceInline, "Content two.")

	results, err := p.IngestAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}
