// Package snapshot provides project state snapshots that capture the full
// state of a project at a point in time — files, config, git info, and
// agent context. Snapshots enable time-travel debugging and state comparison.
package snapshot

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Type represents the snapshot type.
type Type string

const (
	TypeManual  Type = "manual"
	TypeAuto    Type = "auto"
	TypePreOp   Type = "pre-operation"
	TypePostOp  Type = "post-operation"
	TypeMilestone Type = "milestone"
)

// FileEntry represents a file in the snapshot.
type FileEntry struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// Snapshot represents a project state snapshot.
type Snapshot struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        Type        `json:"type"`
	Description string      `json:"description"`
	CreatedAt   time.Time   `json:"created_at"`
	ProjectDir  string      `json:"project_dir"`
	GitBranch   string      `json:"git_branch,omitempty"`
	GitCommit   string      `json:"git_commit,omitempty"`
	GitDirty    bool        `json:"git_dirty"`
	Files       []FileEntry `json:"files"`
	FileCount   int         `json:"file_count"`
	TotalSize   int64       `json:"total_size"`
	Tags        []string    `json:"tags"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Diff represents differences between two snapshots.
type Diff struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
	Unchanged int     `json:"unchanged"`
}

// Store manages snapshots.
type Store struct {
	mu        sync.RWMutex
	dir       string
	snapshots map[string]*Snapshot
}

// NewStore creates a new snapshot store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}
	s := &Store{
		dir:       dir,
		snapshots: make(map[string]*Snapshot),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err == nil {
			s.snapshots[snap.ID] = &snap
		}
	}
}

func (s *Store) save(snap *Snapshot) error {
	data, _ := json.MarshalIndent(snap, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, snap.ID+".json"), data, 0644)
}

// Create creates a new snapshot of a project directory.
func (s *Store) Create(name string, snapType Type, projectDir string, description string) (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := &Snapshot{
		ID:          fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		Name:        name,
		Type:        snapType,
		Description: description,
		CreatedAt:   time.Now(),
		ProjectDir:  projectDir,
		Tags:        []string{},
		Metadata:    make(map[string]string),
	}

	// Scan files
	files, err := s.scanDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	snap.Files = files
	snap.FileCount = len(files)
	for _, f := range files {
		snap.TotalSize += f.Size
	}

	// Get git info
	snap.GitBranch = s.getGitBranch(projectDir)
	snap.GitCommit = s.getGitCommit(projectDir)
	snap.GitDirty = s.getGitDirty(projectDir)

	s.snapshots[snap.ID] = snap
	return snap, s.save(snap)
}

func (s *Store) scanDir(dir string) ([]FileEntry, error) {
	var files []FileEntry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip hidden and common ignore dirs
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			ignored := map[string]bool{
				"node_modules": true, "vendor": true, "__pycache__": true,
				".git": true, "dist": true, "build": true, "target": true,
			}
			if ignored[name] {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			relPath = path
		}

		checksum, _ := s.fileChecksum(path)
		files = append(files, FileEntry{
			Path:     relPath,
			Checksum: checksum,
			Size:     info.Size(),
			Modified: info.ModTime().Format(time.RFC3339),
		})
		return nil
	})
	return files, err
}

func (s *Store) fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:16]), nil
}

func (s *Store) getGitBranch(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref: refs/heads/") {
		return strings.TrimPrefix(line, "ref: refs/heads/")
	}
	return ""
}

func (s *Store) getGitCommit(dir string) string {
	branch := s.getGitBranch(dir)
	if branch == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, ".git", "refs", "heads", branch))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))[:12]
}

func (s *Store) getGitDirty(dir string) bool {
	// Simple check — if we can't determine, assume clean
	return false
}

// Get retrieves a snapshot.
func (s *Store) Get(id string) (*Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[id]
	return snap, ok
}

// List lists snapshots, optionally filtered by type.
func (s *Store) List(snapType Type) []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Snapshot
	for _, snap := range s.snapshots {
		if snapType != "" && snap.Type != snapType {
			continue
		}
		result = append(result, *snap)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Delete removes a snapshot.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.snapshots[id]; !ok {
		return fmt.Errorf("snapshot %s not found", id)
	}
	delete(s.snapshots, id)
	os.Remove(filepath.Join(s.dir, id+".json"))
	return nil
}

// Compare compares two snapshots and returns the diff.
func (s *Store) Compare(id1, id2 string) (*Diff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s1, ok1 := s.snapshots[id1]
	s2, ok2 := s.snapshots[id2]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("one or both snapshots not found")
	}

	// Build file maps
	files1 := make(map[string]FileEntry)
	for _, f := range s1.Files {
		files1[f.Path] = f
	}
	files2 := make(map[string]FileEntry)
	for _, f := range s2.Files {
		files2[f.Path] = f
	}

	diff := &Diff{}

	// Added and modified
	for path, f2 := range files2 {
		f1, exists := files1[path]
		if !exists {
			diff.Added = append(diff.Added, path)
		} else if f1.Checksum != f2.Checksum {
			diff.Modified = append(diff.Modified, path)
		} else {
			diff.Unchanged++
		}
	}

	// Removed
	for path := range files1 {
		if _, exists := files2[path]; !exists {
			diff.Removed = append(diff.Removed, path)
		}
	}

	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Modified)

	return diff, nil
}

// Stats returns snapshot store statistics.
type Stats struct {
	TotalSnapshots int            `json:"total_snapshots"`
	ByType         map[Type]int   `json:"by_type"`
	TotalFiles     int            `json:"total_files"`
	TotalSize      int64          `json:"total_size"`
	Oldest         time.Time      `json:"oldest,omitempty"`
	Newest         time.Time      `json:"newest,omitempty"`
}

// Stats returns store statistics.
func (s *Store) Stats() *Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		TotalSnapshots: len(s.snapshots),
		ByType:         make(map[Type]int),
	}

	for _, snap := range s.snapshots {
		stats.ByType[snap.Type]++
		stats.TotalFiles += snap.FileCount
		stats.TotalSize += snap.TotalSize

		if stats.Oldest.IsZero() || snap.CreatedAt.Before(stats.Oldest) {
			stats.Oldest = snap.CreatedAt
		}
		if snap.CreatedAt.After(stats.Newest) {
			stats.Newest = snap.CreatedAt
		}
	}

	return stats
}
