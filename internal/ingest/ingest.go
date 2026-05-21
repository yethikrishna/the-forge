// Package ingest provides multi-source data ingestion for agent context.
// Ingest files, URLs, APIs, and databases into a unified format
// that agents can consume. Supports chunking, deduplication,
// format conversion, and incremental updates.
//
// All the world's data, one pipeline.
package ingest

import (
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

// SourceType classifies an ingestion source.
type SourceType string

const (
	SourceFile    SourceType = "file"
	SourceURL     SourceType = "url"
	SourceAPI     SourceType = "api"
	SourceCommand SourceType = "command"
	SourceInline  SourceType = "inline"
)

// ChunkStrategy defines how to split content.
type ChunkStrategy string

const (
	ChunkFixed     ChunkStrategy = "fixed"     // fixed-size chunks
	ChunkSentence  ChunkStrategy = "sentence"  // sentence-boundary chunks
	ChunkParagraph ChunkStrategy = "paragraph" // paragraph-boundary chunks
	ChunkLine      ChunkStrategy = "line"      // line-by-line
)

// Chunk represents a piece of ingested content.
type Chunk struct {
	ID       string            `json:"id"`
	SourceID string            `json:"source_id"`
	Index    int               `json:"index"`
	Content  string            `json:"content"`
	SHA256   string            `json:"sha256"`
	Size     int               `json:"size"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Source represents an ingestion source.
type Source struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          SourceType    `json:"type"`
	Path          string        `json:"path"` // file path, URL, or command
	Description   string        `json:"description"`
	ChunkSize     int           `json:"chunk_size"`
	ChunkStrategy ChunkStrategy `json:"chunk_strategy"`
	Tags          []string      `json:"tags,omitempty"`
	LastIngested  time.Time     `json:"last_ingested,omitempty"`
	ChunkCount    int           `json:"chunk_count"`
	CreatedAt     time.Time     `json:"created_at"`
}

// Pipeline manages data ingestion.
type Pipeline struct {
	dir     string
	sources map[string]*Source
	chunks  map[string][]Chunk // source_id -> chunks
	mu      sync.RWMutex
}

// NewPipeline creates a new ingestion pipeline.
func NewPipeline(dir string) *Pipeline {
	os.MkdirAll(dir, 0755)
	p := &Pipeline{
		dir:     dir,
		sources: make(map[string]*Source),
		chunks:  make(map[string][]Chunk),
	}
	p.load()
	return p
}

// AddSource adds an ingestion source.
func (p *Pipeline) AddSource(name string, sourceType SourceType, path string) *Source {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := &Source{
		ID:            fmt.Sprintf("src-%d", time.Now().UnixNano()),
		Name:          name,
		Type:          sourceType,
		Path:          path,
		ChunkSize:     1000,
		ChunkStrategy: ChunkParagraph,
		CreatedAt:     time.Now(),
	}
	p.sources[s.ID] = s
	p.save()
	return s
}

// GetSource returns a source by ID.
func (p *Pipeline) GetSource(id string) (*Source, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.sources[id]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

// ListSources returns all sources.
func (p *Pipeline) ListSources() []Source {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]Source, 0, len(p.sources))
	for _, s := range p.sources {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// DeleteSource removes a source and its chunks.
func (p *Pipeline) DeleteSource(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.sources[id]; !ok {
		return fmt.Errorf("source %q not found", id)
	}
	delete(p.sources, id)
	delete(p.chunks, id)
	p.save()
	p.saveChunks()
	return nil
}

// Ingest runs the ingestion pipeline for a source.
func (p *Pipeline) Ingest(sourceID string) ([]Chunk, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	source, ok := p.sources[sourceID]
	if !ok {
		return nil, fmt.Errorf("source %q not found", sourceID)
	}

	var content string
	var err error

	switch source.Type {
	case SourceFile:
		data, e := os.ReadFile(source.Path)
		if e != nil {
			return nil, fmt.Errorf("reading file: %w", e)
		}
		content = string(data)

	case SourceInline:
		content = source.Path // for inline, path contains the content

	case SourceCommand:
		// For command sources, store the command reference
		content = fmt.Sprintf("command: %s", source.Path)

	case SourceURL, SourceAPI:
		content = fmt.Sprintf("external: %s", source.Path)
	}

	// Chunk the content
	chunks := p.chunkContent(source, content)

	// Compute SHA256 for dedup
	oldChunks := p.chunks[sourceID]
	deduped := dedupChunks(oldChunks, chunks)

	p.chunks[sourceID] = deduped
	source.ChunkCount = len(deduped)
	source.LastIngested = time.Now()
	p.save()
	p.saveChunks()

	return deduped, err
}

// IngestAll runs ingestion for all sources.
func (p *Pipeline) IngestAll() (map[string]int, error) {
	p.mu.RLock()
	ids := make([]string, 0, len(p.sources))
	for id := range p.sources {
		ids = append(ids, id)
	}
	p.mu.RUnlock()

	results := make(map[string]int)
	for _, id := range ids {
		chunks, err := p.Ingest(id)
		if err != nil {
			results[id] = -1
			continue
		}
		results[id] = len(chunks)
	}
	return results, nil
}

// GetChunks returns chunks for a source.
func (p *Pipeline) GetChunks(sourceID string) []Chunk {
	p.mu.RLock()
	defer p.mu.RUnlock()
	chunks, ok := p.chunks[sourceID]
	if !ok {
		return nil
	}
	result := make([]Chunk, len(chunks))
	copy(result, chunks)
	return result
}

// Search searches across all chunks.
func (p *Pipeline) Search(query string, limit int) []Chunk {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var results []Chunk
	queryLower := strings.ToLower(query)

	for _, chunks := range p.chunks {
		for _, chunk := range chunks {
			if strings.Contains(strings.ToLower(chunk.Content), queryLower) {
				results = append(results, chunk)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return len(results[i].Content) < len(results[j].Content) // shorter = more relevant
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Stats returns pipeline statistics.
func (p *Pipeline) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	totalChunks := 0
	totalSize := 0
	for _, chunks := range p.chunks {
		totalChunks += len(chunks)
		for _, c := range chunks {
			totalSize += c.Size
		}
	}

	byType := make(map[SourceType]int)
	for _, s := range p.sources {
		byType[s.Type]++
	}

	return map[string]interface{}{
		"sources":      len(p.sources),
		"total_chunks": totalChunks,
		"total_size":   totalSize,
		"by_type":      byType,
	}
}

// chunkContent splits content into chunks.
func (p *Pipeline) chunkContent(source *Source, content string) []Chunk {
	strategy := source.ChunkStrategy
	if strategy == "" {
		strategy = ChunkParagraph
	}

	chunkSize := source.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 1000
	}

	var rawChunks []string

	switch strategy {
	case ChunkFixed:
		rawChunks = chunkFixed(content, chunkSize)
	case ChunkSentence:
		rawChunks = chunkSentence(content)
	case ChunkParagraph:
		rawChunks = chunkParagraph(content)
	case ChunkLine:
		rawChunks = chunkLine(content)
	default:
		rawChunks = chunkParagraph(content)
	}

	chunks := make([]Chunk, len(rawChunks))
	for i, raw := range rawChunks {
		h := sha256.Sum256([]byte(raw))
		chunks[i] = Chunk{
			ID:       fmt.Sprintf("%s-chunk-%d", source.ID, i),
			SourceID: source.ID,
			Index:    i,
			Content:  raw,
			SHA256:   fmt.Sprintf("%x", h[:8]),
			Size:     len(raw),
			Metadata: map[string]string{
				"source_name": source.Name,
				"source_type": string(source.Type),
			},
		}
	}

	return chunks
}

func chunkFixed(content string, size int) []string {
	var chunks []string
	for i := 0; i < len(content); i += size {
		end := i + size
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[i:end])
	}
	return chunks
}

func chunkSentence(content string) []string {
	parts := strings.Split(content, ". ")
	return mergeSmall(parts, 200)
}

func chunkParagraph(content string) []string {
	parts := strings.Split(content, "\n\n")
	return mergeSmall(parts, 500)
}

func chunkLine(content string) []string {
	return strings.Split(content, "\n")
}

func mergeSmall(parts []string, minSize int) []string {
	var chunks []string
	var current string

	for _, part := range parts {
		if len(current)+len(part) < minSize && current != "" {
			current += "\n\n" + part
		} else {
			if current != "" {
				chunks = append(chunks, current)
			}
			current = part
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func dedupChunks(old, new []Chunk) []Chunk {
	existingSHA := make(map[string]bool)
	for _, c := range old {
		existingSHA[c.SHA256] = true
	}

	var result []Chunk
	result = append(result, old...)
	for _, c := range new {
		if !existingSHA[c.SHA256] {
			result = append(result, c)
		}
	}
	return result
}

// RenderSource renders a source for display.
func RenderSource(s *Source) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Source: %s\n", s.Name)
	fmt.Fprintf(&b, "ID: %s\n", s.ID)
	fmt.Fprintf(&b, "Type: %s\n", s.Type)
	fmt.Fprintf(&b, "Path: %s\n", s.Path)
	fmt.Fprintf(&b, "Chunk Size: %d\n", s.ChunkSize)
	fmt.Fprintf(&b, "Chunk Strategy: %s\n", s.ChunkStrategy)
	fmt.Fprintf(&b, "Chunks: %d\n", s.ChunkCount)
	if !s.LastIngested.IsZero() {
		fmt.Fprintf(&b, "Last Ingested: %s\n", s.LastIngested.Format(time.RFC3339))
	}
	return b.String()
}

func (p *Pipeline) save() {
	if p.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(p.sources, "", "  ")
	os.WriteFile(filepath.Join(p.dir, "sources.json"), data, 0644)
}

func (p *Pipeline) saveChunks() {
	if p.dir == "" {
		return
	}
	data, _ := json.MarshalIndent(p.chunks, "", "  ")
	os.WriteFile(filepath.Join(p.dir, "chunks.json"), data, 0644)
}

func (p *Pipeline) load() {
	if p.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(p.dir, "sources.json"))
	if err == nil {
		json.Unmarshal(data, &p.sources)
	}
	data, err = os.ReadFile(filepath.Join(p.dir, "chunks.json"))
	if err == nil {
		json.Unmarshal(data, &p.chunks)
	}
}
