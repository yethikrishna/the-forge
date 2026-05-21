// Package find provides global search across Forge resources.
// Searches memory, sessions, pipelines, templates, and codebase.
//
// Find anything. Anywhere.
package find

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ResultType is the type of search result.
type ResultType string

const (
	TypeMemory   ResultType = "memory"
	TypeSession  ResultType = "session"
	TypePipeline ResultType = "pipeline"
	TypeTemplate ResultType = "template"
	TypeFile     ResultType = "file"
	TypeConfig   ResultType = "config"
	TypeCode     ResultType = "code"
)

// Result is a single search result.
type Result struct {
	Type      ResultType `json:"type"`
	Title     string     `json:"title"`
	Path      string     `json:"path"`
	Match     string     `json:"match"` // the matching line/snippet
	Line      int        `json:"line,omitempty"`
	Score     float64    `json:"score"` // relevance 0-1
	Timestamp time.Time  `json:"timestamp,omitempty"`
}

// Searcher performs global search across Forge resources.
type Searcher struct {
	workspaceDir string
	forgeDir     string
}

// NewSearcher creates a global searcher.
func NewSearcher(workspaceDir, forgeDir string) *Searcher {
	if forgeDir == "" {
		home, _ := os.UserHomeDir()
		forgeDir = filepath.Join(home, ".forge")
	}
	if workspaceDir == "" {
		workspaceDir, _ = os.Getwd()
	}
	return &Searcher{
		workspaceDir: workspaceDir,
		forgeDir:     forgeDir,
	}
}

// Search performs a global search with optional type filter.
func (s *Searcher) Search(query string, types []ResultType, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 50
	}

	if len(types) == 0 {
		types = []ResultType{TypeMemory, TypeSession, TypePipeline, TypeTemplate, TypeFile, TypeConfig, TypeCode}
	}

	typeSet := make(map[ResultType]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	var allResults []Result

	// Search memory files
	if typeSet[TypeMemory] {
		allResults = append(allResults, s.searchMemory(query)...)
	}

	// Search sessions
	if typeSet[TypeSession] {
		allResults = append(allResults, s.searchSessions(query)...)
	}

	// Search config
	if typeSet[TypeConfig] {
		allResults = append(allResults, s.searchConfig(query)...)
	}

	// Search code/workspace
	if typeSet[TypeCode] || typeSet[TypeFile] {
		allResults = append(allResults, s.searchWorkspace(query, typeSet)...)
	}

	// Sort by score descending
	sortResults(allResults)

	// Limit
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults, nil
}

func (s *Searcher) searchMemory(query string) []Result {
	var results []Result
	memoryDir := filepath.Join(s.workspaceDir, "memory")

	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return nil
	}

	pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(query))

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		fullPath := filepath.Join(memoryDir, e.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if pattern.MatchString(line) {
				results = append(results, Result{
					Type:  TypeMemory,
					Title: e.Name(),
					Path:  fullPath,
					Match: truncate(line, 120),
					Line:  i + 1,
					Score: scoreMatch(line, query),
				})
			}
		}
	}

	return results
}

func (s *Searcher) searchSessions(query string) []Result {
	var results []Result
	sessionsDir := filepath.Join(s.forgeDir, "sessions")

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}

	pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(query))

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		// Search transcript files
		transcriptFile := filepath.Join(sessionsDir, e.Name(), "transcript.json")
		data, err := os.ReadFile(transcriptFile)
		if err != nil {
			continue
		}

		if pattern.Match(data) {
			// Find the matching line
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if pattern.MatchString(line) {
					results = append(results, Result{
						Type:  TypeSession,
						Title: "Session " + e.Name(),
						Path:  transcriptFile,
						Match: truncate(line, 120),
						Line:  i + 1,
						Score: 0.7,
					})
					break // one match per session
				}
			}
		}
	}

	return results
}

func (s *Searcher) searchConfig(query string) []Result {
	var results []Result
	configFile := filepath.Join(s.forgeDir, "openclaw.json")

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil
	}

	pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(query))
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if pattern.MatchString(line) {
			results = append(results, Result{
				Type:  TypeConfig,
				Title: "openclaw.json",
				Path:  configFile,
				Match: truncate(line, 120),
				Line:  i + 1,
				Score: 0.8,
			})
		}
	}

	return results
}

func (s *Searcher) searchWorkspace(query string, typeSet map[ResultType]bool) []Result {
	var results []Result
	pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(query))

	// Walk workspace, limited depth
	filepath.Walk(s.workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// Skip common non-useful dirs
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only search text files
		ext := strings.ToLower(filepath.Ext(path))
		textExts := map[string]bool{
			".go": true, ".md": true, ".json": true, ".yaml": true, ".yml": true,
			".toml": true, ".txt": true, ".py": true, ".ts": true, ".js": true,
			".rs": true, ".html": true, ".css": true,
		}
		if !textExts[ext] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil || len(data) > 1024*1024 { // skip files > 1MB
			return nil
		}

		lines := strings.Split(string(data), "\n")
		matchCount := 0
		for i, line := range lines {
			if matchCount >= 3 { // max 3 matches per file
				break
			}
			if pattern.MatchString(line) {
				rtype := TypeFile
				if ext == ".go" {
					rtype = TypeCode
				}
				results = append(results, Result{
					Type:  rtype,
					Title: filepath.Base(path),
					Path:  path,
					Match: truncate(line, 120),
					Line:  i + 1,
					Score: scoreMatch(line, query),
				})
				matchCount++
			}
		}
		return nil
	})

	return results
}

func scoreMatch(line, query string) float64 {
	lower := strings.ToLower(line)
	q := strings.ToLower(query)

	// Exact match = high score
	if strings.Contains(lower, q) {
		score := 0.5
		// Boost if it's in a title/header
		if strings.HasPrefix(strings.TrimSpace(lower), "#") {
			score = 0.9
		}
		// Boost if short line (title-like)
		if len(line) < 80 {
			score += 0.1
		}
		return score
	}

	return 0.3
}

func sortResults(results []Result) {
	// Simple bubble sort by score (fine for small lists)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FormatResults formats search results for display.
func FormatResults(results []Result, query string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s\n", query)
	}

	s := fmt.Sprintf("Found %d results for: %s\n\n", len(results), query)
	for i, r := range results {
		s += fmt.Sprintf("  %d. [%s] %s\n", i+1, r.Type, r.Title)
		if r.Path != "" {
			rel := r.Path
			s += fmt.Sprintf("     %s", rel)
			if r.Line > 0 {
				s += fmt.Sprintf(":%d", r.Line)
			}
			s += "\n"
		}
		if r.Match != "" {
			s += fmt.Sprintf("     %s\n", r.Match)
		}
		s += "\n"
	}

	return s
}

// FormatResultsJSON formats results as JSON.
func FormatResultsJSON(results []Result) (string, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
