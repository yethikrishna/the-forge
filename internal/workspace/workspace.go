// Package workspace provides multi-repo context management for the forge.
// A workspace spans many forges, many anvils, one purpose.
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Repo represents a git repository in the workspace.
type Repo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	URL       string    `json:"url,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	LastSync  time.Time `json:"last_sync,omitempty"`
	Indexed   bool      `json:"indexed"`
	FileCount int       `json:"file_count"`
	LineCount int       `json:"line_count"`
	Languages []string  `json:"languages,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
}

// Workspace is a collection of repos.
type Workspace struct {
	Name        string           `json:"name"`
	Path        string           `json:"path"`
	Repos       map[string]*Repo `json:"repos"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Description string           `json:"description,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

// Manager manages workspaces.
type Manager struct {
	dir string
	mu  sync.RWMutex
}

// NewManager creates a workspace manager.
func NewManager(dir string) *Manager {
	os.MkdirAll(dir, 0o755)
	return &Manager{dir: dir}
}

// Create creates a new workspace.
func (m *Manager) Create(name, description string) (*Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wsPath := filepath.Join(m.dir, name)
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}

	ws := &Workspace{
		Name:        name,
		Path:        wsPath,
		Repos:       make(map[string]*Repo),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Description: description,
		Config:      make(map[string]string),
	}

	if err := m.save(ws); err != nil {
		return nil, err
	}

	return ws, nil
}

// Get retrieves a workspace.
func (m *Manager) Get(name string) (*Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.load(name)
}

// List returns all workspace names.
func (m *Manager) List() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			// Check for workspace.json
			if _, err := os.Stat(filepath.Join(m.dir, e.Name(), "workspace.json")); err == nil {
				names = append(names, e.Name())
			}
		}
	}

	return names, nil
}

// Delete removes a workspace.
func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wsPath := filepath.Join(m.dir, name)
	return os.RemoveAll(wsPath)
}

// AddRepo adds a repository to a workspace.
func (m *Manager) AddRepo(wsName, repoName, repoPath, url string) (*Repo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws, err := m.load(wsName)
	if err != nil {
		return nil, err
	}

	repo := &Repo{
		Name: repoName,
		Path: repoPath,
		URL:  url,
	}

	// Analyze repo
	m.analyzeRepo(repo)

	ws.Repos[repoName] = repo
	ws.UpdatedAt = time.Now().UTC()

	if err := m.save(ws); err != nil {
		return nil, err
	}

	return repo, nil
}

// RemoveRepo removes a repository from a workspace.
func (m *Manager) RemoveRepo(wsName, repoName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws, err := m.load(wsName)
	if err != nil {
		return nil
	}

	delete(ws.Repos, repoName)
	ws.UpdatedAt = time.Now().UTC()
	return m.save(ws)
}

// Search searches across all repos in a workspace.
func (m *Manager) Search(wsName, query string, limit int) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ws, err := m.load(wsName)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	query = strings.ToLower(query)

	for _, repo := range ws.Repos {
		// Simple file content search
		err := filepath.Walk(repo.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if len(results) >= limit {
				return filepath.SkipAll
			}

			// Skip binary and hidden files
			if strings.HasPrefix(info.Name(), ".") || info.Size() > 100*1024 {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			content := string(data)
			if strings.Contains(strings.ToLower(content), query) {
				relPath, _ := filepath.Rel(repo.Path, path)
				results = append(results, SearchResult{
					Repo:    repo.Name,
					Path:    relPath,
					Line:    0, // simplified
					Content: truncate(content, 200),
				})
			}

			return nil
		})
		if err != nil {
			continue
		}
	}

	return results, nil
}

// SearchResult is a search match.
type SearchResult struct {
	Repo    string `json:"repo"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// Stats returns workspace statistics.
func (m *Manager) Stats(wsName string) (map[string]interface{}, error) {
	ws, err := m.Get(wsName)
	if err != nil {
		return nil, err
	}

	totalFiles := 0
	totalLines := 0
	langSet := make(map[string]bool)

	for _, repo := range ws.Repos {
		totalFiles += repo.FileCount
		totalLines += repo.LineCount
		for _, lang := range repo.Languages {
			langSet[lang] = true
		}
	}

	var languages []string
	for lang := range langSet {
		languages = append(languages, lang)
	}

	return map[string]interface{}{
		"name":       ws.Name,
		"repos":      len(ws.Repos),
		"files":      totalFiles,
		"lines":      totalLines,
		"languages":  languages,
		"created_at": ws.CreatedAt,
		"updated_at": ws.UpdatedAt,
	}, nil
}

func (m *Manager) analyzeRepo(repo *Repo) {
	if repo.Path == "" {
		return
	}

	fileCount := 0
	lineCount := 0
	langMap := make(map[string]bool)

	filepath.Walk(repo.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden and vendor
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		if strings.Contains(path, "vendor/") || strings.Contains(path, "node_modules/") {
			return nil
		}

		fileCount++

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if lang := extToLang(ext); lang != "" {
			langMap[lang] = true
		}

		if info.Size() < 100*1024 {
			data, err := os.ReadFile(path)
			if err == nil {
				lineCount += strings.Count(string(data), "\n") + 1
			}
		}

		return nil
	})

	repo.FileCount = fileCount
	repo.LineCount = lineCount
	for lang := range langMap {
		repo.Languages = append(repo.Languages, lang)
	}
}

func extToLang(ext string) string {
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js", ".ts", ".jsx", ".tsx":
		return "JavaScript/TypeScript"
	case ".rs":
		return "Rust"
	case ".java":
		return "Java"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp":
		return "C++"
	case ".rb":
		return "Ruby"
	case ".sh", ".bash":
		return "Shell"
	case ".yaml", ".yml":
		return "YAML"
	case ".json":
		return "JSON"
	case ".md":
		return "Markdown"
	case ".sql":
		return "SQL"
	default:
		return ""
	}
}

func (m *Manager) save(ws *Workspace) error {
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	path := filepath.Join(m.dir, ws.Name, "workspace.json")
	return os.WriteFile(path, data, 0o644)
}

func (m *Manager) load(name string) (*Workspace, error) {
	path := filepath.Join(m.dir, name, "workspace.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %s", name)
	}

	var ws Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("invalid workspace: %w", err)
	}

	return &ws, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
